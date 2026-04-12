import fs from 'fs';
import path from 'path';
import readline from 'readline';
import {
  buildContent,
  buildClaudeSkill,
  buildClaudeMd,
  buildCursorSkillFormat,
  buildCodexAgentsMd,
  buildSkillMarkdown,
  getClaudeMdPath,
  getCodexAgentsMdPath,
  getDestination,
  getSkillDestination,
  getSkillClaudeSettings,
  listRuleSuffixes,
  listTargets
} from './build';
import { patchCodexAgentsMd, patchRuleContent } from './patch';
import {
  listAgents,
  listSkills,
  resolveAgents,
  resolveSkills,
  isValidAgent,
  isValidSkill,
  type Language
} from './agents';
import {
  findProjectRoot,
  loadConfig,
  saveConfig,
  isCiMode,
  parseTargets,
  type Target
} from './config';
import { BALLAST_VERSION } from './version';

function withImplicitAgents(agents: string[]): string[] {
  const next = [...agents];
  if (next.includes('linting') && !next.includes('git-hooks')) {
    next.push('git-hooks');
  }
  return next;
}

function prompt(question: string): Promise<string> {
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout
  });
  return new Promise((resolve) => {
    rl.question(question, (answer) => {
      rl.close();
      resolve((answer || '').trim());
    });
  });
}

function resolveTsHookMode(
  projectRoot: string,
  language: Language
): 'standalone' | 'monorepo' {
  if (language !== 'typescript') {
    return 'standalone';
  }

  const configPath = path.join(projectRoot, '.rulesrc.json');
  if (!fs.existsSync(configPath)) {
    return 'standalone';
  }

  try {
    const raw = JSON.parse(fs.readFileSync(configPath, 'utf8')) as {
      languages?: unknown;
      paths?: Record<string, unknown>;
    };
    const languages = Array.isArray(raw.languages)
      ? raw.languages.filter(
          (language): language is string => typeof language === 'string'
        )
      : [];
    if (new Set(languages).size > 1) {
      return 'monorepo';
    }
    const pathKeys = raw.paths ? Object.keys(raw.paths) : [];
    if (pathKeys.length > 1) {
      return 'monorepo';
    }
  } catch {
    // Fall through to workspace-based detection.
  }

  if (hasWorkspaceMonorepo(projectRoot)) {
    return 'monorepo';
  }

  return 'standalone';
}

function hasWorkspaceMonorepo(projectRoot: string): boolean {
  const root = path.resolve(projectRoot);
  if (!hasWorkspaceManifest(root)) {
    return false;
  }
  return countPackageJsonFiles(root) > 1;
}

function hasWorkspaceManifest(root: string): boolean {
  const rootPackageJson = path.join(root, 'package.json');
  const pnpmWorkspaceYaml = path.join(root, 'pnpm-workspace.yaml');
  if (fs.existsSync(pnpmWorkspaceYaml)) {
    return true;
  }
  if (!fs.existsSync(rootPackageJson)) {
    return false;
  }
  try {
    const raw = JSON.parse(fs.readFileSync(rootPackageJson, 'utf8')) as {
      workspaces?: unknown;
    };
    return Array.isArray(raw.workspaces) || !!raw.workspaces;
  } catch {
    return false;
  }
}

function countPackageJsonFiles(root: string): number {
  const ignoredDirs = new Set([
    '.git',
    'node_modules',
    'dist',
    'build',
    'coverage',
    '.next',
    '.turbo',
    '.pnpm-store'
  ]);
  let count = 0;

  function walk(dir: string, depth: number): void {
    if (depth > 4) return;
    let entries: fs.Dirent[];
    try {
      entries = fs.readdirSync(dir, { withFileTypes: true });
    } catch {
      return;
    }
    for (const entry of entries) {
      if (entry.isDirectory()) {
        if (ignoredDirs.has(entry.name)) continue;
        walk(path.join(dir, entry.name), depth + 1);
        continue;
      }
      if (entry.isFile() && entry.name === 'package.json') {
        count += 1;
      }
    }
  }

  walk(root, 0);
  return count;
}

async function promptTargets(): Promise<Target[]> {
  const targets = listTargets();
  const line = await prompt(
    `AI platform(s) (${targets.join(', ')}, comma-separated): `
  );
  const parsed = parseTargets(line);
  if (parsed.targets.length > 0 && parsed.invalidTargets.length === 0) {
    return parsed.targets;
  }
  console.error(`Invalid target. Choose one of: ${targets.join(', ')}`);
  return promptTargets();
}

/**
 * Interactive: ask for agents (comma-separated or "all")
 */
async function promptAgents(
  language: Language = 'typescript'
): Promise<string[]> {
  const agents = listAgents(language);
  const line = await prompt(
    `Agents (comma-separated or "all") [${agents.join(', ')}]: `
  );
  if (!line) return agents;
  const list =
    line.toLowerCase() === 'all'
      ? ['all']
      : line.split(',').map((s) => s.trim());
  const resolved = resolveAgents(list, language);
  if (resolved.length === 0) {
    console.error(
      `Invalid agents. Use "all" or comma-separated: ${agents.join(', ')}`
    );
    return promptAgents(language);
  }
  return resolved;
}

async function promptSkills(
  language: Language = 'typescript'
): Promise<string[]> {
  const skills = listSkills(language);
  if (skills.length === 0) {
    return [];
  }
  const line = await prompt(
    `Skills (comma-separated, "all", or blank for none) [${skills.join(', ')}]: `
  );
  if (!line) return [];
  const list =
    line.toLowerCase() === 'all'
      ? ['all']
      : line.split(',').map((s) => s.trim());
  const resolved = resolveSkills(list, language);
  if (resolved.length === 0) {
    console.error(
      `Invalid skills. Use "all" or comma-separated: ${skills.join(', ')}`
    );
    return promptSkills(language);
  }
  return resolved;
}

export interface ResolveTargetAndAgentsOptions {
  projectRoot?: string;
  target?: string;
  targets?: string[];
  agents?: string | string[];
  skills?: string | string[];
  yes?: boolean;
  language?: Language;
}

/**
 * Resolve target and agents from config + flags; in CI with no config, return null and caller should exit with error
 */
export async function resolveTargetAndAgents(
  options: ResolveTargetAndAgentsOptions = {}
): Promise<{ targets: Target[]; agents: string[]; skills: string[] } | null> {
  const projectRoot = options.projectRoot ?? findProjectRoot();
  const language = options.language ?? 'typescript';
  const config = loadConfig(projectRoot, language);
  const ci = isCiMode() || options.yes;

  const parsedTargetsFromFlag = parseTargets([
    ...(options.targets ?? []),
    options.target
  ]);
  if (parsedTargetsFromFlag.invalidTargets.length > 0) {
    throw new Error(`Invalid --target. Use: ${listTargets().join(', ')}`);
  }
  const targetsFromFlag = parsedTargetsFromFlag.targets;
  const agentsFromFlag = options.agents;

  if (
    config &&
    targetsFromFlag.length === 0 &&
    agentsFromFlag === undefined &&
    options.skills === undefined
  ) {
    return {
      targets: config.targets,
      agents: withImplicitAgents(config.agents),
      skills: config.skills ?? []
    };
  }

  const targets =
    targetsFromFlag.length > 0 ? targetsFromFlag : (config?.targets ?? []);
  let agents = config?.agents;
  if (agentsFromFlag != null) {
    const resolvedAgents = resolveAgents(agentsFromFlag, language);
    if (resolvedAgents.length === 0) {
      throw new Error(
        `Invalid --agent. Use: ${listAgents(language).join(', ')}`
      );
    }
    agents = [
      ...new Set(
        withImplicitAgents([...(config?.agents ?? []), ...resolvedAgents])
      )
    ];
  } else if (config?.agents) {
    agents = withImplicitAgents(config.agents);
  }
  const resolvedNewSkills =
    options.skills != null ? resolveSkills(options.skills, language) : null;
  if (resolvedNewSkills !== null && resolvedNewSkills.length === 0) {
    throw new Error(`Invalid --skill. Use: ${listSkills(language).join(', ')}`);
  }
  const skills =
    resolvedNewSkills !== null
      ? [...new Set([...(config?.skills ?? []), ...resolvedNewSkills])]
      : (config?.skills ?? []);

  if (
    targets.length > 0 &&
    ((agents && agents.length > 0) || skills.length > 0)
  ) {
    return { targets, agents: agents ?? [], skills };
  }

  if (ci) {
    return null;
  }

  const resolvedTargets = targets.length > 0 ? targets : await promptTargets();
  const resolvedAgents = withImplicitAgents(
    agents?.length ? agents : await promptAgents(language)
  );
  const resolvedSkills = skills.length ? skills : await promptSkills(language);
  return {
    targets: resolvedTargets,
    agents: resolvedAgents,
    skills: resolvedSkills
  };
}

export interface InstallOptions {
  projectRoot: string;
  target: Target;
  agents: string[];
  skills?: string[];
  language?: Language;
  force?: boolean;
  patch?: boolean;
  patchClaudeMd?: boolean;
  saveConfig?: boolean;
}

export interface InstallResult {
  installed: string[];
  /** Per-rule: which (agent, ruleSuffix) files were written (ruleSuffix '' = main) */
  installedRules: Array<{ agentId: string; ruleSuffix: string }>;
  installedSkills: string[];
  installedSupportFiles: string[];
  skipped: string[];
  skippedSkills: string[];
  skippedSupportFiles: string[];
  errors: Array<{ agent: string; error: string }>;
}

/**
 * Deep-merge skill claude-settings.json into .claude/settings.json.
 * Only merges known safe keys (permissions.allow). Creates the file if absent.
 * Skips entries already present so installs are idempotent.
 */
function mergeSkillClaudeSettings(
  projectRoot: string,
  skillSettings: Record<string, unknown>
): void {
  const settingsPath = path.join(projectRoot, '.claude', 'settings.json');
  let existing: Record<string, unknown> = {};
  if (fs.existsSync(settingsPath)) {
    // Throw on corrupt file rather than silently overwriting user settings.
    existing = JSON.parse(fs.readFileSync(settingsPath, 'utf8')) as Record<
      string,
      unknown
    >;
  }

  const incoming = skillSettings as {
    permissions?: { allow?: unknown };
  };
  const rawAllow = incoming.permissions?.allow;
  // Validate that allow is an array of strings — reject anything else to
  // avoid injecting non-string values into the permissions file.
  if (!Array.isArray(rawAllow)) return;
  const incomingAllow = rawAllow.filter(
    (r): r is string => typeof r === 'string'
  );
  if (incomingAllow.length === 0) return;

  const existingPerms =
    (existing.permissions as { allow?: string[] } | undefined) ?? {};
  const existingAllow: string[] = existingPerms.allow ?? [];
  const merged = [
    ...existingAllow,
    ...incomingAllow.filter((rule) => !existingAllow.includes(rule))
  ];
  existing.permissions = { ...existingPerms, allow: merged };

  const dir = path.dirname(settingsPath);
  if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(
    settingsPath,
    JSON.stringify(existing, null, 2) + '\n',
    'utf8'
  );
}

function ensureGitignoreEntry(projectRoot: string, entry: string): void {
  const gitignorePath = path.join(projectRoot, '.gitignore');
  const normalizedEntry = entry.trim();
  if (!normalizedEntry) {
    return;
  }

  if (!fs.existsSync(gitignorePath)) {
    fs.writeFileSync(gitignorePath, `${normalizedEntry}\n`, 'utf8');
    return;
  }

  const content = fs.readFileSync(gitignorePath, 'utf8');
  const lines = content.split(/\r?\n/);
  if (lines.some((line) => line.trim() === normalizedEntry)) {
    return;
  }
  const separator = content.length === 0 || content.endsWith('\n') ? '' : '\n';
  fs.writeFileSync(
    gitignorePath,
    `${content}${separator}${normalizedEntry}\n`,
    'utf8'
  );
}

/**
 * Install agents for the given target into projectRoot. Single policy: do not overwrite unless force.
 */
export function install(options: InstallOptions): InstallResult {
  const {
    projectRoot,
    target,
    agents,
    skills = [],
    language = 'typescript',
    force = false,
    patch = false,
    patchClaudeMd = false,
    saveConfig: persist
  } = options;
  const effectiveAgents = withImplicitAgents(agents);
  const installed: string[] = [];
  const installedRules: Array<{ agentId: string; ruleSuffix: string }> = [];
  const installedSkills: string[] = [];
  const installedSupportFiles: string[] = [];
  const skipped: string[] = [];
  const skippedSkills: string[] = [];
  const skippedSupportFiles: string[] = [];
  const errors: Array<{ agent: string; error: string }> = [];
  const processedAgentIds = new Set<string>();
  const processedSkillIds = new Set<string>();
  const disableSupportFiles = process.env.BALLAST_DISABLE_SUPPORT_FILES === '1';

  try {
    ensureGitignoreEntry(projectRoot, '.ballast/');
  } catch (err) {
    errors.push({
      agent: 'gitignore',
      error: err instanceof Error ? err.message : String(err)
    });
  }

  if (persist) {
    saveConfig(
      {
        targets: [target],
        agents: effectiveAgents,
        skills,
        ballastVersion: BALLAST_VERSION,
        languages: [language]
      },
      projectRoot
    );
  }
  const hookMode = resolveTsHookMode(projectRoot, language);

  for (const agentId of effectiveAgents) {
    if (!isValidAgent(agentId, language)) {
      errors.push({ agent: agentId, error: 'Unknown agent' });
      continue;
    }
    let agentInstalled = false;
    let agentSkipped = false;
    let agentProcessed = false;
    try {
      const suffixes = listRuleSuffixes(agentId, language);
      for (const ruleSuffix of suffixes) {
        const { dir, file } = getDestination(
          agentId,
          target,
          projectRoot,
          ruleSuffix || undefined,
          language
        );
        const fileExists = fs.existsSync(file);
        if (!fs.existsSync(dir)) {
          fs.mkdirSync(dir, { recursive: true });
        }
        const content = buildContent(
          agentId,
          target,
          ruleSuffix || undefined,
          language,
          { hookMode }
        );
        if (fileExists && !force && !patch) {
          agentSkipped = true;
          agentProcessed = true;
          continue;
        }
        const nextContent =
          fileExists && !force && patch
            ? patchRuleContent(fs.readFileSync(file, 'utf8'), content, target)
            : content;
        fs.writeFileSync(file, nextContent, 'utf8');
        installedRules.push({ agentId, ruleSuffix: ruleSuffix || '' });
        agentInstalled = true;
        agentProcessed = true;
      }
      if (agentProcessed) processedAgentIds.add(agentId);
      if (agentInstalled) installed.push(agentId);
      if (agentSkipped && !agentInstalled) skipped.push(agentId);
    } catch (err) {
      errors.push({
        agent: agentId,
        error: err instanceof Error ? err.message : String(err)
      });
    }
  }

  for (const skillId of skills) {
    if (!isValidSkill(skillId, language)) {
      errors.push({ agent: skillId, error: 'Unknown skill' });
      continue;
    }
    try {
      const { dir, file } = getSkillDestination(skillId, target, projectRoot);
      const fileExists = fs.existsSync(file);
      if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
      }
      if (fileExists && !force) {
        skippedSkills.push(skillId);
        processedSkillIds.add(skillId);
        continue;
      }
      switch (target) {
        case 'cursor':
          fs.writeFileSync(file, buildCursorSkillFormat(skillId), 'utf8');
          break;
        case 'claude': {
          fs.writeFileSync(file, buildClaudeSkill(skillId));
          const skillSettings = getSkillClaudeSettings(skillId);
          if (skillSettings) {
            try {
              mergeSkillClaudeSettings(projectRoot, skillSettings);
            } catch (mergeErr) {
              errors.push({
                agent: skillId,
                error: `Skill installed but failed to merge claude-settings.json: ${mergeErr instanceof Error ? mergeErr.message : String(mergeErr)}`
              });
            }
          }
          break;
        }
        case 'opencode':
        case 'codex':
          fs.writeFileSync(file, buildSkillMarkdown(skillId), 'utf8');
          break;
        default:
          throw new Error(`Unknown target: ${target}`);
      }
      installedSkills.push(skillId);
      processedSkillIds.add(skillId);
    } catch (err) {
      errors.push({
        agent: skillId,
        error: err instanceof Error ? err.message : String(err)
      });
    }
  }

  if (!disableSupportFiles && target === 'claude') {
    const claudeMdPath = getClaudeMdPath(projectRoot);
    const shouldPatchClaudeMd = patch || patchClaudeMd;
    if (fs.existsSync(claudeMdPath) && !force && !shouldPatchClaudeMd) {
      skippedSupportFiles.push(claudeMdPath);
    } else {
      try {
        const content = buildClaudeMd(
          Array.from(processedAgentIds),
          Array.from(processedSkillIds),
          language
        );
        const nextContent =
          fs.existsSync(claudeMdPath) && !force && shouldPatchClaudeMd
            ? patchCodexAgentsMd(fs.readFileSync(claudeMdPath, 'utf8'), content)
            : content;
        fs.writeFileSync(claudeMdPath, nextContent, 'utf8');
        installedSupportFiles.push(claudeMdPath);
      } catch (err) {
        errors.push({
          agent: 'claude',
          error: err instanceof Error ? err.message : String(err)
        });
      }
    }
  }

  if (!disableSupportFiles && target === 'codex') {
    const agentsMdPath = getCodexAgentsMdPath(projectRoot);
    if (fs.existsSync(agentsMdPath) && !force && !patch) {
      skippedSupportFiles.push(agentsMdPath);
    } else {
      try {
        const content = buildCodexAgentsMd(
          Array.from(processedAgentIds),
          Array.from(processedSkillIds),
          language
        );
        const nextContent =
          fs.existsSync(agentsMdPath) && !force && patch
            ? patchCodexAgentsMd(fs.readFileSync(agentsMdPath, 'utf8'), content)
            : content;
        fs.writeFileSync(agentsMdPath, nextContent, 'utf8');
        installedSupportFiles.push(agentsMdPath);
      } catch (err) {
        errors.push({
          agent: 'codex',
          error: err instanceof Error ? err.message : String(err)
        });
      }
    }
  }

  return {
    installed,
    installedRules,
    installedSkills,
    installedSupportFiles,
    skipped,
    skippedSkills,
    skippedSupportFiles,
    errors
  };
}

export interface RunInstallOptions {
  projectRoot?: string;
  target?: string;
  targets?: string[];
  agents?: string[];
  all?: boolean;
  skills?: string[];
  allSkills?: boolean;
  language?: Language;
  force?: boolean;
  patch?: boolean;
  yes?: boolean;
}

async function promptYesNo(
  question: string,
  defaultAnswer: boolean = false
): Promise<boolean> {
  const suffix = defaultAnswer ? ' [Y/n]: ' : ' [y/N]: ';
  const line = (await prompt(question + suffix)).toLowerCase();
  if (!line) return defaultAnswer;
  return line === 'y' || line === 'yes';
}

/**
 * Run install flow: resolve target/agents (interactive or from config/flags), then install.
 * In CI mode with no .rulesrc.json, --target and (--agent X or --all) are required.
 */
export async function runInstall(
  options: RunInstallOptions = {}
): Promise<number> {
  const projectRoot = options.projectRoot ?? findProjectRoot();
  const language = options.language ?? 'typescript';
  const priorConfig = loadConfig(projectRoot, language);
  const agentsFromFlag = options.all
    ? 'all'
    : Array.isArray(options.agents) && options.agents.length === 0
      ? undefined
      : options.agents;
  const skillsFromFlag = options.allSkills
    ? 'all'
    : Array.isArray(options.skills) && options.skills.length === 0
      ? undefined
      : options.skills;
  const resolved = await resolveTargetAndAgents({
    projectRoot,
    target: options.target,
    targets: options.targets,
    agents: agentsFromFlag,
    skills: skillsFromFlag,
    yes: options.yes,
    language
  });

  if (!resolved) {
    console.error(
      'In CI/non-interactive mode (--yes or CI env), --target and at least one of --agent/--all or --skill/--all-skills are required when .rulesrc.json is missing.'
    );
    console.error(
      'Example: ballast-typescript install --yes --target cursor --agent linting --skill owasp-security-scan'
    );
    return 1;
  }

  const { targets, agents, skills } = resolved;
  const explicitAgentSelection =
    Boolean(options.all) || options.agents !== undefined;
  const agentsToPersist =
    explicitAgentSelection || agents.length > 0
      ? agents
      : withImplicitAgents(priorConfig?.agents ?? []);
  saveConfig(
    {
      targets,
      agents: agentsToPersist,
      skills,
      ballastVersion: BALLAST_VERSION,
      languages: [language]
    },
    projectRoot
  );

  for (const target of targets) {
    const claudeMdPath = getClaudeMdPath(projectRoot);
    let patchClaudeMd = false;
    if (target === 'claude' && fs.existsSync(claudeMdPath) && !options.force) {
      if (options.patch) {
        patchClaudeMd = true;
      } else if (!isCiMode() && !options.yes) {
        patchClaudeMd = await promptYesNo(
          `Existing CLAUDE.md found at ${claudeMdPath}. Patch the Installed agent rules section?`
        );
      }
    }
    const result = install({
      projectRoot,
      target,
      agents,
      skills,
      language,
      force: options.force ?? false,
      patch: options.patch ?? false,
      patchClaudeMd,
      saveConfig: false
    });

    if (result.errors.length > 0) {
      result.errors.forEach(({ agent, error }) => {
        console.error(`Error installing ${agent}: ${error}`);
      });
      return 1;
    }

    if (result.installedRules.length > 0) {
      console.log(`Installed for ${target}: ${result.installed.join(', ')}`);
      result.installedRules.forEach(({ agentId, ruleSuffix }) => {
        const { file } = getDestination(
          agentId,
          target,
          projectRoot,
          ruleSuffix || undefined,
          language
        );
        const label = ruleSuffix ? `${agentId}-${ruleSuffix}` : agentId;
        console.log(`  ${label} -> ${file}`);
      });
    }
    if (result.installedSkills.length > 0) {
      console.log(
        `Installed skills for ${target}: ${result.installedSkills.join(', ')}`
      );
      result.installedSkills.forEach((skillId) => {
        const { file } = getSkillDestination(skillId, target, projectRoot);
        console.log(`  ${skillId} -> ${file}`);
      });
    }
    if (result.installedSupportFiles.length > 0) {
      result.installedSupportFiles.forEach((file) => {
        const label = file.endsWith('CLAUDE.md') ? 'CLAUDE.md' : 'AGENTS.md';
        console.log(`  ${label} -> ${file}`);
      });
    }
    if (result.skipped.length > 0) {
      console.log(
        `Skipped (already present; use --force to overwrite): ${result.skipped.join(', ')}`
      );
    }
    if (result.skippedSupportFiles.length > 0) {
      console.log(
        `Skipped support files (already present; use --force to overwrite): ${result.skippedSupportFiles.join(
          ', '
        )}`
      );
    }
    if (result.skippedSkills.length > 0) {
      console.log(
        `Skipped skills (already present; use --force to overwrite): ${result.skippedSkills.join(', ')}`
      );
    }
    if (
      result.installed.length === 0 &&
      result.installedSkills.length === 0 &&
      result.skipped.length === 0 &&
      result.skippedSkills.length === 0 &&
      result.errors.length === 0
    ) {
      console.log('Nothing to install.');
    }
  }

  return 0;
}

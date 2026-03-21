import fs from 'fs';
import readline from 'readline';
import {
  buildContent,
  buildClaudeMd,
  buildCodexAgentsMd,
  getClaudeMdPath,
  getCodexAgentsMdPath,
  getDestination,
  listRuleSuffixes,
  listTargets
} from './build';
import { patchCodexAgentsMd, patchRuleContent } from './patch';
import {
  listAgents,
  resolveAgents,
  isValidAgent,
  type Language
} from './agents';
import { findProjectRoot, loadConfig, saveConfig, isCiMode } from './config';
import type { Target } from './config';

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

/**
 * Interactive: ask for target (cursor | claude | opencode | codex)
 */
async function promptTarget(): Promise<Target> {
  const targets = listTargets();
  const line = await prompt(`AI platform (${targets.join(', ')}): `);
  const t = line.toLowerCase();
  if (targets.includes(t)) return t as Target;
  console.error(`Invalid target. Choose one of: ${targets.join(', ')}`);
  return promptTarget();
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

export interface ResolveTargetAndAgentsOptions {
  projectRoot?: string;
  target?: string;
  agents?: string | string[];
  yes?: boolean;
  language?: Language;
}

/**
 * Resolve target and agents from config + flags; in CI with no config, return null and caller should exit with error
 */
export async function resolveTargetAndAgents(
  options: ResolveTargetAndAgentsOptions = {}
): Promise<{ target: Target; agents: string[] } | null> {
  const projectRoot = options.projectRoot ?? findProjectRoot();
  const language = options.language ?? 'typescript';
  const config = loadConfig(projectRoot, language);
  const ci = isCiMode() || options.yes;

  const targetFromFlag = options.target;
  const agentsFromFlag = options.agents;

  if (config && !targetFromFlag && agentsFromFlag === undefined) {
    return { target: config.target, agents: config.agents };
  }

  const target = targetFromFlag ?? config?.target;
  const agents =
    agentsFromFlag != null
      ? resolveAgents(agentsFromFlag, language)
      : config?.agents;

  if (target && agents && agents.length > 0) {
    return { target: target as Target, agents };
  }

  if (ci) {
    return null;
  }

  const resolvedTarget = (target ?? (await promptTarget())) as Target;
  const resolvedAgents = agents?.length ? agents : await promptAgents(language);
  return { target: resolvedTarget, agents: resolvedAgents };
}

export interface InstallOptions {
  projectRoot: string;
  target: Target;
  agents: string[];
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
  installedSupportFiles: string[];
  skipped: string[];
  skippedSupportFiles: string[];
  errors: Array<{ agent: string; error: string }>;
}

/**
 * Install agents for the given target into projectRoot. Single policy: do not overwrite unless force.
 */
export function install(options: InstallOptions): InstallResult {
  const {
    projectRoot,
    target,
    agents,
    language = 'typescript',
    force = false,
    patch = false,
    patchClaudeMd = false,
    saveConfig: persist
  } = options;
  const installed: string[] = [];
  const installedRules: Array<{ agentId: string; ruleSuffix: string }> = [];
  const installedSupportFiles: string[] = [];
  const skipped: string[] = [];
  const skippedSupportFiles: string[] = [];
  const errors: Array<{ agent: string; error: string }> = [];
  const processedAgentIds = new Set<string>();
  const disableSupportFiles = process.env.BALLAST_DISABLE_SUPPORT_FILES === '1';

  if (persist) {
    saveConfig({ target, agents, languages: [language] }, projectRoot);
  }

  for (const agentId of agents) {
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
          language
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

  if (!disableSupportFiles && target === 'claude') {
    const claudeMdPath = getClaudeMdPath(projectRoot);
    const shouldPatchClaudeMd = patch || patchClaudeMd;
    if (fs.existsSync(claudeMdPath) && !force && !shouldPatchClaudeMd) {
      skippedSupportFiles.push(claudeMdPath);
    } else {
      try {
        const content = buildClaudeMd(Array.from(processedAgentIds), language);
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
    installedSupportFiles,
    skipped,
    skippedSupportFiles,
    errors
  };
}

export interface RunInstallOptions {
  projectRoot?: string;
  target?: string;
  agents?: string[];
  all?: boolean;
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
  const agentsFromFlag = options.all ? 'all' : options.agents;
  const resolved = await resolveTargetAndAgents({
    projectRoot,
    target: options.target,
    agents: agentsFromFlag,
    yes: options.yes,
    language
  });

  if (!resolved) {
    console.error(
      'In CI/non-interactive mode (--yes or CI env), --target and --agent (or --all) are required when .rulesrc.json is missing.'
    );
    console.error(
      'Example: ballast-typescript install --yes --target cursor --agent linting'
    );
    return 1;
  }

  const { target, agents } = resolved;
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
    language,
    force: options.force ?? false,
    patch: options.patch ?? false,
    patchClaudeMd,
    saveConfig: true
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
  if (
    result.installed.length === 0 &&
    result.skipped.length === 0 &&
    result.errors.length === 0
  ) {
    console.log('Nothing to install.');
  }

  return 0;
}

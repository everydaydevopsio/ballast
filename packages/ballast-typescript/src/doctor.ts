import { spawnSync } from 'child_process';
import fs from 'fs';
import path from 'path';
import { findProjectRoot, getRulesrcFilename, loadConfig } from './config';
import type { PublishingProfile, Target } from './config';
import type { Language } from './agents';
import {
  buildContent,
  getRuleMarkerId,
  listRuleSuffixes,
  parseRuleMarker,
  stripRuleMarker,
  verifyRuleChecksum
} from './build';
import { BALLAST_VERSION } from './version';

export interface InstalledCliStatus {
  name: string;
  version: string | null;
  path: string | null;
}

export type AppType = 'cli' | 'web' | 'api' | 'library' | 'sdk' | 'unknown';

export interface DoctorReport {
  currentCli: string;
  currentVersion: string;
  configPath: string | null;
  configVersion: string | null;
  configTargets: string[];
  configAgents: string[];
  configSkills: string[];
  configLanguages: string[];
  configPaths: Record<string, string[]>;
  configTools: Record<string, string[]>;
  configDiscoveryExcludePaths: string[];
  configTaskSystem: string | null;
  configDeploymentModel: string | null;
  configPublishingProfiles: string[];
  installed: InstalledCliStatus[];
  detectedAppType: AppType;
  ruleFiles: RuleFileStatus[];
  recommendations: string[];
}

export type RuleFileState = 'ok' | 'drifted' | 'stale' | 'unowned';

export interface RuleFileStatus {
  path: string;
  target: Target;
  ruleId: string | null;
  status: RuleFileState;
}

const CLI_NAMES = [
  'ballast-typescript',
  'ballast-python',
  'ballast-go'
] as const;

function compareVersions(left: string, right: string): number {
  if (left === right) return 0;
  const leftParts = left.split('.').map((part) => Number.parseInt(part, 10));
  const rightParts = right.split('.').map((part) => Number.parseInt(part, 10));
  const leftNumeric = !leftParts.some(Number.isNaN);
  const rightNumeric = !rightParts.some(Number.isNaN);
  if (leftNumeric && !rightNumeric) return 1;
  if (!leftNumeric && rightNumeric) return -1;
  if (!leftNumeric || !rightNumeric) {
    return left.localeCompare(right);
  }
  const length = Math.max(leftParts.length, rightParts.length);
  for (let index = 0; index < length; index += 1) {
    const delta = (leftParts[index] ?? 0) - (rightParts[index] ?? 0);
    if (delta !== 0) return delta;
  }
  return 0;
}

function latestVersion(values: Array<string | null | undefined>): string {
  return (
    values
      .filter(
        (value): value is string => typeof value === 'string' && value !== ''
      )
      .sort(compareVersions)
      .at(-1) ?? BALLAST_VERSION
  );
}

function detectInstalledCli(name: string): InstalledCliStatus {
  const pathCheck = spawnSync('bash', ['-lc', `command -v ${name}`], {
    encoding: 'utf8'
  });
  if (pathCheck.status !== 0) {
    return { name, version: null, path: null };
  }
  const cliPath = pathCheck.stdout.trim();
  const versionCheck = spawnSync(name, ['--version'], { encoding: 'utf8' });
  if (versionCheck.status !== 0) {
    return { name, version: null, path: cliPath };
  }
  return {
    name,
    version: versionCheck.stdout.trim() || null,
    path: cliPath
  };
}

function fileExists(dir: string, ...names: string[]): boolean {
  return names.some((name) => fs.existsSync(path.join(dir, name)));
}

function hasPackageJsonBin(dir: string): boolean {
  const pkgPath = path.join(dir, 'package.json');
  if (!fs.existsSync(pkgPath)) return false;
  try {
    const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8')) as {
      bin?: unknown;
    };
    return (
      pkg.bin !== null &&
      pkg.bin !== undefined &&
      typeof pkg.bin !== 'undefined' &&
      !(
        typeof pkg.bin === 'object' &&
        Object.keys(pkg.bin as object).length === 0
      )
    );
  } catch {
    return false;
  }
}

function hasFrontendIndicators(dir: string): boolean {
  return fileExists(
    dir,
    'next.config.js',
    'next.config.ts',
    'next.config.mjs',
    'vite.config.js',
    'vite.config.ts',
    'vite.config.mjs',
    'nuxt.config.js',
    'nuxt.config.ts',
    'svelte.config.js',
    'remix.config.js'
  );
}

function hasApiIndicators(dir: string): boolean {
  return (
    fileExists(dir, 'go.mod', 'pyproject.toml') ||
    fileExists(dir, 'routes', 'handlers', 'api', 'server') ||
    fileExists(dir, 'app.ts', 'server.ts', 'main.go', 'app.py', 'main.py')
  );
}

/**
 * Heuristically detect the app type from filesystem markers in the project root.
 * Returns the most specific match; falls back to 'unknown'.
 */
export function detectAppType(projectRoot: string): AppType {
  const hasDockerfile = fileExists(projectRoot, 'Dockerfile');
  const hasGoReleaser = fileExists(
    projectRoot,
    '.goreleaser.yaml',
    '.goreleaser.yml'
  );
  const hasPackageJson = fileExists(projectRoot, 'package.json');
  const hasBin = hasPackageJsonBin(projectRoot);

  // CLI indicators: GoReleaser config or package.json with bin entries
  if (hasGoReleaser || hasBin) {
    return 'cli';
  }

  // Container-deployed apps require a Dockerfile
  if (hasDockerfile) {
    if (hasFrontendIndicators(projectRoot)) {
      return 'web';
    }
    if (hasApiIndicators(projectRoot)) {
      return 'api';
    }
    // Dockerfile present but no clear frontend or API markers — default to web
    return 'web';
  }

  // Library / SDK indicators: package.json, go.mod, or pyproject.toml without Dockerfile or bin
  if (
    hasPackageJson ||
    fileExists(projectRoot, 'go.mod') ||
    fileExists(projectRoot, 'pyproject.toml')
  ) {
    return 'library';
  }

  return 'unknown';
}

const APP_TYPE_PUBLISH_RULE: Record<AppType, string | null> = {
  cli: 'publishing-cli',
  web: 'publishing-web',
  api: 'publishing-api',
  library: 'publishing-libraries',
  sdk: 'publishing-sdks',
  unknown: null
};

function refreshConfigCommand(
  report: Pick<
    DoctorReport,
    'currentCli' | 'configTargets' | 'configAgents' | 'configSkills'
  >
): string | null {
  void report;
  return 'ballast install --refresh-config';
}

function withImplicitAgents(agents: string[]): string[] {
  const next = [...agents];
  if (next.includes('linting') && !next.includes('git-hooks')) {
    next.push('git-hooks');
  }
  return next;
}

export function buildDoctorReport(
  currentCli: string,
  currentVersion: string,
  configPath: string | null,
  configVersion: string | null,
  configTargets: string[],
  configAgents: string[],
  configSkills: string[],
  configLanguages: string[],
  configPaths: Record<string, string[]>,
  configTools: Record<string, string[]>,
  configDiscoveryExcludePaths: string[],
  configTaskSystem: string | null,
  configDeploymentModel: string | null,
  configPublishingProfiles: string[],
  installed: InstalledCliStatus[],
  detectedAppType: AppType = 'unknown',
  ruleFiles: RuleFileStatus[] = []
): DoctorReport {
  const targetVersion = latestVersion([
    currentVersion,
    configVersion,
    ...installed.map((item) => item.version)
  ]);
  const recommendations: string[] = [];
  let needsCliFix = false;

  for (const item of installed) {
    if (!item.version) {
      needsCliFix = true;
      continue;
    }
    if (compareVersions(item.version, targetVersion) < 0) {
      needsCliFix = true;
    }
  }

  if (needsCliFix) {
    recommendations.push(
      'Run ballast doctor --fix to install or upgrade local Ballast CLIs.'
    );
  }

  if (
    configPath &&
    (!configVersion || compareVersions(configVersion, targetVersion) < 0)
  ) {
    const refresh = refreshConfigCommand({
      currentCli,
      configTargets,
      configAgents,
      configSkills
    });
    recommendations.push(
      refresh
        ? `Refresh ${path.basename(configPath)} to Ballast ${targetVersion}: ${refresh}`
        : `Refresh ${path.basename(configPath)} with a current Ballast install command.`
    );
  }

  const suggestedRule = APP_TYPE_PUBLISH_RULE[detectedAppType];
  if (suggestedRule && configPath && !configAgents.includes('publishing')) {
    recommendations.push(
      `Detected app type: ${detectedAppType} — add the publishing agent: ballast install --agents publishing`
    );
  }

  for (const ruleFile of ruleFiles) {
    if (ruleFile.status === 'drifted') {
      recommendations.push(
        `Refresh drifted rule file ${ruleFile.path}: ballast install --refresh-config`
      );
    }
    if (ruleFile.status === 'stale') {
      recommendations.push(
        `Remove stale managed rule file ${ruleFile.path}: ballast doctor --fix`
      );
    }
  }

  return {
    currentCli,
    currentVersion,
    configPath,
    configVersion,
    configTargets,
    configAgents,
    configSkills,
    configLanguages,
    configPaths,
    configTools,
    configDiscoveryExcludePaths,
    configTaskSystem,
    configDeploymentModel,
    configPublishingProfiles,
    installed,
    detectedAppType,
    ruleFiles,
    recommendations
  };
}

interface RuleConfig {
  targets: Target[];
  agents: string[];
  languages: string[];
  paths: Record<string, string[]>;
  taskSystem?: string | null;
  deploymentModel?: string | null;
  publishingProfiles?: PublishingProfile[];
}

const TARGET_RULE_DIRS: Record<Target, string[]> = {
  cursor: ['.cursor/rules'],
  claude: ['.claude/rules'],
  opencode: ['.opencode'],
  codex: ['.codex/rules'],
  gemini: ['.gemini/rules']
};

function isRuleFile(filePath: string): boolean {
  return ['.md', '.mdc'].includes(path.extname(filePath));
}

function walkRuleFiles(dir: string): string[] {
  if (!fs.existsSync(dir)) return [];
  const files: string[] = [];
  const walk = (current: string): void => {
    for (const entry of fs.readdirSync(current, { withFileTypes: true })) {
      const next = path.join(current, entry.name);
      if (entry.isDirectory()) {
        walk(next);
        continue;
      }
      if (entry.isFile() && isRuleFile(next)) {
        files.push(next);
      }
    }
  };
  walk(dir);
  return files;
}

function configuredLanguages(config: RuleConfig): Language[] {
  const values =
    config.languages.length > 0 ? config.languages : ['typescript'];
  return values.filter((value): value is Language =>
    ['typescript', 'python', 'go', 'ansible', 'terraform', 'dart'].includes(
      value
    )
  );
}

function configuredRuleKeys(config: RuleConfig): Set<string> {
  const active = new Set<string>();
  for (const target of config.targets) {
    for (const language of configuredLanguages(config)) {
      for (const agentId of withImplicitAgents(config.agents)) {
        try {
          for (const suffix of listRuleSuffixes(
            agentId,
            language,
            config.publishingProfiles
          )) {
            active.add(
              `${target}:${getRuleMarkerId(agentId, language, suffix || undefined)}`
            );
          }
        } catch {
          // Ignore invalid or unavailable agents in saved config; doctor reports
          // existing marked files as stale rather than failing the whole report.
        }
      }
    }
  }
  return active;
}

function parseRuleId(
  ruleId: string
): { language: Language; agentId: string; ruleSuffix?: string } | null {
  const [language, agentId, ...suffixParts] = ruleId.split('/');
  if (!language || !agentId) return null;
  if (
    !['typescript', 'python', 'go', 'ansible', 'terraform', 'dart'].includes(
      language
    )
  ) {
    return null;
  }
  return {
    language: language as Language,
    agentId,
    ruleSuffix: suffixParts.length > 0 ? suffixParts.join('/') : undefined
  };
}

function canonicalRuleContent(
  target: Target,
  ruleId: string,
  config: RuleConfig
): string | null {
  const parsed = parseRuleId(ruleId);
  if (!parsed) return null;
  const variables: Record<string, string> = {
    ...(parsed.agentId === 'tasks' && config.taskSystem
      ? { taskSystem: config.taskSystem }
      : {}),
    ...(parsed.agentId === 'publishing' && config.deploymentModel
      ? { deploymentModel: config.deploymentModel }
      : {})
  };
  const hookMode =
    parsed.agentId === 'git-hooks'
      ? ruleHookMode(parsed.language, config)
      : undefined;
  const options = {
    ...(hookMode ? { hookMode } : {}),
    ...(Object.keys(variables).length > 0 ? { variables } : {})
  };
  try {
    return buildContent(
      parsed.agentId,
      target,
      parsed.ruleSuffix,
      parsed.language,
      Object.keys(options).length > 0 ? options : undefined
    );
  } catch {
    return null;
  }
}

function ruleHookMode(
  language: Language,
  config: RuleConfig
): 'pre-commit' | 'husky' {
  if (language !== 'typescript') {
    return 'pre-commit';
  }
  if (new Set(config.languages).size > 1) {
    return 'pre-commit';
  }
  if (Object.keys(config.paths).length > 1) {
    return 'pre-commit';
  }
  return 'husky';
}

export function collectRuleFileStatuses(
  projectRoot: string,
  config: RuleConfig
): RuleFileStatus[] {
  const active = configuredRuleKeys(config);
  const statuses: RuleFileStatus[] = [];
  for (const [target, dirs] of Object.entries(TARGET_RULE_DIRS) as Array<
    [Target, string[]]
  >) {
    for (const relativeDir of dirs) {
      const dir = path.join(projectRoot, relativeDir);
      for (const filePath of walkRuleFiles(dir)) {
        const content = fs.readFileSync(filePath, 'utf8');
        const marker = parseRuleMarker(content);
        if (!marker) {
          statuses.push({
            path: filePath,
            target,
            ruleId: null,
            status: 'unowned'
          });
          continue;
        }
        const key = `${target}:${marker.ruleId}`;
        if (!active.has(key)) {
          statuses.push({
            path: filePath,
            target,
            ruleId: marker.ruleId,
            status: 'stale'
          });
          continue;
        }
        const canonical = canonicalRuleContent(target, marker.ruleId, config);
        const drifted =
          !verifyRuleChecksum(content) ||
          !canonical ||
          stripRuleMarker(content) !== stripRuleMarker(canonical);
        statuses.push({
          path: filePath,
          target,
          ruleId: marker.ruleId,
          status: drifted ? 'drifted' : 'ok'
        });
      }
    }
  }
  return statuses.sort((left, right) => left.path.localeCompare(right.path));
}

export function removeStaleRuleFiles(ruleFiles: RuleFileStatus[]): string[] {
  const removed: string[] = [];
  for (const ruleFile of ruleFiles) {
    if (ruleFile.status !== 'stale' || ruleFile.ruleId === null) {
      continue;
    }
    fs.rmSync(ruleFile.path, { force: true });
    removed.push(ruleFile.path);
  }
  return removed;
}

function formatConfigPaths(
  languages: string[],
  paths: Record<string, string[]>
): string | null {
  const orderedKeys = [
    ...languages.filter((language) => Array.isArray(paths[language])),
    ...Object.keys(paths)
      .filter((language) => !languages.includes(language))
      .sort()
  ];
  const entries = orderedKeys.flatMap((language) => {
    const values = paths[language];
    return Array.isArray(values) && values.length > 0
      ? [`${language}=${values.join(',')}`]
      : [];
  });
  return entries.length > 0 ? entries.join('; ') : null;
}

function formatConfigTools(
  languages: string[],
  tools: Record<string, string[]>
): string | null {
  const orderedKeys = [
    ...languages.filter((language) => Array.isArray(tools[language])),
    ...Object.keys(tools)
      .filter((language) => !languages.includes(language))
      .sort()
  ];
  const entries = orderedKeys.flatMap((language) => {
    const values = tools[language];
    return Array.isArray(values) && values.length > 0
      ? [`${language}=${values.join(',')}`]
      : [];
  });
  return entries.length > 0 ? entries.join('; ') : null;
}

function formatRuleFilePath(report: DoctorReport, filePath: string): string {
  if (!path.isAbsolute(filePath)) return filePath;
  const root = report.configPath
    ? path.dirname(report.configPath)
    : process.cwd();
  const relative = path.relative(root, filePath);
  return relative && !relative.startsWith('..') ? relative : filePath;
}

function formatRuleStatus(status: RuleFileState): string {
  switch (status) {
    case 'ok':
      return 'ok';
    case 'drifted':
      return 'DRIFTED - run ballast install --refresh-config to restore';
    case 'stale':
      return 'STALE - run ballast doctor --fix to remove';
    case 'unowned':
      return 'unowned - not managed by Ballast, skipped';
    default:
      return status;
  }
}

export function formatDoctorReport(report: DoctorReport): string {
  const lines = [
    'Ballast doctor',
    `Current CLI: ${report.currentCli} ${report.currentVersion}`,
    '',
    'Installed CLIs:'
  ];

  for (const item of report.installed) {
    if (!item.path) {
      lines.push(`- ${item.name}: not found`);
      continue;
    }
    lines.push(`- ${item.name}: ${item.version ?? 'unknown'} (${item.path})`);
  }

  if (report.detectedAppType !== 'unknown') {
    lines.push('', `Detected app type: ${report.detectedAppType}`);
  }

  lines.push('', 'Rule files:');
  if (report.ruleFiles.length === 0) {
    lines.push('- none found');
  } else {
    for (const ruleFile of report.ruleFiles) {
      lines.push(
        `- ${formatRuleFilePath(report, ruleFile.path)} [${formatRuleStatus(ruleFile.status)}]`
      );
    }
  }

  lines.push('', 'Config:');
  if (!report.configPath) {
    lines.push('- .rulesrc.json: not found');
  } else {
    lines.push(`- file: ${report.configPath}`);
    lines.push(`- ballastVersion: ${report.configVersion ?? 'missing'}`);
    if (report.configTargets.length > 0) {
      lines.push(`- targets: ${report.configTargets.join(', ')}`);
    }
    if (report.configAgents.length > 0) {
      lines.push(`- agents: ${report.configAgents.join(', ')}`);
    }
    if (report.configSkills.length > 0) {
      lines.push(`- skills: ${report.configSkills.join(', ')}`);
    }
    if (report.configLanguages.length > 0) {
      lines.push(`- languages: ${report.configLanguages.join(', ')}`);
    }
    const formattedPaths = formatConfigPaths(
      report.configLanguages,
      report.configPaths
    );
    if (formattedPaths) {
      lines.push(`- paths: ${formattedPaths}`);
    }
    const formattedTools = formatConfigTools(
      report.configLanguages,
      report.configTools
    );
    if (formattedTools) {
      lines.push(`- tools: ${formattedTools}`);
    }
    if (report.configDiscoveryExcludePaths.length > 0) {
      lines.push(
        `- discovery.excludePaths: ${report.configDiscoveryExcludePaths.join(',')}`
      );
    }
    if (report.configTaskSystem) {
      lines.push(`- taskSystem: ${report.configTaskSystem}`);
    }
    if (report.configDeploymentModel) {
      lines.push(`- deploymentModel: ${report.configDeploymentModel}`);
    }
    if (report.configPublishingProfiles.length > 0) {
      lines.push(
        `- publishingProfiles: ${report.configPublishingProfiles.join(', ')}`
      );
    }
  }

  lines.push('', 'Recommendations:');
  if (report.recommendations.length === 0) {
    lines.push('- No action needed.');
  } else {
    for (const recommendation of report.recommendations) {
      lines.push(`- ${recommendation}`);
    }
  }

  return `${lines.join('\n')}\n`;
}

export function runDoctor(options: { fix?: boolean } = {}): number {
  const projectRoot = findProjectRoot();
  const configPath = path.join(projectRoot, getRulesrcFilename());
  const config = loadConfig(projectRoot);
  const ruleFiles = collectRuleFileStatuses(projectRoot, {
    targets: config?.targets ?? [],
    agents: config?.agents ?? [],
    languages: config?.languages ?? [],
    paths: config?.paths ?? {},
    taskSystem: config?.taskSystem ?? null,
    deploymentModel: config?.deploymentModel ?? null,
    publishingProfiles: config?.publishingProfiles ?? []
  });
  const removedRuleFiles =
    options.fix && config ? removeStaleRuleFiles(ruleFiles) : [];
  const nextRuleFiles =
    removedRuleFiles.length > 0
      ? ruleFiles.filter(
          (ruleFile) => !removedRuleFiles.includes(ruleFile.path)
        )
      : ruleFiles;
  const report = buildDoctorReport(
    'ballast-typescript',
    BALLAST_VERSION,
    config ? configPath : null,
    config?.ballastVersion ?? null,
    config?.targets ?? [],
    config?.agents ?? [],
    config?.skills ?? [],
    config?.languages ?? [],
    config?.paths ?? {},
    config?.tools ?? {},
    config?.discovery?.excludePaths ?? [],
    config?.taskSystem ?? null,
    config?.deploymentModel ?? null,
    config?.publishingProfiles ?? [],
    CLI_NAMES.map((name) => detectInstalledCli(name)),
    detectAppType(projectRoot),
    nextRuleFiles
  );
  if (removedRuleFiles.length > 0) {
    process.stdout.write('Removed stale managed rule files:\n');
    for (const filePath of removedRuleFiles) {
      process.stdout.write(`- ${filePath}\n`);
    }
    process.stdout.write('\n');
  }
  process.stdout.write(formatDoctorReport(report));
  return 0;
}

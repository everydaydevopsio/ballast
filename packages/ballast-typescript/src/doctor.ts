import { spawnSync } from 'child_process';
import fs from 'fs';
import path from 'path';
import { findProjectRoot, getRulesrcFilename, loadConfig } from './config';
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
  installed: InstalledCliStatus[];
  detectedAppType: AppType;
  recommendations: string[];
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
  installed: InstalledCliStatus[],
  detectedAppType: AppType = 'unknown'
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
    installed,
    detectedAppType,
    recommendations
  };
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

export function runDoctor(): number {
  const projectRoot = findProjectRoot();
  const configPath = path.join(projectRoot, getRulesrcFilename());
  const config = loadConfig(projectRoot);
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
    CLI_NAMES.map((name) => detectInstalledCli(name)),
    detectAppType(projectRoot)
  );
  process.stdout.write(formatDoctorReport(report));
  return 0;
}

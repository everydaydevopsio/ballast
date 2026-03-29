import { spawnSync } from 'child_process';
import path from 'path';
import { findProjectRoot, getRulesrcFilename, loadConfig } from './config';
import { BALLAST_VERSION } from './version';

export interface InstalledCliStatus {
  name: string;
  version: string | null;
  path: string | null;
}

export interface DoctorReport {
  currentCli: string;
  currentVersion: string;
  configPath: string | null;
  configVersion: string | null;
  configTargets: string[];
  configAgents: string[];
  installed: InstalledCliStatus[];
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

function refreshConfigCommand(
  report: Pick<DoctorReport, 'currentCli' | 'configTargets' | 'configAgents'>
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
  installed: InstalledCliStatus[]
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
      configAgents
    });
    recommendations.push(
      refresh
        ? `Refresh ${path.basename(configPath)} to Ballast ${targetVersion}: ${refresh}`
        : `Refresh ${path.basename(configPath)} with a current Ballast install command.`
    );
  }

  return {
    currentCli,
    currentVersion,
    configPath,
    configVersion,
    configTargets,
    configAgents,
    installed,
    recommendations
  };
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
    CLI_NAMES.map((name) => detectInstalledCli(name))
  );
  process.stdout.write(formatDoctorReport(report));
  return 0;
}

import fs from 'fs';
import path from 'path';
import { LANGUAGES } from './agents';

const RULESRC_FILENAME = '.rulesrc.json';
const LEGACY_TYPESCRIPT_RULESRC_FILENAME = '.rulesrc.ts.json';
const TARGETS = ['cursor', 'claude', 'opencode', 'codex', 'gemini'] as const;
export const TASK_SYSTEMS = ['github', 'jira', 'linear'] as const;
export const DEFAULT_TASK_SYSTEM = 'github' as const;
export const DEPLOYMENT_MODELS = [
  'none',
  'kubernetes',
  'serverless',
  'server',
  'hosted'
] as const;
export const DEFAULT_DEPLOYMENT_MODEL = 'none' as const;

export type Target = (typeof TARGETS)[number];
export type TaskSystem = (typeof TASK_SYSTEMS)[number];
export type DeploymentModel = (typeof DEPLOYMENT_MODELS)[number];

export interface RulesConfig {
  targets: Target[];
  agents: string[];
  skills?: string[];
  ballastVersion?: string;
  languages?: string[];
  paths?: Record<string, string[]>;
  taskSystem?: TaskSystem;
  deploymentModel?: DeploymentModel;
}

export function getRulesrcFilename(): string {
  return RULESRC_FILENAME;
}

export function getLegacyRulesrcFilename(
  language: string = 'typescript'
): string {
  if (language === 'typescript') return LEGACY_TYPESCRIPT_RULESRC_FILENAME;
  return `.rulesrc.${language}.json`;
}

export function parseTargets(raw: unknown): {
  targets: Target[];
  invalidTargets: string[];
} {
  const values = Array.isArray(raw) ? raw : [raw];
  const seen = new Set<Target>();
  const invalidSeen = new Set<string>();

  for (const value of values) {
    if (typeof value !== 'string') continue;
    for (const part of value.split(',')) {
      const token = part.trim().toLowerCase();
      if (!token) continue;
      if (TARGETS.includes(token as Target)) {
        seen.add(token as Target);
        continue;
      }
      invalidSeen.add(token);
    }
  }

  return {
    targets: Array.from(seen),
    invalidTargets: Array.from(invalidSeen)
  };
}

export function normalizeTargets(raw: unknown): Target[] {
  return parseTargets(raw).targets;
}

function hasConfigFile(dir: string): boolean {
  if (
    fs.existsSync(path.join(dir, RULESRC_FILENAME)) ||
    fs.existsSync(path.join(dir, LEGACY_TYPESCRIPT_RULESRC_FILENAME))
  ) {
    return true;
  }
  return LANGUAGES.some((language) =>
    fs.existsSync(path.join(dir, getLegacyRulesrcFilename(language)))
  );
}

function hasProjectMarker(dir: string): boolean {
  return (
    fs.existsSync(path.join(dir, 'package.json')) ||
    fs.existsSync(path.join(dir, 'go.mod')) ||
    fs.existsSync(path.join(dir, 'pyproject.toml')) ||
    fs.existsSync(path.join(dir, 'ansible.cfg')) ||
    fs.existsSync(path.join(dir, 'site.yml')) ||
    fs.existsSync(path.join(dir, 'playbook.yml')) ||
    fs.existsSync(path.join(dir, 'requirements.yml')) ||
    fs.existsSync(path.join(dir, 'requirements.yaml')) ||
    fs.existsSync(path.join(dir, '.terraform-version')) ||
    fs.existsSync(path.join(dir, 'main.tf')) ||
    fs.existsSync(path.join(dir, 'providers.tf')) ||
    fs.existsSync(path.join(dir, 'versions.tf')) ||
    fs.existsSync(path.join(dir, 'terraform.tf'))
  );
}

function isGitBoundary(dir: string): boolean {
  const gitPath = path.join(dir, '.git');
  if (!fs.existsSync(gitPath)) return false;
  const stat = fs.statSync(gitPath);
  if (stat.isFile()) return true;
  return (
    fs.existsSync(path.join(gitPath, 'HEAD')) ||
    fs.existsSync(path.join(gitPath, 'config'))
  );
}

export function findProjectRoot(cwd: string = process.cwd()): string {
  let dir = path.resolve(cwd);
  const root = path.parse(dir).root;
  while (dir !== root) {
    if (hasConfigFile(dir) || hasProjectMarker(dir)) {
      return dir;
    }
    if (isGitBoundary(dir)) {
      return dir;
    }
    dir = path.dirname(dir);
  }
  return cwd;
}

export function loadConfig(
  projectRoot?: string,
  language: string = 'typescript'
): RulesConfig | null {
  const root = projectRoot ?? findProjectRoot();
  const fileCandidates = [
    getRulesrcFilename(),
    getLegacyRulesrcFilename(language)
  ];
  const filePath = fileCandidates
    .map((name) => path.join(root, name))
    .find((candidate) => fs.existsSync(candidate));
  if (!filePath) return null;
  try {
    return normalizeRulesConfig(JSON.parse(fs.readFileSync(filePath, 'utf8')));
  } catch {
    return null;
  }
}

export function saveConfig(config: RulesConfig, projectRoot?: string): void {
  const root = projectRoot ?? findProjectRoot();
  const filePath = path.join(root, getRulesrcFilename());
  const existing = loadRawConfig(filePath);
  let nextConfig: RulesConfig = {
    targets: normalizeTargets(config.targets),
    agents: config.agents,
    ballastVersion: config.ballastVersion ?? existing?.ballastVersion
  };
  const nextSkills =
    config.skills && config.skills.length > 0
      ? config.skills
      : existing?.skills && existing.skills.length > 0
        ? existing.skills
        : undefined;
  if (nextSkills) {
    nextConfig = {
      ...nextConfig,
      skills: nextSkills
    };
  }

  const mergedLanguages = mergeLanguages(existing, config);
  const mergedPaths = mergePaths(existing, config, mergedLanguages);
  if (mergedLanguages.length > 0) {
    nextConfig = {
      ...nextConfig,
      languages: mergedLanguages,
      paths: mergedPaths
    };
  }

  const taskSystem = config.taskSystem ?? existing?.taskSystem;
  if (taskSystem) {
    nextConfig = { ...nextConfig, taskSystem };
  }

  const deploymentModel = config.deploymentModel ?? existing?.deploymentModel;
  if (deploymentModel) {
    nextConfig = { ...nextConfig, deploymentModel };
  }

  fs.writeFileSync(filePath, JSON.stringify(nextConfig, null, 2), 'utf8');
}

function loadRawConfig(filePath: string): RulesConfig | null {
  if (!fs.existsSync(filePath)) return null;
  try {
    return normalizeRulesConfig(JSON.parse(fs.readFileSync(filePath, 'utf8')));
  } catch {
    return null;
  }
}

function normalizeRulesConfig(data: unknown): RulesConfig | null {
  if (!data || typeof data !== 'object') {
    return null;
  }
  const record = data as {
    target?: unknown;
    targets?: unknown;
    agents?: unknown;
    skills?: unknown;
    ballastVersion?: unknown;
    languages?: unknown;
    paths?: unknown;
    taskSystem?: unknown;
    deploymentModel?: unknown;
  };
  const targets = normalizeTargets(record.targets ?? record.target);
  if (targets.length === 0 || !Array.isArray(record.agents)) {
    return null;
  }
  const agents = record.agents.filter(
    (agent): agent is string => typeof agent === 'string'
  );
  if (agents.length !== record.agents.length) {
    return null;
  }

  const config: RulesConfig = { targets, agents };
  if (Array.isArray(record.skills)) {
    config.skills = record.skills.filter(
      (skill): skill is string => typeof skill === 'string'
    );
  }
  if (typeof record.ballastVersion === 'string') {
    config.ballastVersion = record.ballastVersion;
  }
  if (Array.isArray(record.languages)) {
    config.languages = record.languages.filter(
      (language): language is string => typeof language === 'string'
    );
  }
  if (record.paths && typeof record.paths === 'object') {
    config.paths = Object.fromEntries(
      Object.entries(record.paths).flatMap(([key, value]) =>
        Array.isArray(value) &&
        value.every((item): item is string => typeof item === 'string')
          ? [[key, value]]
          : []
      )
    );
  }
  if (
    typeof record.taskSystem === 'string' &&
    (TASK_SYSTEMS as readonly string[]).includes(record.taskSystem)
  ) {
    config.taskSystem = record.taskSystem as TaskSystem;
  }
  if (
    typeof record.deploymentModel === 'string' &&
    (DEPLOYMENT_MODELS as readonly string[]).includes(record.deploymentModel)
  ) {
    config.deploymentModel = record.deploymentModel as DeploymentModel;
  }
  return config;
}

function mergeLanguages(
  existing: RulesConfig | null,
  config: RulesConfig
): string[] {
  const languages = new Set<string>(existing?.languages ?? []);
  for (const language of config.languages ?? []) {
    languages.add(language);
  }
  return Array.from(languages);
}

function mergePaths(
  existing: RulesConfig | null,
  config: RulesConfig,
  languages: string[]
): Record<string, string[]> {
  const merged: Record<string, string[]> = { ...(existing?.paths ?? {}) };
  for (const language of config.languages ?? []) {
    if (!merged[language] || merged[language].length === 0) {
      merged[language] = ['.'];
    }
  }
  for (const language of languages) {
    if (!merged[language] || merged[language].length === 0) {
      merged[language] = ['.'];
    }
  }
  return merged;
}

export function isCiMode(): boolean {
  return (
    process.env.CI === 'true' ||
    process.env.CI === '1' ||
    process.env.TF_BUILD === 'true' ||
    process.env.GITHUB_ACTIONS === 'true' ||
    process.env.GITLAB_CI === 'true'
  );
}

export { RULESRC_FILENAME };

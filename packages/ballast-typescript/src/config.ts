import fs from 'fs';
import path from 'path';
import { LANGUAGES } from './agents';

const RULESRC_FILENAME = '.rulesrc.json';
const LEGACY_TYPESCRIPT_RULESRC_FILENAME = '.rulesrc.ts.json';
const TARGETS = ['cursor', 'claude', 'opencode', 'codex', 'gemini'] as const;
export const TASK_SYSTEMS = ['github', 'jira', 'linear', 'none'] as const;
export const DEFAULT_TASK_SYSTEM = 'github' as const;
export const DEPLOYMENT_MODELS = [
  'none',
  'kubernetes',
  'serverless',
  'server',
  'docker',
  'hosted'
] as const;
export const DEFAULT_DEPLOYMENT_MODEL = 'none' as const;
export const PUBLISHING_PROFILES = [
  'cli',
  'apps',
  'web',
  'api',
  'libraries',
  'sdks',
  'apt',
  'brew'
] as const;
export const DEFAULT_LANGUAGE_TOOLS: Record<string, string[]> = {
  python: ['uv', 'pyenv'],
  typescript: ['pnpm', 'corepack'],
  go: ['go', 'gofumpt', 'golangci-lint'],
  terraform: ['tfenv', 'tflint', 'trivy'],
  ansible: ['ansible-lint', 'molecule'],
  dart: ['flutter', 'fvm'],
  docker: ['docker', 'hadolint', 'trivy']
};

export type Target = (typeof TARGETS)[number];
export type TaskSystem = (typeof TASK_SYSTEMS)[number];
export type DeploymentModel = (typeof DEPLOYMENT_MODELS)[number];
export type PublishingProfile = (typeof PUBLISHING_PROFILES)[number];

export interface DiscoveryConfig {
  excludePaths?: string[];
}

export interface RulesConfig {
  targets: Target[];
  agents: string[];
  skills?: string[];
  ballastVersion?: string;
  languages?: string[];
  paths?: Record<string, string[]>;
  tools?: Record<string, string[]>;
  discovery?: DiscoveryConfig;
  taskSystem?: TaskSystem;
  deploymentModel?: DeploymentModel;
  publishingProfiles?: PublishingProfile[];
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

function hasDockerfileMarker(dir: string): boolean {
  try {
    return fs
      .readdirSync(dir, { withFileTypes: true })
      .some(
        (entry) =>
          entry.isFile() &&
          (entry.name === 'Dockerfile' ||
            entry.name === 'Containerfile' ||
            entry.name.startsWith('Dockerfile.') ||
            entry.name.startsWith('Containerfile.'))
      );
  } catch {
    return false;
  }
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
    fs.existsSync(path.join(dir, 'terraform.tf')) ||
    fs.existsSync(path.join(dir, 'pubspec.yaml')) ||
    fs.existsSync(path.join(dir, 'analysis_options.yaml')) ||
    fs.existsSync(path.join(dir, '.metadata')) ||
    hasDockerfileMarker(dir) ||
    fs.existsSync(path.join(dir, 'compose.yaml')) ||
    fs.existsSync(path.join(dir, 'compose.yml')) ||
    fs.existsSync(path.join(dir, 'docker-compose.yaml')) ||
    fs.existsSync(path.join(dir, 'docker-compose.yml'))
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
  const start = path.resolve(cwd);
  let dir = start;
  const root = path.parse(dir).root;
  while (dir !== root) {
    if (hasConfigFile(dir) || hasProjectMarker(dir)) {
      if (dir !== start && !isGitBoundary(dir)) {
        return start;
      }
      return dir;
    }
    if (isGitBoundary(dir)) {
      return dir;
    }
    dir = path.dirname(dir);
  }
  return start;
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
  const mergedTools = mergeTools(existing, config, mergedLanguages);
  if (mergedLanguages.length > 0) {
    nextConfig = {
      ...nextConfig,
      languages: mergedLanguages,
      paths: mergedPaths,
      tools: mergedTools
    };
  }

  const discovery = config.discovery ?? existing?.discovery;
  if (discovery !== undefined) {
    nextConfig = { ...nextConfig, discovery };
  }

  const taskSystem = config.taskSystem ?? existing?.taskSystem;
  if (taskSystem) {
    nextConfig = { ...nextConfig, taskSystem };
  }

  const deploymentModel = config.deploymentModel ?? existing?.deploymentModel;
  if (deploymentModel) {
    nextConfig = { ...nextConfig, deploymentModel };
  }

  const publishingProfiles =
    config.publishingProfiles ?? existing?.publishingProfiles;
  if (publishingProfiles !== undefined) {
    nextConfig = { ...nextConfig, publishingProfiles };
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
    tools?: unknown;
    discovery?: unknown;
    taskSystem?: unknown;
    deploymentModel?: unknown;
    publishingProfiles?: unknown;
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
  if (record.tools && typeof record.tools === 'object') {
    config.tools = normalizeTools(record.tools);
  }
  const discovery = normalizeDiscovery(record.discovery);
  if (discovery) {
    config.discovery = discovery;
  }
  if (
    typeof record.taskSystem === 'string' &&
    (TASK_SYSTEMS as readonly string[]).includes(record.taskSystem)
  ) {
    config.taskSystem = record.taskSystem as TaskSystem;
  }
  if (typeof record.deploymentModel === 'string') {
    const deploymentModel = record.deploymentModel.trim().toLowerCase();
    if ((DEPLOYMENT_MODELS as readonly string[]).includes(deploymentModel)) {
      config.deploymentModel = deploymentModel as DeploymentModel;
    }
  }
  if (Array.isArray(record.publishingProfiles)) {
    config.publishingProfiles = normalizePublishingProfiles(
      record.publishingProfiles
    );
  }
  return config;
}

function normalizeDiscovery(raw: unknown): DiscoveryConfig | undefined {
  if (!raw || typeof raw !== 'object') return undefined;
  const record = raw as { excludePaths?: unknown };
  const discovery: DiscoveryConfig = {};
  if (Array.isArray(record.excludePaths)) {
    const excludePaths = uniqueToolList(record.excludePaths);
    if (excludePaths.length > 0) {
      discovery.excludePaths = excludePaths;
    }
  }
  return Object.keys(discovery).length > 0 ? discovery : undefined;
}

export function normalizeTools(raw: unknown): Record<string, string[]> {
  if (!raw || typeof raw !== 'object') return {};
  return Object.fromEntries(
    Object.entries(raw).flatMap(([key, value]) => {
      const language = key.trim().toLowerCase();
      if (!language || !Array.isArray(value)) return [];
      const tools = uniqueToolList(value);
      return tools.length > 0 ? [[language, tools]] : [];
    })
  );
}

function uniqueToolList(values: unknown[]): string[] {
  const seen = new Set<string>();
  const tools: string[] = [];
  for (const value of values) {
    if (typeof value !== 'string') continue;
    const tool = value.trim().toLowerCase();
    if (!tool || seen.has(tool)) continue;
    seen.add(tool);
    tools.push(tool);
  }
  return tools;
}

export function normalizePublishingProfiles(
  values: unknown[]
): PublishingProfile[] {
  const aliases: Record<string, PublishingProfile> = {
    app: 'apps',
    library: 'libraries',
    sdk: 'sdks'
  };
  const profiles = new Set<PublishingProfile>();
  for (const value of values) {
    if (typeof value !== 'string') continue;
    const token = value.trim().toLowerCase();
    const profile = aliases[token] ?? token;
    if ((PUBLISHING_PROFILES as readonly string[]).includes(profile)) {
      profiles.add(profile as PublishingProfile);
    }
  }
  return Array.from(profiles);
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

function mergeTools(
  existing: RulesConfig | null,
  config: RulesConfig,
  languages: string[]
): Record<string, string[]> {
  const merged: Record<string, string[]> = { ...(existing?.tools ?? {}) };
  for (const [language, tools] of Object.entries(config.tools ?? {})) {
    const normalizedLanguage = language.trim().toLowerCase();
    if (!normalizedLanguage) continue;
    if (tools.length > 0) {
      merged[normalizedLanguage] = [...tools];
    }
  }
  for (const language of languages) {
    const normalizedLanguage = language.trim().toLowerCase();
    if (
      normalizedLanguage &&
      (!merged[normalizedLanguage] || merged[normalizedLanguage].length === 0)
    ) {
      merged[normalizedLanguage] =
        DEFAULT_LANGUAGE_TOOLS[normalizedLanguage] ?? [];
    }
  }
  return Object.fromEntries(
    Object.entries(merged).filter(([, tools]) => tools.length > 0)
  );
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

import fs from 'fs';
import path from 'path';
import { LANGUAGES } from './agents';

const RULESRC_FILENAME = '.rulesrc.json';
const LEGACY_TYPESCRIPT_RULESRC_FILENAME = '.rulesrc.ts.json';
const TARGETS = ['cursor', 'claude', 'opencode', 'codex'] as const;

export type Target = (typeof TARGETS)[number];

export interface RulesConfig {
  targets: Target[];
  agents: string[];
  skills?: string[];
  ballastVersion?: string;
  languages?: string[];
  paths?: Record<string, string[]>;
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

export function normalizeTargets(raw: unknown): Target[] {
  const values = Array.isArray(raw) ? raw : [raw];
  const seen = new Set<Target>();
  for (const value of values) {
    if (typeof value !== 'string') continue;
    for (const part of value.split(',')) {
      const target = part.trim().toLowerCase() as Target;
      if (TARGETS.includes(target) && !seen.has(target)) {
        seen.add(target);
      }
    }
  }
  return Array.from(seen);
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

export function findProjectRoot(cwd: string = process.cwd()): string {
  let dir = path.resolve(cwd);
  const root = path.parse(dir).root;
  while (dir !== root) {
    if (hasConfigFile(dir) || fs.existsSync(path.join(dir, 'package.json'))) {
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

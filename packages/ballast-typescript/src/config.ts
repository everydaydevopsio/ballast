import fs from 'fs';
import path from 'path';
import { LANGUAGES } from './agents';

const RULESRC_FILENAME = '.rulesrc.json';

export type Target = 'cursor' | 'claude' | 'opencode' | 'codex';

export interface RulesConfig {
  target: Target;
  agents: string[];
}

export function getRulesrcFilename(language: string = 'typescript'): string {
  if (language === 'typescript') return RULESRC_FILENAME;
  return `.rulesrc.${language}.json`;
}

function hasConfigFile(dir: string): boolean {
  if (fs.existsSync(path.join(dir, RULESRC_FILENAME))) return true;
  return LANGUAGES.some((language) =>
    fs.existsSync(path.join(dir, getRulesrcFilename(language)))
  );
}

/**
 * Find project root (directory containing .rulesrc.json or package.json)
 */
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

/**
 * Load .rulesrc.json from project root
 */
export function loadConfig(
  projectRoot?: string,
  language: string = 'typescript'
): RulesConfig | null {
  const root = projectRoot ?? findProjectRoot();
  const filePath = path.join(root, getRulesrcFilename(language));
  if (!fs.existsSync(filePath)) return null;
  try {
    const raw = fs.readFileSync(filePath, 'utf8');
    const data = JSON.parse(raw) as unknown;
    if (
      !data ||
      typeof data !== 'object' ||
      !('target' in data) ||
      !Array.isArray((data as RulesConfig).agents)
    )
      return null;
    return {
      target: (data as RulesConfig).target,
      agents: (data as RulesConfig).agents
    };
  } catch {
    return null;
  }
}

/**
 * Save .rulesrc.json to project root
 */
export function saveConfig(
  config: RulesConfig,
  projectRoot?: string,
  language: string = 'typescript'
): void {
  const root = projectRoot ?? findProjectRoot();
  const filePath = path.join(root, getRulesrcFilename(language));
  fs.writeFileSync(
    filePath,
    JSON.stringify({ target: config.target, agents: config.agents }, null, 2),
    'utf8'
  );
}

/**
 * Detect if running in CI (non-interactive) mode
 */
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

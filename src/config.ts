import fs from 'fs';
import path from 'path';

const RULESRC_FILENAME = '.rulesrc.json';

export type Target = 'cursor' | 'claude' | 'opencode';

export interface RulesConfig {
  target: Target;
  agents: string[];
}

/**
 * Find project root (directory containing .rulesrc.json or package.json)
 */
export function findProjectRoot(cwd: string = process.cwd()): string {
  let dir = path.resolve(cwd);
  const root = path.parse(dir).root;
  while (dir !== root) {
    if (
      fs.existsSync(path.join(dir, RULESRC_FILENAME)) ||
      fs.existsSync(path.join(dir, 'package.json'))
    ) {
      return dir;
    }
    dir = path.dirname(dir);
  }
  return cwd;
}

/**
 * Load .rulesrc.json from project root
 */
export function loadConfig(projectRoot?: string): RulesConfig | null {
  const root = projectRoot ?? findProjectRoot();
  const filePath = path.join(root, RULESRC_FILENAME);
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
export function saveConfig(config: RulesConfig, projectRoot?: string): void {
  const root = projectRoot ?? findProjectRoot();
  const filePath = path.join(root, RULESRC_FILENAME);
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

import fs from 'fs';
import path from 'path';

const PACKAGE_ROOT = path.resolve(__dirname, '..');

function hasAgents(root: string): boolean {
  return fs.existsSync(path.join(root, 'agents', 'common'));
}

function resolveAgentsRoot(): string {
  const repoRootOverride = process.env.BALLAST_REPO_ROOT;
  if (repoRootOverride) {
    const resolved = path.resolve(repoRootOverride);
    if (!hasAgents(resolved)) {
      throw new Error(
        `BALLAST_REPO_ROOT does not contain agents/: ${resolved}`
      );
    }
    return resolved;
  }

  if (hasAgents(PACKAGE_ROOT)) {
    return PACKAGE_ROOT;
  }

  // Monorepo fallback for local development and tests.
  const monorepoRoot = path.resolve(PACKAGE_ROOT, '..', '..');
  if (hasAgents(monorepoRoot)) {
    return monorepoRoot;
  }

  return PACKAGE_ROOT;
}

const AGENTS_ROOT = resolveAgentsRoot();

export const LANGUAGES = ['typescript', 'python', 'go'] as const;
export type Language = (typeof LANGUAGES)[number];

export const COMMON_AGENT_IDS = ['local-dev', 'cicd', 'observability'] as const;
export const LANGUAGE_AGENT_IDS = ['linting', 'logging', 'testing'] as const;
export const AGENT_IDS = [...COMMON_AGENT_IDS, ...LANGUAGE_AGENT_IDS] as const;
export type AgentId = (typeof AGENT_IDS)[number];

/**
 * Resolve path to an agent directory
 */
export function getAgentDir(
  agentId: string,
  language: Language = 'typescript'
): string {
  if ((COMMON_AGENT_IDS as readonly string[]).includes(agentId)) {
    return path.join(AGENTS_ROOT, 'agents', 'common', agentId);
  }
  return path.join(AGENTS_ROOT, 'agents', language, agentId);
}

/**
 * Get list of available agent ids
 */
export function listAgents(_language: Language = 'typescript'): string[] {
  void _language;
  return AGENT_IDS.slice();
}

/**
 * Check if agent id is valid
 */
export function isValidAgent(
  agentId: string,
  _language: Language = 'typescript'
): boolean {
  void _language;
  return (AGENT_IDS as readonly string[]).includes(agentId);
}

/**
 * Resolve "all" to list of all agent ids
 */
export function resolveAgents(
  agents: string | string[],
  language: Language = 'typescript'
): string[] {
  if (Array.isArray(agents)) {
    const hasAll = agents.some((a) => a === 'all');
    if (hasAll) return AGENT_IDS.slice();
    return agents.filter((a) => isValidAgent(a, language));
  }
  if (agents === 'all') return AGENT_IDS.slice();
  return isValidAgent(agents, language) ? [agents] : [];
}

export { PACKAGE_ROOT };

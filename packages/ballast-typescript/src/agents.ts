import path from 'path';

// In monorepo layout this package lives at packages/ballast-typescript.
// Resolve to repository root so shared agent content can be loaded from /agents.
const PACKAGE_ROOT = path.join(__dirname, '..', '..', '..');

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
    return path.join(PACKAGE_ROOT, 'agents', 'common', agentId);
  }
  return path.join(PACKAGE_ROOT, 'agents', language, agentId);
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

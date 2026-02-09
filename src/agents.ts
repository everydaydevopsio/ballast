import path from 'path';

const PACKAGE_ROOT = path.join(__dirname, '..');

export const AGENT_IDS = [
  'linting',
  'local-dev',
  'cicd',
  'observability',
  'logging',
  'testing'
] as const;
export type AgentId = (typeof AGENT_IDS)[number];

/**
 * Resolve path to an agent directory
 */
export function getAgentDir(agentId: string): string {
  return path.join(PACKAGE_ROOT, 'agents', agentId);
}

/**
 * Get list of available agent ids
 */
export function listAgents(): string[] {
  return AGENT_IDS.slice();
}

/**
 * Check if agent id is valid
 */
export function isValidAgent(agentId: string): boolean {
  return (AGENT_IDS as readonly string[]).includes(agentId);
}

/**
 * Resolve "all" to list of all agent ids
 */
export function resolveAgents(agents: string | string[]): string[] {
  if (Array.isArray(agents)) {
    const hasAll = agents.some((a) => a === 'all');
    if (hasAll) return AGENT_IDS.slice();
    return agents.filter((a) => isValidAgent(a));
  }
  if (agents === 'all') return AGENT_IDS.slice();
  return isValidAgent(agents) ? [agents] : [];
}

export { PACKAGE_ROOT };

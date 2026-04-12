import fs from 'fs';
import path from 'path';

const PACKAGE_ROOT = path.resolve(__dirname, '..');

function hasAgents(root: string): boolean {
  return fs.existsSync(path.join(root, 'agents', 'common'));
}

function hasSkills(root: string): boolean {
  return fs.existsSync(path.join(root, 'skills', 'common'));
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

export const LANGUAGES = [
  'typescript',
  'python',
  'go',
  'ansible',
  'terraform'
] as const;
export type Language = (typeof LANGUAGES)[number];

export const COMMON_AGENT_IDS = [
  'local-dev',
  'docs',
  'cicd',
  'observability',
  'publishing',
  'git-hooks'
] as const;
export const LANGUAGE_AGENT_IDS = ['linting', 'logging', 'testing'] as const;
export const AGENT_IDS = [...COMMON_AGENT_IDS, ...LANGUAGE_AGENT_IDS] as const;
export type AgentId = (typeof AGENT_IDS)[number];
export const COMMON_SKILL_IDS = [
  'owasp-security-scan',
  'aws-health-review',
  'aws-live-health-review',
  'aws-weekly-security-review',
  'github-health-check'
] as const;
export const SKILL_IDS = [...COMMON_SKILL_IDS] as const;
export type SkillId = (typeof SKILL_IDS)[number];

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

export function getSkillDir(skillId: string): string {
  if (!hasSkills(AGENTS_ROOT)) {
    throw new Error(
      `BALLAST_REPO_ROOT does not contain skills/: ${AGENTS_ROOT}`
    );
  }
  return path.join(AGENTS_ROOT, 'skills', 'common', skillId);
}

export function listSkills(_language: Language = 'typescript'): string[] {
  void _language;
  return SKILL_IDS.slice();
}

export function isValidSkill(
  skillId: string,
  _language: Language = 'typescript'
): boolean {
  void _language;
  return (SKILL_IDS as readonly string[]).includes(skillId);
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

export function resolveSkills(
  skills: string | string[],
  language: Language = 'typescript'
): string[] {
  if (Array.isArray(skills)) {
    const hasAll = skills.some((value) => value === 'all');
    if (hasAll) return SKILL_IDS.slice();
    return skills.filter((value) => isValidSkill(value, language));
  }
  if (skills === 'all') return SKILL_IDS.slice();
  return isValidSkill(skills, language) ? [skills] : [];
}

export { PACKAGE_ROOT };

import fs from 'fs';
import path from 'path';
import { getAgentDir } from './agents';
import type { Target } from './config';

const TARGETS: Target[] = ['cursor', 'claude', 'opencode'];

/**
 * Read agent content.md
 */
export function getContent(agentId: string): string {
  const dir = getAgentDir(agentId);
  const file = path.join(dir, 'content.md');
  if (!fs.existsSync(file)) {
    throw new Error(`Agent "${agentId}" has no content.md`);
  }
  return fs.readFileSync(file, 'utf8');
}

/**
 * Read agent template file
 */
export function getTemplate(agentId: string, filename: string): string {
  const dir = getAgentDir(agentId);
  const file = path.join(dir, 'templates', filename);
  if (!fs.existsSync(file)) {
    throw new Error(`Agent "${agentId}" missing template: ${filename}`);
  }
  return fs.readFileSync(file, 'utf8');
}

/**
 * Build content for Cursor (.mdc = frontmatter + content)
 */
export function buildCursorFormat(agentId: string): string {
  const frontmatter = getTemplate(agentId, 'cursor-frontmatter.yaml');
  const content = getContent(agentId);
  return frontmatter + '\n' + content;
}

/**
 * Build content for Claude (header + content)
 */
export function buildClaudeFormat(agentId: string): string {
  const header = getTemplate(agentId, 'claude-header.md');
  const content = getContent(agentId);
  return header + content;
}

/**
 * Build content for OpenCode (YAML frontmatter + content)
 */
export function buildOpenCodeFormat(agentId: string): string {
  const frontmatter = getTemplate(agentId, 'opencode-frontmatter.yaml');
  const content = getContent(agentId);
  return frontmatter + '\n' + content;
}

/**
 * Build content for the given agent and target
 */
export function buildContent(agentId: string, target: Target): string {
  switch (target) {
    case 'cursor':
      return buildCursorFormat(agentId);
    case 'claude':
      return buildClaudeFormat(agentId);
    case 'opencode':
      return buildOpenCodeFormat(agentId);
    default:
      throw new Error(`Unknown target: ${target}`);
  }
}

/**
 * Get destination path for installed agent file
 */
export function getDestination(
  agentId: string,
  target: Target,
  projectRoot: string
): { dir: string; file: string } {
  const root = path.resolve(projectRoot);
  switch (target) {
    case 'cursor': {
      const dir = path.join(root, '.cursor', 'rules');
      const file = path.join(dir, `${agentId}.mdc`);
      return { dir, file };
    }
    case 'claude': {
      const dir = path.join(root, '.claude', 'rules');
      const file = path.join(dir, `${agentId}.md`);
      return { dir, file };
    }
    case 'opencode': {
      const dir = path.join(root, '.opencode');
      const file = path.join(dir, `${agentId}.md`);
      return { dir, file };
    }
    default:
      throw new Error(`Unknown target: ${target}`);
  }
}

/**
 * List supported targets
 */
export function listTargets(): string[] {
  return TARGETS.slice();
}

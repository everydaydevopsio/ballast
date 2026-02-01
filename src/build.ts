import fs from 'fs';
import path from 'path';
import { getAgentDir } from './agents';
import type { Target } from './config';

const TARGETS: Target[] = ['cursor', 'claude', 'opencode'];

/** Rule file convention: content.md (main) and content-<suffix>.md (e.g. content-mcp.md) */
const CONTENT_PREFIX = 'content';
const CONTENT_MAIN = `${CONTENT_PREFIX}.md`;

/**
 * List rule suffixes for an agent. content.md → suffix ''; content-<suffix>.md → suffix.
 * At least one of content.md or content-*.md must exist.
 */
export function listRuleSuffixes(agentId: string): string[] {
  const dir = getAgentDir(agentId);
  if (!fs.existsSync(dir)) {
    throw new Error(`Agent "${agentId}" has no content.md or content-*.md`);
  }
  const suffixes: string[] = [];
  if (fs.existsSync(path.join(dir, CONTENT_MAIN))) {
    suffixes.push('');
  }
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  for (const e of entries) {
    if (
      !e.isFile() ||
      !e.name.startsWith(CONTENT_PREFIX + '-') ||
      !e.name.endsWith('.md')
    )
      continue;
    const stem = e.name.slice(0, -3);
    const suffix = stem.slice(CONTENT_PREFIX.length + 1);
    if (suffix) suffixes.push(suffix);
  }
  if (suffixes.length === 0) {
    throw new Error(`Agent "${agentId}" has no content.md or content-*.md`);
  }
  return suffixes;
}

/**
 * Read agent content for a rule. ruleSuffix '' or undefined = content.md; else content-<suffix>.md.
 */
export function getContent(agentId: string, ruleSuffix?: string): string {
  const dir = getAgentDir(agentId);
  const basename = ruleSuffix
    ? `${CONTENT_PREFIX}-${ruleSuffix}.md`
    : CONTENT_MAIN;
  const file = path.join(dir, basename);
  if (!fs.existsSync(file)) {
    throw new Error(`Agent "${agentId}" has no ${basename}`);
  }
  return fs.readFileSync(file, 'utf8');
}

/**
 * Read agent template file. Tries rule-specific template first (e.g. cursor-frontmatter-mcp.yaml).
 */
export function getTemplate(
  agentId: string,
  filename: string,
  ruleSuffix?: string
): string {
  const dir = getAgentDir(agentId);
  const base = filename.replace(/\.[^.]+$/, '');
  const ext = path.extname(filename);
  if (ruleSuffix) {
    const ruleFile = path.join(dir, 'templates', `${base}-${ruleSuffix}${ext}`);
    if (fs.existsSync(ruleFile)) {
      return fs.readFileSync(ruleFile, 'utf8');
    }
  }
  const file = path.join(dir, 'templates', filename);
  if (!fs.existsSync(file)) {
    throw new Error(`Agent "${agentId}" missing template: ${filename}`);
  }
  return fs.readFileSync(file, 'utf8');
}

/**
 * Build content for Cursor (.mdc = frontmatter + content)
 */
export function buildCursorFormat(
  agentId: string,
  ruleSuffix?: string
): string {
  const frontmatter = getTemplate(
    agentId,
    'cursor-frontmatter.yaml',
    ruleSuffix
  );
  const content = getContent(agentId, ruleSuffix);
  return frontmatter + '\n' + content;
}

/**
 * Build content for Claude (header + content)
 */
export function buildClaudeFormat(
  agentId: string,
  ruleSuffix?: string
): string {
  const header = getTemplate(agentId, 'claude-header.md', ruleSuffix);
  const content = getContent(agentId, ruleSuffix);
  return header + content;
}

/**
 * Build content for OpenCode (YAML frontmatter + content)
 */
export function buildOpenCodeFormat(
  agentId: string,
  ruleSuffix?: string
): string {
  const frontmatter = getTemplate(
    agentId,
    'opencode-frontmatter.yaml',
    ruleSuffix
  );
  const content = getContent(agentId, ruleSuffix);
  return frontmatter + '\n' + content;
}

/**
 * Build content for the given agent, target, and optional rule suffix
 */
export function buildContent(
  agentId: string,
  target: Target,
  ruleSuffix?: string
): string {
  switch (target) {
    case 'cursor':
      return buildCursorFormat(agentId, ruleSuffix);
    case 'claude':
      return buildClaudeFormat(agentId, ruleSuffix);
    case 'opencode':
      return buildOpenCodeFormat(agentId, ruleSuffix);
    default:
      throw new Error(`Unknown target: ${target}`);
  }
}

/**
 * Get destination path for installed agent file. ruleSuffix '' or undefined = main rule; else <agentId>-<suffix>.
 */
export function getDestination(
  agentId: string,
  target: Target,
  projectRoot: string,
  ruleSuffix?: string
): { dir: string; file: string } {
  const root = path.resolve(projectRoot);
  const basename = ruleSuffix ? `${agentId}-${ruleSuffix}` : agentId;
  switch (target) {
    case 'cursor': {
      const dir = path.join(root, '.cursor', 'rules');
      const file = path.join(dir, `${basename}.mdc`);
      return { dir, file };
    }
    case 'claude': {
      const dir = path.join(root, '.claude', 'rules');
      const file = path.join(dir, `${basename}.md`);
      return { dir, file };
    }
    case 'opencode': {
      const dir = path.join(root, '.opencode');
      const file = path.join(dir, `${basename}.md`);
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

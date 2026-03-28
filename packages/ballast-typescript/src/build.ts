import fs from 'fs';
import path from 'path';
import YAML from 'yaml';
import {
  COMMON_AGENT_IDS,
  COMMON_SKILL_IDS,
  getAgentDir,
  getSkillDir
} from './agents';
import type { Target } from './config';
import type { Language } from './agents';
import pkg from '../package.json';

const TARGETS: Target[] = ['cursor', 'claude', 'opencode', 'codex'];
const REPO_ROOT = path.resolve(__dirname, '..', '..', '..');
const SOURCE_AGENTS_ROOT = path.join(REPO_ROOT, 'agents');
const HOOK_GUIDANCE_TOKEN = '{{BALLAST_HOOK_GUIDANCE}}';

type HookMode = 'standalone' | 'monorepo';

interface BuildOptions {
  hookMode?: HookMode;
}

interface SkillEntry {
  name: string;
  data: Buffer;
}

function getRuleSubdir(): string | null {
  const value = process.env.BALLAST_RULE_SUBDIR?.trim();
  if (!value) {
    return null;
  }
  if (!/^[A-Za-z0-9_-]+$/.test(value)) {
    throw new Error(
      `Invalid BALLAST_RULE_SUBDIR "${value}". Only [A-Za-z0-9_-] are allowed.`
    );
  }
  return value;
}

function getScopedBasename(
  ruleSubdir: string | null,
  basename: string
): string {
  if (!ruleSubdir || ruleSubdir === 'common') {
    return basename;
  }
  if (basename.startsWith(`${ruleSubdir}-`)) {
    return basename;
  }
  return `${ruleSubdir}-${basename}`;
}

function getRuleBasename(
  agentId: string,
  language: Language,
  ruleSuffix?: string
): string {
  const basename = ruleSuffix ? `${agentId}-${ruleSuffix}` : agentId;
  if ((COMMON_AGENT_IDS as readonly string[]).includes(agentId)) {
    return basename;
  }
  return `${language}-${basename}`;
}

function getPreferredAgentDir(agentId: string, language: Language): string {
  const sourceDir = (COMMON_AGENT_IDS as readonly string[]).includes(agentId)
    ? path.join(SOURCE_AGENTS_ROOT, 'common', agentId)
    : path.join(SOURCE_AGENTS_ROOT, language, agentId);
  if (fs.existsSync(sourceDir)) {
    return sourceDir;
  }
  return getAgentDir(agentId, language);
}

function getPreferredSkillDir(skillId: string): string {
  const sourceDir = (COMMON_SKILL_IDS as readonly string[]).includes(skillId)
    ? path.join(REPO_ROOT, 'skills', 'common', skillId)
    : path.join(REPO_ROOT, 'skills', 'typescript', skillId);
  if (fs.existsSync(sourceDir)) {
    return sourceDir;
  }
  return getSkillDir(skillId);
}

function getHookMode(
  agentId: string,
  language: Language,
  options?: BuildOptions
): HookMode {
  if (options?.hookMode) {
    return options.hookMode;
  }
  void agentId;
  void language;
  return 'standalone';
}

function renderHookGuidance(
  agentId: string,
  language: Language,
  options?: BuildOptions
): string {
  if (agentId !== 'linting') {
    return '';
  }

  const hookMode = getHookMode(agentId, language, options);
  if (language === 'typescript') {
    if (hookMode === 'monorepo') {
      return [
        '## Set Up Git Hooks with Husky',
        '',
        'Use Husky for this monorepo.',
        '',
        '- Install and initialize Husky.',
        "- Create `.husky/pre-commit` with the repo's fast lint command, such as `npx lint-staged`.",
        "- Create `.husky/pre-push` with the repo's unit test command, and for TypeScript monorepos run the build before the tests when the test command depends on generated output.",
        '- Keep the hook file executable with `chmod +x .husky/pre-commit`.',
        '- Keep `.husky/pre-push` executable with `chmod +x .husky/pre-push`.',
        "- Keep the hook in sync with the repo's linting workflow whenever the command changes."
      ].join('\n');
    }

    return [
      '## Git Hooks',
      '',
      'Use `pre-commit` for this repository layout.',
      '',
      '- Create `.pre-commit-config.yaml` at the repo root.',
      '- Install hooks with `pre-commit install`.',
      '- Install the pre-push hook with `pre-commit install --hook-type pre-push`.',
      '- Configure `.pre-commit-config.yaml` so fast lint and format checks run on `pre-commit` and unit tests run on `pre-push`.',
      '- Keep the configuration current with `pre-commit autoupdate`.',
      '- Verify the hook configuration with `pre-commit run --all-files`.'
    ].join('\n');
  }

  if (language === 'python') {
    return [
      '## Git Hooks',
      '',
      'Use `pre-commit` for Python projects.',
      '',
      '- Create `.pre-commit-config.yaml` at the repo root.',
      '- Install hooks with `pre-commit install`.',
      '- Install the pre-push hook with `pre-commit install --hook-type pre-push`.',
      '- Configure `.pre-commit-config.yaml` so unit tests run on `pre-push`.',
      '- Keep the configuration current with `pre-commit autoupdate`.',
      '- Re-run `pre-commit run --all-files` after hook changes.'
    ].join('\n');
  }

  if (language === 'go') {
    return [
      '## Git Hooks',
      '',
      'Use `pre-commit` for Go projects, and fan out to language-local configs with `sub-pre-commit` when needed.',
      '',
      '- Create or update `.pre-commit-config.yaml` at the repo root.',
      '- Use `sub-pre-commit` hooks to invoke nested `.pre-commit-config.yaml` files in Go subprojects.',
      '- Install hooks with `pre-commit install` and `pre-commit install --hook-type pre-push`.',
      '- Configure the pre-push stage to run Go unit tests for each module.',
      '- Keep the configuration current with `pre-commit autoupdate`.',
      '- Verify the hook configuration with `pre-commit run --all-files`.'
    ].join('\n');
  }

  return '';
}

function applyHookGuidance(
  content: string,
  agentId: string,
  language: Language,
  options?: BuildOptions
): string {
  if (!content.includes(HOOK_GUIDANCE_TOKEN)) {
    return content;
  }
  return content.replace(
    HOOK_GUIDANCE_TOKEN,
    renderHookGuidance(agentId, language, options)
  );
}

/** Rule file convention: content.md (main) and content-<suffix>.md (e.g. content-mcp.md) */
const CONTENT_PREFIX = 'content';
const CONTENT_MAIN = `${CONTENT_PREFIX}.md`;

/**
 * List rule suffixes for an agent. content.md → suffix ''; content-<suffix>.md → suffix.
 * At least one of content.md or content-*.md must exist.
 */
export function listRuleSuffixes(
  agentId: string,
  language: Language = 'typescript'
): string[] {
  const dir = getPreferredAgentDir(agentId, language);
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
export function getContent(
  agentId: string,
  ruleSuffix?: string,
  language: Language = 'typescript',
  options?: BuildOptions
): string {
  const dir = getPreferredAgentDir(agentId, language);
  const basename = ruleSuffix
    ? `${CONTENT_PREFIX}-${ruleSuffix}.md`
    : CONTENT_MAIN;
  const file = path.join(dir, basename);
  if (!fs.existsSync(file)) {
    throw new Error(`Agent "${agentId}" has no ${basename}`);
  }
  return applyHookGuidance(
    fs.readFileSync(file, 'utf8'),
    agentId,
    language,
    options
  );
}

/**
 * Read agent template file. Tries rule-specific template first (e.g. cursor-frontmatter-mcp.yaml).
 */
export function getTemplate(
  agentId: string,
  filename: string,
  ruleSuffix?: string,
  language: Language = 'typescript'
): string {
  const dir = getPreferredAgentDir(agentId, language);
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

function getSkillFile(skillId: string, relativePath: string): string {
  return path.join(getPreferredSkillDir(skillId), relativePath);
}

export function getSkillContent(skillId: string): string {
  const file = getSkillFile(skillId, 'SKILL.md');
  if (!fs.existsSync(file)) {
    throw new Error(`Skill "${skillId}" missing SKILL.md`);
  }
  return fs.readFileSync(file, 'utf8');
}

function splitSkillDocument(content: string): {
  frontmatter: string | null;
  body: string;
} {
  const match = content.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n?/);
  if (!match || match.index !== 0) {
    return { frontmatter: null, body: content.trimStart() };
  }
  return {
    frontmatter: match[0].trimEnd(),
    body: content.slice(match[0].length).trimStart()
  };
}

function parseSkillMetadata(skillId: string): {
  name: string;
  description: string;
  body: string;
  raw: string;
} {
  const raw = getSkillContent(skillId);
  const { frontmatter, body } = splitSkillDocument(raw);
  if (!frontmatter) {
    throw new Error(`Skill "${skillId}" is missing YAML frontmatter`);
  }
  const metadata = YAML.parse(frontmatter.replace(/^---\r?\n|\r?\n---$/g, ''));
  const name =
    typeof metadata?.name === 'string' && metadata.name.trim()
      ? metadata.name.trim()
      : skillId;
  const description =
    typeof metadata?.description === 'string' && metadata.description.trim()
      ? metadata.description.trim()
      : `Skill ${skillId}`;
  return { name, description, body, raw };
}

function listSkillReferenceFiles(skillId: string): string[] {
  const referencesDir = getSkillFile(skillId, 'references');
  if (!fs.existsSync(referencesDir)) {
    return [];
  }
  const files: string[] = [];
  const walk = (dir: string, prefix: string): void => {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    entries.sort((left, right) => left.name.localeCompare(right.name));
    for (const entry of entries) {
      const relativePath = prefix ? `${prefix}/${entry.name}` : entry.name;
      if (entry.isDirectory()) {
        walk(path.join(dir, entry.name), relativePath);
        continue;
      }
      files.push(relativePath);
    }
  };
  walk(referencesDir, '');
  return files;
}

function crc32(buffer: Buffer): number {
  let crc = 0xffffffff;
  for (const byte of buffer) {
    crc ^= byte;
    for (let index = 0; index < 8; index += 1) {
      const mask = -(crc & 1);
      crc = (crc >>> 1) ^ (0xedb88320 & mask);
    }
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function makeStoredZip(entries: SkillEntry[]): Buffer {
  const localParts: Buffer[] = [];
  const centralParts: Buffer[] = [];
  let offset = 0;

  for (const entry of entries) {
    const name = Buffer.from(entry.name, 'utf8');
    const data = entry.data;
    const checksum = crc32(data);
    const local = Buffer.alloc(30);
    local.writeUInt32LE(0x04034b50, 0);
    local.writeUInt16LE(20, 4);
    local.writeUInt16LE(0, 6);
    local.writeUInt16LE(0, 8);
    local.writeUInt16LE(0, 10);
    local.writeUInt16LE(0, 12);
    local.writeUInt32LE(checksum, 14);
    local.writeUInt32LE(data.length, 18);
    local.writeUInt32LE(data.length, 22);
    local.writeUInt16LE(name.length, 26);
    local.writeUInt16LE(0, 28);
    localParts.push(local, name, data);

    const central = Buffer.alloc(46);
    central.writeUInt32LE(0x02014b50, 0);
    central.writeUInt16LE(20, 4);
    central.writeUInt16LE(20, 6);
    central.writeUInt16LE(0, 8);
    central.writeUInt16LE(0, 10);
    central.writeUInt16LE(0, 12);
    central.writeUInt16LE(0, 14);
    central.writeUInt32LE(checksum, 16);
    central.writeUInt32LE(data.length, 20);
    central.writeUInt32LE(data.length, 24);
    central.writeUInt16LE(name.length, 28);
    central.writeUInt16LE(0, 30);
    central.writeUInt16LE(0, 32);
    central.writeUInt16LE(0, 34);
    central.writeUInt16LE(0, 36);
    central.writeUInt32LE(0, 38);
    central.writeUInt32LE(offset, 42);
    centralParts.push(central, name);

    offset += local.length + name.length + data.length;
  }

  const centralDirectory = Buffer.concat(centralParts);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(0, 4);
  end.writeUInt16LE(0, 6);
  end.writeUInt16LE(entries.length, 8);
  end.writeUInt16LE(entries.length, 10);
  end.writeUInt32LE(centralDirectory.length, 12);
  end.writeUInt32LE(offset, 16);
  end.writeUInt16LE(0, 20);

  return Buffer.concat([...localParts, centralDirectory, end]);
}

export function buildCursorSkillFormat(skillId: string): string {
  const skill = parseSkillMetadata(skillId);
  return [
    '---',
    `description: ${JSON.stringify(skill.description)}`,
    'alwaysApply: false',
    '---',
    '',
    skill.body.trimEnd()
  ].join('\n');
}

export function buildSkillMarkdown(skillId: string): string {
  return parseSkillMetadata(skillId).body.trimEnd() + '\n';
}

export function buildClaudeSkill(skillId: string): Buffer {
  const entries: SkillEntry[] = [
    {
      name: 'SKILL.md',
      data: Buffer.from(getSkillContent(skillId), 'utf8')
    }
  ];
  for (const relativePath of listSkillReferenceFiles(skillId)) {
    const fullPath = getSkillFile(
      skillId,
      path.join('references', relativePath)
    );
    entries.push({
      name: `references/${relativePath.replace(/\\/g, '/')}`,
      data: fs.readFileSync(fullPath)
    });
  }
  return makeStoredZip(entries);
}

/**
 * Build content for Cursor (.mdc = frontmatter + content)
 */
function normalizeFrontmatter(template: string): string {
  const trimmed = template.trim();
  if (trimmed.startsWith('---')) {
    return template;
  }
  return `---\n${trimmed}\n---`;
}

export function buildCursorFormat(
  agentId: string,
  ruleSuffix?: string,
  language: Language = 'typescript',
  options?: BuildOptions
): string {
  const frontmatter = normalizeFrontmatter(
    getTemplate(agentId, 'cursor-frontmatter.yaml', ruleSuffix, language)
  );
  const content = getContent(agentId, ruleSuffix, language, options);
  return frontmatter + '\n' + content;
}

/**
 * Build content for Claude (header + content)
 */
export function buildClaudeFormat(
  agentId: string,
  ruleSuffix?: string,
  language: Language = 'typescript',
  options?: BuildOptions
): string {
  const header = getTemplate(agentId, 'claude-header.md', ruleSuffix, language);
  const content = getContent(agentId, ruleSuffix, language, options);
  return header + content;
}

/**
 * Build content for OpenCode (YAML frontmatter + content)
 */
export function buildOpenCodeFormat(
  agentId: string,
  ruleSuffix?: string,
  language: Language = 'typescript',
  options?: BuildOptions
): string {
  const frontmatter = normalizeFrontmatter(
    getTemplate(agentId, 'opencode-frontmatter.yaml', ruleSuffix, language)
  );
  const content = getContent(agentId, ruleSuffix, language, options);
  return frontmatter + '\n' + content;
}

function getCodexHeader(
  agentId: string,
  ruleSuffix?: string,
  language: Language = 'typescript'
): string {
  let codexError: unknown;
  try {
    return getTemplate(agentId, 'codex-header.md', ruleSuffix, language);
  } catch (err) {
    codexError = err;
  }
  try {
    return getTemplate(agentId, 'claude-header.md', ruleSuffix, language);
  } catch (claudeError) {
    const codexMsg =
      codexError instanceof Error ? codexError.message : String(codexError);
    const claudeMsg =
      claudeError instanceof Error ? claudeError.message : String(claudeError);
    throw new Error(
      `Agent "${agentId}" missing Codex header: tried codex-header.md (${codexMsg}) and fallback claude-header.md (${claudeMsg})`,
      { cause: claudeError }
    );
  }
}

/**
 * Build content for Codex (header + content)
 */
export function buildCodexFormat(
  agentId: string,
  ruleSuffix?: string,
  language: Language = 'typescript',
  options?: BuildOptions
): string {
  const header = getCodexHeader(agentId, ruleSuffix, language);
  const content = getContent(agentId, ruleSuffix, language, options);
  return header + content;
}

export function extractDescriptionFromFrontmatter(
  frontmatter: string
): string | null {
  try {
    // Extract content between --- delimiters to avoid multi-document parse error
    const match = frontmatter.match(/^---\r?\n([\s\S]*?)\r?\n---/);
    const yamlContent = match ? match[1] : frontmatter;
    const parsed = YAML.parse(yamlContent);
    const description = parsed?.description;
    if (typeof description === 'string') {
      const trimmed = description.trim();
      return trimmed || null;
    }
    return null;
  } catch {
    return null;
  }
}

export function getCodexRuleDescription(
  agentId: string,
  ruleSuffix?: string,
  language: Language = 'typescript'
): string | null {
  try {
    const frontmatter = getTemplate(
      agentId,
      'cursor-frontmatter.yaml',
      ruleSuffix,
      language
    );
    return extractDescriptionFromFrontmatter(frontmatter);
  } catch {
    return null;
  }
}

export function getSkillDescription(skillId: string): string {
  return parseSkillMetadata(skillId).description;
}

export function buildCodexAgentsMd(
  agents: string[],
  skills: string[] = [],
  language: Language = 'typescript'
): string {
  const lines: string[] = [];
  lines.push('# AGENTS.md');
  lines.push('');
  lines.push(
    'This file provides guidance to Codex (CLI and app) for working in this repository.'
  );
  lines.push('');
  lines.push('## Installed agent rules');
  lines.push('');
  lines.push(`Created by Ballast v${pkg.version}. Do not edit this section.`);
  lines.push('');
  lines.push(
    'Read and follow these rule files in `.codex/rules/` when they apply:'
  );
  lines.push('');
  for (const agentId of agents) {
    const suffixes = listRuleSuffixes(agentId, language);
    for (const ruleSuffix of suffixes) {
      const basename = getRuleBasename(agentId, language, ruleSuffix);
      const description =
        getCodexRuleDescription(agentId, ruleSuffix, language) ??
        `Rules for ${basename}`;
      lines.push(`- \`.codex/rules/${basename}.md\` — ${description}`);
    }
  }
  if (skills.length > 0) {
    lines.push('');
    lines.push('## Installed skills');
    lines.push('');
    lines.push(`Created by Ballast v${pkg.version}. Do not edit this section.`);
    lines.push('');
    lines.push(
      'Read and use these skill files in `.codex/rules/` when they are relevant:'
    );
    lines.push('');
    for (const skillId of skills) {
      lines.push(
        `- \`.codex/rules/${skillId}.md\` — ${getSkillDescription(skillId)}`
      );
    }
  }
  lines.push('');
  return lines.join('\n');
}

export function buildClaudeMd(
  agents: string[],
  skills: string[] = [],
  language: Language = 'typescript'
): string {
  const lines: string[] = [];
  lines.push('# CLAUDE.md');
  lines.push('');
  lines.push(
    'This file provides guidance to Claude Code for working in this repository.'
  );
  lines.push('');
  lines.push('## Installed agent rules');
  lines.push('');
  lines.push(`Created by Ballast v${pkg.version}. Do not edit this section.`);
  lines.push('');
  lines.push(
    'Read and follow these rule files in `.claude/rules/` when they apply:'
  );
  lines.push('');
  for (const agentId of agents) {
    const suffixes = listRuleSuffixes(agentId, language);
    for (const ruleSuffix of suffixes) {
      const basename = getRuleBasename(agentId, language, ruleSuffix);
      const description =
        getCodexRuleDescription(agentId, ruleSuffix, language) ??
        `Rules for ${basename}`;
      lines.push(`- \`.claude/rules/${basename}.md\` — ${description}`);
    }
  }
  if (skills.length > 0) {
    lines.push('');
    lines.push('## Installed skills');
    lines.push('');
    lines.push(`Created by Ballast v${pkg.version}. Do not edit this section.`);
    lines.push('');
    lines.push(
      'Read and use these skill files in `.claude/skills/` when they are relevant:'
    );
    lines.push('');
    for (const skillId of skills) {
      lines.push(
        `- \`.claude/skills/${skillId}.skill\` — ${getSkillDescription(skillId)}`
      );
    }
  }
  lines.push('');
  return lines.join('\n');
}

/**
 * Build content for the given agent, target, and optional rule suffix
 */
export function buildContent(
  agentId: string,
  target: Target,
  ruleSuffix?: string,
  language: Language = 'typescript',
  options?: BuildOptions
): string {
  switch (target) {
    case 'cursor':
      return buildCursorFormat(agentId, ruleSuffix, language, options);
    case 'claude':
      return buildClaudeFormat(agentId, ruleSuffix, language, options);
    case 'opencode':
      return buildOpenCodeFormat(agentId, ruleSuffix, language, options);
    case 'codex':
      return buildCodexFormat(agentId, ruleSuffix, language, options);
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
  ruleSuffix?: string,
  language: Language = 'typescript'
): { dir: string; file: string } {
  const root = path.resolve(projectRoot);
  const rawBasename = getRuleBasename(agentId, language, ruleSuffix);
  const ruleSubdir = getRuleSubdir();
  const basename = getScopedBasename(ruleSubdir, rawBasename);
  switch (target) {
    case 'cursor': {
      const dir = ruleSubdir
        ? path.join(root, '.cursor', 'rules', ruleSubdir)
        : path.join(root, '.cursor', 'rules');
      const file = path.join(dir, `${basename}.mdc`);
      return { dir, file };
    }
    case 'claude': {
      const dir = ruleSubdir
        ? path.join(root, '.claude', 'rules', ruleSubdir)
        : path.join(root, '.claude', 'rules');
      const file = path.join(dir, `${basename}.md`);
      return { dir, file };
    }
    case 'opencode': {
      const dir = ruleSubdir
        ? path.join(root, '.opencode', ruleSubdir)
        : path.join(root, '.opencode');
      const file = path.join(dir, `${basename}.md`);
      return { dir, file };
    }
    case 'codex': {
      const dir = ruleSubdir
        ? path.join(root, '.codex', 'rules', ruleSubdir)
        : path.join(root, '.codex', 'rules');
      const file = path.join(dir, `${basename}.md`);
      return { dir, file };
    }
    default:
      throw new Error(`Unknown target: ${target}`);
  }
}

export function getSkillDestination(
  skillId: string,
  target: Target,
  projectRoot: string
): { dir: string; file: string } {
  const root = path.resolve(projectRoot);
  switch (target) {
    case 'cursor': {
      const dir = path.join(root, '.cursor', 'rules');
      return { dir, file: path.join(dir, `${skillId}.mdc`) };
    }
    case 'claude': {
      const dir = path.join(root, '.claude', 'skills');
      return { dir, file: path.join(dir, `${skillId}.skill`) };
    }
    case 'opencode': {
      const dir = path.join(root, '.opencode', 'skills');
      return { dir, file: path.join(dir, `${skillId}.md`) };
    }
    case 'codex': {
      const dir = path.join(root, '.codex', 'rules');
      return { dir, file: path.join(dir, `${skillId}.md`) };
    }
    default:
      throw new Error(`Unknown target: ${target}`);
  }
}

/**
 * Get destination for Codex AGENTS.md
 */
export function getCodexAgentsMdPath(projectRoot: string): string {
  return path.join(path.resolve(projectRoot), 'AGENTS.md');
}

export function getClaudeMdPath(projectRoot: string): string {
  return path.join(path.resolve(projectRoot), 'CLAUDE.md');
}

/**
 * List supported targets
 */
export function listTargets(): string[] {
  return TARGETS.slice();
}

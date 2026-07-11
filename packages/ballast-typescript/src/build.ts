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

const TARGETS: Target[] = ['cursor', 'claude', 'opencode', 'codex', 'gemini'];
const REPO_ROOT = path.resolve(__dirname, '..', '..', '..');
const SOURCE_AGENTS_ROOT = path.join(REPO_ROOT, 'agents');
const GIT_HOOKS_GUIDANCE_TOKEN = '{{BALLAST_GIT_HOOKS_GUIDANCE}}';
const GIT_HOOKS_PRE_COMMIT_GLOB_TOKEN = '{{BALLAST_GIT_HOOKS_PRE_COMMIT_GLOB}}';
const DEPLOYMENT_MODEL_GUIDANCE_TOKEN = '{{BALLAST_DEPLOYMENT_MODEL_GUIDANCE}}';
const BALLAST_REPO_URL = 'https://github.com/everydaydevopsio/ballast';
const BALLAST_MANAGED_COMMENT = `<!-- Created by [Ballast](${BALLAST_REPO_URL}) v${pkg.version}. Do not edit this section. -->`;

type HookMode = 'pre-commit' | 'husky';

interface BuildOptions {
  hookMode?: HookMode;
  variables?: Record<string, string>;
}

interface SkillEntry {
  name: string;
  data: Buffer;
}

function getCreatedByBallastLine(): string {
  return `Created by [Ballast](${BALLAST_REPO_URL}) v${pkg.version}. Do not edit this section.`;
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
  return 'pre-commit';
}

function renderGitHooksGuidance(
  language: Language,
  options?: BuildOptions
): string {
  const hookMode = getHookMode('git-hooks', language, options);
  if (language === 'typescript') {
    if (hookMode === 'husky') {
      return [
        'Use Husky for TypeScript-only repositories.',
        '',
        '- Install and initialize Husky.',
        "- Create `.husky/pre-commit` with the repo's fast lint command, such as `npx lint-staged`, and prefer the repo formatter or linter when it already exists.",
        '- Include fast formatting checks for both `.yaml` and `.yml` files in the lint-staged, repo formatter, or repo linter configuration.',
        "- Create `.husky/pre-push` with the detected or canonical package-manager test command, and run the repo's required build or typecheck command before tests when that is the repo convention.",
        '- Keep the hook file executable with `chmod +x .husky/pre-commit`.',
        '- Keep `.husky/pre-push` executable with `chmod +x .husky/pre-push`.',
        "- Keep the hook in sync with the repo's linting workflow whenever the command changes."
      ].join('\n');
    }

    return [
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

  if (language === 'ansible') {
    return [
      'Use `pre-commit` for Ansible repositories.',
      '',
      '- Create or update `.pre-commit-config.yaml` at the repo root.',
      '- Install hooks with `pre-commit install`.',
      '- Install the pre-push hook with `pre-commit install --hook-type pre-push`.',
      '- Run `ansible-lint`, `yamllint`, and `ansible-playbook --syntax-check` from the hook configuration.',
      '- Keep secrets out of logs and commits; prefer Ansible Vault or external secret stores.',
      '- Keep the configuration current with `pre-commit autoupdate`.'
    ].join('\n');
  }

  if (language === 'terraform') {
    return [
      'Use `pre-commit` for Terraform repositories.',
      '',
      '- Create or update `.pre-commit-config.yaml` at the repo root.',
      '- Commit `.terraform-version` and use `tfenv install` plus `tfenv use` before running Terraform commands.',
      '- Install hooks with `pre-commit install`.',
      '- Install the pre-push hook with `pre-commit install --hook-type pre-push`.',
      '- Run `terraform fmt -check -recursive`, `terraform init -backend=false`, `terraform validate`, `tflint --init`, `tflint --recursive`, and `trivy config .` from the hook configuration; keep `tfsec` only for legacy-compatible pipelines.',
      '- Keep `.terraform/`, state files, and plan files out of Git.',
      '- Keep the configuration current with `pre-commit autoupdate`.'
    ].join('\n');
  }

  return '';
}

function renderGitHooksPreCommitGlob(
  agentId: string,
  language: Language,
  options?: BuildOptions
): string {
  if (agentId !== 'git-hooks') {
    return '';
  }
  if (
    language === 'typescript' &&
    getHookMode(agentId, language, options) === 'husky'
  ) {
    return '';
  }
  return "  - '.pre-commit-config.yaml'";
}

function applyHookTemplateVariables(
  content: string,
  agentId: string,
  language: Language,
  options?: BuildOptions
): string {
  if (!content.includes(GIT_HOOKS_PRE_COMMIT_GLOB_TOKEN)) {
    return content;
  }
  return content.replaceAll(
    GIT_HOOKS_PRE_COMMIT_GLOB_TOKEN,
    renderGitHooksPreCommitGlob(agentId, language, options)
  );
}

function applyHookGuidance(
  content: string,
  agentId: string,
  language: Language,
  options?: BuildOptions
): string {
  if (agentId !== 'git-hooks' || !content.includes(GIT_HOOKS_GUIDANCE_TOKEN)) {
    return content;
  }
  return content.replace(
    GIT_HOOKS_GUIDANCE_TOKEN,
    renderGitHooksGuidance(language, options)
  );
}

function renderDeploymentModelGuidance(options?: BuildOptions): string {
  const deploymentModel = options?.variables?.deploymentModel ?? 'none';
  switch (deploymentModel) {
    case 'kubernetes':
      return [
        'Kubernetes: local Helm chart + external ArgoCD GitOps.',
        '',
        '- Application repository ownership:',
        '  - keep the Helm chart in `charts/<app>/` in the application repository',
        '  - keep reusable chart defaults in `charts/<app>/values.yaml`',
        '  - keep chart templates, probes, service, ingress, and workload manifests with the app code they deploy',
        '  - publish container images to GHCR or Docker Hub and capture the immutable image digest',
        '- GitOps repository ownership:',
        '  - keep ArgoCD `Application` or `ApplicationSet` configuration in a separate GitOps repository',
        '  - keep environment-specific ArgoCD sources, destinations, sync policy, and promotion rules there',
        '  - keep environment-specific values files in the GitOps repo when environments differ by cluster, namespace, domain, secret reference, or scaling policy',
        '- CI/CD flow:',
        '  - build, test, and publish the app image from the application repository',
        '  - update `charts/<app>/` in the app repo only when chart templates or defaults change',
        '  - update the GitOps repository when an environment should point at a new image tag or digest',
        '  - prefer digest pinning for production deployments and include the image tag for human traceability',
        '  - use a fine-grained token or GitHub App credential scoped only to the GitOps repository',
        '- Do not move the Helm chart to the GitOps repo just to update image references. Keep chart ownership with the app and environment ownership with GitOps.'
      ].join('\n');
    case 'serverless':
      return [
        'Serverless deployment model for managed function or container platforms such as AWS Lambda, Cloud Run, Azure Functions, or equivalent services.',
        '',
        '- Keep infrastructure definitions or platform manifests close to the service unless the team has a dedicated infra repository.',
        '- Build immutable artifacts before deployment and promote the same artifact between preview, staging, and production when the platform supports it.',
        '- Use least-privilege OIDC or scoped deploy credentials; do not store long-lived cloud keys in the repository.',
        '- Keep environment variables and secrets in the platform secret manager, not in generated workflow files.',
        '- Include smoke checks after deploy that hit a health endpoint, function URL, or representative invocation.',
        '- Document rollback as reverting the deployed version, alias, revision, or traffic split.'
      ].join('\n');
    case 'server':
      return [
        'Server deployment model for self-managed VM, VPS, or bare-metal deployments.',
        '',
        '- Build a versioned artifact or container image in CI; do not build production artifacts manually on the server.',
        '- Deploy through a repeatable script or workflow that transfers the artifact, updates configuration, restarts the service manager, and verifies health.',
        '- Use `systemd`, Docker Compose, Nomad, or the existing service manager consistently and document the owner.',
        '- Keep secrets outside the repo in the server secret store, environment manager, or deployment platform.',
        '- Include health checks and rollback steps for the previous artifact or image digest.',
        '- Avoid SSH commands that mutate production without logging the artifact version and result.'
      ].join('\n');
    case 'hosted':
      return [
        'Hosted app platform deployment model for services such as Vercel, Netlify, Render, Railway, Fly.io, or similar app platforms.',
        '',
        '- Keep platform configuration in the app repo when the platform supports checked-in config files.',
        '- Keep environment variables and secrets in the hosted platform, not in generated workflows.',
        '- Use preview deployments for pull requests when the platform supports them.',
        '- Promote to production from protected branches, release tags, or explicit platform promotion controls.',
        '- Run smoke checks against the deployed preview or production URL before marking deployment complete.',
        '- Document platform ownership, project name, production URL, and rollback procedure.'
      ].join('\n');
    case 'none':
    default:
      return 'No app deployment model is configured. Keep library, SDK, and CLI publishing guidance active, but do not assume Kubernetes, serverless, hosted-platform, or self-managed server deployment ownership until the repository sets `deploymentModel`.';
  }
}

function applyDeploymentModelGuidance(
  content: string,
  agentId: string,
  options?: BuildOptions
): string {
  if (
    agentId !== 'publishing' ||
    !content.includes(DEPLOYMENT_MODEL_GUIDANCE_TOKEN)
  ) {
    return content;
  }
  return content.replaceAll(
    DEPLOYMENT_MODEL_GUIDANCE_TOKEN,
    renderDeploymentModelGuidance(options)
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
  let raw = fs.readFileSync(file, 'utf8');
  if (options?.variables) {
    for (const [key, value] of Object.entries(options.variables)) {
      raw = raw.replaceAll(`{{${key}}}`, value);
    }
  }
  return applyDeploymentModelGuidance(
    applyHookGuidance(raw, agentId, language, options),
    agentId,
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

/**
 * Return the parsed claude-settings.json for a skill, or null if none exists.
 * Used by the install step to merge skill-specific permissions into .claude/settings.json.
 */
export function getSkillClaudeSettings(
  skillId: string
): Record<string, unknown> | null {
  const file = getSkillFile(skillId, 'claude-settings.json');
  if (!fs.existsSync(file)) return null;
  // Let parse errors propagate so the installer can report them in errors[].
  return JSON.parse(fs.readFileSync(file, 'utf8')) as Record<string, unknown>;
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
    BALLAST_MANAGED_COMMENT,
    '',
    skill.body.trimEnd()
  ].join('\n');
}

export function buildSkillMarkdown(skillId: string): string {
  return (
    [
      BALLAST_MANAGED_COMMENT,
      '',
      parseSkillMetadata(skillId).body.trimEnd()
    ].join('\n') + '\n'
  );
}

export function buildClaudeSkill(
  skillId: string,
  skillContent?: string
): Buffer {
  const entries: SkillEntry[] = [
    {
      name: 'SKILL.md',
      data: Buffer.from(skillContent ?? getSkillContent(skillId), 'utf8')
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
    applyHookTemplateVariables(
      getTemplate(agentId, 'cursor-frontmatter.yaml', ruleSuffix, language),
      agentId,
      language,
      options
    )
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

function renderGeminiMandates(): string {
  return [
    '## Gemini Mandates',
    '',
    '### Narrative Flow',
    'Always use the `update_topic` tool at the beginning of a task and when transitioning between major strategic phases. Provide a concise `title` and a detailed `summary` (5-10 sentences) that recaps completed work and outlines the immediate strategic intent.',
    '',
    '### Context Efficiency',
    '- **Surgical Reads:** Use `start_line` and `end_line` in `read_file` to minimize context usage.',
    '- **Parallelism:** Execute independent searches and reads in parallel whenever possible.',
    '- **Topic Search:** Use `grep_search` to identify points of interest before reading entire files.',
    '',
    '### Strategic Orchestration',
    'Delegate complex, repetitive, or high-volume tasks to specialized sub-agents (`codebase_investigator`, `generalist`) to keep the main session history lean and efficient.',
    '',
    ''
  ].join('\n');
}

function findGeminiHeader(
  agentId: string,
  ruleSuffix?: string,
  language: Language = 'typescript'
): string {
  try {
    return getTemplate(agentId, 'gemini-header.md', ruleSuffix, language);
  } catch (geminiError) {
    try {
      return getTemplate(agentId, 'claude-header.md', ruleSuffix, language);
    } catch (claudeError) {
      try {
        return getTemplate(agentId, 'codex-header.md', ruleSuffix, language);
      } catch (codexError) {
        const geminiMsg =
          geminiError instanceof Error
            ? geminiError.message
            : String(geminiError);
        const claudeMsg =
          claudeError instanceof Error
            ? claudeError.message
            : String(claudeError);
        const codexMsg =
          codexError instanceof Error ? codexError.message : String(codexError);
        throw new Error(
          `Agent "${agentId}" missing Gemini header: tried gemini-header.md (${geminiMsg}), fallback claude-header.md (${claudeMsg}), and fallback codex-header.md (${codexMsg})`,
          { cause: codexError }
        );
      }
    }
  }
}

function getGeminiHeader(
  agentId: string,
  ruleSuffix?: string,
  language: Language = 'typescript'
): string {
  const header = findGeminiHeader(agentId, ruleSuffix, language);
  return header + '\n---\n\n' + renderGeminiMandates();
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

function getRepositoryFactsSection(): string[] {
  const fromEnv = loadRepositoryFactsSection();
  if (fromEnv) return fromEnv;
  return [
    '## Repository Facts',
    '',
    'Use this section for durable repo-specific facts that agents repeatedly need. Prefer facts stored here over re-deriving them with shell commands on every task.',
    '',
    'Keep only stable, reviewable metadata here. Do not store secrets, credentials, or ephemeral runtime state.',
    '',
    'Suggested facts to record:',
    '',
    '- Canonical GitHub repo: `<OWNER/REPO>`',
    '- Default branch: `<main>`',
    '- Primary package manager: `<pnpm | npm | yarn | uv | go>`',
    '- Version-file locations agents should check first: `<.nvmrc, packageManager, pyproject.toml, go.mod, etc.>`',
    '- Canonical config files: `<paths agents should read before falling back to discovery>`',
    '- Primary CI workflows: `<workflow filenames>`',
    '- Primary release/publish workflows: `<workflow filenames>`',
    '- Preferred build/test/lint/format/coverage commands: `<commands>`',
    '- Coverage threshold: `<value>`',
    '- Generated or protected paths agents should avoid editing directly: `<paths>`',
    '',
    'Update this section when those facts change. If live runtime state is required, discover it separately instead of treating it as a durable repo fact.'
  ];
}

function loadRepositoryFactsSection(): string[] | null {
  const path = process.env.BALLAST_REPOSITORY_FACTS_FILE?.trim();
  if (!path) return null;
  try {
    const parsed = JSON.parse(fs.readFileSync(path, 'utf8')) as {
      repositoryFactsSection?: unknown;
    };
    if (!Array.isArray(parsed.repositoryFactsSection)) return null;
    if (
      !parsed.repositoryFactsSection.every(
        (line): line is string => typeof line === 'string'
      )
    ) {
      return null;
    }
    return parsed.repositoryFactsSection.length > 0
      ? parsed.repositoryFactsSection
      : null;
  } catch {
    return null;
  }
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
    'This file provides shared repository guidance for agent tools that read AGENTS.md.'
  );
  lines.push('');
  lines.push(...getRepositoryFactsSection());
  lines.push('');
  lines.push('## Installed agent rules');
  lines.push('');
  lines.push(getCreatedByBallastLine());
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
    lines.push(getCreatedByBallastLine());
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
  lines.push(...getRepositoryFactsSection());
  lines.push('');
  lines.push('## Installed agent rules');
  lines.push('');
  lines.push(getCreatedByBallastLine());
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
    lines.push(getCreatedByBallastLine());
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
 * Build content for Gemini (header + content)
 */
export function buildGeminiFormat(
  agentId: string,
  ruleSuffix?: string,
  language: Language = 'typescript',
  options?: BuildOptions
): string {
  const header = getGeminiHeader(agentId, ruleSuffix, language);
  const content = getContent(agentId, ruleSuffix, language, options);
  return header + content;
}

export function buildGeminiMd(
  agents: string[],
  skills: string[] = [],
  language: Language = 'typescript'
): string {
  const lines: string[] = [];
  lines.push('# GEMINI.md');
  lines.push('');
  lines.push(
    'This file provides guidance to Gemini CLI for working in this repository.'
  );
  lines.push('');
  lines.push(...getRepositoryFactsSection());
  lines.push('');
  lines.push('## Memory Tiering');
  lines.push('');
  lines.push(
    'Follow these routing rules for persisting long-lived facts and preferences:'
  );
  lines.push('');
  lines.push(
    '- **Team-shared (Repository)**: Use this `GEMINI.md` file for architecture, workflows, and repo-wide rules.'
  );
  lines.push(
    '- **Private (Local Setup)**: Use the private project memory (`MEMORY.md` in the ballast memory folder) for local machine notes or private workflows.'
  );
  lines.push(
    '- **Global (Personal)**: Use the global personal memory (`~/.gemini/GEMINI.md`) for cross-project personal coding preferences.'
  );
  lines.push('');
  lines.push('---');
  lines.push('');
  lines.push('## Installed agent rules');
  lines.push('');
  lines.push(getCreatedByBallastLine());
  lines.push('');
  lines.push(
    'Read and follow these rule files in `.gemini/rules/` when they apply:'
  );
  lines.push('');
  for (const agentId of agents) {
    const suffixes = listRuleSuffixes(agentId, language);
    for (const ruleSuffix of suffixes) {
      const basename = getRuleBasename(agentId, language, ruleSuffix);
      const description =
        getCodexRuleDescription(agentId, ruleSuffix, language) ??
        `Rules for ${basename}`;
      lines.push(`- \`.gemini/rules/${basename}.md\` — ${description}`);
    }
  }
  if (skills.length > 0) {
    lines.push('');
    lines.push('## Installed skills');
    lines.push('');
    lines.push(getCreatedByBallastLine());
    lines.push('');
    lines.push(
      'Read and use these skill files in `.gemini/rules/` when they are relevant:'
    );
    lines.push('');
    for (const skillId of skills) {
      lines.push(
        `- \`.gemini/rules/${skillId}.md\` — ${getSkillDescription(skillId)}`
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
  let result: string;
  switch (target) {
    case 'cursor':
      result = buildCursorFormat(agentId, ruleSuffix, language, options);
      break;
    case 'claude':
      result = buildClaudeFormat(agentId, ruleSuffix, language, options);
      break;
    case 'gemini':
      result = buildGeminiFormat(agentId, ruleSuffix, language, options);
      break;
    case 'opencode':
      result = buildOpenCodeFormat(agentId, ruleSuffix, language, options);
      break;
    case 'codex':
      result = buildCodexFormat(agentId, ruleSuffix, language, options);
      break;
    default:
      throw new Error(`Unknown target: ${target}`);
  }
  if (options?.variables) {
    for (const [key, value] of Object.entries(options.variables)) {
      result = result.replaceAll(`{{${key}}}`, value);
    }
  }
  return result;
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
    case 'gemini': {
      const dir = ruleSubdir
        ? path.join(root, '.gemini', 'rules', ruleSubdir)
        : path.join(root, '.gemini', 'rules');
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
    case 'gemini': {
      const dir = path.join(root, '.gemini', 'rules');
      return { dir, file: path.join(dir, `${skillId}.md`) };
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

export function getGeminiMdPath(projectRoot: string): string {
  return path.join(path.resolve(projectRoot), 'GEMINI.md');
}

/**
 * List supported targets
 */
export function listTargets(): string[] {
  return TARGETS.slice();
}

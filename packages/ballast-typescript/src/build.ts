import fs from 'fs';
import crypto from 'crypto';
import path from 'path';
import YAML from 'yaml';
import {
  COMMON_AGENT_IDS,
  COMMON_SKILL_IDS,
  getAgentDir,
  getAgentsContentRoot,
  getSkillDir,
  SKILL_IDS
} from './agents';
import { OPT_IN_PUBLISHING_PROFILES } from './config';
import type { PublishingProfile, Target } from './config';
import type { Language } from './agents';
import pkg from '../package.json';

const TARGETS: Target[] = ['cursor', 'claude', 'opencode', 'codex', 'gemini'];
const REPO_ROOT = path.resolve(__dirname, '..', '..', '..');
const SOURCE_AGENTS_ROOT = path.join(REPO_ROOT, 'agents');
const GIT_HOOKS_GUIDANCE_TOKEN = '{{BALLAST_GIT_HOOKS_GUIDANCE}}';
const GIT_HOOKS_PRE_COMMIT_GLOB_TOKEN = '{{BALLAST_GIT_HOOKS_PRE_COMMIT_GLOB}}';
const DEPLOYMENT_MODEL_GUIDANCE_TOKEN = '{{BALLAST_DEPLOYMENT_MODEL_GUIDANCE}}';
const TASK_SYSTEM_GUIDANCE_TOKEN = '{{BALLAST_TASK_SYSTEM_GUIDANCE}}';
const BALLAST_REPO_URL = 'https://github.com/everydaydevopsio/ballast';
const BALLAST_MANAGED_COMMENT = `<!-- Created by [Ballast](${BALLAST_REPO_URL}) v${pkg.version}. Do not edit this section. -->`;

type HookMode = 'pre-commit' | 'husky';

interface BuildOptions {
  hookMode?: HookMode;
  tools?: Record<string, string[]>;
  variables?: Record<string, string>;
}

export interface RuleMarker {
  ruleId: string;
  version: string;
  checksum: string;
}

interface SkillEntry {
  name: string;
  data: Buffer;
}

const RULE_MARKER_PATTERN =
  /^<!-- ballast:rule\s+id="([^"]+)"\s+version="([^"]+)"\s+checksum="([a-fA-F0-9]+)"\s*-->\r?\n?/;
const FRONTMATTER_PATTERN = /^---\r?\n[\s\S]*?\r?\n---\r?\n?/;

function getCreatedByBallastLine(): string {
  return `Created by [Ballast](${BALLAST_REPO_URL}) v${pkg.version}. Do not edit this section.`;
}

function renderRepositoryToolPolicy(tools?: Record<string, string[]>): string {
  const entries = Object.entries(tools ?? {})
    .map(
      ([language, values]) =>
        [
          language.trim().toLowerCase(),
          values.map((value) => value.trim().toLowerCase()).filter(Boolean)
        ] as const
    )
    .filter(([language, values]) => language.length > 0 && values.length > 0)
    .sort(([left], [right]) => left.localeCompare(right));

  if (entries.length === 0) return '';

  const lines = [
    '## Repository Tool Policy',
    '',
    '- Check `.rulesrc.json` `tools` before adding, installing, or running language tooling.',
    `- Configured tools: ${entries
      .map(([language, values]) => `${language}=${values.join(',')}`)
      .join('; ')}.`
  ];

  const pythonTools =
    entries.find(([language]) => language === 'python')?.[1] ?? [];
  if (pythonTools.includes('uv')) {
    lines.push(
      '- For Python commands, prefer `uv run <command>` and `uv add ...` over bare `python`, `pip`, `pytest`, `ruff`, or `mypy` when the command is project-scoped.'
    );
  }

  const typescriptTools =
    entries.find(([language]) => language === 'typescript')?.[1] ?? [];
  if (typescriptTools.includes('pnpm')) {
    lines.push(
      '- For TypeScript commands, prefer `pnpm`/`pnpm exec` over `npm`/`npx` when the command is project-scoped.'
    );
  }

  lines.push('');
  return `${lines.join('\n')}\n`;
}

/**
 * Render the tool policy as manifest lines nested under the managed
 * "Installed agent rules" section (### heading so section patching keeps
 * treating it as part of that section).
 */
function renderRepositoryToolPolicyManifestLines(
  tools?: Record<string, string[]>
): string[] {
  const policy = renderRepositoryToolPolicy(tools);
  if (!policy) return [];
  const lines = policy.trimEnd().split('\n');
  lines[0] = '### Repository Tool Policy';
  return [...lines, ''];
}

function insertRepositoryToolPolicy(
  content: string,
  options?: BuildOptions
): string {
  const policy = renderRepositoryToolPolicy(options?.tools);
  if (!policy) return content;
  if (content.includes('## Repository Tool Policy')) return content;

  const frontmatter = content.match(FRONTMATTER_PATTERN);
  const prefix = frontmatter ? frontmatter[0] : '';
  const body = frontmatter ? content.slice(frontmatter[0].length) : content;
  const insertAt = body.indexOf('\n## ');
  if (insertAt === -1) {
    return `${prefix}${policy}${body}`;
  }
  return `${prefix}${body.slice(0, insertAt)}\n\n${policy}${body.slice(insertAt + 1)}`;
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

export function shouldEmitRuleForSubdir(
  agentId: string,
  ruleSubdir: string | null = getRuleSubdir()
): boolean {
  if (!ruleSubdir) {
    return true;
  }
  const isCommonAgent = (COMMON_AGENT_IDS as readonly string[]).includes(
    agentId
  );
  if (ruleSubdir === 'common') {
    return isCommonAgent;
  }
  return !isCommonAgent;
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

export function getRuleMarkerId(
  agentId: string,
  language: Language,
  ruleSuffix?: string
): string {
  return [language, agentId, ruleSuffix].filter(Boolean).join('/');
}

export function parseRuleMarker(content: string): RuleMarker | null {
  const frontmatter = content.match(FRONTMATTER_PATTERN);
  const markerContent = frontmatter
    ? content.slice(frontmatter[0].length)
    : content;
  const match = markerContent.match(RULE_MARKER_PATTERN);
  if (!match) return null;
  return {
    ruleId: match[1],
    version: match[2],
    checksum: match[3].toLowerCase()
  };
}

export function stripRuleMarker(content: string): string {
  const frontmatter = content.match(FRONTMATTER_PATTERN);
  if (!frontmatter) {
    return content.replace(RULE_MARKER_PATTERN, '');
  }
  const body = content.slice(frontmatter[0].length);
  return `${frontmatter[0]}${body.replace(RULE_MARKER_PATTERN, '')}`;
}

export function calculateRuleChecksum(content: string): string {
  return crypto
    .createHash('sha256')
    .update(stripRuleMarker(content), 'utf8')
    .digest('hex');
}

export function verifyRuleChecksum(content: string): boolean {
  const marker = parseRuleMarker(content);
  if (!marker) return false;
  return calculateRuleChecksum(content) === marker.checksum;
}

function addRuleMarker(content: string, ruleId: string): string {
  const body = stripRuleMarker(content);
  const marker = `<!-- ballast:rule id="${ruleId}" version="${pkg.version}" checksum="${calculateRuleChecksum(body)}" -->`;
  const frontmatter = body.match(FRONTMATTER_PATTERN);
  if (frontmatter) {
    return `${frontmatter[0]}${marker}\n${body.slice(frontmatter[0].length)}`;
  }
  return `${marker}\n${body}`;
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
  const gitleaksHookGuidance =
    '- Add the official `gitleaks` pre-commit hook in `.pre-commit-config.yaml` for secret detection; do not generate or call a repo-local no-secrets shell script.';
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
      gitleaksHookGuidance,
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
      gitleaksHookGuidance,
      '- Keep Bandit and `pip-audit` in CI or explicit security-review workflows unless this repository opts into running them from hooks.',
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
      gitleaksHookGuidance,
      '- Keep `govulncheck`, fuzzing, and `go test -race` in CI, pre-push, or explicit security-review workflows unless this repository opts into running them at commit time.',
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
      '- Run `ansible-lint` and `yamllint` from the pre-commit stage.',
      '- Run `ansible-playbook --syntax-check` for representative top-level playbooks from the pre-push stage.',
      gitleaksHookGuidance,
      '- Prefer `ansible-lint --profile=safety` in CI or explicit security-review workflows when the repository is ready for safety-oriented rules.',
      '- Keep secrets out of logs and commits; prefer Ansible Vault or external secret stores.',
      '- Keep the configuration current with `pre-commit autoupdate`; rerun `pre-commit run --all-files` after hook changes.'
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
      gitleaksHookGuidance,
      '- Keep deeper IaC static analysis, policy checks, and cloud/runtime posture scanning in CI or operational review workflows.',
      '- Keep the configuration current with `pre-commit autoupdate`.'
    ].join('\n');
  }

  if (language === 'dart') {
    return [
      'Use `pre-commit` for Dart and Flutter repositories.',
      '',
      '- Create or update `.pre-commit-config.yaml` at the Flutter app root or monorepo root.',
      '- Install hooks with `pre-commit install`.',
      '- Install the pre-push hook with `pre-commit install --hook-type pre-push`.',
      '- Run `dart format --set-exit-if-changed .` and `flutter analyze` on `pre-commit`.',
      '- Run `flutter test` on `pre-push`; keep `flutter test integration_test` in CI or device-backed jobs when emulators are required.',
      '- Keep `.dart_tool/`, `build/`, and platform build output out of Git.',
      gitleaksHookGuidance,
      '- Keep the configuration current with `pre-commit autoupdate`.'
    ].join('\n');
  }

  if (language === 'docker') {
    return [
      'Use `pre-commit` for Dockerfile and container configuration repositories.',
      '',
      '- Create or update `.pre-commit-config.yaml` at the repo root.',
      '- Install hooks with `pre-commit install`.',
      '- Install the pre-push hook with `pre-commit install --hook-type pre-push`.',
      '- Run `hadolint` for Dockerfiles and `docker compose config` for Compose files from the pre-commit stage.',
      '- Run image build and smoke checks from the pre-push stage only when they are deterministic and do not require registry credentials.',
      gitleaksHookGuidance,
      '- Keep image vulnerability scans in CI or pre-push when local Docker availability is reliable.',
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
        'Deployment guidance is active (`deploymentModel: kubernetes`). Apply web/API deployment workflow guidance for repositories that own this deployment model.',
        '',
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
        'Deployment guidance is active (`deploymentModel: serverless`). Apply web/API deployment workflow guidance for repositories that own this deployment model.',
        '',
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
        'Deployment guidance is active (`deploymentModel: server`). Apply web/API deployment workflow guidance for repositories that own this deployment model.',
        '',
        'Server deployment model for self-managed VM, VPS, or bare-metal deployments.',
        '',
        '- Build a versioned artifact or container image in CI; do not build production artifacts manually on the server.',
        '- Deploy through a repeatable script or workflow that transfers the artifact, updates configuration, restarts the service manager, and verifies health.',
        '- Use `systemd`, Docker Compose, Nomad, or the existing service manager consistently and document the owner.',
        '- Keep secrets outside the repo in the server secret store, environment manager, or deployment platform.',
        '- Include health checks and rollback steps for the previous artifact or image digest.',
        '- Avoid SSH commands that mutate production without logging the artifact version and result.'
      ].join('\n');
    case 'docker':
      return [
        'Deployment guidance is active (`deploymentModel: docker`). Apply container image publishing and runtime handoff guidance for repositories that own Docker images.',
        '',
        'Docker deployment model for repositories whose primary deployable artifact is a Docker or OCI image.',
        '',
        '- Build images in CI from the release tag or protected branch, not manually on a server.',
        '- Publish immutable tags and capture the image digest for deployment handoff.',
        '- Support GHCR and Docker Hub explicitly; choose public or private visibility based on repository policy and audience.',
        '- Keep registry credentials scoped to the publish job. Prefer `GITHUB_TOKEN` packages permissions for GHCR and repository secrets for Docker Hub tokens.',
        '- Run Dockerfile linting, image build smoke tests, and an image vulnerability scan before publishing.',
        '- Do not assume systemd, SSH, Kubernetes, hosted-platform, or serverless rollout ownership unless another deployment model or repo docs explicitly add that layer.'
      ].join('\n');
    case 'hosted':
      return [
        'Deployment guidance is active (`deploymentModel: hosted`). Apply web/API deployment workflow guidance for repositories that own this deployment model.',
        '',
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
      return 'No app deployment model is configured (`deploymentModel: none`). Deployment guidance is reference-only. Deployment is inactive: keep library, SDK, CLI, and optional container publishing guidance active, but do not create deploy-on-main workflows, deployment-state updates, Kubernetes, serverless, hosted-platform, Docker registry, or self-managed server deployment ownership until the repository sets an active `deploymentModel`.';
  }
}

const CONDITIONAL_TOKEN_PATTERNS: Record<string, RegExp> = {
  TASK_SYSTEM:
    /\{\{BALLAST_IF_TASK_SYSTEM:([a-z, -]+)\}\}\r?\n?([\s\S]*?)\{\{BALLAST_END_IF_TASK_SYSTEM\}\}\r?\n?/g,
  TARGET:
    /\{\{BALLAST_IF_TARGET:([a-z, -]+)\}\}\r?\n?([\s\S]*?)\{\{BALLAST_END_IF_TARGET\}\}\r?\n?/g
};

/**
 * Strip {{BALLAST_IF_<kind>:<names>}}...{{BALLAST_END_IF_<kind>}} blocks whose
 * comma-separated name list does not satisfy the matcher.
 */
function applyConditionalTokenBlocks(
  content: string,
  kind: keyof typeof CONDITIONAL_TOKEN_PATTERNS,
  matches: (name: string) => boolean
): string {
  if (!content.includes(`{{BALLAST_IF_${kind}:`)) {
    return content;
  }
  return content.replace(
    CONDITIONAL_TOKEN_PATTERNS[kind],
    (_match, names: string, inner: string) =>
      names
        .split(',')
        .map((name) => name.trim())
        .filter(Boolean)
        .some(matches)
        ? inner
        : ''
  );
}

const DEPLOYMENT_CONDITIONAL_PATTERN =
  /\{\{BALLAST_IF_DEPLOYMENT:([a-z, -]+)\}\}\r?\n?([\s\S]*?)\{\{BALLAST_END_IF_DEPLOYMENT\}\}\r?\n?/g;

/**
 * Strip {{BALLAST_IF_DEPLOYMENT:<models>}}...{{BALLAST_END_IF_DEPLOYMENT}}
 * blocks whose model list does not match the configured deployment model.
 * The special name `active` matches any model except `none`.
 */
function applyDeploymentConditionalBlocks(
  content: string,
  deploymentModel: string
): string {
  if (!content.includes('{{BALLAST_IF_DEPLOYMENT:')) {
    return content;
  }
  return content.replace(
    DEPLOYMENT_CONDITIONAL_PATTERN,
    (_match, models: string, inner: string) => {
      const names = models
        .split(',')
        .map((name) => name.trim())
        .filter(Boolean);
      const keep = names.some(
        (name) =>
          name === deploymentModel ||
          (name === 'active' && deploymentModel !== 'none')
      );
      return keep ? inner : '';
    }
  );
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

function renderTaskSystemGuidance(options?: BuildOptions): string {
  const taskSystem = options?.variables?.taskSystem ?? '{{taskSystem}}';
  const taskSystemName = renderTaskSystemDisplayName(taskSystem);
  if (taskSystem === 'none') {
    return [
      '## Activation',
      '',
      'External issue tracking is disabled (`taskSystem: none`). This repository has no external task system configured. Do not require GitHub Issues, Jira, Linear, or MCP-backed ticket creation for routine branch work.',
      '',
      'Use `tasks/todo.md` as the structured branch-local task artifact. If work must survive beyond the current branch, ask the user where they want durable follow-up tracked before creating external issues or tickets.',
      '',
      '## MCP Server Setup',
      '',
      'No task-system MCP server is required while `taskSystem` is `none`. Configure GitHub Issues, Jira, or Linear MCP only after the repository changes its saved `taskSystem` value or the user explicitly asks for that integration.',
      '',
      '## Using Work Items',
      '',
      '- Do not create external issues or tickets by default.',
      '- When preparing a PR, triage `tasks/todo.md` and either resolve items, keep them as branch-local evidence, or ask the user where durable follow-up belongs.',
      '- Keep credentials out of committed files; use environment variables or platform secret stores if a task-system integration is added later.'
    ].join('\n');
  }

  return [
    '## Activation',
    '',
    `External issue tracking is active (\`taskSystem: ${taskSystem}\`). This repository uses **${taskSystemName}** as the system of record for all planned work, follow-up tasks, bugs, and feature requests. All durable work items must be created there, not left only in local notes or branch files.`
  ].join('\n');
}

function renderTaskSystemDisplayName(taskSystem: string): string {
  switch (taskSystem) {
    case 'github':
      return 'GitHub';
    case 'jira':
      return 'Jira';
    case 'linear':
      return 'Linear';
    default:
      return taskSystem;
  }
}

function applyTaskSystemGuidance(
  content: string,
  agentId: string,
  options?: BuildOptions
): string {
  if (agentId !== 'tasks' || !content.includes(TASK_SYSTEM_GUIDANCE_TOKEN)) {
    return content;
  }
  if (options?.variables?.taskSystem === 'none') {
    return (
      content.slice(0, content.indexOf(TASK_SYSTEM_GUIDANCE_TOKEN)) +
      renderTaskSystemGuidance(options)
    );
  }
  return content.replaceAll(
    TASK_SYSTEM_GUIDANCE_TOKEN,
    renderTaskSystemGuidance(options)
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
  language: Language = 'typescript',
  publishingProfiles?: readonly PublishingProfile[]
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
  const namedSuffixes: string[] = [];
  for (const e of entries) {
    if (
      !e.isFile() ||
      !e.name.startsWith(CONTENT_PREFIX + '-') ||
      !e.name.endsWith('.md')
    )
      continue;
    const stem = e.name.slice(0, -3);
    const suffix = stem.slice(CONTENT_PREFIX.length + 1);
    if (suffix) namedSuffixes.push(suffix);
  }
  // Sort for deterministic output across filesystems, matching the Go and
  // Python backends.
  suffixes.push(...namedSuffixes.sort());
  if (suffixes.length === 0) {
    throw new Error(`Agent "${agentId}" has no content.md or content-*.md`);
  }
  if (agentId === 'publishing') {
    if (publishingProfiles !== undefined && publishingProfiles.length > 0) {
      const available = new Set(suffixes);
      return publishingProfiles.filter((profile) => available.has(profile));
    }
    // Opt-in variants are reference-only unless explicitly configured; do not
    // emit them into the always-loaded rule set by default.
    return suffixes.filter(
      (suffix) =>
        !(OPT_IN_PUBLISHING_PROFILES as readonly string[]).includes(suffix)
    );
  }
  return suffixes;
}

const INCLUDE_SEGMENT_PATTERN = /^[A-Za-z0-9._-]+$/;

/**
 * Includes must be relative, forward-slash-separated .md paths whose segments
 * contain only safe characters — rejecting absolute paths, Windows drive or
 * rooted paths, and traversal on every platform.
 */
function isValidIncludePath(includePath: string): boolean {
  if (!includePath.endsWith('.md')) return false;
  const segments = includePath.split('/');
  return segments.every(
    (segment) =>
      INCLUDE_SEGMENT_PATTERN.test(segment) &&
      segment !== '.' &&
      segment !== '..'
  );
}

const INCLUDE_PATTERN = /\{\{include:([^}]+)\}\}/g;
const MAX_INCLUDE_DEPTH = 10;

/**
 * Resolve {{include:<path>.md}} tokens against the agents content root. The
 * fragment body is inserted with trailing whitespace trimmed so tokens can sit
 * inline in a content file. Fragments may include other fragments; recursion
 * and missing files fail the build with a clear error.
 */
export function resolveContentIncludes(
  content: string,
  agentsContentRoot?: string,
  stack: string[] = []
): string {
  if (!content.includes('{{include:')) {
    return content;
  }
  return content.replace(INCLUDE_PATTERN, (_match, rawPath: string) => {
    const includePath = rawPath.trim();
    if (!isValidIncludePath(includePath)) {
      throw new Error(
        `Invalid include path "${includePath}": must be a relative, forward-slash .md path under agents/`
      );
    }
    if (stack.includes(includePath)) {
      throw new Error(
        `Recursive include detected for "${includePath}" (chain: ${[...stack, includePath].join(' -> ')})`
      );
    }
    if (stack.length >= MAX_INCLUDE_DEPTH) {
      throw new Error(
        `Include depth exceeded (max ${MAX_INCLUDE_DEPTH}) at "${includePath}" (chain: ${[...stack, includePath].join(' -> ')})`
      );
    }
    // Mirror content precedence: monorepo source checkout first, then the
    // packaged agents root.
    const roots = agentsContentRoot
      ? [agentsContentRoot]
      : [SOURCE_AGENTS_ROOT, getAgentsContentRoot()];
    const file = roots
      .map((root) => path.join(root, includePath))
      .find((candidate) => fs.existsSync(candidate));
    if (!file) {
      throw new Error(
        `Missing include fragment: ${includePath} (searched ${roots.join(', ')})`
      );
    }
    const fragment = fs.readFileSync(file, 'utf8');
    return resolveContentIncludes(fragment, agentsContentRoot, [
      ...stack,
      includePath
    ]).trimEnd();
  });
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
  let raw = resolveContentIncludes(fs.readFileSync(file, 'utf8'));
  if (options?.variables) {
    for (const [key, value] of Object.entries(options.variables)) {
      const replacement =
        key === 'taskSystem' ? renderTaskSystemDisplayName(value) : value;
      raw = raw.replaceAll(`{{${key}}}`, replacement);
    }
  }
  return applyTaskSystemGuidance(
    applyDeploymentModelGuidance(
      applyDeploymentConditionalBlocks(
        applyHookGuidance(raw, agentId, language, options),
        options?.variables?.deploymentModel ?? 'none'
      ),
      agentId,
      options
    ),
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

export function buildCodexSkillMarkdown(skillId: string): string {
  const content = getSkillContent(skillId).trimEnd();
  const frontmatterMatch = content.match(/^---\n[\s\S]*?\n---\n?/);
  if (!frontmatterMatch) {
    return [BALLAST_MANAGED_COMMENT, '', content, ''].join('\n');
  }
  const frontmatter = frontmatterMatch[0].trimEnd();
  const body = content.slice(frontmatterMatch[0].length).trimStart();
  return [frontmatter, '', BALLAST_MANAGED_COMMENT, '', body, ''].join('\n');
}

export function copyCodexSkillResources(
  skillId: string,
  destinationDir: string
): void {
  const sourceDir = getPreferredSkillDir(skillId);
  for (const entry of fs.readdirSync(sourceDir, { withFileTypes: true })) {
    if (entry.name === 'SKILL.md' || entry.name === 'claude-settings.json') {
      continue;
    }
    const source = path.join(sourceDir, entry.name);
    const destination = path.join(destinationDir, entry.name);
    fs.cpSync(source, destination, {
      recursive: true,
      force: true,
      errorOnExist: false
    });
  }
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
  language: Language = 'typescript',
  publishingProfiles?: readonly PublishingProfile[],
  tools?: Record<string, string[]>
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
  lines.push(...renderRepositoryToolPolicyManifestLines(tools));
  lines.push(
    'Read and follow these rule files in `.codex/rules/` when they apply:'
  );
  lines.push('');
  for (const agentId of agents) {
    const suffixes = listRuleSuffixes(agentId, language, publishingProfiles);
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
      'Read and use these skill files in `.codex/skills/` when they are relevant:'
    );
    lines.push('');
    for (const skillId of skills) {
      lines.push(
        `- \`.codex/skills/${skillId}/SKILL.md\` — ${getSkillDescription(skillId)}`
      );
    }
  }
  lines.push('');
  return lines.join('\n');
}

export function buildClaudeMd(
  agents: string[],
  skills: string[] = [],
  language: Language = 'typescript',
  publishingProfiles?: readonly PublishingProfile[],
  tools?: Record<string, string[]>
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
  lines.push(...renderRepositoryToolPolicyManifestLines(tools));
  lines.push(
    'Read and follow these rule files in `.claude/rules/` when they apply:'
  );
  lines.push('');
  for (const agentId of agents) {
    const suffixes = listRuleSuffixes(agentId, language, publishingProfiles);
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
  language: Language = 'typescript',
  publishingProfiles?: readonly PublishingProfile[],
  tools?: Record<string, string[]>
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
  lines.push(...renderRepositoryToolPolicyManifestLines(tools));
  lines.push(
    'Read and follow these rule files in `.gemini/rules/` when they apply:'
  );
  lines.push('');
  for (const agentId of agents) {
    const suffixes = listRuleSuffixes(agentId, language, publishingProfiles);
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
  const configuredTaskSystem =
    options?.variables?.taskSystem?.trim().toLowerCase() || 'github';
  result = applyConditionalTokenBlocks(
    result,
    'TASK_SYSTEM',
    (name) => name === configuredTaskSystem
  );
  result = applyConditionalTokenBlocks(
    result,
    'TARGET',
    (name) => name === target
  );
  if (options?.variables) {
    for (const [key, value] of Object.entries(options.variables)) {
      result = result.replaceAll(`{{${key}}}`, value);
    }
  }
  // Manifest-bearing targets (claude, codex, gemini) get the tool policy once
  // in their manifest's "Installed agent rules" section instead of per rule.
  if (target === 'cursor' || target === 'opencode') {
    result = insertRepositoryToolPolicy(result, options);
  }
  return addRuleMarker(result, getRuleMarkerId(agentId, language, ruleSuffix));
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
      const dir = path.join(root, '.codex', 'skills', skillId);
      return { dir, file: path.join(dir, 'SKILL.md') };
    }
    default:
      throw new Error(`Unknown target: ${target}`);
  }
}

export function getLegacyCodexSkillDestination(
  skillId: string,
  projectRoot: string
): { dir: string; file: string } {
  const root = path.resolve(projectRoot);
  const dir = path.join(root, '.codex', 'rules');
  return { dir, file: path.join(dir, `${skillId}.md`) };
}

export function getAllSkillIds(): string[] {
  return SKILL_IDS.slice();
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

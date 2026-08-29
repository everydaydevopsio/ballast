import path from 'path';
import {
  getContent,
  getTemplate,
  getSkillContent,
  listRuleSuffixes,
  buildCursorFormat,
  buildClaudeFormat,
  buildOpenCodeFormat,
  buildCodexFormat,
  buildContent,
  buildClaudeSkill,
  buildCodexSkillMarkdown,
  buildCursorSkillFormat,
  buildSkillMarkdown,
  buildClaudeMd,
  buildCodexAgentsMd,
  buildGeminiMd,
  getClaudeMdPath,
  getCodexAgentsMdPath,
  getCodexRuleDescription,
  getSkillDescription,
  extractDescriptionFromFrontmatter,
  getDestination,
  getSkillDestination,
  listTargets,
  parseRuleMarker,
  shouldEmitRuleForSubdir,
  verifyRuleChecksum
} from './build';
import { COMMON_SKILL_IDS } from './agents';

describe('build', () => {
  describe('managed rule marker', () => {
    test('buildContent writes a machine-readable rule marker', () => {
      const content = buildContent('linting', 'codex', undefined, 'typescript');

      expect(content).toMatch(
        /^<!-- ballast:rule id="typescript\/linting" version="[^"]+" checksum="[a-f0-9]{64}" -->\n/
      );
      expect(parseRuleMarker(content)).toEqual(
        expect.objectContaining({
          ruleId: 'typescript/linting',
          version: expect.any(String),
          checksum: expect.stringMatching(/^[a-f0-9]{64}$/)
        })
      );
      expect(verifyRuleChecksum(content)).toBe(true);
    });

    test('verifyRuleChecksum detects body drift after the marker', () => {
      const content = buildContent('linting', 'codex', undefined, 'typescript');
      const drifted = `${content}\nManual edit\n`;

      expect(verifyRuleChecksum(drifted)).toBe(false);
    });

    test('parseRuleMarker ignores markers outside the generated location', () => {
      const content = [
        '# Manual rule',
        '',
        'Example:',
        '<!-- ballast:rule id="typescript/linting" version="5.0.0" checksum="0123456789abcdef" -->',
        ''
      ].join('\n');

      expect(parseRuleMarker(content)).toBeNull();
      expect(verifyRuleChecksum(content)).toBe(false);
    });

    test('parseRuleMarker accepts markers directly after frontmatter', () => {
      const content = buildContent(
        'linting',
        'cursor',
        undefined,
        'typescript'
      );

      expect(content).toMatch(
        /^---\n[\s\S]*?\n---\n<!-- ballast:rule id="typescript\/linting"/
      );
      expect(parseRuleMarker(content)).toEqual(
        expect.objectContaining({
          ruleId: 'typescript/linting'
        })
      );
      expect(verifyRuleChecksum(content)).toBe(true);
    });
  });

  describe('listRuleSuffixes', () => {
    test('returns only main rule for linting (no content-*.md)', () => {
      expect(listRuleSuffixes('linting')).toEqual(['']);
    });

    test('returns only main rule for logging', () => {
      expect(listRuleSuffixes('logging')).toEqual(['']);
    });

    test('returns only main rule for testing', () => {
      expect(listRuleSuffixes('testing')).toEqual(['']);
    });

    test('returns env, mcp, license, and badges for local-dev', () => {
      expect(listRuleSuffixes('local-dev')).toContain('env');
      expect(listRuleSuffixes('local-dev')).toContain('mcp');
      expect(listRuleSuffixes('local-dev')).toContain('license');
      expect(listRuleSuffixes('local-dev')).toContain('badges');
      expect(listRuleSuffixes('local-dev').length).toBe(4);
    });

    test('returns only main rule for docs', () => {
      expect(listRuleSuffixes('docs')).toEqual(['']);
    });

    test('returns libraries, sdks, and apps for publishing by default', () => {
      expect(listRuleSuffixes('publishing')).toContain('libraries');
      expect(listRuleSuffixes('publishing')).toContain('sdks');
      expect(listRuleSuffixes('publishing')).toContain('apps');
      expect(listRuleSuffixes('publishing')).toContain('cli');
      expect(listRuleSuffixes('publishing')).toContain('brew');
      expect(listRuleSuffixes('publishing')).toContain('apt');
      expect(listRuleSuffixes('publishing')).toContain('web');
      expect(listRuleSuffixes('publishing')).toContain('api');
      expect(listRuleSuffixes('publishing').length).toBe(8);
    });

    test('returns selected publishing profiles when configured', () => {
      const suffixes = listRuleSuffixes('publishing', 'typescript', [
        'cli',
        'apps'
      ]);

      expect(suffixes).toHaveLength(2);
      expect(suffixes).toEqual(expect.arrayContaining(['cli', 'apps']));
    });

    test('treats empty publishingProfiles as unfiltered', () => {
      const suffixes = listRuleSuffixes('publishing', 'typescript', []);

      expect(suffixes).toHaveLength(8);
      expect(suffixes).toEqual(
        expect.arrayContaining([
          'libraries',
          'sdks',
          'apps',
          'cli',
          'brew',
          'apt',
          'web',
          'api'
        ])
      );
    });

    test('returns only main rule for plan-lifecycle', () => {
      expect(listRuleSuffixes('plan-lifecycle')).toEqual(['']);
    });

    test('throws for unknown agent', () => {
      expect(() => listRuleSuffixes('nonexistent')).toThrow(/content.md/);
    });
  });

  describe('getContent', () => {
    test('returns content for linting agent', () => {
      const content = getContent('linting');
      expect(content).toContain('TypeScript linting specialist');
      expect(content).toContain('## Your Responsibilities');
    });

    test('returns content for logging agent', () => {
      const content = getContent('logging');
      expect(content).toContain('Centralized Logging Agent');
      expect(content).toContain('pino-browser');
      expect(content).toContain('/api/logs');
    });

    test('returns env content for local-dev with ruleSuffix env', () => {
      const content = getContent('local-dev', 'env');
      expect(content).toContain('Local Development Environment Agent');
      expect(content).toContain('docker-compose.local.yaml');
      expect(content).toContain('Makefile');
      expect(content).toContain('make up-local');
      expect(content).toContain('## Branch Before Code');
      expect(content).toContain('git branch --show-current');
      expect(content).toContain(
        'gh repo view --json defaultBranchRef --jq .defaultBranchRef.name'
      );
      expect(content).toContain('If that command fails for any reason');
      expect(content).toContain('strip the `origin/` prefix');
      expect(content).toContain(
        'If both default-branch detection methods fail'
      );
      expect(content).toContain('current branch name is empty');
      expect(content).toContain('issue-212-branch-before-code');
      expect(content).toContain('Read-only investigation');
    });

    test('returns Copilot review loop guidance for local-dev PR workflow', () => {
      const content = getContent('local-dev', 'env');
      expect(content).toContain(
        'Address review comments directly and stop only when required checks are green and actionable comments are resolved.'
      );
      expect(content).toContain(
        'Inspect failing checks with `gh`; summarize the failure.'
      );
      expect(content).toContain('gh pr checks');
      expect(content).toContain('gh pr view');
      expect(content).toContain('GitHub MCP');
    });

    test('returns mcp content for local-dev with ruleSuffix mcp', () => {
      const content = getContent('local-dev', 'mcp');
      expect(content).toContain('tasks');
      expect(content).toContain('ballast install');
    });

    test('returns language-neutral common rule content', () => {
      const cicd = buildCodexFormat('cicd', undefined, 'go');
      const localDev = buildCodexFormat('local-dev', 'env', 'go');
      const observability = buildCodexFormat('observability', undefined, 'go');

      expect(cicd).toContain("repository's configured languages and runtimes");
      expect(localDev).toContain(
        "repository's configured languages and runtimes"
      );
      expect(observability).toContain(
        "repository's configured languages and runtimes"
      );
      expect(cicd).not.toContain(
        'CI/CD specialist for TypeScript/JavaScript projects'
      );
      expect(localDev).not.toContain(
        'local development environment specialist for TypeScript/JavaScript projects'
      );
      expect(observability).not.toContain(
        'observability specialist for TypeScript/JavaScript applications'
      );
    });

    test('returns tasks task-system content with default variable substitution', () => {
      const content = getContent('tasks', 'task-system', 'typescript', {
        variables: { taskSystem: 'github' }
      });
      expect(content).toContain('github');
      expect(content).toContain('**GitHub** as the system of record');
      expect(content).toContain('"configure MCP for GitHub"');
      expect(content).not.toContain('**github**');
      expect(content).toContain(
        'External issue tracking is active (`taskSystem: github`).'
      );
      expect(content).toContain('MCP Server Setup');
      expect(content).not.toContain('{{taskSystem}}');
    });

    test('returns tasks task-system content with linear substituted', () => {
      const content = getContent('tasks', 'task-system', 'typescript', {
        variables: { taskSystem: 'linear' }
      });
      expect(content).toContain('linear');
      expect(content).not.toContain('{{taskSystem}}');
    });

    test('returns tasks task-system content for none without mandatory tracker guidance', () => {
      const content = getContent('tasks', 'task-system', 'typescript', {
        variables: { taskSystem: 'none' }
      });

      expect(content).toContain('taskSystem: none');
      expect(content).toContain(
        'External issue tracking is disabled (`taskSystem: none`).'
      );
      expect(content).toContain('No task-system MCP server is required');
      expect(content).toContain(
        'Do not create external issues or tickets by default.'
      );
      expect(content).not.toContain('{{taskSystem}}');
      expect(content).not.toContain(
        'All durable work items must be created there'
      );
      expect(content).not.toContain('configure MCP for none');
    });

    test('returns tasks todo content', () => {
      const content = getContent('tasks', 'todo');
      expect(content).toContain('tasks/todo.md');
      expect(content).toContain('triage');
      expect(content).not.toContain('tasks/TODO.md');
      expect(content).not.toContain('{{taskSystem}}');
    });

    test('returns structured tasks todo templates aligned with execution templates', () => {
      const content = getContent('tasks', 'todo');
      expect(content).toContain('# Task: <title>');
      expect(content).toContain('## Acceptance Criteria');
      expect(content).toContain('## Test Strategy');
      expect(content).toContain('Failure-path tests:');
      expect(content).toContain('Requirement-to-test mapping:');
      expect(content).toContain('## Rollback Strategy');
      expect(content).toContain('## Outcome');
      expect(content).toContain('Lightweight tasks may omit optional sections');
      expect(content).toContain(
        'must remain a subset of the structured template'
      );
    });

    test('returns canonical lessons and issue output templates in task guidance', () => {
      const content = getContent('tasks', 'todo');
      expect(content).toContain('tasks/lessons.md');
      expect(content).toContain('# Lessons');
      expect(content).toContain('Root cause pattern:');
      expect(content).toContain('### Issue #N: <Short Description>');
      expect(content).toContain('**Severity:** <Critical|High|Medium|Low>');
      expect(content).toContain('**Option A (Recommended)**');
      expect(content).toContain('**Decision Request**');
    });

    test('returns plan-lifecycle content', () => {
      const content = getContent('plan-lifecycle');
      expect(content).toContain('Plan -> ADR lifecycle');
      expect(content).toContain('Create a plan when');
      expect(content).toContain('Skip a plan when');
      expect(content).toContain('plans/plan-<feature-name>.md');
      expect(content).toContain('## Maintaining The Plan');
      expect(content).toContain(
        'Graduate `plans/plan-<feature-name>.md` to an ADR'
      );
      expect(content).toContain('## ADR Template');
      expect(content).toContain('## ADR Management Rules');
      expect(content).toContain('## Quick Reference');
      expect(content).toContain(
        'Defer `tasks/todo.md` behavior to the branch-local TODO tracking rule'
      );
      expect(content).toContain('task system work item');
      expect(content).not.toContain('Jira');
    });

    test('returns tasks task-system content unsubstituted when no variables', () => {
      const content = getContent('tasks', 'task-system');
      expect(content).toContain('{{taskSystem}}');
    });

    test('returns docs content', () => {
      const content = getContent('docs');
      expect(content).toContain('Documentation Agent');
      expect(content).toContain('Default to a GitHub-readable Markdown');
      expect(content).toContain('publish-docs');
      expect(content).toContain('Mermaid');
    });

    test('returns publishing libraries content', () => {
      const content = getContent('publishing', 'libraries');
      expect(content).toContain('Publishing Libraries Agent');
      expect(content).toContain('release_type');
      expect(content).toContain('patch');
      expect(content).toContain('minor');
      expect(content).toContain('major');
      expect(content).toContain('bump_and_tag');
      expect(content).toContain(
        'WyriHaximus/github-action-get-previous-tag@v2'
      );
      expect(content).toContain('WyriHaximus/github-action-next-semvers');
      expect(content).toContain('npm publish --access public --provenance');
      expect(content).toContain('PyPI');
      expect(content).toContain('GitHub Releases');
    });

    test('returns publishing apps content for Kubernetes GitOps deployments', () => {
      const content = getContent('publishing', 'apps', 'typescript', {
        variables: { deploymentModel: 'kubernetes' }
      });
      expect(content).toContain('release_type');
      expect(content).toContain('v<version>');
      expect(content).toContain(
        'WyriHaximus/github-action-get-previous-tag@v2'
      );
      expect(content).toContain('WyriHaximus/github-action-next-semvers');
      expect(content).toContain('ghcr.io');
      expect(content).toContain('Docker Hub');
      expect(content).toContain('charts/<app>/');
      expect(content).toContain('ArgoCD');
      expect(content).toContain('GitOps repository');
      expect(content).toContain('image digest');
      expect(content).toContain('## App Deployment Model');
      expect(content).toContain(
        'Deployment guidance is active (`deploymentModel: kubernetes`).'
      );
      expect(content).not.toContain('## Kubernetes');
    });

    test('returns hosted deployment guidance when deploymentModel is hosted', () => {
      const content = getContent('publishing', 'apps', 'typescript', {
        variables: { deploymentModel: 'hosted' }
      });
      expect(content).toContain('Vercel');
      expect(content).toContain('Railway');
      expect(content).toContain('preview deployments');
    });

    test('returns docker deployment guidance when deploymentModel is docker', () => {
      const content = buildContent('publishing', 'codex', 'web', 'typescript', {
        variables: { deploymentModel: 'docker' }
      });

      expect(content).toContain('deploymentModel: docker');
      expect(content).toContain('GHCR and Docker Hub');
      expect(content).toContain('image vulnerability scan');
    });

    test('returns compact Kubernetes GitOps web deployment guidance', () => {
      const content = getContent('publishing', 'web', 'typescript');
      expect(content).toContain('deploymentModel: none');
      expect(content).toContain('Deployment is inactive');
      expect(content).toContain('Deployment guidance is reference-only');
      expect(content).toContain('do not create deploy-on-main workflows');
      expect(content).toContain('Do not paste full workflow templates');
      expect(content).not.toContain(
        'Kubernetes Workflow Template (`deploy.yml`)'
      );
      expect(content).toContain('release_tag');
      expect(content).toContain('refs/tags/v<version>');
      expect(content).toContain('GitOps handoff');
      expect(content).toContain('pushed digest');
      expect(content).toContain('never create unprefixed release tags');
    });

    test('keeps REST API baseline goals platform neutral', () => {
      const content = getContent('publishing', 'api', 'typescript');
      expect(content).toContain('Deployment is inactive');
      expect(content).toContain('Deployment guidance is reference-only');
      expect(content).toContain('do not create deploy-on-main workflows');
      expect(content).toContain(
        'health and readiness endpoints that the configured runtime can use'
      );
      expect(content).toContain(
        'Scope Kubernetes probes and Helm chart templates to repositories with `deploymentModel: kubernetes`.'
      );
      expect(content).toContain(
        'Apply this section only when `deploymentModel` is `kubernetes`.'
      );
      expect(content).not.toContain(
        'Ensure the API exposes a health endpoint that Kubernetes probes can use.'
      );
    });

    test('marks optional publishing variants as reference-only until configured', () => {
      const brew = getContent('publishing', 'brew');
      const apt = getContent('publishing', 'apt');

      for (const content of [brew, apt]) {
        expect(content).toContain('## Activation');
        expect(content).toContain(
          'This optional publishing variant is inactive by default.'
        );
        expect(content).toContain(
          'Treat this rule as reference-only unless it is explicitly configured'
        );
      }
    });

    test('returns TypeScript testing content with web smoke and E2E placement guidance', () => {
      const content = getContent('testing', undefined, 'typescript');
      expect(content).toContain('web smoke test');
      expect(content).toContain('live route or health endpoint');
      expect(content).toContain(
        'Prefer Playwright only when Playwright markers already exist, or when the repo has a real browser application surface and no existing browser E2E framework.'
      );
      expect(content).toContain(
        'Run fast unit tests and targeted smoke checks during local work, put deterministic build/typecheck plus smoke checks in pre-push, and run full smoke/E2E gates in CI.'
      );
      expect(content).toContain('one critical user workflow');
    });

    test('returns TypeScript testing content with integration framework detection guidance', () => {
      const content = getContent('testing', undefined, 'typescript');
      expect(content).toContain(
        'Detect existing unit, integration, and browser E2E frameworks before adding or replacing test tooling.'
      );
      for (const marker of [
        'Jest',
        'Vitest',
        'Cypress',
        'Playwright',
        'WebdriverIO',
        'Selenium',
        'Puppeteer',
        'Testing Library'
      ]) {
        expect(content).toContain(marker);
      }
      expect(content).toContain(
        'Preserve an existing browser E2E framework unless the user explicitly asks to migrate.'
      );
      expect(content).toContain(
        'Prefer Playwright only when Playwright markers already exist, or when the repo has a real browser application surface and no existing browser E2E framework.'
      );
      expect(content).toContain(
        'Do not add browser E2E tooling to library-only, CLI-only, infrastructure-only, or backend-only repositories without a user-facing browser surface.'
      );
    });

    test('returns TypeScript testing content with TDD process discipline', () => {
      const content = getContent('testing', undefined, 'typescript');
      expect(content).toContain('## TDD Process Discipline');
      expect(content).toContain(
        'Tooling setup and process discipline are separate responsibilities'
      );
      for (const required of [
        'Start from acceptance criteria',
        'Write a failing test first',
        'Confirm the test fails for the right reason',
        'Implement the minimum change',
        'Refactor only after the relevant tests are green',
        'Proof of completion',
        'Failure-path coverage',
        'Traceability'
      ]) {
        expect(content).toContain(required);
      }
    });

    test('returns Python testing content with web smoke and E2E placement guidance', () => {
      const content = getContent('testing', undefined, 'python');
      expect(content).toContain('web smoke test');
      expect(content).toContain('live route or health endpoint');
      expect(content).toContain(
        'Prefer Playwright only when Playwright markers already exist, or when the repo has a real browser application surface and no existing browser E2E framework.'
      );
      expect(content).toContain(
        'Run fast unit tests and targeted smoke checks during local work, put deterministic build/typecheck plus smoke checks in pre-push, and run full smoke/E2E gates in CI.'
      );
    });

    test('returns Python testing content with integration framework detection guidance', () => {
      const content = getContent('testing', undefined, 'python');
      expect(content).toContain(
        'Detect existing unit, integration, API/service, and browser E2E frameworks before adding or replacing test tooling.'
      );
      for (const marker of [
        'pytest',
        'unittest',
        'tox',
        'nox',
        'Robot Framework',
        'Selenium',
        'pytest-playwright',
        'Playwright',
        'FastAPI TestClient',
        'Django test client'
      ]) {
        expect(content).toContain(marker);
      }
      expect(content).toContain(
        'Preserve an existing browser E2E framework unless the user explicitly asks to migrate.'
      );
      expect(content).toContain(
        'Prefer Playwright only when Playwright markers already exist, or when the repo has a real browser application surface and no existing browser E2E framework.'
      );
      expect(content).toContain(
        'Do not add browser E2E tooling to library-only, CLI-only, infrastructure-only, or backend-only repositories without a user-facing browser surface.'
      );
    });

    test('returns Python testing content with TDD process discipline', () => {
      const content = getContent('testing', undefined, 'python');
      expect(content).toContain('## TDD Process Discipline');
      expect(content).toContain(
        'Tooling setup and process discipline are separate responsibilities'
      );
      for (const required of [
        'Start from acceptance criteria',
        'Write a failing test first',
        'Confirm the test fails for the right reason',
        'Implement the minimum change',
        'Refactor only after the relevant tests are green',
        'Proof of completion',
        'Failure-path coverage',
        'Traceability'
      ]) {
        expect(content).toContain(required);
      }
    });

    test('returns CLI publishing content with packaged-command smoke guidance', () => {
      const content = getContent('publishing', 'cli');
      expect(content).toContain('packaged-command smoke');
      expect(content).toContain('built artifact');
      expect(content).toContain('--help');
      expect(content).toContain('--version');
      expect(content).toContain('representative command');
      expect(content).toContain(
        'Keep local packaged-command smoke checks fast, run them in pre-push when the packaged artifact can be built deterministically, and require them in CI before publish jobs.'
      );
    });

    test('throws for unknown agent', () => {
      expect(() => getContent('nonexistent')).toThrow(/content.md/);
    });

    test('throws for missing optional rule', () => {
      expect(() => getContent('linting', 'mcp')).toThrow(/content-mcp.md/);
    });

    test('returns python-specific linting content without hook guidance when language is python', () => {
      const content = getContent('linting', undefined, 'python');
      expect(content).toContain('Python linting specialist');
      expect(content).toContain('Ruff');
      expect(content).not.toContain('.pre-commit-config.yaml');
      expect(content).not.toContain('pre-commit install');
      expect(content).not.toContain('pre-commit install --hook-type pre-push');
      expect(content).not.toContain('pre-commit autoupdate');
    });

    test('returns go-specific linting content without hook guidance', () => {
      const content = getContent('linting', undefined, 'go');
      expect(content).toContain('Go linting specialist');
      expect(content).not.toContain('.pre-commit-config.yaml');
      expect(content).not.toContain('sub-pre-commit');
      expect(content).not.toContain('pre-commit install --hook-type pre-push');
      expect(content).not.toContain('pre-commit autoupdate');
    });

    test('returns pre-commit git-hooks content for python', () => {
      const content = getContent('git-hooks', undefined, 'python');
      expect(content).toContain('Git hook specialist');
      expect(content).toContain('.pre-commit-config.yaml');
      expect(content).toContain('gitleaks');
      expect(content).not.toContain('scripts/check-no-secrets.sh');
      expect(content).toContain('pre-commit install --hook-type pre-push');
      expect(content).toContain('pre-commit autoupdate');
      expect(content).toContain('Bandit');
      expect(content).toContain('pip-audit');
    });

    test('returns go git-hooks command guidance', () => {
      const content = getContent('git-hooks', undefined, 'go');
      expect(content).toContain('sub-pre-commit');
      expect(content).toContain('pre-commit install --hook-type pre-push');
      expect(content).toContain('Go unit tests');
      expect(content).toContain('gitleaks');
      expect(content).toContain('govulncheck');
      expect(content).toContain('go test -race');
      expect(content).not.toContain('scripts/check-no-secrets.sh');
    });

    test('returns ansible git-hooks command guidance', () => {
      const content = getContent('git-hooks', undefined, 'ansible');
      expect(content).toContain('ansible-lint');
      expect(content).toContain('yamllint');
      expect(content).toContain('ansible-playbook --syntax-check');
      expect(content).toContain('pre-push stage');
      expect(content).toContain('pre-commit autoupdate');
      expect(content).toContain('gitleaks');
      expect(content).toContain('ansible-lint --profile=safety');
      expect(content).not.toContain(
        'Use Husky for TypeScript-only repositories.'
      );
      expect(content).not.toContain('Husky');
      expect(content).not.toContain('lint-staged');
      expect(content).not.toContain('npx lint-staged');
      expect(content).not.toContain('scripts/check-no-secrets.sh');
    });

    test('returns ansible cicd guidance without unsupported dependabot ecosystem', () => {
      const content = getContent('cicd', undefined, 'ansible');
      expect(content).toContain('Dependabot does not support Ansible Galaxy');
      expect(content).toContain('requirements.yml');
      expect(content).toContain('github-actions');
      expect(content).not.toContain("package-ecosystem: 'ansible");
      expect(content).not.toContain('ansible-galaxy');
    });

    test('returns initialized terraform git-hooks command guidance', () => {
      const content = getContent('git-hooks', undefined, 'terraform');
      expect(content).toContain('terraform fmt -check -recursive');
      expect(content).toContain('terraform init -backend=false');
      expect(content).toContain('terraform validate');
      expect(content).toContain('tflint --init');
      expect(content).toContain('tflint --recursive');
      expect(content).toContain('trivy config .');
      expect(content).toContain('tfsec');
      expect(content).toContain('gitleaks');
      expect(content).not.toContain('scripts/check-no-secrets.sh');
      expect(content).toContain('cloud/runtime posture scanning');
    });

    test('returns flutter dart git-hooks command guidance', () => {
      const content = getContent('git-hooks', undefined, 'dart');
      expect(content).toContain('dart format --set-exit-if-changed .');
      expect(content).toContain('flutter analyze');
      expect(content).toContain('flutter test');
      expect(content).toContain('flutter test integration_test');
      expect(content).toContain('.dart_tool/');
      expect(content).toContain('gitleaks');
      expect(content).not.toContain('scripts/check-no-secrets.sh');
    });

    test('returns docker git-hooks command guidance', () => {
      const content = getContent('git-hooks', undefined, 'docker');
      expect(content).toContain('Dockerfile and container configuration');
      expect(content).toContain('hadolint');
      expect(content).toContain('docker compose config');
      expect(content).toContain('pre-commit install --hook-type pre-push');
      expect(content).toContain('image vulnerability scans');
      expect(content).toContain('gitleaks');
      expect(content).not.toContain('scripts/check-no-secrets.sh');
    });

    test('returns husky git-hooks content for typescript', () => {
      const content = getContent('git-hooks', undefined, 'typescript', {
        hookMode: 'husky'
      });
      expect(content).toContain('Use Husky for TypeScript-only repositories.');
      expect(content).toContain('.yaml');
      expect(content).toContain('.yml');
      expect(content).toContain('lint-staged');
      expect(content).toContain('repo formatter');
      expect(content).toContain('.husky/pre-push');
      expect(content).toContain('package-manager test command');
      expect(content).toContain('build or typecheck');
      expect(content).not.toContain('pre-commit install');
    });

    test('returns go-specific testing content when language is go', () => {
      const content = getContent('testing', undefined, 'go');
      expect(content).toContain('Go testing specialist');
      expect(content).toContain('go test ./...');
    });

    test('returns Go testing content with integration framework detection guidance', () => {
      const content = getContent('testing', undefined, 'go');
      expect(content).toContain(
        'Detect existing unit, integration, API/service, and browser E2E frameworks before adding or replacing test tooling.'
      );
      for (const marker of [
        'go test',
        'integration build tags',
        '_integration_test.go',
        'httptest',
        'Selenium',
        'chromedp',
        'rod',
        'agouti',
        'Playwright'
      ]) {
        expect(content).toContain(marker);
      }
      expect(content).toContain(
        'Preserve an existing browser E2E framework unless the user explicitly asks to migrate.'
      );
      expect(content).toContain(
        'Prefer Playwright only when Playwright markers already exist, or when the repo has a real browser application surface and no existing browser E2E framework.'
      );
      expect(content).toContain(
        'Do not add browser E2E tooling to library-only, CLI-only, infrastructure-only, or backend-only repositories without a user-facing browser surface.'
      );
    });

    test('returns Go testing content with TDD process discipline', () => {
      const content = getContent('testing', undefined, 'go');
      expect(content).toContain('## TDD Process Discipline');
      expect(content).toContain(
        'Tooling setup and process discipline are separate responsibilities'
      );
      for (const required of [
        'Start from acceptance criteria',
        'Write a failing test first',
        'Confirm the test fails for the right reason',
        'Implement the minimum change',
        'Refactor only after the relevant tests are green',
        'Proof of completion',
        'Failure-path coverage',
        'Traceability'
      ]) {
        expect(content).toContain(required);
      }
    });
  });

  describe('skills', () => {
    test('reads content for every common skill id', () => {
      for (const skillId of COMMON_SKILL_IDS) {
        const content = getSkillContent(skillId);
        expect(content).toContain(`name: ${skillId}`);
      }
    });

    test('reads skill content', () => {
      const content = getSkillContent('owasp-security-scan');
      expect(content).toContain('name: owasp-security-scan');
      expect(content).toContain('# OWASP Security Scan Skill');
    });

    test('reads aws health skill content', () => {
      const content = getSkillContent('aws-health-review');
      expect(content).toContain('name: aws-health-review');
      expect(content).toContain('# AWS Health Review');
    });

    test('reads github health skill content', () => {
      const content = getSkillContent('github-health-check');
      expect(content).toContain('name: github-health-check');
      expect(content).toContain('# GitHub Repository Health Check Skill');
      expect(content).toContain(
        '## Check 6 — GitHub Security Feature Enablement'
      );
      expect(content).toContain(
        '`HIGH`: Private vulnerability reporting must be enabled for public repositories only'
      );
      expect(content).toContain('--- Dependabot malware alerts ---');
      expect(content).toContain(
        '## Check 15 — Public and Private Repository Best Practices'
      );
    });

    test('reads github pr copilot cycle skill content', () => {
      const content = getSkillContent('github-pr-copilot-cycle');
      expect(content).toContain('name: github-pr-copilot-cycle');
      expect(content).toContain('# GitHub PR Copilot Cycle Skill');
      expect(content).toContain('Run at most three Copilot review cycles.');
    });

    test('builds cursor skill format', () => {
      const content = buildCursorSkillFormat('owasp-security-scan');
      expect(content).toContain('alwaysApply: false');
      expect(content).toContain('# OWASP Security Scan Skill');
    });

    test('builds markdown skill format', () => {
      const content = buildSkillMarkdown('owasp-security-scan');
      expect(content).toContain('# OWASP Security Scan Skill');
      expect(content).not.toContain('name: owasp-security-scan');
    });

    test('builds native codex skill markdown with frontmatter preserved', () => {
      const content = buildCodexSkillMarkdown('owasp-security-scan');
      expect(content).toMatch(/^---\nname: owasp-security-scan/m);
      expect(content).toContain('Created by [Ballast]');
      expect(content).toContain('# OWASP Security Scan Skill');
    });

    test('ballast audit skill documents both 5 KB and 10 KB thresholds', () => {
      const content = buildSkillMarkdown('ballast-audit');
      expect(content).toContain('-size +5k');
      expect(content).toContain('-size +10k');
      expect(content).not.toContain('name: ballast-audit');
    });

    test('builds claude skill zip with references', () => {
      const archive = buildClaudeSkill('owasp-security-scan');
      expect(archive.subarray(0, 4).toString('hex')).toBe('504b0304');
      expect(archive.includes(Buffer.from('SKILL.md'))).toBe(true);
      expect(archive.includes(Buffer.from('references/owasp-mapping.md'))).toBe(
        true
      );
    });

    test('gets skill description and destination', () => {
      expect(getSkillDescription('owasp-security-scan')).toContain(
        'Run OWASP-aligned security scans'
      );
      expect(
        getSkillDestination('owasp-security-scan', 'claude', '/tmp/project')
      ).toEqual({
        dir: path.join('/tmp/project', '.claude', 'skills'),
        file: path.join(
          '/tmp/project',
          '.claude',
          'skills',
          'owasp-security-scan.skill'
        )
      });
    });

    test('gets aws live health skill description', () => {
      expect(getSkillDescription('aws-live-health-review')).toContain(
        'Run a read-only AWS live health review'
      );
    });

    test('gets ballast audit skill description', () => {
      expect(getSkillDescription('ballast-audit')).toContain(
        'audit AI rule and skill files for context density, duplication, and bloat'
      );
    });
  });

  describe('getTemplate', () => {
    test('reads cursor frontmatter for linting', () => {
      const t = getTemplate('linting', 'cursor-frontmatter.yaml');
      expect(t).toContain('alwaysApply: false');
      expect(t).toContain('globs:');
    });

    test('reads rule-specific cursor frontmatter for local-dev mcp', () => {
      const t = getTemplate('local-dev', 'cursor-frontmatter.yaml', 'mcp');
      expect(t).toContain('GitHub MCP');
      expect(t).toContain('Jira/Linear/GitHub');
    });

    test('reads rule-specific cursor frontmatter for publishing sdks', () => {
      const t = getTemplate('publishing', 'cursor-frontmatter.yaml', 'sdks');
      expect(t).toContain('SDK publishing specialist');
    });

    test('reads cursor frontmatter for docs', () => {
      const t = getTemplate('docs', 'cursor-frontmatter.yaml');
      expect(t).toContain('Documentation specialist');
      expect(t).toContain('docusaurus.config.*');
    });

    test('reads claude header for linting', () => {
      const t = getTemplate('linting', 'claude-header.md');
      expect(t).toContain('TypeScript Linting Rules');
    });

    test('reads opencode frontmatter for linting', () => {
      const t = getTemplate('linting', 'opencode-frontmatter.yaml');
      expect(t).toContain('mode: subagent');
    });
  });

  describe('buildCursorFormat', () => {
    test('combines frontmatter with content for linting', () => {
      const result = buildCursorFormat('linting');
      expect(result).toMatch(/^---\n/);
      expect(result).toContain('alwaysApply: false');
      expect(result).toContain('## Your Responsibilities');
    });

    test('wraps plain yaml frontmatter for publishing templates', () => {
      const result = buildCursorFormat('publishing', 'libraries');
      expect(result).toMatch(/^---\n/);
      expect(result).toContain("description: 'Library publishing specialist");
      expect(result).toContain('\n---\n# Publishing Libraries Agent');
    });
  });

  describe('buildClaudeFormat', () => {
    test('combines header with content for linting', () => {
      const result = buildClaudeFormat('linting');
      expect(result).toContain('# TypeScript Linting Rules');
      expect(result).toContain('## Your Responsibilities');
      expect(result).not.toContain('mode: subagent');
    });
  });

  describe('buildOpenCodeFormat', () => {
    test('combines frontmatter with content for linting', () => {
      const result = buildOpenCodeFormat('linting');
      expect(result).toMatch(/^---\n/);
      expect(result).toContain('mode: subagent');
      expect(result).toContain('## Your Responsibilities');
    });

    test('wraps plain yaml frontmatter for publishing templates', () => {
      const result = buildOpenCodeFormat('publishing', 'apps');
      expect(result).toMatch(/^---\n/);
      expect(result).toContain('mode: subagent');
      expect(result).toContain('\n---\n# Publishing Apps Agent');
    });
  });

  describe('buildCodexFormat', () => {
    test('combines header with content for linting', () => {
      const result = buildCodexFormat('linting');
      expect(result).toContain('# TypeScript Linting Rules');
      expect(result).toContain('## Your Responsibilities');
    });

    test('includes smoke and E2E guidance in generated Codex rules', () => {
      const testing = buildCodexFormat('testing');
      expect(testing).toContain('web smoke test');
      expect(testing).toContain(
        'Prefer Playwright only when Playwright markers already exist, or when the repo has a real browser application surface and no existing browser E2E framework.'
      );
      expect(testing).toContain(
        'Run fast unit tests and targeted smoke checks during local work, put deterministic build/typecheck plus smoke checks in pre-push, and run full smoke/E2E gates in CI.'
      );

      const pythonTesting = buildCodexFormat('testing', undefined, 'python');
      expect(pythonTesting).toContain('web smoke test');
      expect(pythonTesting).toContain(
        'Prefer Playwright only when Playwright markers already exist, or when the repo has a real browser application surface and no existing browser E2E framework.'
      );
      expect(pythonTesting).toContain(
        'Run fast unit tests and targeted smoke checks during local work, put deterministic build/typecheck plus smoke checks in pre-push, and run full smoke/E2E gates in CI.'
      );

      const publishingCli = buildCodexFormat('publishing', 'cli');
      expect(publishingCli).toContain('packaged-command smoke');
      expect(publishingCli).toContain('--help');
      expect(publishingCli).toContain('--version');
      expect(publishingCli).toContain('representative command');
      expect(publishingCli).toContain(
        'Keep local packaged-command smoke checks fast, run them in pre-push when the packaged artifact can be built deterministically, and require them in CI before publish jobs.'
      );
    });

    test('includes framework detection guidance in generated Codex testing rules', () => {
      const testing = buildCodexFormat('testing');
      expect(testing).toContain('Cypress');
      expect(testing).toContain('WebdriverIO');
      expect(testing).toContain(
        'Preserve an existing browser E2E framework unless the user explicitly asks to migrate.'
      );
      expect(testing).toContain(
        'Prefer Playwright only when Playwright markers already exist, or when the repo has a real browser application surface and no existing browser E2E framework.'
      );

      const pythonTesting = buildCodexFormat('testing', undefined, 'python');
      expect(pythonTesting).toContain('Robot Framework');
      expect(pythonTesting).toContain('pytest-playwright');
      expect(pythonTesting).toContain(
        'Do not add browser E2E tooling to library-only, CLI-only, infrastructure-only, or backend-only repositories without a user-facing browser surface.'
      );

      const goTesting = buildCodexFormat('testing', undefined, 'go');
      expect(goTesting).toContain('chromedp');
      expect(goTesting).toContain('integration build tags');
      expect(goTesting).toContain(
        'Preserve an existing browser E2E framework unless the user explicitly asks to migrate.'
      );
    });

    test('includes TDD process discipline in generated Codex testing rules', () => {
      for (const language of ['typescript', 'python', 'go'] as const) {
        const testing = buildCodexFormat('testing', undefined, language);
        expect(testing).toContain('## TDD Process Discipline');
        expect(testing).toContain(
          'Tooling setup and process discipline are separate responsibilities'
        );
        expect(testing).toContain('Write a failing test first');
        expect(testing).toContain(
          'Confirm the test fails for the right reason'
        );
        expect(testing).toContain('Failure-path coverage');
        expect(testing).toContain('Traceability');
      }
    });
  });

  describe('getCodexRuleDescription', () => {
    test('reads description from cursor frontmatter', () => {
      const description = getCodexRuleDescription('linting');
      expect(description).toContain('TypeScript linting specialist');
    });
  });

  describe('extractDescriptionFromFrontmatter', () => {
    test('extracts single-line quoted description', () => {
      const frontmatter = `---
description: 'TypeScript linting specialist'
alwaysApply: false
---`;
      expect(extractDescriptionFromFrontmatter(frontmatter)).toBe(
        'TypeScript linting specialist'
      );
    });

    test('extracts multi-line literal block (|) description', () => {
      const frontmatter = `---
description: |
  First line
  Second line
alwaysApply: false
---`;
      expect(extractDescriptionFromFrontmatter(frontmatter)).toBe(
        'First line\nSecond line'
      );
    });

    test('extracts multi-line folded block (>) description', () => {
      const frontmatter = `---
description: >
  This is a folded
  block scalar
alwaysApply: false
---`;
      expect(extractDescriptionFromFrontmatter(frontmatter)).toBe(
        'This is a folded block scalar'
      );
    });

    test('handles single-quoted string with escaped single quote', () => {
      const frontmatter = `---
description: 'It''s great'
alwaysApply: false
---`;
      expect(extractDescriptionFromFrontmatter(frontmatter)).toBe("It's great");
    });
  });

  describe('buildCodexAgentsMd', () => {
    test('lists codex rule files with descriptions', () => {
      const content = buildCodexAgentsMd(['linting'], ['owasp-security-scan']);
      expect(content).toContain('# AGENTS.md');
      expect(content).toContain('## Repository Facts');
      expect(content).toContain('Canonical GitHub repo: `<OWNER/REPO>`');
      expect(content).toContain(
        'Prefer facts stored here over re-deriving them with shell commands on every task.'
      );
      expect(content).toMatch(
        /Created by \[Ballast]\(https:\/\/github\.com\/everydaydevopsio\/ballast\) v[0-9A-Za-z._-]+\. Do not edit this section\./
      );
      expect(content).toContain('`.codex/rules/typescript-linting.md`');
      expect(content).toContain('TypeScript linting specialist');
      expect(content).toContain('## Installed skills');
      expect(content).toContain('`.codex/skills/owasp-security-scan/SKILL.md`');
    });

    test('lists plan-lifecycle rule for codex', () => {
      const content = buildCodexAgentsMd(['plan-lifecycle']);
      expect(content).toContain('`.codex/rules/plan-lifecycle.md`');
      expect(content).toContain(
        'Plan lifecycle - create, maintain, and graduate plans to ADRs'
      );
    });

    test('lists only selected publishing profiles for codex', () => {
      const content = buildCodexAgentsMd(['publishing'], [], 'typescript', [
        'cli',
        'apps'
      ]);
      expect(content).toContain('`.codex/rules/publishing-cli.md`');
      expect(content).toContain('`.codex/rules/publishing-apps.md`');
      expect(content).not.toContain('`.codex/rules/publishing-web.md`');
      expect(content).not.toContain('`.codex/rules/publishing-api.md`');
      expect(content).not.toContain('`.codex/rules/publishing-libraries.md`');
      expect(content).not.toContain('`.codex/rules/publishing-sdks.md`');
      expect(content).not.toContain('`.codex/rules/publishing-apt.md`');
      expect(content).not.toContain('`.codex/rules/publishing-brew.md`');
    });

    test('includes repository tool policy once inside installed agent rules section', () => {
      const content = buildCodexAgentsMd(
        ['linting'],
        [],
        'typescript',
        undefined,
        {
          Python: ['UV', 'pyenv'],
          TypeScript: ['Pnpm', 'corepack']
        }
      );
      expect(content).toContain('### Repository Tool Policy');
      expect(content).toContain('python=uv,pyenv');
      expect(content).toContain('typescript=pnpm,corepack');
      expect(content).toContain('uv run <command>');
      expect(content).toContain('pnpm exec');
      const rulesIndex = content.indexOf('## Installed agent rules');
      const policyIndex = content.indexOf('### Repository Tool Policy');
      expect(policyIndex).toBeGreaterThan(rulesIndex);
      expect(content.match(/Repository Tool Policy/g)).toHaveLength(1);
    });

    test('omits repository tool policy when no tools configured', () => {
      const content = buildCodexAgentsMd(['linting'], [], 'typescript');
      expect(content).not.toContain('Repository Tool Policy');
    });
  });

  describe('buildClaudeMd', () => {
    test('lists claude rule files with descriptions', () => {
      const content = buildClaudeMd(['linting'], ['owasp-security-scan']);
      expect(content).toContain('# CLAUDE.md');
      expect(content).toContain('## Repository Facts');
      expect(content).toContain('Primary CI workflows: `<workflow filenames>`');
      expect(content).toMatch(
        /Created by \[Ballast]\(https:\/\/github\.com\/everydaydevopsio\/ballast\) v[0-9A-Za-z._-]+\. Do not edit this section\./
      );
      expect(content).toContain('`.claude/rules/typescript-linting.md`');
      expect(content).toContain('TypeScript linting specialist');
      expect(content).toContain('## Installed skills');
      expect(content).toContain('`.claude/skills/owasp-security-scan.skill`');
    });

    test('lists plan-lifecycle rule for claude', () => {
      const content = buildClaudeMd(['plan-lifecycle']);
      expect(content).toContain('`.claude/rules/plan-lifecycle.md`');
      expect(content).toContain(
        'Plan lifecycle - create, maintain, and graduate plans to ADRs'
      );
    });

    test('lists only selected publishing profiles for claude', () => {
      const content = buildClaudeMd(['publishing'], [], 'typescript', [
        'libraries'
      ]);
      expect(content).toContain('`.claude/rules/publishing-libraries.md`');
      expect(content).not.toContain('`.claude/rules/publishing-cli.md`');
      expect(content).not.toContain('`.claude/rules/publishing-apps.md`');
    });

    test('includes repository tool policy once inside installed agent rules section', () => {
      const content = buildClaudeMd(['linting'], [], 'typescript', undefined, {
        python: ['uv', 'pyenv']
      });
      expect(content).toContain('### Repository Tool Policy');
      expect(content).toContain('python=uv,pyenv');
      expect(content.indexOf('### Repository Tool Policy')).toBeGreaterThan(
        content.indexOf('## Installed agent rules')
      );
      expect(content.match(/Repository Tool Policy/g)).toHaveLength(1);
    });
  });

  describe('buildGeminiMd', () => {
    test('lists gemini rule files with repository facts, memory tiering, and skills', () => {
      const content = buildGeminiMd(['linting'], ['owasp-security-scan']);
      expect(content).toContain('# GEMINI.md');
      expect(content).toContain('## Repository Facts');
      expect(content).toContain('## Memory Tiering');
      expect(content).toMatch(
        /Created by \[Ballast]\(https:\/\/github\.com\/everydaydevopsio\/ballast\) v[0-9A-Za-z._-]+\. Do not edit this section\./
      );
      expect(content).toContain('## Installed agent rules');
      expect(content).toContain('`.gemini/rules/typescript-linting.md`');
      expect(content).toContain('TypeScript linting specialist');
      expect(content).toContain('## Installed skills');
      expect(content).toContain('`.gemini/rules/owasp-security-scan.md`');
    });

    test('includes repository tool policy once inside installed agent rules section', () => {
      const content = buildGeminiMd(['linting'], [], 'typescript', undefined, {
        python: ['uv', 'pyenv']
      });
      expect(content).toContain('### Repository Tool Policy');
      expect(content).toContain('python=uv,pyenv');
      expect(content.indexOf('### Repository Tool Policy')).toBeGreaterThan(
        content.indexOf('## Installed agent rules')
      );
      expect(content.match(/Repository Tool Policy/g)).toHaveLength(1);
    });
  });

  describe('buildContent', () => {
    test('cursor returns mdc-style content', () => {
      const result = buildContent('linting', 'cursor');
      expect(result).toMatch(/^---\n/);
      expect(result).toContain('globs:');
    });

    test('pre-commit typescript linting content no longer includes hook guidance', () => {
      const result = buildContent('linting', 'codex', undefined, 'typescript', {
        hookMode: 'pre-commit'
      });
      expect(result).not.toContain(
        'Use `pre-commit` for this repository layout.'
      );
      expect(result).not.toContain('Install hooks with `pre-commit install`.');
      expect(result).not.toContain(
        'Install the pre-push hook with `pre-commit install --hook-type pre-push`.'
      );
      expect(result).not.toContain('Set Up Git Hooks with Husky');
      expect(result).not.toContain(
        'Use Husky for TypeScript-only repositories.'
      );
      expect(result).not.toContain('Configure lint-staged');
    });

    test('typescript git-hooks content is husky based', () => {
      const result = buildContent(
        'git-hooks',
        'codex',
        undefined,
        'typescript',
        {
          hookMode: 'husky'
        }
      );
      expect(result).toContain('Use Husky for TypeScript-only repositories.');
      expect(result).toContain('npx lint-staged');
      expect(result).toContain('.yaml');
      expect(result).toContain('.yml');
      expect(result).toContain('.husky/pre-push');
      expect(result).toContain('package-manager test command');
      expect(result).toContain('build or typecheck');
      expect(result).not.toContain('pre-commit install');
    });

    test('claude returns header + content', () => {
      const result = buildContent('linting', 'claude');
      expect(result).toContain('# TypeScript Linting Rules');
    });

    test('opencode returns yaml frontmatter + content', () => {
      const result = buildContent('linting', 'opencode');
      expect(result).toContain('permission:');
    });

    test('codex returns header + content', () => {
      const result = buildContent('linting', 'codex');
      expect(result).toContain('# TypeScript Linting Rules');
    });

    test('manifest-bearing targets omit the repository tool policy from rules', () => {
      for (const target of ['claude', 'codex', 'gemini'] as const) {
        const result = buildContent('testing', target, undefined, 'python', {
          tools: {
            Python: ['UV', 'pyenv'],
            TypeScript: ['Pnpm', 'corepack']
          }
        });
        expect(result).not.toContain('Repository Tool Policy');
      }
    });

    test('cursor and opencode rules keep the configured repository tools policy', () => {
      for (const target of ['cursor', 'opencode'] as const) {
        const result = buildContent('testing', target, undefined, 'python', {
          tools: {
            Python: ['UV', 'pyenv'],
            TypeScript: ['Pnpm', 'corepack']
          }
        });
        expect(result).toContain('## Repository Tool Policy');
        expect(result).toContain('python=uv,pyenv');
        expect(result).toContain('typescript=pnpm,corepack');
        expect(result).toContain('uv run <command>');
        expect(result).toContain('pnpm exec');
      }
    });

    test('throws for unknown target', () => {
      expect(() => buildContent('linting', 'unknown' as 'cursor')).toThrow(
        /Unknown target/
      );
    });

    test('tasks task-system with variables resolves {{taskSystem}} in templates', () => {
      for (const target of ['cursor', 'claude', 'opencode', 'codex'] as const) {
        const result = buildContent(
          'tasks',
          target,
          'task-system',
          'typescript',
          {
            variables: { taskSystem: 'jira' }
          }
        );
        expect(result).not.toContain('{{taskSystem}}');
        expect(result).toContain('jira');
      }
    });
  });

  describe('getDestination', () => {
    const projectRoot = path.join(__dirname, '..', 'fixtures', 'project');

    test('cursor returns .cursor/rules/<agent>.mdc', () => {
      const { dir, file } = getDestination('linting', 'cursor', projectRoot);
      expect(dir).toBe(path.join(projectRoot, '.cursor', 'rules'));
      expect(file).toBe(
        path.join(projectRoot, '.cursor', 'rules', 'typescript-linting.mdc')
      );
    });

    test('claude returns .claude/rules/<agent>.md', () => {
      const { dir, file } = getDestination('linting', 'claude', projectRoot);
      expect(dir).toBe(path.join(projectRoot, '.claude', 'rules'));
      expect(file).toBe(
        path.join(projectRoot, '.claude', 'rules', 'typescript-linting.md')
      );
    });

    test('opencode returns .opencode/<agent>.md', () => {
      const { dir, file } = getDestination('linting', 'opencode', projectRoot);
      expect(dir).toBe(path.join(projectRoot, '.opencode'));
      expect(file).toBe(
        path.join(projectRoot, '.opencode', 'typescript-linting.md')
      );
    });

    test('codex returns .codex/rules/<agent>.md', () => {
      const { dir, file } = getDestination('linting', 'codex', projectRoot);
      expect(dir).toBe(path.join(projectRoot, '.codex', 'rules'));
      expect(file).toBe(
        path.join(projectRoot, '.codex', 'rules', 'typescript-linting.md')
      );
    });

    test('codex agents.md path returns project root AGENTS.md', () => {
      const agentsMd = getCodexAgentsMdPath(projectRoot);
      expect(agentsMd).toBe(path.join(projectRoot, 'AGENTS.md'));
    });

    test('claude md path returns project root CLAUDE.md', () => {
      const claudeMd = getClaudeMdPath(projectRoot);
      expect(claudeMd).toBe(path.join(projectRoot, 'CLAUDE.md'));
    });

    test('cursor with ruleSuffix returns .cursor/rules/<agent>-<suffix>.mdc', () => {
      const { dir, file } = getDestination(
        'local-dev',
        'cursor',
        projectRoot,
        'env'
      );
      expect(dir).toBe(path.join(projectRoot, '.cursor', 'rules'));
      expect(file).toBe(
        path.join(projectRoot, '.cursor', 'rules', 'local-dev-env.mdc')
      );
    });

    test('claude with plan-lifecycle returns common rule path', () => {
      const { dir, file } = getDestination(
        'plan-lifecycle',
        'claude',
        projectRoot
      );
      expect(dir).toBe(path.join(projectRoot, '.claude', 'rules'));
      expect(file).toBe(
        path.join(projectRoot, '.claude', 'rules', 'plan-lifecycle.md')
      );
    });

    test('throws for unknown target', () => {
      expect(() =>
        getDestination('linting', 'unknown' as 'cursor', projectRoot)
      ).toThrow(/Unknown target/);
    });

    test('rejects invalid BALLAST_RULE_SUBDIR values', () => {
      process.env.BALLAST_RULE_SUBDIR = '../escape';
      expect(() => getDestination('linting', 'codex', projectRoot)).toThrow(
        /Invalid BALLAST_RULE_SUBDIR/
      );
      delete process.env.BALLAST_RULE_SUBDIR;
    });

    test('emits only language-specific rules in language rule subdirs', () => {
      expect(shouldEmitRuleForSubdir('cicd', 'typescript')).toBe(false);
      expect(shouldEmitRuleForSubdir('publishing', 'typescript')).toBe(false);
      expect(shouldEmitRuleForSubdir('linting', 'typescript')).toBe(true);
      expect(shouldEmitRuleForSubdir('testing', 'typescript')).toBe(true);
      expect(shouldEmitRuleForSubdir('cicd', 'common')).toBe(true);
      expect(shouldEmitRuleForSubdir('linting', 'common')).toBe(false);
    });
  });

  describe('listTargets', () => {
    test('returns cursor, claude, opencode, codex, gemini', () => {
      expect(listTargets()).toEqual([
        'cursor',
        'claude',
        'opencode',
        'codex',
        'gemini'
      ]);
    });
  });
});

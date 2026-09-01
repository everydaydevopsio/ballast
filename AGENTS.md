# AGENTS.md

This file provides shared repository guidance for agent tools that read AGENTS.md.

## Repository Facts

Use this section for durable repo-specific facts that agents repeatedly need. Prefer facts stored here over re-deriving them with shell commands on every task.

Keep only stable, reviewable metadata here. Do not store secrets, credentials, or ephemeral runtime state.

Suggested facts to record:

- Canonical GitHub repo: `everydaydevopsio/ballast`
- Default branch: `main`
- Primary package manager: `pnpm`
- Version-file locations agents should check first: `.nvmrc, package.json`
- Canonical config files: `.prettierrc`
- Primary CI workflows: `ci.yml`
- Primary release/publish workflows: `publish-cli.yml, publish-go.yml, publish-python.yml, publish.typescript.yml, publish.yml`
- Preferred build/test/lint/format/coverage commands: `make build, package.json:test, package.json:lint, package.json:build`
- Coverage threshold: `<value>`
- Generated or protected paths agents should avoid editing directly: `dist/, coverage/`

Update this section when those facts change. If live runtime state is required, discover it separately instead of treating it as a durable repo fact.

- Root `.rulesrc.json` targets are repo policy. Keep them aligned with every checked-in Ballast-managed target surface.
- Agent and skill registries are duplicated across generated backend packages. When adding, renaming, or removing `agents/common/*`, language agent directories, or `skills/common/*`, update every backend registry and keep parity tests that compare registries to packaged content directories passing.
- Do not edit checked-in `.claude/` or `.codex/` generated rule outputs directly. Change the source templates/content under repo-root `agents/` and `skills/` instead.
- Checked-in `.claude/` and `.codex/` generated outputs are created by running `ballast upgrade --patch`; use that command to regenerate them after repo-root `agents/`, `skills/`, Ballast sync/build scripts, or root target config change.

## Installed agent rules

Created by Ballast. Do not edit this section.

### Repository Tool Policy

- Check `.rulesrc.json` `tools` before adding, installing, or running language tooling.
- Configured tools: docker=docker,hadolint,trivy; go=go,gofumpt,golangci-lint; python=uv,pyenv; typescript=pnpm,corepack.
- For Python commands, prefer `uv run <command>` and `uv add ...` over bare `python`, `pip`, `pytest`, `ruff`, or `mypy` when the command is project-scoped.
- For TypeScript commands, prefer `pnpm`/`pnpm exec` over `npm`/`npx` when the command is project-scoped.

Read and follow these rule files in `.codex/rules/` when they apply:

- `.codex/rules/common/local-dev-badges.md` — Rules for common/local-dev-badges
- `.codex/rules/common/local-dev-env.md` — Rules for common/local-dev-env
- `.codex/rules/common/local-dev-license.md` — Rules for common/local-dev-license
- `.codex/rules/common/docs.md` — Rules for common/docs
- `.codex/rules/common/cicd.md` — Rules for common/cicd
- `.codex/rules/common/observability.md` — Rules for common/observability
- `.codex/rules/common/publishing-api.md` — Rules for common/publishing-api
- `.codex/rules/common/publishing-apps.md` — Rules for common/publishing-apps
- `.codex/rules/common/publishing-cli.md` — Rules for common/publishing-cli
- `.codex/rules/common/publishing-libraries.md` — Rules for common/publishing-libraries
- `.codex/rules/common/publishing-sdks.md` — Rules for common/publishing-sdks
- `.codex/rules/common/publishing-web.md` — Rules for common/publishing-web
- `.codex/rules/common/git-hooks.md` — Rules for common/git-hooks
- `.codex/rules/common/plan-lifecycle.md` — Rules for common/plan-lifecycle
- `.codex/rules/common/tasks-task-system.md` — Rules for common/tasks-task-system
- `.codex/rules/common/tasks-todo.md` — Rules for common/tasks-todo
- `.codex/rules/common/spec-kit.md` — Rules for common/spec-kit
- `.codex/rules/typescript/typescript-linting.md` — Rules for typescript/linting
- `.codex/rules/typescript/typescript-logging.md` — Rules for typescript/logging
- `.codex/rules/typescript/typescript-testing.md` — Rules for typescript/testing
- `.codex/rules/python/python-linting.md` — Rules for python/linting
- `.codex/rules/python/python-logging.md` — Rules for python/logging
- `.codex/rules/python/python-testing.md` — Rules for python/testing
- `.codex/rules/go/go-linting.md` — Rules for go/linting
- `.codex/rules/go/go-logging.md` — Rules for go/logging
- `.codex/rules/go/go-testing.md` — Rules for go/testing
- `.codex/rules/docker/docker-linting.md` — Rules for docker/linting
- `.codex/rules/docker/docker-logging.md` — Rules for docker/logging
- `.codex/rules/docker/docker-testing.md` — Rules for docker/testing

## Installed skills

Created by Ballast. Do not edit this section.

Read and use these skill files in `.codex/skills/` when they are relevant:

- `.codex/skills/owasp-security-scan/SKILL.md` — run an OWASP-aligned security audit across Go, TypeScript, and Python projects
- `.codex/skills/aws-health-review/SKILL.md` — run a weekly read-only AWS health review covering configuration, performance, errors, and warnings
- `.codex/skills/aws-live-health-review/SKILL.md` — run a read-only AWS live health review for current EC2, RDS, ALB, CloudWatch alarms, and logs
- `.codex/skills/aws-weekly-security-review/SKILL.md` — run a weekly read-only AWS security baseline review and generate a prioritized findings report
- `.codex/skills/github-health-check/SKILL.md` — run a comprehensive GitHub repository health check covering CI status, code quality, branch hygiene, and repo configuration
- `.codex/skills/github-pr-copilot-cycle/SKILL.md` — create or update a GitHub PR, request Copilot review, triage and fix Copilot comments, push fixes, check CI, and repeat up to three cycles
- `.codex/skills/ballast-audit/SKILL.md` — audit AI rule and skill files for context density, duplication, and bloat
- `.codex/skills/ballast-project-maintenance/SKILL.md` — inspect, bootstrap, and repair Ballast-managed repository state including .ballast/ local tools
- `.codex/skills/speckit-bootstrap/SKILL.md` — initialize or repair GitHub Spec Kit in an existing repository using native agent skills
- `.codex/skills/speckit-reverse-engineer/SKILL.md` — reverse-engineer an existing application into a high-level GitHub Spec Kit baseline
- `.codex/skills/speckit-delivery/SKILL.md` — orchestrate GitHub Spec Kit's native skills for a bounded product change

## Codex code review expectations

- Follow `docs/code_review.md` for code reviews.
- Reviews should prioritize correctness, security, regressions, missing tests, and maintainability.
- Avoid style-only feedback unless it hides a defect or contradicts this repository's formatter/linter rules.
- For implementation tasks, run the smallest relevant test, lint, or type-check command before reporting completion.
- Never include secrets in prompts, commits, logs, or generated documentation.

### Review severity guidelines

- Flag P0/P1 issues only when there is a concrete failure mode or credible risk.
- Treat missing tests as P1 when the change affects behavior, auth, billing, persistence, migrations, concurrency, permissions, or user-visible output.
- Treat documentation gaps as P1 only when the change alters setup, public APIs, release/deploy steps, or user-visible behavior.
- Use P3/Nit only for polish, and never block merge on personal preference.
<!-- END CODEX REVIEWER INSTALLER: agents-review-expectations -->

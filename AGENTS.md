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
- Install behavior lives in four places, not one. `packages/ballast-typescript/src/{build,install}.ts`, `packages/ballast-go/cmd/ballast-go/main.go`, and `packages/ballast-python/ballast/cli.py` each independently decide where a rule or skill is written, what it contains, and how `--patch` treats it; `cli/ballast/main.go` owns the wrapper's own copies of those paths, its stale-file removal, and its manifest text. Any change to a destination path, emitted content, patch semantics, manifest wording, or legacy-path cleanup must land in all four. Which backend runs depends on the repository's languages, so a TypeScript-only change passes every TypeScript test while leaving Go, Python, Ansible, Terraform, Dart, and Docker repositories broken.
- The e2e and smoke scripts under `scripts/` invoke a bare `ballast` resolved from `PATH`. Run them without putting the freshly built binaries first and they silently exercise the installed release instead of the working tree, reporting a pass that means nothing. Reproduce the CI wiring before trusting a local run: build with `pnpm run build` and `make build-go build-cli`, then place `ballast`, `ballast-go`, and shims for `ballast-typescript` and `ballast-python` in a directory prepended to `PATH` (see the `Create local backend shims` step in `.github/workflows/examples-smoke.yml`). `SKIP_BUILD=1` reuses whatever was built last, so drop it after editing a backend.
- Skills are entirely Ballast-authored and every backend replaces them wholesale. Do not reintroduce section-merging for skills: heading names cannot distinguish a team-authored section from one upstream deleted, so merging both keeps stale text for headings that still exist and resurrects sections that were removed. Rules still merge under `--patch` to preserve user-authored sections.
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

- `.codex/rules/common/local-dev-autonomy.md` — Rules for common/local-dev-autonomy
- `.codex/rules/common/local-dev-badges.md` — Rules for common/local-dev-badges
- `.codex/rules/common/local-dev-env.md` — Rules for common/local-dev-env
- `.codex/rules/common/local-dev-license.md` — Rules for common/local-dev-license
- `.codex/rules/common/docs.md` — Rules for common/docs
- `.codex/rules/common/cicd.md` — Rules for common/cicd
- `.codex/rules/common/observability.md` — Rules for common/observability
- `.codex/rules/common/publishing.md` — Rules for common/publishing
- `.codex/rules/common/publishing-cli.md` — Rules for common/publishing-cli
- `.codex/rules/common/publishing-libraries.md` — Rules for common/publishing-libraries
- `.codex/rules/common/git-hooks.md` — Rules for common/git-hooks
- `.codex/rules/common/plan-lifecycle.md` — Rules for common/plan-lifecycle
- `.codex/rules/common/tasks-task-system.md` — Rules for common/tasks-task-system
- `.codex/rules/common/tasks-todo.md` — Rules for common/tasks-todo
- `.codex/rules/common/spec-kit.md` — Rules for common/spec-kit
- `.codex/rules/common/testing-process.md` — Rules for common/testing-process
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
- `.codex/skills/ballast-audit/SKILL.md` — audit a Ballast installation for stale, unowned, oversized, and irrelevant rules and skills, and report the narrowest config that still covers the repository
- `.codex/skills/agent-performance-audit/SKILL.md` — use distilled bridgectl agent-performance findings to audit Ballast rules and skills for evidence-backed improvements
- `.codex/skills/ballast-project-maintenance/SKILL.md` — inspect, bootstrap, and repair Ballast-managed repository state including .ballast/ local tools
- `.codex/skills/speckit-bootstrap/SKILL.md` — initialize or repair GitHub Spec Kit in an existing repository using native agent skills
- `.codex/skills/speckit-reverse-engineer/SKILL.md` — reverse-engineer an existing application into a high-level GitHub Spec Kit baseline
- `.codex/skills/speckit-delivery/SKILL.md` — orchestrate GitHub Spec Kit's native skills for a bounded product change
- `.codex/skills/docker-registry-publish/SKILL.md` — set up Docker image publishing to GHCR or Docker Hub with public or private registry visibility

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

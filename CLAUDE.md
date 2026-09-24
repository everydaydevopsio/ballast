# CLAUDE.md

This file provides guidance to Claude Code for working in this repository.

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
- Editing repo-root `skills/` or `agents/` is only half the change. Each backend serves that content from a packaged copy under `packages/*` (`packages/ballast-typescript/skills/`, `packages/ballast-go/cmd/*/skills/`, `packages/ballast-python/ballast/skills/`), all tracked in git. Run `pnpm --filter @everydaydevopsio/ballast run sync:content` and commit the result, then regenerate `.claude/` and `.codex/`. Local dev resolves to the repo root and hides the gap, so tests pass while the packaged copies stay stale; `prepack` syncs at publish time, so the published package is right while the repository's own artifacts are wrong.
- Install behavior lives in four places, not one. `packages/ballast-typescript/src/{build,install}.ts`, `packages/ballast-go/cmd/ballast-go/main.go`, and `packages/ballast-python/ballast/cli.py` each independently decide where a rule or skill is written, what it contains, and how `--patch` treats it; `cli/ballast/main.go` owns the wrapper's own copies of those paths, its stale-file removal, and its manifest text. Any change to a destination path, emitted content, patch semantics, manifest wording, or legacy-path cleanup must land in all four. Which backend runs depends on the repository's languages, so a TypeScript-only change passes every TypeScript test while leaving Go, Python, Ansible, Terraform, Dart, and Docker repositories broken.
- The e2e and smoke scripts under `scripts/` invoke a bare `ballast` resolved from `PATH`. Run them without putting the freshly built binaries first and they silently exercise the installed release instead of the working tree, reporting a pass that means nothing. Reproduce the CI wiring before trusting a local run: build with `pnpm run build` and `make build-go build-cli`, then place `ballast`, `ballast-go`, and shims for `ballast-typescript` and `ballast-python` in a directory prepended to `PATH` (see the `Create local backend shims` step in `.github/workflows/examples-smoke.yml`). `SKIP_BUILD=1` reuses whatever was built last, so drop it after editing a backend.
- Skills are entirely Ballast-authored and every backend replaces them wholesale. Do not reintroduce section-merging for skills: heading names cannot distinguish a team-authored section from one upstream deleted, so merging both keeps stale text for headings that still exist and resurrects sections that were removed. Rules still merge under `--patch` to preserve user-authored sections.
- Do not edit checked-in `.claude/` or `.codex/` generated rule outputs directly. Change the source templates/content under repo-root `agents/` and `skills/`, then regenerate the local Ballast-managed outputs.
- When repo-root `agents/`, `skills/`, Ballast sync/build scripts, or root target config change, regenerate and commit the corresponding local Ballast-managed `.claude/` and `.codex/` outputs in the same PR.

## Installed agent rules

Created by Ballast. Do not edit this section.

### Repository Tool Policy

- Check `.rulesrc.json` `tools` before adding, installing, or running language tooling.
- Configured tools: docker=docker,hadolint,trivy; go=go,gofumpt,golangci-lint; python=uv,pyenv; typescript=pnpm,corepack.
- For Python commands, prefer `uv run <command>` and `uv add ...` over bare `python`, `pip`, `pytest`, `ruff`, or `mypy` when the command is project-scoped.
- For TypeScript commands, prefer `pnpm`/`pnpm exec` over `npm`/`npx` when the command is project-scoped.

Read and follow these rule files in `.claude/rules/` when they apply:

- `.claude/rules/common/local-dev-autonomy.md` — Rules for common/local-dev-autonomy
- `.claude/rules/common/local-dev-badges.md` — Rules for common/local-dev-badges
- `.claude/rules/common/local-dev-env.md` — Rules for common/local-dev-env
- `.claude/rules/common/local-dev-license.md` — Rules for common/local-dev-license
- `.claude/rules/common/docs.md` — Rules for common/docs
- `.claude/rules/common/cicd.md` — Rules for common/cicd
- `.claude/rules/common/observability.md` — Rules for common/observability
- `.claude/rules/common/publishing.md` — Rules for common/publishing
- `.claude/rules/common/publishing-cli.md` — Rules for common/publishing-cli
- `.claude/rules/common/publishing-libraries.md` — Rules for common/publishing-libraries
- `.claude/rules/common/git-hooks.md` — Rules for common/git-hooks
- `.claude/rules/common/plan-lifecycle.md` — Rules for common/plan-lifecycle
- `.claude/rules/common/tasks-task-system.md` — Rules for common/tasks-task-system
- `.claude/rules/common/tasks-todo.md` — Rules for common/tasks-todo
- `.claude/rules/common/spec-kit.md` — Rules for common/spec-kit
- `.claude/rules/common/testing-process.md` — Rules for common/testing-process
- `.claude/rules/typescript/typescript-linting.md` — Rules for typescript/linting
- `.claude/rules/typescript/typescript-logging.md` — Rules for typescript/logging
- `.claude/rules/typescript/typescript-testing.md` — Rules for typescript/testing
- `.claude/rules/python/python-linting.md` — Rules for python/linting
- `.claude/rules/python/python-logging.md` — Rules for python/logging
- `.claude/rules/python/python-testing.md` — Rules for python/testing
- `.claude/rules/go/go-linting.md` — Rules for go/linting
- `.claude/rules/go/go-logging.md` — Rules for go/logging
- `.claude/rules/go/go-testing.md` — Rules for go/testing
- `.claude/rules/docker/docker-linting.md` — Rules for docker/linting
- `.claude/rules/docker/docker-logging.md` — Rules for docker/logging
- `.claude/rules/docker/docker-testing.md` — Rules for docker/testing

## Installed skills

Created by Ballast. Do not edit this section.

These skills are registered with Claude Code. Invoke one by name (for example `/ballast-audit`) when it is relevant:

- `/owasp-security-scan` — run an OWASP-aligned security audit across Go, TypeScript, and Python projects
- `/aws-health-review` — run a weekly read-only AWS health review covering configuration, performance, errors, and warnings
- `/aws-live-health-review` — run a read-only AWS live health review for current EC2, RDS, ALB, CloudWatch alarms, and logs
- `/aws-weekly-security-review` — run a weekly read-only AWS security baseline review and generate a prioritized findings report
- `/github-health-check` — run a comprehensive GitHub repository health check covering CI status, code quality, branch hygiene, and repo configuration
- `/github-pr-copilot-cycle` — create or update a GitHub PR, request Copilot review, triage and fix Copilot comments, push fixes, check CI, and repeat up to three cycles
- `/ballast-audit` — audit a Ballast installation for stale, unowned, oversized, and irrelevant rules and skills, and report the narrowest config that still covers the repository
- `/agent-performance-audit` — use distilled bridgectl agent-performance findings to audit Ballast rules and skills for evidence-backed improvements
- `/ballast-project-maintenance` — inspect, bootstrap, and repair Ballast-managed repository state including .ballast/ local tools
- `/speckit-bootstrap` — initialize or repair GitHub Spec Kit in an existing repository using native agent skills
- `/speckit-reverse-engineer` — reverse-engineer an existing application into a high-level GitHub Spec Kit baseline
- `/speckit-delivery` — orchestrate GitHub Spec Kit's native skills for a bounded product change
- `/docker-registry-publish` — set up Docker image publishing to GHCR or Docker Hub with public or private registry visibility

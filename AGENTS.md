# AGENTS.md

This file provides shared repository guidance for agent tools that read AGENTS.md.

## Repository Facts

Use this section for durable repo-specific facts that agents repeatedly need. Prefer facts stored here over re-deriving them with shell commands on every task.

Keep only stable, reviewable metadata here. Do not store secrets, credentials, or ephemeral runtime state.

Suggested facts to record:

- Canonical GitHub repo: `<OWNER/REPO>`
- Default branch: `<main>`
- Primary package manager: `<pnpm | npm | yarn | uv | go>`
- Version-file locations agents should check first: `<.nvmrc, packageManager, pyproject.toml, go.mod, etc.>`
- Canonical config files: `<paths agents should read before falling back to discovery>`
- Primary CI workflows: `<workflow filenames>`
- Primary release/publish workflows: `<workflow filenames>`
- Preferred build/test/lint/format/coverage commands: `<commands>`
- Coverage threshold: `<value>`
- Generated or protected paths agents should avoid editing directly: `<paths>`

Update this section when those facts change. If live runtime state is required, discover it separately instead of treating it as a durable repo fact.

- Root `.rulesrc.json` targets are repo policy. Keep them aligned with every checked-in Ballast-managed target surface.
- When repo-root `agents/`, `skills/`, Ballast sync/build scripts, or root target config change, regenerate and commit the corresponding local Ballast-managed `.claude/` and `.codex/` outputs in the same PR.

## Installed agent rules

Created by Ballast. Do not edit this section.

Read and follow these rule files in `.codex/rules/` when they apply:

- `.codex/rules/common/local-dev-badges.md` — Rules for common/local-dev-badges
- `.codex/rules/common/local-dev-env.md` — Rules for common/local-dev-env
- `.codex/rules/common/local-dev-license.md` — Rules for common/local-dev-license
- `.codex/rules/common/local-dev-mcp.md` — Rules for common/local-dev-mcp
- `.codex/rules/common/docs.md` — Rules for common/docs
- `.codex/rules/common/cicd.md` — Rules for common/cicd
- `.codex/rules/common/observability.md` — Rules for common/observability
- `.codex/rules/common/publishing-libraries.md` — Rules for common/publishing-libraries
- `.codex/rules/common/publishing-sdks.md` — Rules for common/publishing-sdks
- `.codex/rules/common/publishing-apps.md` — Rules for common/publishing-apps
- `.codex/rules/common/git-hooks.md` — Rules for common/git-hooks
- `.codex/rules/common/tasks.md` — Rules for common/tasks
- `.codex/rules/typescript/typescript-linting.md` — Rules for typescript/linting
- `.codex/rules/typescript/typescript-logging.md` — Rules for typescript/logging
- `.codex/rules/typescript/typescript-testing.md` — Rules for typescript/testing
- `.codex/rules/python/python-linting.md` — Rules for python/linting
- `.codex/rules/python/python-logging.md` — Rules for python/logging
- `.codex/rules/python/python-testing.md` — Rules for python/testing
- `.codex/rules/go/go-linting.md` — Rules for go/linting
- `.codex/rules/go/go-logging.md` — Rules for go/logging
- `.codex/rules/go/go-testing.md` — Rules for go/testing
- `.codex/rules/ansible/ansible-linting.md` — Rules for ansible/linting
- `.codex/rules/ansible/ansible-logging.md` — Rules for ansible/logging
- `.codex/rules/ansible/ansible-testing.md` — Rules for ansible/testing
- `.codex/rules/terraform/terraform-linting.md` — Rules for terraform/linting
- `.codex/rules/terraform/terraform-logging.md` — Rules for terraform/logging
- `.codex/rules/terraform/terraform-testing.md` — Rules for terraform/testing

## Installed skills

Created by Ballast. Do not edit this section.

Read and use these skill files in `.codex/rules/` when they are relevant:

- `.codex/rules/owasp-security-scan.md` — run an OWASP-aligned security audit across Go, TypeScript, and Python projects
- `.codex/rules/github-health-check.md` — run a comprehensive GitHub repository health check covering CI status, code quality, branch hygiene, and repo configuration
- `.codex/rules/aws-health-review.md` — run a weekly read-only AWS health review covering configuration, performance, errors, and warnings
- `.codex/rules/aws-live-health-review.md` — run a read-only AWS live health review for current EC2, RDS, ALB, CloudWatch alarms, and logs
- `.codex/rules/aws-weekly-security-review.md` — run a weekly read-only AWS security baseline review and generate a prioritized findings report
- `.codex/rules/ballast-audit.md` — audit AI rule and skill files for context density, duplication, and bloat


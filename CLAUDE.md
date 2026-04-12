# CLAUDE.md

This file provides guidance to Claude Code for working in this repository.

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

## Installed agent rules

Created by [Ballast](https://github.com/everydaydevopsio/ballast) v5.6.5. Do not edit this section.

Read and follow these rule files in `.claude/rules/` when they apply:

- `.claude/rules/local-dev-badges.md` — Add standard badges (CI, Release, License, GitHub Release, npm) to the top of README.md
- `.claude/rules/local-dev-env.md` — Local development environment specialist - reproducible dev setup, DX, and documentation
- `.claude/rules/local-dev-license.md` — License setup - ensure LICENSE file, package.json license field, and README reference (default MIT; overridable in AGENTS.md/CLAUDE.md)
- `.claude/rules/local-dev-mcp.md` — Optional: use GitHub MCP and issues MCP (Jira/Linear/GitHub) for local-dev context
- `.claude/rules/docs.md` — Documentation specialist - GitHub Markdown docs by default, or maintain existing Docusaurus sites with publish-docs automation
- `.claude/rules/cicd.md` — CI/CD specialist - pipeline design, quality gates, and deployment
- `.claude/rules/observability.md` — Observability specialist - logging, tracing, metrics, and SLOs
- `.claude/rules/publishing-apps.md` — App publishing specialist - npmjs for Node apps, PyPI for Python apps, GitHub Releases for Go apps
- `.claude/rules/publishing-libraries.md` — Library publishing specialist - npmjs for TypeScript, PyPI for Python, GitHub tags/releases for Go
- `.claude/rules/publishing-sdks.md` — SDK publishing specialist - npmjs for TypeScript SDKs, PyPI for Python SDKs, GitHub tags/releases for Go SDKs
- `.claude/rules/git-hooks.md` — Git hook specialist - configure pre-commit, pre-push, and Husky workflows that match the repository layout
- `.claude/rules/typescript-linting.md` — TypeScript linting specialist - implements comprehensive linting and code formatting for TypeScript/JavaScript projects
- `.claude/rules/typescript-logging.md` — Centralized logging specialist - configures Pino with Fluentd for Node/Next.js, and pino-browser to /api/logs
- `.claude/rules/typescript-testing.md` — Testing specialist - sets up Jest (default) or Vitest for Vite projects, 50% coverage, and test step in build GitHub Action

## Installed skills

Created by [Ballast](https://github.com/everydaydevopsio/ballast) v5.6.5. Do not edit this section.

Read and use these skill files in `.claude/skills/` when they are relevant:

- `.claude/skills/owasp-security-scan.skill` — Run OWASP-aligned security scans across Go, TypeScript, and Python codebases. Use this skill whenever the user asks to: scan for security vulnerabilities, run OWASP checks, audit dependencies, find CVEs, check for injection flaws, run SAST or SCA analysis, review code security, or harden their app against the OWASP Top 10. Also trigger for phrases like "security audit", "check my code for vulns", "are my dependencies safe", or any mention of gosec, bandit, semgrep, or npm audit in a security context. Covers Go, TypeScript/JavaScript, and Python with language-specific tools plus cross-language Semgrep rulesets.
- `.claude/skills/github-health-check.skill` — Run a comprehensive GitHub repository health check. Use this skill whenever the user asks to: check GitHub health, audit the repo, check CI status, review open PRs, merge Dependabot PRs, check code coverage, check security alerts, check Snyk integration, keep GitHub in good shape, or any variation of "how is the repo doing". Also trigger for: "check dependabot PRs", "any PRs to merge", "check branch status", "repo health", "GitHub status check", "what needs attention in GitHub", "tidy up GitHub".

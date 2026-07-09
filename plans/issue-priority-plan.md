# Plan: Open Issue Review and Next Priorities

**Status:** Updated after #211 and #144 closure
**Created:** 2026-07-09
**Source:** Review of open GitHub issues and requested follow-up list

## Summary

Reviewed all open GitHub issues for `everydaydevopsio/ballast` and compared them against the requested follow-up list. Several items were already covered by existing issues. Eight gaps were created as new GitHub issues.

## Completed Work

- #212: Require agents to create a new branch before modifying code — completed and merged via PR #216.
- #211: Add a development-environment bootstrap command for AI agents — implemented and merged via PR #217.
  - Added `ballast setup-dev` to bootstrap agent development environments from the project root.
  - Detects Node package managers from `package.json` and lockfiles, enables Corepack for `pnpm` and `yarn`, runs the appropriate install command, and exits successfully when no setup steps are detected.
  - Hardened package-manager execution with an allowlist so untrusted `packageManager` values are ignored.
  - Added focused Go tests for declared package managers, lockfile fallback, no-op behavior, unsafe package-manager values, and failure output.
  - Updated `PRD.md`, README/docs, local-dev guidance, generated `.claude/` and `.codex/` rule outputs, and package payload copies.
- #208: Define .ballast bootstrap behavior and Ballast skill support — implemented on branch `issue-208-ballast-bootstrap-skill`.
  - Defined `.ballast/` as generated repository-local tool state that is ignored by git and safe to recreate.
  - Added `ballast doctor` local-state reporting for missing and incomplete `.ballast/`, `.ballast/bin`, and `.ballast/tools`.
  - Added remediation guidance pointing operators to `ballast doctor --fix` or `ballast install-cli`.
  - Added `ballast-project-maintenance` as a common skill for Ballast repository status, bootstrap, and repair workflows.
  - Updated docs, local-dev guidance, package mirrors, generated local `.claude/` and `.codex/` outputs, and focused tests.
- #144: Bug: ballast upgrade refresh skips installed skills instead of updating them — implemented on `main` and closed after verification.
  - Config-refresh flows now set managed skill refresh mode so `install --refresh-config`, `upgrade`, and `doctor --fix` rewrite selected skill files without requiring `--force`.
  - Ordinary install behavior still skips existing skill files unless `--patch` or `--force` is selected.
  - Verified with TypeScript, Python, Go, wrapper unit tests, and the upgrade-refresh smoke script.

## Open Issue Inventory

As of 2026-07-09 after closing completed #211 and #144, the repository has these open issues:

- #215: Define Kubernetes deployment guidance with local Helm chart and external ArgoCD config
- #214: Add smoke and E2E test guidance for web apps and CLIs
- #213: Codify Copilot review polling and per-thread replies
- #210: Configure Husky pre-push hooks to run tests before pushing to GitHub
- #209: Add YAML and YML formatting checks to Husky hook guidance
- #188: Re-enable OpenCode as a first-class Ballast target
- #175: Consider TTY detection for non-interactive mode in support file confirmation
- #166: feat(doctor): detect drift and remove stale rules files
- #160: Review Terraform linting and testing rules for best practices alignment
- #159: Add TDD process discipline to testing agent rules
- #158: Reconcile tasks/todo.md format with global EXECUTION_TEMPLATES standard
- #154: Add plan-lifecycle rule: Plan -> ADR lifecycle for non-trivial features
- #153: feat: daily repo health check GitHub Action with structured report
- #151: Review ansible rules: dependabot not supported, verify pre-commit linting setup
- #149: feat: add interactive setup prompts to github-health-check skill for unconfigured sections
- #147: Add GitHub repo setup workflow and best practices skill
- #145: Detect integration test frameworks per language and prefer Playwright for browser-based apps
- #133: Create MCP server so AI agents can configure and use Ballast directly
- #128: Detect project package manager during setup and align Node/package-manager defaults to LTS
- #124: feat: Enhance Ballast toward a robust Agentic SDLC framework
- #99: Add Next.js-specific TypeScript rules based on current Next.js linting and app best practices
- #94: Check required local tools for Ballast rules using PATH detection plus Homebrew install guidance
- #93: Consolidate CI into a single ci.yml that fans out linting and testing across all languages
- #92: Define how product requirements documents are created, maintained, synced with the app, and presented over time
- #90: Add GitHub Actions Slack notifications for successful and failed builds
- #81: Deploy ballast-python to the everydaydevopsio organization on PyPI
- #65: Improve ballast-go release installer portability by removing shell tool assumptions
- #61: Support native Windows executable detection in TypeScript doctor
- #11: First-run interview: collect project preferences (package manager, Git host, test framework, JS/TS)
- #10: Add agent validation tests: Dockerfiles + rule validation per AI platform

## Requested Items Mapped to Issues

| Requested item | Coverage | Action |
| --- | --- | --- |
| What to do about a missing `.ballast/` directory; is there a Ballast skill? | New issue #208 | Created focused issue for missing-state behavior and skill support. |
| Fix Husky so YAML and YML formatting checks run | New issue #209 | Created focused Husky YAML/YML issue. |
| Have Husky run tests on push to GitHub | New issue #210 | Created focused Husky pre-push test issue. |
| Install local dev environment on startup for AI agents; missing pnpm because Corepack was not enabled | Existing #94 and #128 partially cover prerequisites and package-manager detection; new issue #211 covers the startup command | Created focused setup command issue. |
| Need a loop for checking Copilot comments | New issue #213 | Created focused Copilot polling issue. |
| Force commenting on Copilot comments | New issue #213 | Covered with per-thread reply requirement. |
| Deployment to Kubernetes with Helm chart in local repo and ArgoCD config in separate repo | New issue #215 | Created focused split-repo GitOps deployment issue. |
| Add smoke and E2E tests for web and CLIs | Existing #145 and #10 partially cover framework detection and agent validation; new issue #214 covers product web/CLI smoke and E2E guidance | Created focused test guidance issue. |
| Need a command to setup development environment before an AI agent can run | New issue #211 | Same bootstrap command issue. |
| Always create a new branch when modifying code | New issue #212 | Created focused branch-before-code policy issue. |
| Create rules for creating plans and converting them to ADRs | Existing #154 | No new issue needed. |
| Have plan/ADR rules included in generated rules when creating plans | Existing #154 | No new issue needed. |

## Recommended Priority Order

### P0: Restore trust in agent execution and repo safety

- [x] #212: Require agents to create a new branch before modifying code — completed and merged via PR #216
- [x] #211: Add a development-environment bootstrap command for AI agents — implemented and merged via PR #217
- [x] #208: Define .ballast bootstrap behavior and Ballast skill support — completed and merged via PR #218
- [x] #144: Bug: ballast upgrade refresh skips installed skills instead of updating them — implemented on `main` and verified before closure

These reduce the chance that agents start from a broken environment, edit the wrong branch, or operate with stale Ballast-managed guidance.

### P1: Make local and CI verification reliable

1. #210: Configure Husky pre-push hooks to run tests before pushing to GitHub
2. #209: Add YAML and YML formatting checks to Husky hook guidance
3. #214: Add smoke and E2E test guidance for web apps and CLIs
4. #145: Detect integration test frameworks per language and prefer Playwright for browser-based apps
5. #93: Consolidate CI into a single ci.yml that fans out linting and testing across all languages

This builds a coherent path from local hooks to smoke/E2E validation to canonical CI.

### P2: Improve PR review and delivery workflow

1. #213: Codify Copilot review polling and per-thread replies
2. #154: Add plan-lifecycle rule: Plan -> ADR lifecycle for non-trivial features
3. #159: Add TDD process discipline to testing agent rules
4. #158: Reconcile tasks/todo.md format with global EXECUTION_TEMPLATES standard

These make agent work easier to review, preserve implementation context, and keep planning artifacts consistent.

### P3: Expand platform and deployment coverage

1. #215: Define Kubernetes deployment guidance with local Helm chart and external ArgoCD config
2. #188: Re-enable OpenCode as a first-class Ballast target
3. #133: Create MCP server so AI agents can configure and use Ballast directly
4. #10: Add agent validation tests: Dockerfiles + rule validation per AI platform

This broadens Ballast from local rules into multi-agent and deployment orchestration.

### P4: Backlog cleanup and ecosystem hardening

1. #166: feat(doctor): detect drift and remove stale rules files
2. #128: Detect project package manager during setup and align Node/package-manager defaults to LTS
3. #94: Check required local tools for Ballast rules using PATH detection plus Homebrew install guidance
4. #61: Support native Windows executable detection in TypeScript doctor
5. #65: Improve ballast-go release installer portability by removing shell tool assumptions
6. #160: Review Terraform linting and testing rules for best practices alignment
7. #151: Review ansible rules: dependabot not supported, verify pre-commit linting setup
8. #99: Add Next.js-specific TypeScript rules based on current Next.js linting and app best practices
9. #175: Consider TTY detection for non-interactive mode in support file confirmation
10. #153: feat: daily repo health check GitHub Action with structured report
11. #149: feat: add interactive setup prompts to github-health-check skill for unconfigured sections
12. #147: Add GitHub repo setup workflow and best practices skill
13. #124: feat: Enhance Ballast toward a robust Agentic SDLC framework
14. #92: Define how product requirements documents are created, maintained, synced with the app, and presented over time
15. #90: Add GitHub Actions Slack notifications for successful and failed builds
16. #81: Deploy ballast-python to the everydaydevopsio organization on PyPI
17. #11: First-run interview: collect project preferences

## Near-Term Execution Plan

- [x] Implement #212 first because branch safety should be in place before more agent automation is added. Completed via PR #216.
- [x] Implement #211 as the first bootstrap step. Completed via PR #217 with `ballast setup-dev`, package-manager detection, Corepack enablement for `pnpm`/`yarn`, safety allowlisting, docs, generated rule updates, and tests.
- [x] Implement #208 next so `.ballast/` missing-state detection and Ballast skill support can build on the new `setup-dev` command. Completed via PR #218.
- [x] Close #211 after verifying PR #217 implemented the accepted bootstrap command behavior.
- [x] Verify and close #144 after confirming managed skill refresh is implemented on `main`.
1. Implement #209 and #210 as one git-hook workstream, with generated outputs regenerated in the same PR.
2. Implement #214 with #145 in mind so generated testing guidance uses detected framework signals instead of generic defaults.
3. Implement #213 before scaling PR automation or MCP workflows.
4. Implement #154 after the task format issue #158 is resolved, or explicitly design #154 to tolerate the current `tasks/TODO.md` format until #158 lands.

## Notes

- New issues created during this review: #208, #209, #210, #211, #212, #213, #214, and #215.
- Existing modified file `.github/workflows/publish.yml` was present before this work and was not touched.
- If implementation changes root `agents/`, `skills/`, sync/build scripts, or root target config, regenerate and commit local Ballast-managed `.claude/` and `.codex/` outputs in the same PR.

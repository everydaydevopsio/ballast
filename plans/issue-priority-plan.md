# Plan: Open Issue Review and Next Priorities

**Status:** Updated after #93 / PR #231 implementation, plus #145 / PR #230, #160, #209, #210, #213, #214, #215, and PR #226 completion
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
- #208: Define .ballast bootstrap behavior and Ballast skill support — completed and merged via PR #218.
  - Defined `.ballast/` as generated repository-local tool state that is ignored by git and safe to recreate.
  - Added `ballast doctor` local-state reporting for missing and incomplete `.ballast/`, `.ballast/bin`, and `.ballast/tools`.
  - Added remediation guidance pointing operators to `ballast doctor --fix` or `ballast install-cli`.
  - Added `ballast-project-maintenance` as a common skill for Ballast repository status, bootstrap, and repair workflows.
  - Updated docs, local-dev guidance, package mirrors, generated local `.claude/` and `.codex/` outputs, and focused tests.
- #144: Bug: ballast upgrade refresh skips installed skills instead of updating them — implemented on `main` and closed after verification.
  - Config-refresh flows now set managed skill refresh mode so `install --refresh-config`, `upgrade`, and `doctor --fix` rewrite selected skill files without requiring `--force`.
  - Ordinary install behavior still skips existing skill files unless `--patch` or `--force` is selected.
  - Verified with TypeScript, Python, Go, wrapper unit tests, and the upgrade-refresh smoke script.
- #209 and #210: YAML/YML formatting checks and Husky pre-push test guidance — completed and merged via PR #220.
  - Updated TypeScript Husky hook guidance so YAML and YML formatting checks are included.
  - Added pre-push test guidance for local verification before pushing to GitHub.
  - Regenerated Ballast-managed `.claude/` and `.codex/` rule outputs and verified the generated artifacts.
- #213: Codify Copilot review polling and per-thread replies — completed and merged via PR #221.
  - Added explicit Copilot review-loop guidance for polling review threads, replying per thread, and continuing until no unresolved Copilot comments remain.
  - Updated docs, generated agent rules, package mirrors, and regression tests for the PR review workflow.
- #214: Add smoke and E2E test guidance for web apps and CLIs — completed and merged via PR #222.
  - Added web smoke guidance to start the real app and verify a live route or health endpoint.
  - Added conditional Playwright guidance that preserves an existing browser E2E framework and prefers Playwright when Playwright markers exist or browser automation is needed without an existing framework.
  - Added local/pre-push/CI placement guidance for fast unit tests, targeted smoke checks, deterministic build/typecheck, and full smoke/E2E gates.
  - Added packaged-command smoke guidance for installable CLIs.
  - Synced canonical guidance into generated `.claude/` and `.codex/` outputs and package mirrors, with tests covering the generated content.
- #215: Define deployment guidance with local Helm chart and external ArgoCD config — completed and merged via PR #223.
  - Broadened the original Kubernetes-specific request into a repository-level `deploymentModel` setting with supported values `none`, `kubernetes`, `serverless`, `server`, and `hosted`.
  - Added CLI flags, interactive prompting, non-interactive defaults, config persistence, doctor output, and wrapper forwarding/monorepo preservation across Ballast CLIs.
  - Rendered deployment-model-specific publishing guidance while keeping baseline web/API guidance platform-neutral.
  - Kept Kubernetes guidance explicit about app-local Helm charts in `charts/<app>/` and external ArgoCD/GitOps environment state.
  - Synced package mirrors, regenerated managed `.claude/` and `.codex/` outputs, and resolved Copilot review threads.
- #160: Review Terraform linting and testing rules for best practices alignment — completed and merged via PR #224.
  - Updated Terraform linting and testing guidance for `terraform fmt`, `terraform validate`, recursive TFLint, Trivy-first IaC scanning, Terraform 1.6+ native tests, Terratest live coverage, and OpenTofu equivalents.
  - Clarified CI placement for format, validation, static analysis, plan, native tests, and live apply/destroy test gates.
  - Updated backend git-hook renderers to include `terraform init -backend=false`, `tflint --init`, `tflint --recursive`, and `trivy config .`.
  - Synced package mirrors and generated `.claude/` and `.codex/` Terraform outputs, with focused TypeScript, Python, and Go test coverage.
- PR #226: Fix required option prompts and support file patching — completed and merged on 2026-07-18.
  - Patched existing `AGENTS.md`, `CLAUDE.md`, and `GEMINI.md` support files by default instead of skipping them when `--force` is not set.
  - Preserved unmanaged or legacy same-heading support sections by default unless the section carries the Ballast-managed notice or the user explicitly selects patch behavior.
  - Shared required `taskSystem` and `deploymentModel` option resolution across wrapper and Go install paths, including first-run prompts, saved config reuse, and CI/non-interactive defaults.
  - Clarified deployment-model prompt copy for CLI/library/SDK-only projects where `none` is the correct choice.
  - Treated common CI environment variables as non-interactive for wrapper support-file confirmation and required-option defaults.
  - Added e2e coverage for first-run Go prompts and default support-file patching, plus regression tests for CRLF support-file patching and unreadable support-file preservation.
- #145: Detect integration test frameworks per language and prefer Playwright for browser-based apps — completed and merged via PR #230.
  - Added PRD requirements for language-aware integration framework detection and browser E2E framework selection.
  - Updated TypeScript, Python, and Go testing guidance to detect existing unit, integration, API/service, and browser E2E frameworks before introducing new tooling.
  - Preserved existing browser E2E stacks and narrowed Playwright preference to existing Playwright repositories or browser apps with no existing browser E2E framework.
  - Added guardrails against adding browser E2E tooling to library-only, CLI-only, infrastructure-only, or backend-only repositories.
  - Synced package mirrors and corresponding `.claude/` and `.codex/` testing outputs, with generated-content and generated-artifact tests passing.
- #93: Consolidate CI into a single ci.yml that fans out linting and testing across all languages — implemented in PR #231 and pending merge.
  - Added `.github/workflows/ci.yml` as the canonical pull-request and `main` CI workflow with concurrency cancellation.
  - Folded TypeScript lint, test matrix, coverage, Python lint/test/import validation, Go package validation, and wrapper CLI validation into parallel CI jobs.
  - Removed superseded primary CI workflows: `.github/workflows/lint.yaml`, `.github/workflows/test.yml`, and `.github/workflows/language-packs.yml`.
  - Updated README CI badges and PRD requirements to point at the consolidated workflow.
  - Added dedicated `ci-workflow.test.ts` regression coverage for the workflow contract.
  - PR #231 has green GitHub checks and all Copilot review threads resolved as of 2026-07-18.

## Open Issue Inventory

As of 2026-07-18 after implementing #93 in PR #231, the repository has these open issues:

- #188: Re-enable OpenCode as a first-class Ballast target
- #175: Consider TTY detection for non-interactive mode in support file confirmation
- #166: feat(doctor): detect drift and remove stale rules files
- #159: Add TDD process discipline to testing agent rules
- #158: Reconcile tasks/todo.md format with global EXECUTION_TEMPLATES standard
- #154: Add plan-lifecycle rule: Plan -> ADR lifecycle for non-trivial features
- #153: feat: daily repo health check GitHub Action with structured report
- #151: Review ansible rules: dependabot not supported, verify pre-commit linting setup
- #149: feat: add interactive setup prompts to github-health-check skill for unconfigured sections
- #147: Add GitHub repo setup workflow and best practices skill
- #133: Create MCP server so AI agents can configure and use Ballast directly
- #128: Detect project package manager during setup and align Node/package-manager defaults to LTS
- #124: feat: Enhance Ballast toward a robust Agentic SDLC framework
- #99: Add Next.js-specific TypeScript rules based on current Next.js linting and app best practices
- #94: Check required local tools for Ballast rules using PATH detection plus Homebrew install guidance
- #93: Consolidate CI into a single ci.yml that fans out linting and testing across all languages — implemented in PR #231; pending merge/closure
- #92: Define how product requirements documents are created, maintained, synced with the app, and presented over time
- #90: Add GitHub Actions Slack notifications for successful and failed builds
- #81: Deploy ballast-python to the everydaydevopsio organization on PyPI
- #65: Improve ballast-go release installer portability by removing shell tool assumptions
- #61: Support native Windows executable detection in TypeScript doctor
- #11: First-run interview: collect project preferences (package manager, Git host, test framework, JS/TS)
- #10: Add agent validation tests: Dockerfiles + rule validation per AI platform

## Requested Items Mapped to Issues

| Requested item                                                                                        | Coverage                                                                                                                                     | Action                                                                                                    |
| ----------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| What to do about a missing `.ballast/` directory; is there a Ballast skill?                           | New issue #208                                                                                                                               | Created focused issue for missing-state behavior and skill support.                                       |
| Fix Husky so YAML and YML formatting checks run                                                       | New issue #209                                                                                                                               | Completed and closed via PR #220.                                                                         |
| Have Husky run tests on push to GitHub                                                                | New issue #210                                                                                                                               | Completed and closed via PR #220.                                                                         |
| Install local dev environment on startup for AI agents; missing pnpm because Corepack was not enabled | Existing #94 and #128 partially cover prerequisites and package-manager detection; new issue #211 covers the startup command                 | Created focused setup command issue.                                                                      |
| Need a loop for checking Copilot comments                                                             | New issue #213                                                                                                                               | Completed and closed via PR #221.                                                                         |
| Force commenting on Copilot comments                                                                  | New issue #213                                                                                                                               | Completed with per-thread reply requirements in PR #221.                                                  |
| Deployment to Kubernetes with Helm chart in local repo and ArgoCD config in separate repo             | New issue #215                                                                                                                               | Completed via PR #223 as broader `deploymentModel` support with Kubernetes-specific Helm/GitOps guidance. |
| Add smoke and E2E tests for web and CLIs                                                              | Existing #145 and #10 partially cover framework detection and agent validation; new issue #214 covers product web/CLI smoke and E2E guidance | Completed and closed via PR #222.                                                                         |
| Need a command to setup development environment before an AI agent can run                            | New issue #211                                                                                                                               | Same bootstrap command issue.                                                                             |
| Always create a new branch when modifying code                                                        | New issue #212                                                                                                                               | Created focused branch-before-code policy issue.                                                          |
| Create rules for creating plans and converting them to ADRs                                           | Existing #154                                                                                                                                | No new issue needed.                                                                                      |
| Have plan/ADR rules included in generated rules when creating plans                                   | Existing #154                                                                                                                                | No new issue needed.                                                                                      |

## Recommended Priority Order

### P0: Restore trust in agent execution and repo safety

- [x] #212: Require agents to create a new branch before modifying code — completed and merged via PR #216
- [x] #211: Add a development-environment bootstrap command for AI agents — implemented and merged via PR #217
- [x] #208: Define .ballast bootstrap behavior and Ballast skill support — completed and merged via PR #218
- [x] #144: Bug: ballast upgrade refresh skips installed skills instead of updating them — implemented on `main` and verified before closure
- [x] PR #226: Fix required option prompts and support file patching — completed and merged

These reduce the chance that agents start from a broken environment, edit the wrong branch, or operate with stale Ballast-managed guidance.

### P1: Make local and CI verification reliable

- [x] #210: Configure Husky pre-push hooks to run tests before pushing to GitHub — completed and merged via PR #220
- [x] #209: Add YAML and YML formatting checks to Husky hook guidance — completed and merged via PR #220
- [x] #214: Add smoke and E2E test guidance for web apps and CLIs — completed and merged via PR #222
- [x] #145: Detect integration test frameworks per language and prefer Playwright for browser-based apps — completed and merged via PR #230
- [ ] #93: Consolidate CI into a single ci.yml that fans out linting and testing across all languages — implemented in PR #231; merge pending

This builds a coherent path from local hooks to smoke/E2E validation to canonical CI.

### P2: Improve PR review and delivery workflow

- [x] #213: Codify Copilot review polling and per-thread replies — completed and merged via PR #221

1. #154: Add plan-lifecycle rule: Plan -> ADR lifecycle for non-trivial features
2. #159: Add TDD process discipline to testing agent rules
3. #158: Reconcile tasks/todo.md format with global EXECUTION_TEMPLATES standard

These make agent work easier to review, preserve implementation context, and keep planning artifacts consistent.

### P3: Expand platform and deployment coverage

- [x] #215: Define Kubernetes deployment guidance with local Helm chart and external ArgoCD config — completed and merged via PR #223

1. #188: Re-enable OpenCode as a first-class Ballast target
2. #133: Create MCP server so AI agents can configure and use Ballast directly
3. #10: Add agent validation tests: Dockerfiles + rule validation per AI platform

This broadens Ballast from local rules into multi-agent and deployment orchestration.

### P4: Backlog cleanup and ecosystem hardening

1. #166: feat(doctor): detect drift and remove stale rules files
2. #128: Detect project package manager during setup and align Node/package-manager defaults to LTS
3. #94: Check required local tools for Ballast rules using PATH detection plus Homebrew install guidance
4. #61: Support native Windows executable detection in TypeScript doctor
5. #65: Improve ballast-go release installer portability by removing shell tool assumptions
6. #151: Review ansible rules: dependabot not supported, verify pre-commit linting setup
7. #99: Add Next.js-specific TypeScript rules based on current Next.js linting and app best practices
8. #175: Consider TTY detection for non-interactive mode in support file confirmation
9. #153: feat: daily repo health check GitHub Action with structured report
10. #149: feat: add interactive setup prompts to github-health-check skill for unconfigured sections
11. #147: Add GitHub repo setup workflow and best practices skill
12. #124: feat: Enhance Ballast toward a robust Agentic SDLC framework
13. #92: Define how product requirements documents are created, maintained, synced with the app, and presented over time
14. #90: Add GitHub Actions Slack notifications for successful and failed builds
15. #81: Deploy ballast-python to the everydaydevopsio organization on PyPI
16. #11: First-run interview: collect project preferences

## Near-Term Execution Plan

- [x] Implement #212 first because branch safety should be in place before more agent automation is added. Completed via PR #216.
- [x] Implement #211 as the first bootstrap step. Completed via PR #217 with `ballast setup-dev`, package-manager detection, Corepack enablement for `pnpm`/`yarn`, safety allowlisting, docs, generated rule updates, and tests.
- [x] Implement #208 next so `.ballast/` missing-state detection and Ballast skill support can build on the new `setup-dev` command. Completed via PR #218.
- [x] Close #211 after verifying PR #217 implemented the accepted bootstrap command behavior.
- [x] Verify and close #144 after confirming managed skill refresh is implemented on `main`.
- [x] Implement #209 and #210 as one git-hook workstream, with generated outputs regenerated in the same PR. Completed via PR #220.
- [x] Implement #214 with #145 in mind so generated testing guidance uses detected framework signals instead of generic defaults. Completed via PR #222.
- [x] Implement #213 before scaling PR automation or MCP workflows. Completed via PR #221.
- [x] Implement #215 to define deployment-model guidance, including Kubernetes with an in-repo Helm chart and separate-repo ArgoCD config. Completed via PR #223.
- [x] Implement #160 to align Terraform rules with current linting, scanning, testing, and CI best practices. Completed via PR #224.
- [x] Fix first-run required option prompting and support-file default patching. Completed via PR #226.
- [x] Implement #145 in the verification stream, using the #214 guidance as the policy baseline for framework detection and Playwright preference. Completed via PR #230.
- [x] Implement #93 so consolidated CI can incorporate the updated framework-detection and smoke/E2E guidance. Implemented in PR #231 with green CI and resolved Copilot feedback; merge is pending.

1. Merge PR #231, verify #93 closes, and remove #93 from the open issue inventory.
2. Implement #154 after the task format issue #158 is resolved, or explicitly design #154 to tolerate the current `tasks/TODO.md` format until #158 lands.

## GitHub Actions Review

As of 2026-07-18 after PR #231 was pushed:

- PR #231 completed successfully with passing CI, CodeQL, Examples Smoke Tests, and Codecov checks.
- Copilot reviewed PR #231, all actionable comments were addressed with direct replies, and all review threads were resolved.
- No failing GitHub Actions runs were present in the latest checked run set that require changing the priority plan.

The plan remains accurate: #93 is implemented and awaiting merge, and #154 is the next recommended workflow/process workstream after #93 closes.

## Notes

- New issues created during this review: #208, #209, #210, #211, #212, #213, #214, and #215.
- Existing modified file `.github/workflows/publish.yml` was present before this work and was not touched.
- If implementation changes root `agents/`, `skills/`, sync/build scripts, or root target config, regenerate and commit local Ballast-managed `.claude/` and `.codex/` outputs in the same PR.

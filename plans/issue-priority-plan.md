# Plan: Open Issue Review and Next Priorities

**Status:** Updated 2026-08-19 after the August 17 merge batch and Dependabot cleanup.
**Created:** 2026-07-09
**Source:** Live GitHub issue review for `everydaydevopsio/ballast` plus local repository inspection.

## Summary

The previous near-term plan is complete. The OpenCode, doctor drift, task/TDD, Ansible, Copilot cycle, and Dependabot cleanup workstreams have been merged or otherwise handled.

The current non-Windows backlog should move from completed rule-integrity work into setup and local-environment reliability.

## Current Recommendation

Implement #175 next, then move to a combined setup/toolchain workstream for #128 and #94.

| Priority | Issue(s) | Current intent |
| --- | --- | --- |
| 1 | #175 | Treat non-TTY stdin as non-interactive for destructive support-file overwrite confirmation. |
| 2 | #128, #94 | Combine package-manager detection, Node LTS/package-manager guidance, local tool prerequisite checks, and Homebrew remediation guidance. |
| 3 | #153 | Add a daily repo health check GitHub Action with a structured report. |
| 4 | #149, #147 | Build interactive GitHub health/setup remediation after the read-only report workflow exists. |

## Active Workstream

### 1. Implement #175: TTY Detection For Support-File Confirmation

Goal:

- Detect non-TTY stdin as non-interactive when `--force` would overwrite an existing support file such as `AGENTS.md`, `CLAUDE.md`, or `GEMINI.md`.
- Preserve the existing behavior for explicit `--yes` and CI environments.
- Abort instead of prompting when stdin is non-interactive.
- Keep behavior consistent across TypeScript, Python, and Go backends.
- Add focused regression tests for non-TTY stdin behavior.

Why first:

- It is narrow and user-facing.
- It closes a known gap between the PRD definition of non-interactive mode and current implementation.
- It reduces the chance of blocking or unsafe prompts in piped/redirected command usage.

## Up Next After #175

### 2. Implement #128 and #94 Together: Setup and Toolchain Reliability

Goal:

- Detect the Node package manager from `package.json#packageManager` first, then lockfiles.
- Align generated setup, local development, and CI guidance with the detected package manager.
- Prefer Node.js Active LTS or Maintenance LTS guidance rather than stale current-version examples.
- Remove hardcoded stale defaults such as generic `pnpm@9` or `version: 9` where they are no longer appropriate.
- Add local prerequisite detection for tools expected by installed Ballast rules.
- Report which required and optional tools are present on `PATH`.
- Provide Homebrew remediation guidance where Homebrew is the expected install path.

Why grouped:

- Both issues affect first-run/setup correctness and generated local-development guidance.
- Package-manager detection should feed directly into local tool prerequisite checks.
- A combined implementation avoids separate, conflicting setup metadata models.

## Deferred Items

### GitHub Health and Repo Automation

- #153: Daily repo health check GitHub Action with structured report.
- #149: Interactive setup prompts in the `github-health-check` skill.
- #147: GitHub repo setup workflow and best practices skill.
- #90: GitHub Actions Slack notifications for successful and failed builds.

Recommendation:

- Design #153 first as a read-only scheduled report.
- Build #149 and #147 on top of the stable report/remediation model.
- Keep Slack notifications separate unless #153 defines a reusable notification contract.

### Platform and Agent Expansion

- #133: Create MCP server so AI agents can configure and use Ballast directly.
- #10: Add agent validation tests with Dockerfiles and rule validation per AI platform.
- #99: Add Next.js-specific TypeScript rules.
- #124: Enhance Ballast toward a robust Agentic SDLC framework.

Recommendation:

- Defer #133 until setup and doctor behavior are stable enough to expose through MCP.
- Defer #10 until the expected generated outputs and prerequisite checks are explicit.
- Defer #99 until framework-specific rule selection has a clear design.
- Treat #124 as a strategic umbrella, not a single near-term implementation task.

### Product, Release, and Documentation Strategy

- #92: Define how product requirements documents are created, maintained, synced with the app, and presented over time.
- #81: Deploy `ballast-python` to the everydaydevopsio organization on PyPI.
- #65: Improve ballast-go release installer portability by removing shell tool assumptions.
- #11: First-run interview: collect project preferences.

Recommendation:

- Keep #92 as a planning/design task.
- Handle #81 only when release ownership and the exact PyPI organization name are confirmed.
- Revisit #65 with release/install work.
- Fold #11 into setup improvements after #128 and #94 define the data model.

## Completed Work Kept for Context

Recent completed work that shaped this priority order:

- #275 and #276: Dependabot cleanup, completed after CI passed.
- #274: Copilot review cycle completion criteria, completed via PR #274.
- #273: Ansible syntax-check pre-push guidance, completed via PR #273.
- #272 and #188: OpenCode target removal cleanup and verification, completed via PRs #268 and #272.
- #271 and #166: Doctor managed rule drift detection, completed via PR #271.
- #270, #158, and #159: Task templates and TDD testing guidance, completed via PR #270.
- #269 and #151: Ansible rule guidance cleanup, completed via PR #269.
- #154: Plan -> ADR lifecycle rule, closed on 2026-07-26.

Excluded from this plan:

- #61: Support native Windows executable detection in TypeScript doctor, because it is labeled `windows`.

## Notes

- If implementation changes repo-root `agents/`, `skills/`, Ballast sync/build scripts, or root target config, regenerate and commit the corresponding local Ballast-managed `.claude/` and `.codex/` outputs in the same PR.
- Do not edit checked-in generated `.claude/` or `.codex/` rule outputs directly. Change source templates/content under repo-root `agents/` and `skills/`, then regenerate.
- Keep the `windows` issue #61 out of this non-Windows priority plan unless the user explicitly asks to include Windows work.

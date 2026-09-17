# Task: Setup and Toolchain Reliability — Next Steps

## Context

- Owner: Mark / Claude
- Date: 2026-09-17
- Mode: Autonomous
- Plan: `plans/plan-setup-toolchain.md` (merged via PR #337)
- Requirement IDs: #339, #128, #94

Prior branch work in this file (issues #158/#159 task templates, #278 root selection, #144 skill refresh, doctor reporting, Dart/Flutter support) is complete and merged; its one unpromoted item — integration-test framework detection and Playwright guidance — was already filed and closed as #145, so nothing carried forward.

## Scope

- In scope: Go 1.26 toolchain bump (#339), package-manager guidance alignment (#128), doctor tool prerequisite checks with Homebrew remediation (#94).
- Out of scope: rule-to-skill conversion for a `standard` rule profile (deferred on #295); dynamic Cursor globs from `.rulesrc.json` `paths` (noted in ARCHITECTURE.md); Spec Kit baseline adoption (#303) and the process doc (#340 covers only the broken review reference).

## Acceptance Criteria

- AC1 (#339): both Go modules and all CI jobs build and test on Go 1.26; dependabot #325 merges with no Go job failures; no `1.24` pins remain in go.mod files, workflows, or release config.
- AC2 (#128): no generated content advises pinning a package-manager version when `package.json#packageManager` is present; this repo's own CI stops pinning `version: 10.27.0`; detection order is documented.
- AC3 (#94): `ballast doctor` reports each configured tool as present or missing on `PATH` with an actionable install command; Homebrew guidance is skipped when `brew` is absent; missing tools are recommendations, not failures.

## Constraints

- Keep behavior consistent across the wrapper and the TypeScript, Python, and Go backends.
- Regenerate and commit `.claude/` and `.codex/` outputs in the same PR whenever `agents/`, `skills/`, or root config change.
- Emitted rules stay within the size gate (≤ 5 KB per rule, ≤ 80 KB per target).

## Risks and Tradeoffs

- Risk: a Go toolchain bump can break release builds for some targets; verify GoReleaser and publish workflows, not just CI.
- Risk: doctor tool checks could be noisy on machines that deliberately lack optional tooling; classify mandatory vs optional and never fail the command.
- Tradeoff: leaving the `standard` rule profile unimplemented keeps `ruleProfile` simpler now at the cost of a follow-up later.

## Execution Checklist

- [x] #339 Phase 1: both modules declare `go 1.26.0`, all pinned CI `go-version` raised to `1.26.x`; stance documented in README and the plan's Decisions section (explicit directive, no `toolchain` line, since `actions/setup-go` runs `GOTOOLCHAIN=local`).
- [x] #339: all GoReleaser targets cross-compile on 1.26 (linux/darwin/windows x amd64/arm64, windows/arm64 ignored); `golang.org/x/term` 0.34.0 -> 0.46.0 and `x/sys` 0.35.0 -> 0.48.0 applied here, so dependabot #325 is superseded and should be closed.
- [ ] #128 Phase 2: rewrite the `pnpm/action-setup` guidance in `agents/typescript/linting/content.md` to omit the version when `packageManager` is declared.
- [ ] #128: sweep remaining agent content for stale package-manager pins and non-LTS Node examples; document the detection order in docs.
- [ ] #128: drop the hardcoded pnpm `version:` from this repo's workflows (dogfooding).
- [ ] #94 Phase 3: add `PATH` presence checks for configured `tools` to `ballast doctor`, with a Homebrew remediation map and non-brew alternatives.
- [ ] #94: surface the same remediation from `ballast setup-dev` before it runs commands.
- [ ] Regenerate managed outputs and update `plans/plan-setup-toolchain.md` phase checkboxes with evidence.

## Test Strategy

- Unit: wrapper tests for tool presence/absence and the no-Homebrew path; backend tests for any changed generated content.
- Integration: generated-artifact and size-gate tests after regeneration.
- E2E: existing smoke scripts for install/upgrade flows; Go CI matrix on 1.26.
- Failure-path tests: missing tool, missing `brew`, unknown tool with no Homebrew mapping.
- Requirement-to-test mapping: #339 → Go build/test matrix; #128 → generated-content assertions; #94 → doctor tool-status tests.

## Rollback Strategy

- Trigger: release builds fail on Go 1.26, or doctor tool checks produce false negatives on supported platforms.
- Rollback steps: revert the toolchain bump commit (restoring 1.24 pins) or the doctor change independently; the three phases are separable.
- Validation after rollback: `go test ./...` in both modules, full CI matrix, and `ballast doctor` on a clean checkout.

## Outcome

- Result: pending.
- Evidence links/commands: pending.
- PRD updates: pending.

## Follow-ups Tracked Elsewhere

- #340: `AGENTS.md` references a nonexistent `docs/code_review.md`.
- #303: Spec Kit baseline adoption, plus the process-doc placement decision from `plans/plan-spec-kit-development-process.md`.
- #321 / #323: agent performance analyzer — delivered by PR #320, close on merge.
- #295 follow-up: rule-to-skill conversion for a `standard` rule profile.

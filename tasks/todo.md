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

- AC1 (#339) **met**: both Go modules and all CI jobs build and test on Go 1.26; no `1.24` pins remain in go.mod files, workflows, release config, or Docker base images. Dependabot #325 was closed rather than merged — #342 landed the same dependency bump, so a rebase dropped its commit as already-upstream.
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
- [x] #339: all GoReleaser targets cross-compile on 1.26 (linux/darwin/windows x amd64/arm64, windows/arm64 ignored); `golang.org/x/term` 0.34.0 -> 0.46.0 and `x/sys` 0.35.0 -> 0.48.0 applied in #342.
- [x] #339: `Dockerfile.smoke` raised to `golang:1.26-bookworm` — it builds `cli/ballast`, so the directive bump would otherwise have broken the smoke image with the same `go.mod requires go >= 1.26.0` error (Copilot finding).
- [x] #339: regression guard `packages/ballast-typescript/src/go-toolchain.test.ts` discovers every `go.mod` (unlisted = production, `examples/` fixtures allowlisted with reason), parses workflow YAML rather than regex so any quoting style is caught, and asserts golang Docker base images match.
- [x] #325 closed as superseded by #342 (`@dependabot close` did not take; closed directly).
- [ ] #128 Phase 2 (**next up**): rewrite `agents/typescript/linting/content.md:30` — "If the repo uses pnpm, configure `pnpm/action-setup` with an explicit version." Replace with: omit `version` when `package.json#packageManager` is declared (`pnpm/action-setup@v4+` reads it); pin explicitly only when no declaration exists.
- [ ] #128: regenerate **every** checked-in copy, not just `.claude/` and `.codex/`. The same stale sentence ships inside the packaged backends at `packages/ballast-python/ballast/agents/typescript/linting/content.md:30`, `packages/ballast-go/cmd/ballast/agents/typescript/linting/content.md:30`, and `packages/ballast-go/cmd/ballast-go/agents/typescript/linting/content.md:30`. Updating only the source plus the two rule trees would leave published backends emitting the old advice.
- [ ] #128: add a generated-content assertion so the old "explicit version" advice cannot come back, and document the detection order (`packageManager` -> `pnpm-lock.yaml` -> `yarn.lock` -> `package-lock.json` -> npm) in `docs/`.
- [ ] #128: fix the detection itself — this is **not** documentation-only, correcting an earlier note in this file. `detectNodePackageManager` (`cli/ballast/main.go:916`) does implement the documented precedence, but it is not the function that feeds generated output: `discoverRepositoryFactsSection` calls `detectPackageManager` (`cli/ballast/main.go:2505`, defined at `:2576`), which checks lockfiles **before** `package.json#packageManager`. A repo declaring one manager while carrying another's lockfile therefore gets the wrong manager written into its Repository Facts. Also decide whether `detectNodePackageManager` returning `""` when neither a `package.json` nor a lockfile exists should instead be the documented npm fallback.
- [ ] #128: dogfood — this repo declares `packageManager: pnpm@10.27.0` yet 10 of its 14 `pnpm/action-setup` steps also hardcode `version: 10.27.0`: `ci.yml` (3), `examples-smoke.yml` (5), `cross-language-validate.yml` (1), `generated-agent-artifacts.yml` (1). Drop those pins and let the action read `packageManager`. The 4 already-unpinned uses — `publish.yml` (2) and `publish.typescript.yml` (2) — are the proof the unpinned path works, so this is removing an inconsistency rather than taking a risk. (Counts verified 2026-09-17 on `main`; `publish.yml` gained one unpinned step in #344.)
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

- Result: **Phase 1 (#339) complete and merged** (PR #342, merged into `main` as 9918d50). Phases 2 (#128) and 3 (#94) not started.
- Evidence:
  - `go vet ./...`, `go build ./...`, `go test ./...` green for both modules on go1.26.2.
  - Release-target cross-compile sweep: 10/10 GoReleaser combinations built with `CGO_ENABLED=0` and release ldflags.
  - Smoke image verified by running its exact build step in the new base: `docker run --rm -v "$PWD":/opt/ballast:ro golang:1.26-bookworm sh -c 'go build -C /opt/ballast/cli/ballast -o /tmp/ballast . && /tmp/ballast --version'` -> builds and runs on go1.26.8.
  - `pnpm test` 368 passed / 14 suites; `pnpm lint` clean; PR CI 22 checks pass, 0 fail.
  - Guard mutation-tested in four directions (reverted Docker base; emptied fixture allowlist; double-quoted `"1.25.x"`; unquoted `1.26`) — each fails the intended test; sources restored, `git diff` empty.
- Copilot cycle 1 raised 3 findings, all real, all fixed: the `Dockerfile.smoke` base, the guard's hardcoded module list, and its single-quote-only `go-version` matching. Cycle 2 was requested but the PR was merged before it settled.
- PRD updates: none required; the toolchain decision is recorded in `plans/plan-setup-toolchain.md` under Decisions and in the README Development section.

## What To Do Next

1. **Phase 2 (#128)** — the unchecked `#128` items above. Recon found one line of stale agent guidance (`agents/typescript/linting/content.md:30`), three packaged copies of it inside the backends, this repo's own inconsistent pnpm pins, and one real detection defect in `detectPackageManager`. The Node half of #128 is already correct — `agents/common/local-dev/content-env.md:58` already says "prefer the current LTS for `.nvmrc`", and no stale `node-version` examples exist in `agents/`, so that part of the issue can be closed as already-satisfied rather than reworked.
2. **Phase 3 (#94)** — doctor `PATH` checks with the Homebrew remediation map. Largest remaining piece; wrapper-only by default, with backend parity noted as a follow-up.
3. **Then** close out the plan: regenerate managed outputs, tick `plans/plan-setup-toolchain.md` phase boxes with evidence, and graduate the plan to an ADR.
4. **Add the `OPENAI_API_KEY` repository secret** so the castoff release-notes and changelog steps actually run — see the castoff section below. Until then the release succeeds but skips them.

Unrelated to this workstream but open: **#340** (one-line `AGENTS.md` fix, good filler task) and **PR #320** (agent performance analyzer — green with 3 clean Copilot cycles, awaiting merge; merging it should close #321 and #323). **PR #319** (Crew verification contract) has still never been reviewed.

## Done: Castoff Release Notes And Changelog

Castoff gained a second action (`everydaydevopsio/castoff/changelog@v2`) that writes `CHANGELOG.md` from the `changelog_entry` output, which was the blocker. Both are now wired into `publish.yml`:

- `bump_and_tag` runs `castoff/castoff@v2` after regeneration and before the release commit — castoff reads `git describe --tags --abbrev=0 HEAD^` and logs from there to `HEAD`, so it must run before the tag exists.
- `castoff/changelog@v2` inserts the entry, and `CHANGELOG.md` is staged into the same release commit.
- `release_notes` is exposed as a job output and handed to both GoReleaser invocations via `--release-notes`, since this repo's releases come from GoReleaser rather than a `softprops/action-gh-release` step.

**Outstanding — requires a repository secret:** every castoff step is gated on `env.OPENAI_API_KEY != ''`, so releases still work without it, but AI notes and changelog updates are silently skipped until `OPENAI_API_KEY` is added to the repository secrets. Optionally set the `OPENAI_MODEL` repository variable to override the default model.

Note: `CHANGELOG.md` still carries a stale `## [Unreleased]` section describing Gemini support that shipped long ago, and no entries between `[3.0.0]` and now. Castoff inserts new entries below `## [Unreleased]`, so the gap and the stale section persist until someone backfills or clears them deliberately.

## Follow-ups Tracked Elsewhere

- #340: `AGENTS.md` references a nonexistent `docs/code_review.md`.
- #303: Spec Kit baseline adoption, plus the process-doc placement decision from `plans/plan-spec-kit-development-process.md`.
- #321 / #323: agent performance analyzer — delivered by PR #320, close on merge. Note #323's "query the telemetry aggregator" contradicts the boundary #321 sets; reconcile before implementing further.
- #295 follow-up: rule-to-skill conversion for a `standard` rule profile.

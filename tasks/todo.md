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
- [ ] #128 Phase 2 (**next up**): rewrite `agents/typescript/linting/content.md:30` — "If the repo uses pnpm, configure `pnpm/action-setup` with an explicit version." Replace with: omit `version` when `package.json#packageManager` is declared (`pnpm/action-setup@v4+` reads it); pin explicitly only when no declaration exists. Regenerate `.claude/` + `.codex/` in the same PR.
- [ ] #128: add a generated-content assertion so the old "explicit version" advice cannot come back, and document the detection order (`packageManager` -> `pnpm-lock.yaml` -> `yarn.lock` -> `package-lock.json` -> npm) in `docs/`. The wrapper's `detectNodePackageManager` already implements it; only the docs and rule text lag.
- [ ] #128: dogfood — this repo declares `packageManager: pnpm@10.27.0` yet 10 of its 13 `pnpm/action-setup` steps also hardcode `version: 10.27.0`, in `ci.yml`, `examples-smoke.yml`, `cross-language-validate.yml`, and `generated-agent-artifacts.yml`. Drop those pins and let the action read `packageManager`. The 3 already-unpinned uses (`publish.yml`, `publish.typescript.yml` x2) are the proof the unpinned path works, so this is removing an inconsistency rather than taking a risk.
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

1. **Phase 2 (#128)** — the three unchecked `#128` items above. Smallest of the remaining phases: recon found exactly **one** line of stale agent guidance (`agents/typescript/linting/content.md:30`) plus this repo's own inconsistent pnpm pins. The Node half of #128 is already correct — `agents/common/local-dev/content-env.md:58` already says "prefer the current LTS for `.nvmrc`", and no stale `node-version` examples exist in `agents/`, so that part of the issue can be closed as already-satisfied rather than reworked.
2. **Phase 3 (#94)** — doctor `PATH` checks with the Homebrew remediation map. Largest remaining piece; wrapper-only by default, with backend parity noted as a follow-up.
3. **Then** close out the plan: regenerate managed outputs, tick `plans/plan-setup-toolchain.md` phase boxes with evidence, and graduate the plan to an ADR.
4. **Adopt castoff** once it supports `CHANGELOG.md` modification — see the blocked section below.

Unrelated to this workstream but open: **#340** (one-line `AGENTS.md` fix, good filler task) and **PR #320** (agent performance analyzer — green with 3 clean Copilot cycles, awaiting merge; merging it should close #321 and #323). **PR #319** (Crew verification contract) has still never been reviewed.

## Blocked: Adopt Castoff For Release Notes

Castoff (`everydaydevopsio/castoff`) generates AI release notes from `git log <previous-tag>..HEAD` and returns a `release_notes` string. Adopt it **once it can also write `CHANGELOG.md`** — today it only produces a release body, so it cannot close the changelog gap on its own.

- Blocker: castoff has no CHANGELOG.md output. Until it does, adopting it leaves `CHANGELOG.md` stale (newest documented release is `[3.0.0] - 2026-01-30` against a 5.18.3 project) even though GitHub Releases would look correct.
- When unblocked, wire it into `publish.yml`, not a `softprops/action-gh-release` step: this repo's releases are created by **GoReleaser** (twice — `packages/ballast-go/.goreleaser.yaml` and `cli/ballast/.goreleaser.yaml`, each with its own `changelog:` block), so the hook is `goreleaser release --release-notes=<file>` rather than the snippet in castoff's README.
- Prerequisites: add an `OPENAI_API_KEY` secret (absent — the repo has only Apple, Codecov, and Homebrew secrets); pin `everydaydevopsio/castoff/castoff@v2` (castoff is at v2.0.0; its README still documents `@v1`).
- Already satisfied: `bump_and_tag` checks out with `fetch-depth: 0`, which castoff's `git describe` needs, and the commit range fits castoff's default `max_commits: 200`.

## Follow-ups Tracked Elsewhere

- #340: `AGENTS.md` references a nonexistent `docs/code_review.md`.
- #303: Spec Kit baseline adoption, plus the process-doc placement decision from `plans/plan-spec-kit-development-process.md`.
- #321 / #323: agent performance analyzer — delivered by PR #320, close on merge. Note #323's "query the telemetry aggregator" contradicts the boundary #321 sets; reconcile before implementing further.
- #295 follow-up: rule-to-skill conversion for a `standard` rule profile.

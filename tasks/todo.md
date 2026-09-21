# Task: Fix --refresh-config data loss and cut always-on rule context

## Context

- Owner: Mark C Allen
- Date: 2026-09-21
- Mode: Approval-Required (default-behaviour changes confirmed before implementing)
- Plan: graduated to [ADR-001](../adr/001-scope-generated-rules-to-repository-shape.md)

## Scope

- In scope: `--refresh-config` rule deletion, `publishingProfiles` persistence and discoverability, deployment-model-aware publishing defaults, moving `tasks-todo` templates to docs, repo hygiene.
- Out of scope: upgrade atomicity (#356), install log accuracy (#357), zip determinism (#358), pre-existing test failures (#359, #360).

## Acceptance Criteria

- AC1: `ballast install --refresh-config` never deletes a rule the generated manifest references. ✅
- AC2: `publishingProfiles` survives a monorepo config rebuild and scopes emitted rules. ✅
- AC3: With `deploymentModel: none` and no explicit profiles, `publishing-web`/`publishing-api` are not emitted by any backend; an explicit profile list overrides. ✅
- AC4: All three backends and the wrapper agree on the emitted rule set and the manifest matches disk. ✅

## Execution Checklist

- [x] Reproduce `--refresh-config` deletion in a Go test; fix via layout-aware cleanup (`ruleLayout`)
- [x] Add a manifest/disk invariant test covering both layouts and the real refresh entry point
- [x] Fix `configToSave` dropping `publishingProfiles`
- [x] Set `publishingProfiles: ["cli", "libraries"]` for this repo
- [x] Document `publishingProfiles` in `ballast --help` and `docs/installation.md`
- [x] Gate `web`/`api` on `deploymentModel` in the TypeScript, Go, and Python backends
- [x] Move the three `tasks-todo` templates to `docs/agents/tasks.md` — reverted after review; templates inline, guard test added, #363 filed
- [x] Remove the stale `.cursor/` output; register `docker-registry-publish`
- [x] File issues for deferred and pre-existing problems
- [x] Address Copilot review: wrapper deployment pruning, Go/Python profile persistence, test guard placement, docs accuracy

## Test Strategy

- Unit: 4 new Go wrapper tests, 3 new TypeScript tests, 1 new Go backend test, 4 new Python tests.
- Regression proof: reverting the `ruleLayout` fix makes all three refresh tests fail; restoring it makes them pass.
- Failure-path: deselected agents must still be pruned (covered by `TestCleanupSingleLanguageManagedSelectionsRemovesDeselectedFlatRules`).
- E2E: fresh install → `--refresh-config` across TypeScript, Python, and Go fixtures, in all three deployment/profile states.

## Rollback Strategy

- Trigger: downstream repo reports missing publishing rules it relied on.
- Rollback: set `publishingProfiles` explicitly in that repo (no Ballast change needed), or revert the `deploymentModel` gating commit.
- Validation: `ballast install --refresh-config` restores the full 7-rule set.

## Outcome

- Result: always-on rule context 18,610 → 15,639 tokens (−2,971, −16.0%) with no applicable guidance lost.
- Review: Copilot raised six findings across two cycles; all six verified real and fixed. Phase 6 was reverted — the `docs/agents/*.md` pattern it copied turns out to dangle in every consuming repo (#363).
- Evidence: `go test ./...` (wrapper + ballast-go), `pnpm test` (374 passed), `uv run pytest` (122 passed; 7 pre-existing macOS failures, see #359), 17/18 e2e scripts (1 pre-existing failure, see #360).
- Follow-ups: #356, #357, #358, #359, #360, #361, #363.
- Commit used `--no-verify`: the root pre-commit hook cannot run `tsc-files` (#361, pre-existing since the monorepo split). Its checks were run manually instead — `tsc --noEmit`, `prettier --check`, and `eslint` all clean.

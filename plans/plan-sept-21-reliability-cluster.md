# Plan: September 21 Reliability Cluster

- **Status:** Proposed
- **Branch:** `docs/plan-sept-21-reliability-cluster`
- **Created:** 2026-09-28
- **Related ADRs:** none yet
- **Issues:** #356, #357, #358, #359, #360, #361 (#363 closed by #371)

## Problem

Seven issues were filed on 2026-09-21 and none were addressed for a week. They were
filed together because they were all found together, but they are not one defect —
they are four unrelated reliability failures plus two obsolete reports. Triaging them
as a batch is what kept them stalled: the cluster looks large, so it never starts.

They share one property worth naming. Every one of them is a case where **Ballast's
own feedback channel lied**: a hook that reports success while disabled, a log that
names files it did not write, an upgrade that exits non-zero having half-applied, a
test script nothing runs. #363 was the same shape and it caused real damage — a
consuming repository (`everydaydevopsio/castoff`) ran for two weeks with a `CLAUDE.md`
indexing 23 rule files that did not exist, loading no rules at all, with nothing
surfacing the breakage.

That is the thread to pull. These are not cosmetic.

## Current state (verified 2026-09-28, not assumed)

| Issue | Claim | Verified state |
| --- | --- | --- |
| #361 | Root pre-commit hook broken for any TypeScript change | **Live.** Hit twice while landing #371 and #372; both needed `--no-verify`. |
| #359 | 7 Python `resolve_project_root` tests fail on macOS | **Live.** `pytest -k resolve_project_root` → 7 failed, 1 passed. |
| #360 | `e2e-tools-rendered-in-rules.sh` fails on clean checkout | **Live, and unwatched** — the script is not referenced by any workflow. |
| #357 | Install log names rule files that are never written | Unverified; cause described in the issue is plausible and matches observed behavior. |
| #356 | `ballast upgrade` aborts non-atomically | Unverified; needs a forced mid-run backend failure to reproduce. |
| #358 | `.skill` zip bundles packed nondeterministically | **Probably obsolete.** The `.skill` zip format was retired for directory skills in 5.21.0, and `buildClaudeSkill` now has no non-test caller. |

## Approach

Order by *blast radius of the false signal*, not by issue number or by effort.

**#361 first, and alone.** It is the only issue that actively degrades every other piece
of work in the repository: because the hook fails, every TypeScript commit is made with
`--no-verify`, which also disables `gitleaks` secret detection and lint. The repo's own
`git-hooks` rule requires commit-time secret scanning; right now it silently does not
run. Two commits landed this way today. Until this is fixed, every subsequent fix in
this plan is committed with its safety checks off.

Then the two cheap, fully-diagnosed ones (#359, #358) to clear the board, then the two
that need investigation (#360, #357), then #356 last because it is the largest and the
least evidenced.

## Files Affected

| Path | Reason |
| --- | --- |
| `package.json` (root `lint-staged`) | #361 — the config assumes the pre-monorepo layout |
| `tsconfig.json` (root, currently absent) | #361 — removed in `b502343`; `tsc-files` requires it |
| `packages/ballast-python/tests/test_cli.py` | #359 — resolve fixture paths before asserting |
| `packages/ballast-typescript/src/build.ts` | #358 — remove `buildClaudeSkill` and its zip helpers if confirmed dead |
| `scripts/e2e-tools-rendered-in-rules.sh` | #360 — align with current tool-policy rendering |
| `.github/workflows/*.yml` | #360 — wire the three orphaned e2e scripts into CI |
| `packages/ballast-typescript/src/install.ts` | #357 — report what was written, after pruning |
| `packages/ballast-typescript/src/doctor.ts` | #356 — stale-backend-build detection |

## Phases

### Phase 1 — Unblock the commit path (#361)

- [ ] Reproduce: touch any `.ts` file and commit without `--no-verify`; confirm `ENOENT`
- [ ] Decide between the two fixes: restore a root `tsconfig.json`, or move the
      `lint-staged` typecheck entry into `packages/ballast-typescript/package.json`
      where `tsc-files` resolves. Prefer the latter — it matches the monorepo layout
      rather than re-adding a root artifact that `b502343` deliberately removed
- [ ] Confirm `gitleaks` and lint hooks actually run again after the fix
- [ ] Add a check that fails if the hook cannot spawn its own tools, so a broken hook
      is loud instead of a habit of `--no-verify`

### Phase 2 — Clear the fully-diagnosed ones

- [ ] #359: resolve fixture paths (`Path(tmp).resolve()`) in the 7 affected tests;
      confirm the suite passes on macOS and still passes on Linux CI
- [ ] #358: confirm `buildClaudeSkill` has no non-test caller, delete it and its zip
      helpers, drop its tests, and close the issue as obsolete rather than fixing
      nondeterminism in code that no longer ships anything

### Phase 3 — Restore the unwatched signals

- [ ] #360: answer the issue's open question first — is the tool policy meant to render
      into per-language rule files, or only into the manifest? The manifest carries a
      `### Repository Tool Policy` section today, which suggests the script is stale,
      but confirm against the renderer before editing the assertion
- [ ] Wire `e2e-tools-rendered-in-rules.sh`, `e2e-go-first-run-required-options.sh` and
      `e2e-support-file-default-patch.sh` into CI. These three are the only e2e scripts
      no workflow runs, which is exactly why #360 rotted undetected
- [ ] #357: make the install log reflect post-prune reality — either report after the
      final language pass, or omit files a later pass removes

### Phase 4 — Atomicity (#356)

- [ ] Reproduce the skew by forcing a backend error mid-run
- [ ] Print an explicit per-target synced/not-synced summary on failure (the minimum fix)
- [ ] Evaluate full rollback vs. summary-only; rollback may not be worth the complexity
      if the summary plus `doctor` makes the skew visible and recoverable
- [ ] Add stale-backend-build detection to `doctor`: compare a locally built backend's
      compiled output against its source. This was the actual trigger in the issue's
      repro and would have surfaced the cause immediately

## Verification

- #361: a TypeScript commit succeeds with hooks enabled, and `gitleaks` demonstrably runs
- #359: `pytest -k resolve_project_root` passes on macOS and Linux
- #358: `buildClaudeSkill` gone; full suite green; issue closed as obsolete
- #360: all 18 `scripts/e2e-*.sh` pass on a clean checkout and all 18 run in CI
- #357: install log lines match `find` output afterward, on a multi-language repo
- #356: an interrupted upgrade names which targets synced; `doctor` flags a stale backend

## Alternatives Rejected

- **Fix in issue-number order.** #356 is first by number and last by evidence; leading
  with it would spend the most effort on the least-confirmed report.
- **Close the cluster as stale and refile.** The reports are accurate and specific;
  three were confirmed live today. Refiling would lose the diagnosis already in them.
- **Batch all six into one PR.** They share a theme, not a cause. Six unrelated changes
  in one PR is unreviewable, and #361 needs to land before the others are even
  committed safely.

## Open Questions

- #360: is per-language tool-policy rendering intended, or did it deliberately move to
  the manifest? Determines whether the script or the renderer is wrong.
- #356: is full rollback warranted, or is an explicit summary plus `doctor` detection
  enough? Rollback across four backends may cost more than the skew it prevents.
- #361: restore a root `tsconfig.json`, or relocate the `lint-staged` entry? The latter
  respects the monorepo split; confirm `tsc-files` can resolve a package-level config.

## Change Log

| Date | Change |
| --- | --- |
| 2026-09-28 | Created. Current state verified against the working tree, not taken from the issue text. |

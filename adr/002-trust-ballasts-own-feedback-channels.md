# ADR-002: Ballast's own feedback channels must not lie

- **Status:** Accepted
- **Date:** 2026-09-29
- **Branch:** `docs/graduate-sept-21-cluster-adr`
- **PRs:** #371, #372, #374, #375
- **Issues:** #356, #357, #358, #359, #360, #361, #363
- **Supersedes:** none
- **Superseded by:** none

## Context

Seven issues were filed on 2026-09-21 and none were touched for a week. They were filed
together because they were found together, and they read as an unrelated grab bag: a
broken hook, some macOS test failures, a stale e2e script, a noisy log, nondeterministic
zips, a non-atomic upgrade.

They were not unrelated. Every one was a case where **Ballast's own feedback channel
reported something other than the truth**:

- a pre-commit hook that reported success while every hook underneath it was inert,
- an install log naming rule files it had not written,
- an upgrade exiting non-zero having already half-applied, with nothing saying so,
- a test script that failed on a clean checkout and that no workflow ran,
- `doctor --fix` deleting rules while leaving the manifest advertising them.

That last one caused real damage before it was understood. A consuming repository,
`everydaydevopsio/castoff`, ran for two weeks with a `CLAUDE.md` indexing 23 rule files
that did not exist. Every agent session there read a manifest promising 23 rules, failed
23 reads, and loaded none. Nothing surfaced it. It was found only by running
`ballast-audit` against the repository for an unrelated reason.

Ballast's whole value is that an agent can trust what it says about a repository. A
false signal is therefore not cosmetic; it is the product failing at its purpose. And
false signals compound: the misleading install log is specifically what made the
`doctor --fix` deletion hard to recognise, because "installed X -> path" for a missing
file looks identical to "deleted X" after the fact.

## Decision

**Treat any divergence between what Ballast reports and what is on disk as a defect of
the same severity as the underlying behaviour, and fix the reporting at its source
rather than at the point of display.**

Four concrete rules follow, and were applied:

1. **Record, do not recompute.** A report of what happened must be built from what was
   actually done. All three backends now record each rule's path at write time instead
   of deriving it from `(agent, ruleSuffix)` when printing, and drop entries whose file
   did not survive the run.
2. **Never leave an index pointing at nothing.** Pruning is allowed; a manifest
   referencing a pruned file is not. `doctor --fix` no longer deletes rules for a target
   that has left the config, because removing a target is `install`'s job and `install`
   rewrites the manifest too.
3. **Prefer a refusal to an unrecoverable success.** Where a failure cannot be reported
   honestly mid-flight, refuse to start. `ballast upgrade` requires a clean
   Ballast-managed tree and a git repository, so a partial run is always undoable with
   `git checkout`; `--force` accepts the risk explicitly.
4. **A check nothing runs is not a check.** Every `scripts/e2e-*.sh` runs in CI. Three of
   eighteen ran nowhere, which is exactly why one of them had been failing on a clean
   checkout undetected.

Sequencing was by **blast radius of the false signal**, not by issue number or effort.

## Alternatives Considered

- **Fix in issue-number order.** #356 is first by number and was last by evidence.
  Leading with it would have spent the most effort on the least-confirmed report.
- **Close the cluster as stale and refile.** The reports were accurate and specific;
  three were confirmed live on inspection. Refiling would have discarded the diagnosis
  already in them.
- **Batch all of it into one PR.** They shared a theme, not a cause. And #361 had to land
  first: until it did, every other fix was committed with its own safety checks disabled.
- **Repair the husky hook rather than remove it** (#361). Rejected once the evidence
  showed lint-staged was never meant to be the hook system here — `.pre-commit-config.yaml`
  is, it is actively maintained, and the `git-hooks` rule mandates it.
- **Build rollback for `ballast upgrade`** (#356). Rejected: it duplicates what git
  already does, across four backends, and would have to be correct in exactly the
  situation where the tool is already failing.
- **Pin the zip compression method** (#358). Rejected as fixing output nobody consumes;
  the `.skill` format was retired in 5.21.0 and the builders were dead code.

## Consequences

### Positive

- A manifest can no longer advertise rules that are not on disk, in any backend.
- `gitleaks`, `detect-private-key`, yamllint and the per-package lint gates actually run
  on commit again. They had been silently inert.
- An interrupted upgrade is recoverable by construction, without rollback machinery.
- All 18 e2e scripts run in CI, closing the gap that let one rot for a week.
- Roughly 290 lines of dead zip code removed from two backends.

### Negative

- `ballast upgrade` is stricter: it now refuses on a dirty Ballast-managed tree and
  outside git. This is a behaviour change for anyone scripting it, mitigated by `--force`.
- Fixtures that run an upgrade must now be git repositories — five wrapper tests and four
  scripts needed updating, and third-party automation may need the same.
- `doctor --fix` deletes less than it used to. Rules under an unconfigured target linger
  until `install --remove-target` runs. Visible beats silent, but it is more steps.

## Implementation Notes

- #361 was diagnosed a layer deeper than reported. `.husky/pre-commit` survived from
  `bd78bae` and `"prepare": "husky"` re-pointed `core.hooksPath` at `.husky/_` on every
  install, so git ran husky and never ran pre-commit at all. The `tsc-files` `ENOENT`
  was the symptom that hid it: it pushed everyone to `--no-verify`, which looked like
  the cause. Fixed by removing the vestige, with a guard test blocking any package from
  reintroducing `prepare`/`husky`/`lint-staged`.
- #359 surfaced the moment #361 landed: with `pre-push` working again, the unit-test gate
  blocked the very next push. Those tests had been failing for every macOS developer all
  along; the broken hook was the only reason nobody was stopped.
- #360 was resolved by confirming against the renderer rather than assuming. Three Python
  tests state the design: manifest-bearing targets omit the tool policy from rules, the
  manifest carries it once, cursor and opencode keep it inline. The script was stale.
- #357's original symptom could not be reproduced on 5.21.0 across three attempts. The
  structural weakness was fixed anyway, with a test pinning the invariant.
- #372's fix was found by writing its regression test first: the test failed, proving the
  `doctor --fix` bug was still live in 5.21.0, contradicting an earlier check that had
  only covered the safe path.

## Verification

- e2e suite **18/18**, all now CI-wired; smoke suite passes.
- TypeScript **382/382**, Python **131/131**, Go backend and wrapper suites pass.
- eslint, prettier, ruff, gofmt and `go vet` clean.
- Every commit after #374 was made with hooks enabled and no `--no-verify`.
- `castoff` verified repaired: 44/44 `ok`, 22 rules per target tracked, and every rule its
  `CLAUDE.md` indexes resolves.

## Lessons Learned

- **A cluster is a triage artifact, not a unit of work.** These seven stalled for a week
  because they looked like one large job. Verifying each against the working tree first
  turned it into four real fixes and two obsolete reports, and made the order obvious.
- **Verify the claim before planning the fix.** Three were live, one was obsolete, two
  were unconfirmed. Planning from the issue text would have produced the wrong sequence
  and, for #358, an elaborate fix to dead code.
- **Fix the tooling that hides problems before the problems.** #361 was not the most
  severe issue, but it was gating: it disabled the checks that would have caught the
  others, and it immediately surfaced #359 the moment it was repaired.
- **Write the regression test before believing a bug is fixed.** The earlier conclusion
  that `doctor --fix` was safe in 5.21.0 was wrong, and only the test proved it.
- **A symptom that everyone routes around is load-bearing.** `--no-verify` had become
  habit, which is precisely why nobody asked what else it was switching off.

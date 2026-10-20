# Task: Selectable code review agent (Copilot, Bosun, or a user-defined skill)

## Context
- Owner: mark@maplabs.com
- Date: 2026-10-19
- Mode: Approval-Required
- PRD Section: n/a — tracked by `plans/plan-code-review-agent-selection.md`
- Requirement IDs: n/a

## Scope
- In scope: `codeReviewer` config plumbed through all four backends; new
  `github-pr-review-cycle` skill supporting Copilot, Bosun, and `skill:<name>`;
  deprecation of `github-pr-copilot-cycle`; docs; regenerated artifacts.
- Out of scope: the Bosun-side review surface, tracked separately in
  `everydaydevopsio/bosun` as `plans/pr-review-surface.md`.

## Acceptance Criteria
- AC1: A repository can select Copilot, Bosun, `none`, or its own reviewer skill, and the
  choice survives reinstall. ✅
- AC2: Default behavior is unchanged for existing repositories (`copilot`). ✅
- AC3: All four backends agree on validation, persistence, and `doctor` output. ✅
- AC4: It is possible to never use Copilot as a reviewer. ✅

## Constraints
- A published skill ID is part of the install contract, so the old ID stays until 6.0.0.
- Skills are copied verbatim with no variable substitution, so the reviewer must be
  resolved at run time rather than templated in.

## Risks and Tradeoffs
- Risk: four-backend drift. Three real parity bugs were found by end-to-end installs
  after all unit suites were green.
- Tradeoff: a flat `codeReviewer` string instead of a nested object. Keeps validation and
  plumbing simple; revisit if per-reviewer options are needed.

## Execution Checklist
- [x] `codeReviewer` through TS config, CLI flag, doctor, install
- [x] Same through the Go wrapper, Go backend, and Python backend
- [x] `github-pr-review-cycle` skill with three dispatch paths
- [x] Deprecate `github-pr-copilot-cycle` (since 5.22.0, remove in 6.0.0)
- [x] All four registries updated, parity tests passing
- [x] `sync:content`; regenerate `.claude/` and `.codex/`; update manifests
- [x] Docs: new skill page, skills index, installation guide

## Test Strategy
- Unit: reviewer normalization and flag parsing in all four backends; config round-trip
  and preservation; registry parity.
- Integration: `repo-generated-artifacts` compares checked-in output to freshly generated.
- E2E: installs into scratch repos per backend, reading back `.rulesrc.json`.
- Failure paths: invalid reviewer rejected with exit 1; invalid saved value dropped
  without failing config load; deprecation warning emitted.
- Requirement-to-test mapping: AC1 → `CodeReviewerTests`, `TestParseCodeReviewerFlag`,
  `config.test.ts` round-trip; AC3 → per-backend E2E installs.

## Rollback Strategy
- Trigger: a repository fails to install or loses its saved reviewer.
- Rollback steps: revert the branch; `codeReviewer` is additive and ignored by older
  Ballast versions, so no config migration is needed.
- Validation after rollback: `ballast install --refresh-config`, confirm `.rulesrc.json`
  and generated rules are unchanged apart from the dropped key.

## Outcome
- Result: Implemented and verified.
- Evidence: TS 393 passed; Python 136 passed; both Go suites ok; `pnpm lint`, `gofmt`,
  `go vet` clean; per-backend install smoke tests confirm persistence and `doctor` output.
- PRD updates: none; see `plans/plan-code-review-agent-selection.md` for the design record
  and implementation notes.

## Follow-ups
- Promoted: [everydaydevopsio/ballast#401](https://github.com/everydaydevopsio/ballast/issues/401)
  — a wrong-typed scalar in `.rulesrc.json` discards the whole config in Go, affecting
  `taskSystem`, `deploymentModel`, `ruleProfile`, and `ballastVersion` as well as
  `codeReviewer`. Pre-existing pattern, surfaced by review of this branch; fixing one field
  in isolation would leave the rest fragile. Also carries the TypeScript equals-form gap for
  the other flags.
- Promoted: [everydaydevopsio/bosun#53](https://github.com/everydaydevopsio/bosun/issues/53)
  — publish findings as a real pull-request review with inline comments. Until that lands,
  `codeReviewer: bosun` uses the conversation-comment path, which the skill already handles.
  Related bosun issues already tracking adjacent work: #28 (failure visibility), #37
  (structured results), #42 (cross-pass campaign).
- Done: graduated to [ADR-005](../adr/005-the-code-review-agent-is-repository-policy.md); `plans/` is empty again.

# Task: Poll both Bosun publication surfaces when deciding a review has settled

## Context
- Owner: mark@maplabs.com
- Date: 2026-10-21
- Mode: Autonomous
- Requirement IDs: n/a — follows `everydaydevopsio/bosun#53`

## Scope
- In scope: the Bosun **Settle** step of `github-pr-review-cycle`, the terminal
  outcome table, the read-findings guidance, and the docs page.
- Out of scope: the Copilot path, the scoring rubric, the cycle bounds.

## Acceptance Criteria
- AC1: A review published as a pull-request review object settles the cycle
  instead of timing out. ✅
- AC2: A review published as a conversation comment still settles, so older
  Bosun deployments keep working. ✅
- AC3: Coverage of the current head is decided by the commit the review names,
  on both surfaces. ✅

## Constraints
- Skills are copied verbatim, so the guidance must work for both Bosun shapes
  without a version check.
- The source skill lives in `skills/common/`; packaged copies are generated by
  `packages/ballast-typescript/scripts/sync-content.sh` and must stay in parity.

## Risks and Tradeoffs
- Risk: an unstamped review from an older Bosun can only be matched by
  timestamp, which is ambiguous for two pushes in quick succession. Called out
  in the skill rather than papered over.

## Execution Checklist
- [x] Settle polls `pulls/{n}/reviews` and `issues/{n}/comments`.
- [x] Prefer `commit_id` on a review object, the metadata line on a comment.
- [x] Terminal outcome table names both surfaces.
- [x] Read-findings covers `### Findings outside the diff` and `### Findings`.
- [x] `docs/skills/github-pr-review-cycle.md` updated.
- [x] Packaged copies synced; installed `.claude`/`.codex` copies refreshed.

## Test Strategy
- Unit: existing parity and registry suites (TypeScript 394, Python 142, Go).
- Failure-path: the outcome table distinguishes failure comments and no review
  on either surface.

## Rollback Strategy
- Trigger: the new polling misreads a settled review.
- Rollback steps: revert the branch; the skill is guidance, with no state.
- Validation after rollback: `pnpm test`.

## Outcome
- Result: complete.
- Evidence links/commands: `pnpm test`, `pytest packages/ballast-python/tests`,
  `go test ./...` in `packages/ballast-go`, whitespace checks.

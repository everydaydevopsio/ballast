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
- Promote before merge: Bosun's review surface
  (`everydaydevopsio/bosun` → `plans/pr-review-surface.md`). Until it publishes a real PR
  review, `codeReviewer: bosun` uses the conversation-comment path. Needs a tracked issue
  in GitHub per the task-system rule.
- At merge: graduate `plans/plan-code-review-agent-selection.md` to an ADR.

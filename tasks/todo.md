# Task: Triage the three remaining plan files

## Context

- Owner: Mark C Allen
- Date: 2026-09-29
- Mode: Approval-Required (each outcome is a scope decision, not a mechanical edit)
- Related ADRs: [ADR-002](../adr/002-trust-ballasts-own-feedback-channels.md)

All three surviving files in `plans/` were kept during the ADR-002 graduation because each
still references open issues. Keeping them was the right call, but none is currently
accurate: one has had its entire priority table completed out from under it, one has
silently had its first phase delivered, and one has sat untouched for a month with its
open questions unanswered.

A stale plan is worse than no plan — it is read as current intent.

## Scope

- In scope: deciding the disposition of each of the three `plans/*.md` files, and bringing
  each into line with reality (update, graduate, or retire).
- Out of scope: doing the implementation work those plans describe (#128, #94, #153, #149,
  #147, #303). Those are tracked as GitHub issues already.

## Acceptance Criteria

- AC1: Every file in `plans/` accurately states its status and what remains.
- AC2: Any plan whose work is complete is graduated to an ADR and removed, per the
  plan-lifecycle rule.
- AC3: Any plan that will not be pursued is removed, with its rationale captured on the
  relevant issue rather than lost.
- AC4: Each remaining open question has an answer recorded, or a ticket carrying it.

## Risks and Tradeoffs

- Risk: retiring a plan discards design thinking that is not recorded anywhere else.
  Mitigate by moving the reasoning to the governing issue or an ADR before deleting.
- Tradeoff (resolved): `issue-priority-plan.md` was the only forward-looking index of what
  Ballast does next, which argued for keeping it. Retired anyway — every entry in its
  priority table is a GitHub issue, so the index duplicated tracking rather than adding to
  it, and a second place to record priority is a second place to go stale.

## Execution Checklist

- [x] **`plans/issue-priority-plan.md` — retired.** Its completed context-hygiene
      workstream (#286–#297) was recorded nowhere durable, so it graduated to
      [ADR-003](../adr/003-budget-the-always-on-rule-payload.md) rather than being deleted
      outright. Its forward priority table added nothing over the GitHub issues it listed
      (#128, #94, #153, #149, #147), all of which remain open and tracked.
- [x] **`plans/plan-setup-toolchain.md` — retired.** Phase 1 (Go 1.26) had already shipped
      via #339/#342/#325, leaving only #128 and #94, which are tracked as issues. The
      Phase 2 and Phase 3 design notes, and the unanswered parity open question, were
      posted to #128 and #94 before deletion so nothing was lost.
- [ ] **`plans/plan-spec-kit-development-process.md` — decide whether to pursue.** Proposed
      2026-08-29, a month old with no work started and four open questions unresolved.
      Related issue #303 is still open. Either commit to it and answer the questions, or
      retire it and record why on #303.
- [ ] Note while triaging: that plan's open question *"should `docs/code_review.md` be
      created, or should `AGENTS.md` point to an existing review document?"* is live issue
      **#340**, filed separately on 2026-09-17. Answer it once, in one place.
- [ ] Promote whichever of these outlive this branch into GitHub issues and record the links
      here, per the task-system rule. `tasks/todo.md` is not durable tracking.

## Test Strategy

Documentation-only; no test changes expected. Verification is by inspection:

- Every link in `plans/README.md` resolves, and each row's Status matches its file.
- No plan references an issue that is closed without saying so.

## Outcome

- Result: _pending_
- Evidence links/commands: _pending_

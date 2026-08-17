# Branch-Local TODO Tracking

Manage `tasks/todo.md` during branch work. Triage all unchecked items before creating a PR.

---
# Structured Task TODO Rules

These rules define how to use lowercase `tasks/todo.md` for branch-scoped planning, execution notes, evidence, and PR triage.

---
You are a branch task tracking specialist. Keep `tasks/todo.md` aligned with the structured execution template, and make sure outstanding work is resolved or promoted before a PR is completed.

## What `tasks/todo.md` Is For

`tasks/todo.md` is the canonical branch-local task artifact. Use it to capture:
- Context, scope, constraints, risks, and acceptance criteria for the current branch.
- Execution checklist items with observable outcomes.
- Test strategy, failure-path coverage, rollback strategy, and completion evidence.
- Small discovered follow-ups that are expected to be resolved in the current branch.

`tasks/todo.md` is not durable external issue tracking. Work that must survive beyond the current branch belongs in the configured task system, with the issue link recorded in `tasks/todo.md`.

## Lightweight Use

Lightweight tasks may omit optional sections that do not apply, but the file must remain a subset of the structured template. Do not switch to a separate flat checklist format. Keep the sections needed to preserve acceptance criteria, execution checklist, test evidence, and outcome.

## When to Add Items Here vs. Create a Ticket Immediately

Add to `tasks/todo.md` when:
- The item is small and likely to be resolved within the current branch.
- The item is a reminder for the current implementation.
- The item needs short-lived context, test evidence, or rollback notes for this branch.

Create a ticket in the configured task system immediately when:
- The item is clearly out of scope for the current branch.
- The item would block another team member or another piece of work.
- The item is a bug that could affect users now or after release.
- You know you will not resolve it in this branch.

## `tasks/todo.md` Template

```markdown
# Task: <title>

## Context
- Owner:
- Date:
- Mode: <Autonomous|Approval-Required>
- PRD Section:
- Requirement IDs:

## Scope
- In scope:
- Out of scope:

## Acceptance Criteria
- AC1:
- AC2:

## Constraints
- Constraint 1

## Risks and Tradeoffs
- Risk:
- Tradeoff:

## Execution Checklist
- [ ] Step 1 with observable outcome
- [ ] Step 2 with observable outcome

## Test Strategy
- Unit:
- Integration:
- E2E:
- Failure-path tests:
- Requirement-to-test mapping:

## Rollback Strategy
- Trigger:
- Rollback steps:
- Validation after rollback:

## Outcome
- Result:
- Evidence links/commands:
- PRD updates:
```

## `tasks/lessons.md` Template

Use `tasks/lessons.md` for durable learning after corrections, regressions, or repeated failure patterns.

```markdown
# Lessons

## <YYYY-MM-DD> <Short Title>
- Incident/bug:
- Root cause pattern:
- Early signal missed:
- Preventative rule:
- Validation added (test/check/alert):
- Next trigger to detect sooner:
```

## Issue Output Template

Use this strict issue output format when presenting work that needs a decision or durable external tracking.

```markdown
### Issue #N: <Short Description>

**Severity:** <Critical|High|Medium|Low>
**User Impact:** <who is affected and how>
**Likelihood:** <High|Medium|Low>
**Time Sensitivity:** <Immediate|This sprint|Backlog>

**Problem**
Concrete explanation with file/line references and example behavior.

**Option A (Recommended)**
- Effort:
- Risk:
- Code Impact:
- Maintenance:

**Option B**
- Effort:
- Risk:
- Code Impact:
- Maintenance:

**Option C (Optional / Do Nothing)**
- Effort:
- Risk:
- Code Impact:
- Maintenance:

**Recommendation**
Explain why Option A is best based on correctness, risk, testability, and maintenance.

**Decision Request**
Proceed with: A (recommended), B, C, or alternate direction?
```

## Before Creating a PR

When preparing a PR, check `tasks/todo.md` for unchecked execution checklist items, unresolved acceptance criteria, missing test evidence, and unfinished outcome notes.

Do not proceed with the PR until each remaining item has been triaged:

1. Resolve it now.
2. Promote it to the configured task system and record the issue link.
3. Remove it only if it is no longer relevant.

## Important Notes

- `tasks/todo.md` is intentionally lowercase.
- `tasks/todo.md` may merge into `main` as the record of branch work.
- Items promoted to tracked issues should include the issue URL before the PR is merged.
- Keep entries short and actionable; move durable design history to the governing PRD, ADR, or issue.

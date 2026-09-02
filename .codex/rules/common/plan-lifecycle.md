<!-- ballast:rule id="typescript/plan-lifecycle" version="5.18.3" checksum="1c806d1446a0a14ad4b7deb46e23c86e653dd0d7be322f44937e9e66bfd04eb2" -->
# Plan Lifecycle

These rules are intended for Codex (CLI and app).

Create and maintain plans for non-trivial work, then graduate completed plans to ADRs.

---
# Plan -> ADR Lifecycle Rules

These rules define the Plan -> ADR lifecycle: when agents create plans, how plans stay current during implementation, and how completed plans graduate into architecture decision records.

---
You are a plan lifecycle specialist. Your role is to preserve implementation context for non-trivial work and turn completed decisions into durable ADRs before merge.

## When To Create A Plan

Create a plan when:
- The change touches more than two files.
- You are unsure of the approach.
- The feature takes more than one session to complete.
- The work involves architectural decisions.

Skip a plan when:
- The change is a single-file fix such as a typo, log line, or rename.
- The entire change fits in one sentence.

## Directory Structure

Use this project-root structure:

```text
<project-root>/
+-- plans/
|   +-- README.md
|   +-- plan-<feature-name>.md
+-- tasks/
|   +-- todo.md
+-- adr/
    +-- README.md
    +-- NNN-<decision-title>.md
```

Defer `tasks/todo.md` behavior to the branch-local TODO tracking rule. Use it for discovered out-of-scope work instead of expanding the plan beyond the feature boundary.

## Plan File Naming

Create plans at `plans/plan-<feature-name>.md`.

- Use kebab-case.
- Be specific: `plan-oauth-google.md`, not `plan-auth.md`.
- After creating the plan, update `plans/README.md` and commit both files.

## Plan Template

```markdown
# Plan: <Feature Name>

**Status:** In Progress
**Branch:** <branch-name>
**Created:** YYYY-MM-DD
**Related ADRs:** _(link any relevant existing ADRs)_

## Problem

What are we solving and why does it matter now?

## Approach

The chosen solution in plain language. What will change and how.

## Files Affected

- `src/...` - reason
- `tests/...` - reason

## Phases

- [ ] Phase 1: Explore and confirm approach
- [ ] Phase 2: Core implementation
- [ ] Phase 3: Tests and edge cases
- [ ] Phase 4: Documentation and cleanup

## Verification

How will we know this works? Commands, tests, or checks to run.

## Alternatives Rejected

| Option | Why rejected |
| --- | --- |
| ... | ... |

## Open Questions

Things still to resolve. Remove entries as they are answered.

## Change Log

| Date | Change |
| --- | --- |
| YYYY-MM-DD | Plan created |
```

## Maintaining The Plan

- Check off phases as they complete.
- If the approach changes, update **Approach** and record the change in **Change Log**.
- Commit plan updates alongside related code changes.
- At the start of each session, read the plan to restore context.
- When you discover real out-of-scope work, add it to `tasks/todo.md` under the branch-local TODO tracking rule instead of widening the plan.

## Graduation Trigger

When the feature is ready to merge, use this trigger phrase:

> "Graduate `plans/plan-<feature-name>.md` to an ADR"

## Graduation Steps

1. Check `tasks/todo.md` for incomplete items (`- [ ]`) added during this feature.
2. For each incomplete item, create a task system work item, then update the line to `- [x] TASK-NNN: <description>`.
3. Determine the next ADR number from `adr/README.md`.
4. Create `adr/NNN-<decision-title>.md` from the plan content.
5. Update `adr/README.md` with the new row.
6. Remove `plans/plan-<feature-name>.md`.
7. Update `plans/README.md` to remove the entry.
8. Commit with `docs: graduate plan-<feature-name> to ADR-NNN`.

Graduation is blocked until every feature-related TODO item is checked off or has a task system work item reference.

## ADR Template

```markdown
# ADR-NNN: <Decision Title>

**Status:** Accepted
**Date:** YYYY-MM-DD
**Branch:** <branch-name>
**PR:** #<number>
**Supersedes:** _(ADR-NNN if replacing an earlier decision)_
**Superseded by:** _(leave blank)_

## Context

Why did this decision need to be made?

## Decision

What was chosen and why.

## Alternatives Considered

| Option | Reason not chosen |
| --- | --- |
| ... | ... |

## Consequences

### Positive

- What becomes easier

### Negative or trade-offs

- What becomes harder or what we gave up

## Implementation Notes

Key details future readers should know.

## Verification

How the decision was validated.

## Lessons Learned

What would you do differently? What worked better than expected?
```

## ADR Management Rules

| Rule | Detail |
| --- | --- |
| Never delete | Mark superseded and create a new ADR |
| Sequential numbering | Zero-padded three digits: `001`, `002`, `003` |
| One decision per ADR | Do not bundle unrelated decisions |
| Status values | `Accepted`, `Deprecated`, `Superseded` |

## Quick Reference

| Situation | Action |
| --- | --- |
| Starting a feature | Create `plans/plan-<name>.md`, commit it |
| New session on existing feature | Continue implementing `plans/plan-<name>.md` |
| Approach changed | Update plan and Change Log, commit with code |
| Phase complete | Check off in plan, commit |
| Discovered out-of-scope work | Add to `tasks/todo.md`, commit alongside current change |
| Ready to merge | Graduate `plans/plan-<name>.md` to an ADR |
| Graduation finds open TODO items | Create task system issues, add references to `tasks/todo.md`, then proceed |
| Decision reversed later | Mark ADR superseded, create a new ADR |
| Small single-file fix | Skip the plan entirely |

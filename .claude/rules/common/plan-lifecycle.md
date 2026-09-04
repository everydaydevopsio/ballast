<!-- ballast:rule id="typescript/plan-lifecycle" version="5.18.3" checksum="81dbdf8bb1131403f4be7ad4df369435b06bf28869a95fdb29c33e642b3faafd" -->
# Plan Lifecycle

Create and maintain plans for non-trivial work, then graduate completed plans to ADRs.

---
# Plan -> ADR Lifecycle Rules

These rules define the Plan -> ADR lifecycle: when agents create plans, how plans stay current during implementation, and how completed plans graduate into architecture decision records.

---
You are a plan lifecycle specialist. Your role is to preserve implementation context for non-trivial work and turn completed decisions into durable ADRs before merge.

## When To Create A Plan

Create a plan when the change touches more than two files, the approach is uncertain, the feature spans multiple sessions, or the work involves architectural decisions. Skip a plan for single-file fixes or changes that fit in one sentence.

## Structure

- Plans live at `plans/plan-<feature-name>.md` (kebab-case, specific: `plan-oauth-google.md`, not `plan-auth.md`); index them in `plans/README.md` and commit both.
- ADRs live at `adr/NNN-<decision-title>.md` with an index in `adr/README.md`.
- Discovered out-of-scope work goes to `tasks/todo.md` under the branch-local TODO tracking rule, not into a widened plan.

## Plan Contents

Each plan is a markdown doc with: a header (Status, Branch, Created date, Related ADRs), **Problem**, **Approach**, **Files Affected** (path + reason), **Phases** (checkboxes, typically explore -> core implementation -> tests and edge cases -> docs and cleanup), **Verification**, **Alternatives Rejected** (option + why), **Open Questions**, and a **Change Log** table (date + change).

## Maintaining The Plan

- Check off phases as they complete; commit plan updates alongside related code changes.
- If the approach changes, update **Approach** and record it in the **Change Log**.
- At the start of each session, read the plan to restore context.

## Graduation

When the feature is ready to merge, the trigger phrase is: "Graduate `plans/plan-<feature-name>.md` to an ADR".

1. Triage incomplete `tasks/todo.md` items from this feature: create task system work items and mark the lines `- [x] TASK-NNN: <description>`. Graduation is blocked until every feature TODO is checked off or referenced.
2. Take the next ADR number from `adr/README.md` and create `adr/NNN-<decision-title>.md` from the plan.
3. Update `adr/README.md`, remove the plan file, update `plans/README.md`, and commit with `docs: graduate plan-<feature-name> to ADR-NNN`.

## ADR Contents

Each ADR contains: a header (Status: Accepted/Deprecated/Superseded, Date, Branch, PR, Supersedes/Superseded-by), **Context**, **Decision**, **Alternatives Considered** (option + reason not chosen), **Consequences** (positive and negative), **Implementation Notes**, **Verification**, and **Lessons Learned**.

## ADR Management Rules

- Never delete an ADR: mark it superseded and create a new one.
- Sequential zero-padded numbering (`001`, `002`, ...), one decision per ADR.

## Quick Reference

| Situation | Action |
| --- | --- |
| Starting a feature | Create `plans/plan-<name>.md`, index it in `plans/README.md`, commit both |
| New session on existing feature | Continue implementing the plan |
| Approach changed | Update plan and Change Log, commit with code |
| Discovered out-of-scope work | Add to `tasks/todo.md`, commit alongside current change |
| Ready to merge | Graduate the plan to an ADR |
| Decision reversed later | Mark ADR superseded, create a new ADR |
| Small single-file fix | Skip the plan entirely |

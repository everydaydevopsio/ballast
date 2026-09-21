<!-- ballast:rule id="typescript/tasks/todo" version="5.19.0" checksum="edb80a226a3cac749d0c8e2fe42da69b643686f3db45136d58ce40e979004ce1" -->
# Branch-Local TODO Tracking

Manage `tasks/todo.md` during branch work. Triage all unchecked items before creating a PR.

---
Keep `tasks/todo.md` aligned with the structured execution template, and make sure outstanding work is resolved or promoted before a PR is completed.

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

## Templates

`tasks/todo.md`, `tasks/lessons.md`, and the issue output format all have canonical skeletons in
`docs/agents/tasks.md`. Read that file when creating or restructuring one of them, and copy the
relevant template verbatim rather than improvising a structure.

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

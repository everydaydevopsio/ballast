# Branch-Local TODO Tracking

These rules are intended for Codex (CLI and app).

Manage `tasks/TODO.md` during branch work. Triage all unchecked items before creating a PR.

---
# Branch-Local TODO Tracking Rules

These rules define how to use `tasks/TODO.md` for branch-scoped working notes and what must happen before a PR is completed.

---
You are a branch task tracking specialist. Your role is to keep `tasks/TODO.md` accurate during a branch and ensure all outstanding items are triaged before the PR is merged.

## What `tasks/TODO.md` Is For

`tasks/TODO.md` is a branch-scoped scratchpad for work that comes up during implementation. Use it to capture:
- Sub-tasks discovered while working that are too small to warrant a ticket right now but must not be forgotten.
- Deferred decisions or follow-up questions for the current branch.
- Small cleanup items that should happen before the PR is done.

`tasks/TODO.md` is **not** a substitute for the configured task system. It is working memory for the current branch, not durable issue tracking.

## When to Add Items Here vs. Create a Ticket Immediately

Add to `tasks/TODO.md` when:
- The item is small and likely to be resolved within the current branch.
- The item is a reminder for yourself mid-implementation.
- You are not sure yet whether it warrants a tracked issue.

Create a ticket in the configured task system immediately when:
- The item is clearly out of scope for the current branch.
- The item would block another team member or another piece of work.
- The item is a bug that could affect users now or after release.
- You know you will not resolve it in this branch.

## File Format

Keep `tasks/TODO.md` as a simple markdown checklist:

```markdown
# TODO

- [ ] Add input validation to the config parser
- [ ] Follow up: confirm rate limit behavior with the API team
- [x] Write tests for the new agent content path
```

Mark items done with `[x]` as you complete them. Leave unchecked items visible so they are not forgotten.

## Before Creating a PR

When the user is about to create a PR or asks you to help prepare one, check whether `tasks/TODO.md` exists and has any unchecked items (`- [ ]`).

If unchecked items remain, **do not proceed with creating the PR** until each item has been triaged. For each unchecked item, ask the user to choose one of:

1. **Resolve it now** — implement or address it before the PR is opened.
2. **Create a task** — open an issue in the configured task system and replace the TODO entry with a link to that issue.
3. **Delete it** — remove it from `tasks/TODO.md` because it is no longer relevant.

Only proceed with the PR once every item is either checked off, linked to a tracked issue, or removed.

## After Triage

Once all items are resolved, the `tasks/TODO.md` file may be:
- Left as a fully checked list (all `[x]`) — this is fine and gives a useful record of what was done.
- Cleared to an empty checklist if there are no remaining items worth keeping.

Do **not** delete `tasks/TODO.md` from the branch. It should merge into `main` so the record of branch work is preserved.

## Important Notes

- `tasks/TODO.md` merges into `main` intentionally — it is not gitignored.
- Items that get promoted to tracked issues should have the issue URL noted in the file before the PR is merged.
- Keep entries short and actionable — this is a scratchpad, not a design document.
- If `tasks/TODO.md` does not exist at PR time, that is fine; no triage is needed.

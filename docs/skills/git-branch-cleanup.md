# `git-branch-cleanup`

The `git-branch-cleanup` skill cleans up local branches, remote branches, and git worktrees. It deletes what git can prove is already merged into the default branch and reports everything else with its pull request status instead of guessing.

## Install It

Examples:

```bash
pnpm exec ballast-typescript install --target codex --skill git-branch-cleanup
ballast-go install --target claude --skill git-branch-cleanup
ballast install --target opencode --skill git-branch-cleanup --yes
```

## What It Does

Without asking:

- Deletes local branches that are ancestors of the default branch.
- Deletes squash-merged and rebase-merged branches, confirmed with `git cherry` plus a merged pull request. These look unmerged to `git branch --merged`, which is why they accumulate.
- Prunes remote-tracking refs for branches deleted upstream, and prunes worktree metadata for directories that no longer exist.
- Removes clean worktrees whose branch is already merged.

After asking, in one batch:

- Unmerged branches, branches whose upstream is `[gone]`, and branches whose pull request was closed without merging.
- Worktrees holding uncommitted changes, unmerged branches, or a lock.
- Remote branch deletion, which is handled as its own confirmed phase and never touches protected branches or branches backing an open pull request.

## What It Reports

- Every unmerged branch with commits ahead, age, pull request number and state, check status, and review decision.
- Branches with **no pull request at all**, excluding the default branch and configured long-lived branches. This is the skill's main signal: an unmerged branch with no pull request is untracked work.
- Branches with unpushed commits that exist only on the local machine.
- Git hygiene recommendations, including enabling `delete_branch_on_merge` and `fetch.prune` so most of this cleanup stops being necessary.

Every automatic deletion is reported with its SHA so it can be restored with `git branch <name> <sha>`.

## Requirements

`git` is required. `gh` is optional — without it the skill runs every git step and reports pull request columns as unknown, rather than skipping the cleanup.

---
name: git-branch-cleanup
description: >
  Clean up local branches, remote branches, and git worktrees. Use this skill
  whenever the user asks to: delete merged branches, clean up branches, prune
  branches, tidy up git, remove stale branches, clean up worktrees, list
  unmerged branches and their PRs, find branches without a PR, "delete local
  branches which are merged into main", "what branches do I still have open",
  "prune remote-tracking refs", "which of my branches have no PR", or any
  variation of "clean up my git branches". Deletes anything already merged into
  the default branch with no extra commits, and asks before touching the rest.
---

# Git Branch and Worktree Cleanup Skill

Removes branches and worktrees that are provably finished, then reports on everything that is not, so nothing in-flight is lost. Merged-and-empty refs are deleted without asking. Everything else is listed with its pull request status and requires an explicit decision.

The governing rule is **evidence before deletion**: a branch is only auto-deleted when git can prove its work is already in the default branch.

---

## Prerequisites

```bash
git rev-parse --is-inside-work-tree
gh auth status
```

`gh` is optional. Without it, run every git step and report PR columns as `unknown (gh unavailable)` instead of skipping the cleanup.

Establish the default branch and remote once; every later step depends on them:

```bash
REMOTE=$(git remote | grep -x origin || git remote | head -1)
DEFAULT_BRANCH=$(gh repo view --json defaultBranchRef --jq '.defaultBranchRef.name' 2>/dev/null \
  || git symbolic-ref --short "refs/remotes/${REMOTE}/HEAD" 2>/dev/null | sed "s|^${REMOTE}/||" \
  || echo main)
echo "Remote: $REMOTE   Default branch: $DEFAULT_BRANCH"
```

If every detection fails, ask the user for the default branch rather than guessing. A wrong default branch turns this skill into a data-loss tool.

---

## Step 0 — Safety Preflight

Run before deleting anything. Any failure here stops the cleanup.

```bash
# Uncommitted work in the current worktree
git status --porcelain

# Current branch — never delete the branch you are standing on
git branch --show-current

# Refresh remote state and drop remote-tracking refs for branches deleted upstream
git fetch --all --prune --tags
```

**Rules:**

- If `git status --porcelain` is non-empty, report the dirty state and continue with read-only analysis; do not switch branches to enable a deletion.
- Never delete the current branch, the default branch, or any branch checked out in a worktree.
- Create a recovery point before the first deletion so every deleted ref can be restored:

  ```bash
  git log --all --oneline --no-decorate > /dev/null  # warm the ref cache
  git for-each-ref --format='%(refname:short) %(objectname)' refs/heads > "/tmp/branch-backup-$(git rev-parse --short HEAD).txt"
  ```

  Keep the file path and mention it in the report. `git reflog` and `git fsck --lost-found` remain the fallback.

---

## Step 1 — Classify Every Local Branch

Classification decides who gets deleted silently and who gets a question. Compute it from git, not from branch names.

```bash
git for-each-ref --format='%(refname:short)|%(upstream:short)|%(upstream:track)|%(committerdate:relative)|%(committerdate:unix)|%(authorname)' refs/heads
```

For each local branch `B` other than `$DEFAULT_BRANCH`:

```bash
# 1. Is every commit on B already reachable from the default branch?
git merge-base --is-ancestor "$B" "${REMOTE}/${DEFAULT_BRANCH}" && echo "ANCESTOR" || echo "NOT_ANCESTOR"

# 2. How many commits does B carry that the default branch does not?
git rev-list --count "${REMOTE}/${DEFAULT_BRANCH}..$B"

# 3. Does B's content already exist in the default branch under a different SHA?
#    Catches squash-merged and rebase-merged branches, which fail the ancestor test.
git cherry "${REMOTE}/${DEFAULT_BRANCH}" "$B" | grep -c '^+' || true
```

**Categories:**

| Category | Test | Action |
|---|---|---|
| `MERGED` | `git merge-base --is-ancestor` succeeds | Auto-delete |
| `SQUASH_MERGED` | Not an ancestor, but `git cherry` reports 0 `+` lines, or its PR is merged and the branch has no commits after the merge | Auto-delete |
| `UNMERGED` | Carries commits absent from the default branch | Report, ask |
| `GONE` | `%(upstream:track)` contains `[gone]` | Report, ask — the remote branch was deleted |
| `NO_UPSTREAM` | No tracking branch configured | Report, ask — work may exist only locally |

The `git cherry` check matters more than the ancestor check in practice: squash merge is the common default on GitHub, and squash-merged branches look unmerged to `git branch --merged`. Treating them as unmerged leaves the branch list permanently cluttered.

Cross-check `SQUASH_MERGED` against the PR before auto-deleting:

```bash
gh pr list --state merged --head "$B" --json number,mergedAt,mergeCommit,title --limit 1
```

A merged PR plus zero commits newer than `mergedAt` confirms the branch is finished. If `gh` is unavailable, rely on `git cherry` alone and say so in the report.

---

## Step 2 — Auto-Delete Merged Local Branches

No confirmation needed for `MERGED` and confirmed `SQUASH_MERGED` branches.

```bash
# Use -d (not -D) for MERGED: git refuses if the branch is not actually merged.
git branch -d "$B"

# SQUASH_MERGED branches require -D because git cannot see the equivalence.
# Only run this after the git cherry / merged-PR check above has passed.
git branch -D "$B"
```

Record each deletion with its SHA so it stays recoverable:

```bash
echo "deleted $B at $(git rev-parse "$B")"
```

**Skip and report instead of deleting when:**

- The branch is the current branch — note that the user should switch to `$DEFAULT_BRANCH` first.
- The branch is checked out in another worktree — `git branch -d` fails; handle it in Step 5.
- `git branch -d` fails for any other reason — report the error verbatim, do not escalate to `-D`.

---

## Step 3 — Report Unmerged Local Branches with PR Status

Every remaining local branch gets a row. Look up its PR with the branch name as the head ref:

```bash
gh pr list --head "$B" --state all --limit 5 \
  --json number,title,state,isDraft,url,updatedAt,reviewDecision,statusCheckRollup,mergeStateStatus
```

Build one row per branch:

| Field | Source |
|---|---|
| Branch | local ref name |
| Ahead / Behind | `git rev-list --left-right --count "${REMOTE}/${DEFAULT_BRANCH}...$B"` |
| Last commit | `%(committerdate:relative)` |
| Upstream | `%(upstream:short)` or `none` |
| PR | number + state (`OPEN` / `DRAFT` / `CLOSED` / `MERGED`) + URL |
| Checks | passing / failing / pending from `statusCheckRollup` |
| Review | `reviewDecision` |

**Highlight these cases explicitly — they are the point of the report:**

- **No PR** — flag loudly. Every unmerged branch is expected to have one. Exclude the default branch and any configured long-lived branches (`develop`, `release/*`, `staging`, `production`) from this flag; name the exclusions in the report.
- **PR closed without merge** — the branch holds abandoned work; it is a deletion candidate but always ask.
- **PR merged but branch still has newer commits** — post-merge commits need their own PR.
- **Branch with `[gone]` upstream** — the remote is gone; usually safe to delete, still ask.
- **Stale** — no commits in 90+ days. Call out the age; stale unmerged work is where silent loss happens.
- **Draft PR open 30+ days** — ask whether it should be closed or finished.

---

## Step 4 — Ask About the Rest

Group the branches needing a decision and ask in a single batch, not one prompt per branch:

```text
These branches are not merged. Delete any?

  [1] fix-login-redirect      3 commits ahead, 12 days old, PR #412 OPEN (checks passing)
  [2] spike-new-parser        8 commits ahead, 94 days old, NO PR  ⚠
  [3] hotfix-cache            1 commit ahead,  40 days old, PR #388 CLOSED (not merged)
  [4] old-feature             2 commits ahead,  6 days old, upstream [gone]

Reply with the numbers to delete, "none", or "all".
```

Apply the answer with `git branch -D` and report each deleted branch's SHA so it can be restored with `git branch <name> <sha>`.

Default to keeping a branch when the answer is ambiguous or absent.

---

## Step 5 — Clean Up Worktrees

Worktrees pin branches and block deletion, so handle them after local branches.

```bash
# Current worktrees with their branches and state
git worktree list --porcelain

# Remove administrative entries for worktree directories that no longer exist
git worktree prune --dry-run -v
git worktree prune -v
```

**Auto-remove without asking:**

- Worktrees whose directory is missing from disk — `git worktree prune` handles these; they are bookkeeping, not work.
- Worktrees whose branch classified as `MERGED` or confirmed `SQUASH_MERGED`, provided the worktree is clean:

  ```bash
  git -C "$WORKTREE_PATH" status --porcelain   # must be empty
  git worktree remove "$WORKTREE_PATH"         # never use --force here
  ```

  Removing the worktree frees the branch; delete the branch afterward in Step 2's pass.

**Always ask before:**

- Removing a worktree with uncommitted or untracked changes. Show `git -C "$WORKTREE_PATH" status --short` so the user sees what is at risk, and never pass `--force` without an explicit yes.
- Removing a worktree on an unmerged branch.
- Removing a locked worktree — report the lock reason from `git worktree list --porcelain` and leave it alone unless the user unlocks it:

  ```bash
  git worktree unlock "$WORKTREE_PATH"
  ```

---

## Step 6 — Clean Up Remote Branches

Remote deletion is the only outward-facing action in this skill and is irreversible for other clones. Treat it as a separate, explicitly-confirmed phase.

```bash
# Remote branches already merged into the default branch
git branch -r --merged "${REMOTE}/${DEFAULT_BRANCH}" \
  | grep -v -E "HEAD|${DEFAULT_BRANCH}\$" \
  | sed "s|^ *${REMOTE}/||"

# Remote branches whose PR is already merged
gh pr list --state merged --limit 100 --json number,headRefName,mergedAt,title

# Remote branches with no PR at all
gh api "/repos/{owner}/{repo}/branches?per_page=100" --jq '.[].name'
```

**Rules:**

- Never delete a protected branch. Check first and exclude every match:

  ```bash
  gh api "/repos/{owner}/{repo}/branches?protected=true&per_page=100" --jq '.[].name'
  gh api "/repos/{owner}/{repo}/rulesets?per_page=100" --jq '.[] | {name, target, enforcement, conditions}'
  ```

- Never delete the default branch or any branch backing an open PR.
- Recommend enabling GitHub's automatic head-branch deletion so this phase stops being necessary:

  ```bash
  gh api "/repos/{owner}/{repo}" --jq '.delete_branch_on_merge'
  gh api -X PATCH "/repos/{owner}/{repo}" -F delete_branch_on_merge=true   # only with user approval
  ```

- Ask once with the full list, then delete the approved set:

  ```bash
  git push "$REMOTE" --delete "$BRANCH"
  ```

- If a remote branch was created by someone else, say so (`gh pr list --head "$BRANCH" --json author`) and default to leaving it.

---

## Step 7 — Repository Maintenance

Deleting refs leaves garbage behind. Finish with the cheap, safe maintenance pass:

```bash
# Expire unreachable reflog entries older than the default window and repack
git gc --prune=now --quiet

# Report repository size before and after when the repo is large
git count-objects -vH
```

Skip `git gc` when branches were deleted in this run and the user may still want to recover them — reflog expiry is what makes a deletion permanent. Prefer to run it only when the session deleted nothing, or when the user explicitly asks to reclaim space. Say which choice you made.

---

## Generate Cleanup Report

```text
## Git Branch & Worktree Cleanup
**Repo**: owner/name
**Default branch**: main
**Date**: <today>

### Deleted Automatically
| Branch | Reason | SHA (for recovery) |
|---|---|---|
| feat-x | merged into main | a1b2c3d |
| fix-y  | squash-merged via PR #401 | e4f5g6h |

Recover any of these with: git branch <name> <sha>

### Unmerged Branches — Action Needed
| Branch | Ahead | Age | PR | Checks | Flag |
|---|---|---|---|---|---|
| fix-login | 3 | 12d | #412 OPEN | passing | — |
| spike-parser | 8 | 94d | **NONE** | — | ⚠ no PR, stale |
| hotfix-cache | 1 | 40d | #388 CLOSED | — | ⚠ PR closed unmerged |

### Branches Without a PR  ⚠
- spike-parser (8 commits, 94 days) — open a PR or delete
(excluded from this check: main, develop, release/*)

### Worktrees
| Path | Branch | State | Action |
|---|---|---|---|
| ../wt-feat-x | feat-x | clean, merged | removed |
| ../wt-spike | spike-parser | 4 uncommitted files | kept — needs decision |
| (missing dir) | — | stale metadata | pruned |

### Remote Branches
- Merged remote branches deleted: N (listed)
- Protected / open-PR branches skipped: N (listed)
- `delete_branch_on_merge`: enabled / **disabled — recommend enabling**

### Recommended Actions
1. [HIGH] ...
2. [MEDIUM] ...

### Summary
X local branches deleted, Y worktrees removed, Z remote branches deleted,
N branches awaiting a decision, M branches missing a PR.
```

---

## Git Hygiene Recommendations

Include any that apply, with the command to fix them:

- **`delete_branch_on_merge` disabled** — enabling it removes most of this skill's remote work.
- **No automatic pruning** — `git config --global fetch.prune true` and `git config --global fetch.pruneTags true` keep remote-tracking refs honest on every fetch.
- **Branch naming is inconsistent** — if most branches follow a convention (`feat/`, `fix/`, `issue-NNN-`), note the outliers; consistent names make this cleanup and PR traceability easier.
- **Long-lived branches diverging** — a branch more than 50 commits behind the default branch will be painful to merge. Report the count and recommend rebasing or closing it.
- **Branches without issue references** — when the repo tracks work in issues, an unmerged branch with no PR and no issue is untracked work.
- **Worktrees accumulating** — more than ~5 active worktrees usually means finished ones were never removed.
- **Unpushed local commits** — report branches whose commits exist nowhere but this machine; they are one `rm -rf` from gone:

  ```bash
  git for-each-ref --format='%(refname:short) %(upstream:track)' refs/heads | grep -E 'ahead|^\S+ $'
  ```

---

## Edge Cases

- **Detached HEAD** — report it; skip current-branch checks and do not switch branches.
- **No remote configured** — run local branch and worktree cleanup only; skip Steps 3 and 6 and say why.
- **`gh` not authenticated** — run every git-only step; report PR columns as unknown and tell the user to run `gh auth login` for the PR view.
- **Fork workflow** — `gh pr list --head` may miss PRs opened from a fork onto upstream. Check with `gh pr list --head "$OWNER:$B"` before declaring a branch PR-less.
- **Branch names with slashes** — always quote branch variables; `feat/x` is a valid ref and a valid path fragment.
- **Shallow clone** — `git merge-base --is-ancestor` can be wrong. Detect with `git rev-parse --is-shallow-repository`; if true, run `git fetch --unshallow` or skip auto-deletion entirely and report everything for review.
- **Submodules** — removing a worktree does not clean its submodule checkouts; note any that remain.
- **Rate limiting** — if `gh` returns 429, report that PR data is incomplete and never let a missing PR lookup justify a deletion.
- **Branch is someone else's** — compare `%(authorname)` against `gh api /user`; default to leaving other people's branches alone even when merged.

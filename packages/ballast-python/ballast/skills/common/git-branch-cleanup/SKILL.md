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

`gh` is optional. Without it, run every git step and report PR columns as `unknown (gh unavailable)` instead of skipping the cleanup. Auto-deletion narrows to branches that are strict ancestors of the default branch, because confirming a squash merge requires the merged PR's head SHA.

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

# 4. Does B carry merge commits that the default branch does not?
#    git cherry compares patch-ids of NON-MERGE commits only, so a merge commit
#    carrying unique conflict resolutions produces zero `+` lines and looks merged.
git rev-list --merges --count "${REMOTE}/${DEFAULT_BRANCH}..$B"
```

**Categories:**

| Category | Test | Action |
|---|---|---|
| `MERGED` | `git merge-base --is-ancestor` succeeds | Auto-delete |
| `SQUASH_MERGED` | Not an ancestor, `git cherry` reports 0 `+` lines, **and** check 4 reports 0 unique merge commits | Auto-delete only after the PR head-SHA check below |
| `UNMERGED` | Carries commits absent from the default branch | Report, ask |
| `UNIQUE_MERGE` | `git cherry` is clean but check 4 found unique merge commits | Report, ask — never auto-delete |
| `GONE` | `%(upstream:track)` contains `[gone]` | Report, ask — the remote branch was deleted |
| `NO_UPSTREAM` | No tracking branch configured | Report, ask — work may exist only locally |

The `git cherry` check matters more than the ancestor check in practice: squash merge is the common default on GitHub, and squash-merged branches look unmerged to `git branch --merged`. Treating them as unmerged leaves the branch list permanently cluttered.

But `git cherry` alone is not sufficient evidence to delete. It compares patch-ids of non-merge commits only, so a branch whose unique content lives in a merge commit's conflict resolution reports zero `+` lines while still being the only place that resolution exists. Check 4 exists to catch exactly that: if it returns anything other than `0`, classify the branch `UNIQUE_MERGE` and route it to Step 4 for a decision, no matter how clean `git cherry` looks.

Confirm `SQUASH_MERGED` against the PR's head SHA before auto-deleting:

```bash
gh pr list --state merged --head "$B" --limit 1 \
  --json number,mergedAt,mergeCommit,headRefOid,baseRefName,title
```

Require **all** of the following before the branch is eligible for Step 2:

- A merged PR exists for `$B`.
- Its `baseRefName` equals `$DEFAULT_BRANCH`. A PR merged into a release branch says nothing about the default branch.
- Its `headRefOid` equals the local tip, `git rev-parse "$B"`. This is the check that matters: the local branch must be the exact commit that was merged.

```bash
LOCAL_TIP=$(git rev-parse "$B")
PR_HEAD=$(gh pr list --state merged --head "$B" --limit 1 --json headRefOid --jq '.[0].headRefOid // ""')
[ -n "$PR_HEAD" ] && [ "$LOCAL_TIP" = "$PR_HEAD" ] && echo "TIP_MATCHES" || echo "TIP_DIFFERS"
```

Do not substitute a timestamp comparison for the head-SHA check. "No commits newer than `mergedAt`" passes for a local commit that was authored *before* the merge and never pushed — the merged PR never contained it, and deleting the branch destroys the only copy. Commit dates also survive rebases and can be backdated outright, so they prove nothing about what was merged.

If `TIP_DIFFERS`, the local branch holds commits the PR did not. Report it under Step 3 and ask; never auto-delete it.

If `gh` is unavailable, no branch qualifies as `SQUASH_MERGED`. Report every non-ancestor branch for a decision and state in the report that squash-merge detection was unavailable. `git cherry` on its own cannot distinguish "already merged" from "unique merge resolution", so it must not be the sole basis for a force-delete.

---

## Step 2 — Auto-Delete Merged Local Branches

No confirmation needed for `MERGED` and confirmed `SQUASH_MERGED` branches. Nothing else is eligible — `UNIQUE_MERGE`, `UNMERGED`, `GONE`, and `NO_UPSTREAM` all go to Step 4.

```bash
# Use -d (not -D) for MERGED: git refuses if the branch is not actually merged,
# so git itself is the second opinion on this category.
git branch -d "$B"
```

`SQUASH_MERGED` is the only category that needs `-D`, because git cannot see patch equivalence. `-D` discards git's own safety check, so re-verify immediately before deleting rather than trusting a classification computed earlier in the run:

```bash
# Re-assert every condition at deletion time. Any failure means skip and report.
LOCAL_TIP=$(git rev-parse "$B")
PR_HEAD=$(gh pr list --state merged --head "$B" --limit 1 --json headRefOid --jq '.[0].headRefOid // ""')
PR_BASE=$(gh pr list --state merged --head "$B" --limit 1 --json baseRefName --jq '.[0].baseRefName // ""')
UNIQUE_MERGES=$(git rev-list --merges --count "${REMOTE}/${DEFAULT_BRANCH}..$B")
UNIQUE_PATCHES=$(git cherry "${REMOTE}/${DEFAULT_BRANCH}" "$B" | grep -c '^+' || true)

if [ "$LOCAL_TIP" = "$PR_HEAD" ] && [ "$PR_BASE" = "$DEFAULT_BRANCH" ] \
   && [ "$UNIQUE_MERGES" = "0" ] && [ "$UNIQUE_PATCHES" = "0" ]; then
  git branch -D "$B"
else
  echo "skip $B — re-verification failed; reporting instead"
fi
```

Record each deletion with its SHA so it stays recoverable. Capture the SHA **before** deleting, not after:

```bash
SHA=$(git rev-parse "$B")
git branch -d "$B" && echo "deleted $B at $SHA"
```

**Skip and report instead of deleting when:**

- The branch is the current branch — note that the user should switch to `$DEFAULT_BRANCH` first.
- The branch is checked out in another worktree — `git branch -d` fails; handle it in Step 5.
- `git branch -d` fails for any other reason — report the error verbatim, do not escalate to `-D`.
- Re-verification above fails for any reason, including `gh` becoming unavailable mid-run.

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
- **`UNIQUE_MERGE`** — `git cherry` is clean but the branch carries merge commits the default branch does not. Say plainly that the branch may hold conflict resolutions that exist nowhere else, and show `git log --merges --oneline "${REMOTE}/${DEFAULT_BRANCH}..$B"` so the user can judge.
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
- Worktrees whose branch classified as `MERGED`, or as `SQUASH_MERGED` confirmed by the Step 2 re-verification, provided the worktree is clean:

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
| feat-x | ancestor of main | a1b2c3d |
| fix-y  | squash-merged via PR #401, tip matches PR head | e4f5g6h |

Recover any of these with: git branch <name> <sha>

### Unmerged Branches — Action Needed
| Branch | Ahead | Age | PR | Checks | Flag |
|---|---|---|---|---|---|
| fix-login | 3 | 12d | #412 OPEN | passing | — |
| spike-parser | 8 | 94d | **NONE** | — | ⚠ no PR, stale |
| hotfix-cache | 1 | 40d | #388 CLOSED | — | ⚠ PR closed unmerged |
| old-merge | 2 | 60d | #377 MERGED | — | ⚠ unique merge commits, not auto-deleted |
| drifted | 1 | 20d | #390 MERGED | — | ⚠ local tip differs from merged PR head |

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
Squash-merge detection: available / unavailable (no gh) — when unavailable,
auto-deletion covered only strict ancestors of main.
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
- **`gh` not authenticated** — run every git-only step; report PR columns as unknown and tell the user to run `gh auth login` for the PR view. Auto-deletion is limited to strict ancestors of the default branch; say so in the report rather than silently widening or skipping it.
- **Fork workflow** — `gh pr list --head` may miss PRs opened from a fork onto upstream. Check with `gh pr list --head "$OWNER:$B"` before declaring a branch PR-less.
- **Branch names with slashes** — always quote branch variables; `feat/x` is a valid ref and a valid path fragment.
- **Shallow clone** — `git merge-base --is-ancestor` can be wrong. Detect with `git rev-parse --is-shallow-repository`; if true, run `git fetch --unshallow` or skip auto-deletion entirely and report everything for review.
- **Submodules** — removing a worktree does not clean its submodule checkouts; note any that remain.
- **Rate limiting** — if `gh` returns 429, report that PR data is incomplete and never let a missing PR lookup justify a deletion.
- **Branch is someone else's** — compare `%(authorname)` against `gh api /user`; default to leaving other people's branches alone even when merged.

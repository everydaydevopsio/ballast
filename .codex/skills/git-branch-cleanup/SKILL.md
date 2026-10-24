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

<!-- Created by [Ballast](https://github.com/everydaydevopsio/ballast) v5.21.3. Do not edit this section. -->

# Git Branch and Worktree Cleanup Skill

Removes branches and worktrees that are provably finished, then reports on everything that is not, so nothing in-flight is lost. Merged-and-empty refs are deleted without asking. Everything else is listed with its pull request status and requires an explicit decision.

The governing rule is **evidence before deletion**: a branch is only auto-deleted when git can prove its work is already in the default branch.

---

## Mode — Decide This First

This skill answers two different kinds of request, and only one of them is allowed to delete anything. Decide which before running a single command.

**Report mode** — the user asked a question about branches. Run Steps 0, 1, 3, and the report. Do not delete a branch, remove a worktree, prune, push a deletion, or run maintenance. Classification is read-only and produces the same table; it simply stops before acting on it.

Treat these as report mode: "what branches do I still have open", "which of my branches have no PR", "list unmerged branches", "show me my worktrees", "which branches are stale", and any other phrasing that asks to see state rather than change it.

**Cleanup mode** — the user asked to change something. Run every step.

Treat these as cleanup mode: "clean up my branches", "delete merged branches", "prune branches", "tidy up git", "remove stale branches", "clean up worktrees", and the combined form "delete the merged ones and list what's left".

When the request is ambiguous, run report mode and end by offering cleanup: showing the user what would be deleted costs nothing and is reversible, while deleting in response to a question is neither. A request that only *mentions* deletion as a hypothetical ("do I have branches I could delete?") is a question, not an instruction.

State the chosen mode in the first line of the report so the user can correct it.

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

# Refresh remote state. --prune deletes remote-tracking refs, so it is a
# cleanup-mode action only.
git fetch --all --tags            # both modes
git fetch --all --prune --tags    # cleanup mode only
```

In report mode, fetch without `--prune`. Pruning deletes remote-tracking refs, which is a state change, and report mode promised not to make any. The cost is that a branch deleted upstream may still show a live-looking upstream; note in the report that remote-tracking refs were not pruned and `[gone]` detection may therefore under-report.

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

For each local branch `B` other than `$DEFAULT_BRANCH`, gather the five facts below. Classification follows from them in order.

```bash
LOCAL_TIP=$(git rev-parse "$B")

# 1. Is every commit on B already reachable from the default branch?
git merge-base --is-ancestor "$B" "${REMOTE}/${DEFAULT_BRANCH}" && echo "ANCESTOR" || echo "NOT_ANCESTOR"

# 2. How many commits does B carry that the default branch does not?
git rev-list --count "${REMOTE}/${DEFAULT_BRANCH}..$B"

# 3. The merged PR whose head ref is B, if one exists.
gh pr list --state merged --head "$B" --limit 1 \
  --json number,headRefOid,baseRefName,mergeCommit,mergedAt,title

# 4. Do B's individual patches already exist in the default branch under
#    different SHAs? Detects rebase and cherry-pick, NOT squash (see below).
#    Capture output and exit status separately: see the warning below.
if CHERRY=$(git cherry "${REMOTE}/${DEFAULT_BRANCH}" "$B" 2>/dev/null); then
  UNIQUE_PATCHES=$(printf '%s\n' "$CHERRY" | grep -c '^+')
  CHERRY_OK=yes
else
  UNIQUE_PATCHES=unknown
  CHERRY_OK=no
fi

# 5. Does B carry merge commits the default branch does not?
#    git cherry compares patch-ids of NON-MERGE commits only, so a merge commit
#    carrying unique conflict resolutions produces zero `+` lines and looks merged.
git rev-list --merges --count "${REMOTE}/${DEFAULT_BRANCH}..$B"
```

**Categories**, evaluated top to bottom — the first match wins:

| Category | Test | Action |
|---|---|---|
| `MERGED` | Check 1 succeeds | Auto-delete |
| `SQUASH_MERGED` | Check 3 found a merged PR whose `headRefOid` equals `$LOCAL_TIP`, whose `baseRefName` equals `$DEFAULT_BRANCH`, and whose `mergeCommit` is reachable from `${REMOTE}/${DEFAULT_BRANCH}` | Auto-delete |
| `UNIQUE_MERGE` | `CHERRY_OK=yes`, check 4 is `0`, but check 5 is greater than `0` | Report, ask — never auto-delete |
| `REBASE_MERGED` | `CHERRY_OK=yes`, check 4 is `0`, and check 5 is `0` | Auto-delete |
| `UNMERGED` | Anything else, including `CHERRY_OK=no` | Report, ask |
| `GONE` | `%(upstream:track)` contains `[gone]` | Report, ask — the remote branch was deleted |
| `NO_UPSTREAM` | No tracking branch configured | Report, ask — work may exist only locally |

### Why the PR check comes before `git cherry`

`git cherry` compares patch-ids of individual commits. A squash merge collapses N commits into one, and the combined patch does not match any individual patch when N is greater than 1. So `git cherry` reports every commit as unmerged for exactly the case this skill exists to handle — the ordinary multi-commit squash-merged branch. Relying on it would classify those branches `UNMERGED` forever and defeat the whole point.

The merged PR is the authority instead. GitHub records which branch head it squashed and where the result landed, which is the fact that actually matters:

```bash
PR=$(gh pr list --state merged --head "$B" --limit 1 \
  --json headRefOid,baseRefName,mergeCommit \
  --jq '.[0] | [.headRefOid, .baseRefName, (.mergeCommit.oid // "")] | @tsv')
PR_HEAD=$(printf '%s' "$PR" | cut -f1)
PR_BASE=$(printf '%s' "$PR" | cut -f2)
PR_MERGE=$(printf '%s' "$PR" | cut -f3)

[ -n "$PR_HEAD" ] \
  && [ "$PR_HEAD" = "$LOCAL_TIP" ] \
  && [ "$PR_BASE" = "$DEFAULT_BRANCH" ] \
  && [ -n "$PR_MERGE" ] \
  && git merge-base --is-ancestor "$PR_MERGE" "${REMOTE}/${DEFAULT_BRANCH}" \
  && echo "SQUASH_CONFIRMED" || echo "NOT_CONFIRMED"
```

All four conditions are required:

- **A merged PR exists** for `$B`.
- **`headRefOid` equals the local tip.** The local branch must be the exact commit that was merged. Without this, a local commit authored before the merge and never pushed still passes — the merged PR never contained it, and deleting the branch destroys the only copy.
- **`baseRefName` equals `$DEFAULT_BRANCH`.** A PR merged into a release branch says nothing about the default branch.
- **`mergeCommit` is reachable from the default branch.** A merged PR whose merge commit was later reverted, force-pushed away, or merged into a branch that never landed is not evidence that the work is in the default branch today.

Do not substitute a timestamp comparison for the head-SHA check. "No commits newer than `mergedAt`" passes for a commit authored before the merge, and commit dates survive rebases and can be backdated outright, so they prove nothing about what was merged.

### What `git cherry` is still for

It remains the right test for rebase and cherry-pick, where individual patches are preserved — a branch rebased onto the default branch and landed commit-by-commit has every patch-id present, and no PR may exist at all. That is the `REBASE_MERGED` path.

But `git cherry` alone is never sufficient evidence. It ignores merge commits entirely, so a branch whose unique content lives in a merge commit's conflict resolution reports zero `+` lines while being the only place that resolution exists. Check 5 catches exactly that: anything other than `0` means `UNIQUE_MERGE`, routed to Step 4 for a decision no matter how clean `git cherry` looks.

**Never write `git cherry ... | grep -c '^+' || true`.** The `|| true` is tempting because `grep -c` exits non-zero when the count is zero, but it also swallows a failure of `git cherry` itself. If patch generation fails — a partial clone that cannot fetch a required blob, a corrupt object, an interrupted fetch — the pipeline emits `0`, and `0` is the value that authorizes a force-delete. Commit traversal can succeed while patch generation fails, so check 5 will not catch it either: the branch looks perfectly merged and gets deleted.

Capture the output and test the exit status separately, as in check 4 above. When `CHERRY_OK=no`, the comparison produced no answer, which is not the same as "no unique patches" — classify `UNMERGED`, report the failure, and let the user decide. Apply the same discipline in Step 2's re-verification.

### When `gh` is unavailable

`SQUASH_MERGED` cannot be established without the PR, so no branch qualifies. `REBASE_MERGED` still works — it is pure git. Report every other non-ancestor branch for a decision, and state in the report that squash-merge detection was unavailable so the user knows the pass was narrower than usual rather than finding nothing to do.

---

## Step 2 — Auto-Delete Merged Local Branches

Only `MERGED`, `SQUASH_MERGED`, and `REBASE_MERGED` are eligible. `UNIQUE_MERGE`, `UNMERGED`, `GONE`, and `NO_UPSTREAM` all go to Step 4.

Skip this step entirely in report mode.

`MERGED` is the easy case — git itself is the second opinion:

```bash
# -d (not -D): git refuses if the branch is not actually merged.
SHA=$(git rev-parse "$B")
git branch -d "$B" && echo "deleted $B at $SHA"
```

`SQUASH_MERGED` and `REBASE_MERGED` need `-D`, because git cannot see patch equivalence. `-D` discards git's own safety check, so re-assert the evidence immediately before deleting rather than trusting a classification computed earlier in the run — the repository may have changed, and `gh` may have stopped responding:

```bash
SHA=$(git rev-parse "$B")

case "$CATEGORY" in
  SQUASH_MERGED)
    PR=$(gh pr list --state merged --head "$B" --limit 1 \
      --json headRefOid,baseRefName,mergeCommit \
      --jq '.[0] | [.headRefOid, .baseRefName, (.mergeCommit.oid // "")] | @tsv')
    PR_HEAD=$(printf '%s' "$PR" | cut -f1)
    PR_BASE=$(printf '%s' "$PR" | cut -f2)
    PR_MERGE=$(printf '%s' "$PR" | cut -f3)
    [ -n "$PR_HEAD" ] && [ "$PR_HEAD" = "$SHA" ] && [ "$PR_BASE" = "$DEFAULT_BRANCH" ] \
      && [ -n "$PR_MERGE" ] \
      && git merge-base --is-ancestor "$PR_MERGE" "${REMOTE}/${DEFAULT_BRANCH}" \
      && OK=yes || OK=no
    ;;
  REBASE_MERGED)
    # Require git cherry to SUCCEED, not merely to print nothing.
    if CHERRY=$(git cherry "${REMOTE}/${DEFAULT_BRANCH}" "$B" 2>/dev/null) \
       && [ "$(printf '%s\n' "$CHERRY" | grep -c '^+')" = "0" ] \
       && [ "$(git rev-list --merges --count "${REMOTE}/${DEFAULT_BRANCH}..$B")" = "0" ]; then
      OK=yes
    else
      OK=no
    fi
    ;;
  *) OK=no ;;
esac

if [ "$OK" = "yes" ]; then
  # Delete the exact commit that was verified. `git branch -D` deletes
  # whatever $B points to now, which may have moved while the gh request
  # above was in flight; update-ref fails instead of deleting the new tip.
  if git update-ref -d "refs/heads/$B" "$SHA"; then
    echo "deleted $B at $SHA"
    # update-ref leaves branch.<name>.* behind; git branch -D would not.
    git config --remove-section "branch.$B" 2>/dev/null || true
  fi
else
  echo "skip $B — re-verification failed; reporting instead"
fi
```

Use `git update-ref -d <ref> <expected-sha>` rather than `git branch -D` on both force-delete paths. The verification above establishes a fact about one specific commit, and the branch can advance between that check and the delete — another process, another agent, or a parallel command. `git branch -D` would delete the new tip and then report the old SHA as the recovery point, so the recorded SHA would not restore what was lost. `update-ref` refuses with `cannot lock ref ... is at X but expected Y`, which is the correct outcome: report it and move on.

`git branch -d` needs no such treatment — it independently re-checks merge status and refuses on its own.

Capture the recovery SHA **before** deleting, as above. Reading it afterwards fails, and a deletion with no recorded SHA is the one thing here that is genuinely hard to undo.

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

Skip this step in report mode — the report already lists these branches, and report mode does not act on them.

Group the branches needing a decision and ask in a single batch, not one prompt per branch:

```text
These branches are not merged. Delete any?

  [1] fix-login-redirect      3 commits ahead, 12 days old, PR #412 OPEN (checks passing)
  [2] spike-new-parser        8 commits ahead, 94 days old, NO PR  ⚠
  [3] hotfix-cache            1 commit ahead,  40 days old, PR #388 CLOSED (not merged)
  [4] old-feature             2 commits ahead,  6 days old, upstream [gone]

Reply with the numbers to delete, "none", or "all".
```

Apply the answer the same way Step 2 does — against the SHA shown in the prompt, not against wherever the branch points when the user finally answers:

```bash
SHA=$(git rev-parse "$B")        # the SHA presented in the list above
git update-ref -d "refs/heads/$B" "$SHA" && echo "deleted $B at $SHA"
git config --remove-section "branch.$B" 2>/dev/null || true
```

Report each deleted branch's SHA so it can be restored with `git branch <name> <sha>`.

`git update-ref` deletes the ref but leaves the branch's `branch.<name>.*` config behind, which `git branch -D` would have cleaned up — hence the `git config --remove-section` line. Run it only after the deletion succeeds, so a refused delete does not strip the upstream of a branch that still exists.

Default to keeping a branch when the answer is ambiguous or absent.

---

## Step 5 — Clean Up Worktrees

Skip this step entirely in report mode: list the worktrees in the report and stop.

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
- Worktrees whose branch classified as `MERGED`, or as `SQUASH_MERGED` / `REBASE_MERGED` confirmed by the Step 2 re-verification, **and** that hold nothing but tracked, committed files:

  ```bash
  # --untracked-files=all --ignored is the point: plain --porcelain hides
  # ignored files, and removing the worktree deletes them with the directory.
  git -C "$WORKTREE_PATH" status --porcelain --untracked-files=all --ignored
  # must print nothing before the next line runs
  git worktree remove "$WORKTREE_PATH"         # never use --force here
  ```

  Removing the worktree frees the branch; delete the branch afterward in Step 2's pass.

  A bare `git status --porcelain` is not a safe gate here. It excludes ignored files, so a worktree holding a `.env`, a local database, generated certificates, or an uncommitted scratch directory reports clean — and `git worktree remove` deletes the directory and everything in it. The branch SHA recorded in Step 2 restores commits; it restores none of that. Ignored files are the one thing in this skill with no recovery path at all.

**Always ask before:**

- Removing a worktree with uncommitted or untracked changes. Show `git -C "$WORKTREE_PATH" status --short` so the user sees what is at risk, and never pass `--force` without an explicit yes.
- Removing a worktree that holds ignored files, even when the branch is merged and the tracked tree is clean. List what is there so the decision is quick:

  ```bash
  # Default untracked handling here on purpose: it collapses node_modules/ to
  # one line instead of listing every file inside it.
  git -C "$WORKTREE_PATH" status --porcelain --ignored | awk '$1 == "!!" {print $2}'
  du -sh "$WORKTREE_PATH"
  ```

  Often it is only `node_modules/` or a build directory and the user will say yes immediately. Sometimes it is the only copy of a working `.env`. Showing the list is what lets them tell the difference, so show it rather than deciding on their behalf.
- Removing a worktree on an unmerged branch.
- Removing a locked worktree — report the lock reason from `git worktree list --porcelain` and leave it alone unless the user unlocks it:

  ```bash
  git worktree unlock "$WORKTREE_PATH"
  ```

---

## Step 6 — Clean Up Remote Branches

Skip this step entirely in report mode.

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

- Record each candidate's remote tip **at the moment you present the list**, and include it in what you show:

  ```bash
  EXPECTED=$(git ls-remote --heads "$REMOTE" "$BRANCH" | cut -f1)
  ```

- Ask once with the full list, then delete the approved set with a lease on that recorded tip:

  ```bash
  git push --force-with-lease="refs/heads/$BRANCH:$EXPECTED" \
    "$REMOTE" ":refs/heads/$BRANCH"
  ```

  A plain `git push --delete` deletes whatever the branch points to now, not what the user approved. Approval is not instant — the user reads the list, thinks, and answers — and a collaborator can push to one of those branches in that window. The lease makes git refuse with `stale info` instead, leaving the branch intact.

  Treat a lease failure as new information, not an error to retry past: re-read the branch, show what arrived, and ask again. Never fall back to `--delete` to force it through.

- If a remote branch was created by someone else, say so (`gh pr list --head "$BRANCH" --json author`) and default to leaving it.

---

## Step 7 — Repository Maintenance

Skip this step entirely in report mode.

Deleting refs leaves garbage behind. Finish with the cheap, safe maintenance pass:

```bash
# Repack using git's normal grace period. No --prune=now.
git gc --quiet

# Report repository size before and after when the repo is large
git count-objects -vH
```

Never use `git gc --prune=now` here. "This session deleted nothing" does not make immediate pruning safe: it still destroys unreachable objects left by *earlier* deletions, which are exactly the objects a user recovers from after a mistake last week, and it can race a concurrent git process that has written objects but not yet updated the ref pointing at them. Plain `git gc` keeps the default two-week grace period and reclaims the same space a fortnight later.

Reserve `--prune=now` for maintenance the user explicitly asks for, on a repository with no concurrent writers, and say plainly that it makes earlier deletions unrecoverable.

Skip `git gc` entirely when branches were deleted in this run and the user may still want them back. Say which choice you made.

---

## Generate Cleanup Report

```text
## Git Branch & Worktree Cleanup
**Mode**: report (nothing was changed) | cleanup
**Repo**: owner/name
**Default branch**: main
**Date**: <today>

### Deleted Automatically
| Branch | Reason | SHA (for recovery) |
|---|---|---|
| feat-x | ancestor of main | a1b2c3d |
| fix-y  | squash-merged via PR #401 (tip = PR head, merge commit in main) | e4f5g6h |
| chore-z | rebase-merged, all patches present in main | b7c8d9e |

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
- **No automatic branch pruning** — `git config --global fetch.prune true` keeps remote-tracking branches honest on every fetch.

  Do not recommend `fetch.pruneTags` alongside it. With both set, every fetch deletes local tags the fetched remote does not have, across every repository the global config covers: local-only release tags, tags that came from a second remote, and tags created before a remote was re-pointed all disappear, and an unannotated tag that is not an ancestor of any branch is gone for good. Tag pruning is a deliberate, repository-specific cleanup, not a hygiene default — if the user wants it, set it per repository with `git config fetch.pruneTags true` after confirming every local tag also exists on that remote.
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
- **Partial or shallow clone** — patch generation can fail while commit traversal succeeds, so `git cherry` may error out on a branch whose history looks complete. Treat any `git cherry` failure as `UNMERGED` and say so; never let a failed comparison read as a clean one.
- **Shallow clone** — `git merge-base --is-ancestor` can be wrong. Detect with `git rev-parse --is-shallow-repository`; if true, run `git fetch --unshallow` or skip auto-deletion entirely and report everything for review.
- **Submodules** — removing a worktree does not clean its submodule checkouts; note any that remain.
- **Rate limiting** — if `gh` returns 429, report that PR data is incomplete and never let a missing PR lookup justify a deletion.
- **Branch is someone else's** — compare `%(authorname)` against `gh api /user`; default to leaving other people's branches alone even when merged.

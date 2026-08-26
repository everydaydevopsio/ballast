---
name: github-pr-copilot-cycle
description: >
  Manage a GitHub pull request feedback loop with Copilot review. Use when
  asked to create or update a PR, request Copilot as reviewer, collect Copilot
  review comments, score whether comments need human input, fix actionable
  feedback, reply or resolve comments, push fixes, check CI, and repeat the
  Copilot review cycle until no unresolved Copilot comments remain or three
  cycles have completed.
---

<!-- Created by [Ballast](https://github.com/everydaydevopsio/ballast) v5.18.0. Do not edit this section. -->

# GitHub PR Copilot Cycle Skill

Drive a PR from local branch to a bounded Copilot review loop. Use `gh` for GitHub operations and keep the PR branch as the single source of truth.

## Preconditions

Run from the repository root:

```bash
gh auth status
gh repo view --json nameWithOwner,defaultBranchRef
git status --short
git branch --show-current
```

If the worktree has unrelated user changes, preserve them. Do not rewrite history, force-push, or delete branches unless the user explicitly asked for that.

## Create Or Update The PR

1. Confirm the branch is not the default branch.
2. Run the repo's smallest relevant tests before opening or updating the PR.
3. Push the branch:

```bash
git push -u origin HEAD
```

4. Create the PR with Copilot requested as a reviewer:

```bash
gh pr create --fill --reviewer "@copilot"
```

For an existing PR, request or re-request Copilot review by PR number:

```bash
PR_NUMBER=$(gh pr view --json number --jq .number)
REQUESTED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
HEAD_OID=$(gh pr view --json headRefOid --jq .headRefOid)
gh pr edit "$PR_NUMBER" --add-reviewer "@copilot"
```

Use `@copilot` only with `--reviewer` or `--add-reviewer`. Do not use `--add-assignee`, `copilot-pull-request-reviewer[bot]`, `github-copilot[bot]`, or the requested-reviewers API as a substitute for requesting Copilot code review.

## Cycle Limit

Run at most three Copilot review cycles. A cycle is:

1. Record `PR_NUMBER`, `REQUESTED_AT`, and `HEAD_OID`, then request or re-request Copilot review.
2. Wait until Copilot review activity for the current head commit has settled.
3. Score unresolved Copilot comments.
4. Fix, reply, resolve, test, push.
5. Check PR CI.

Stop before three cycles only when all of these are true:

- There are no unresolved Copilot review threads.
- `gh pr view "$PR_NUMBER" --json reviewRequests` shows Copilot is not present in pending review requests.
- `gh pr view "$PR_NUMBER" --json latestReviews,reviews` shows a Copilot review submitted after `$REQUESTED_AT` and attached to `$HEAD_OID` when the API includes the review commit, or Copilot produced unresolved threads from that request and they have been handled.
- A final review-thread query after that settled review still shows zero unresolved Copilot threads.

Do not treat a single immediate "no unresolved threads" poll after requesting Copilot as complete. Copilot can accept the request, clear the review request, and publish comments later. If the review request disappears but no new Copilot review is visible yet, keep polling with backoff until a Copilot review appears, unresolved Copilot threads appear, or a reasonable timeout is reached. If the timeout is reached, report the PR as blocked/pending Copilot rather than complete.

Stop immediately and ask the user when any unresolved comment needs human input.

## Wait For Copilot To Settle

Before requesting or re-requesting Copilot, set the PR number and record the current head commit and request time:

```bash
PR_NUMBER=$(gh pr view --json number --jq .number)
REQUESTED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
HEAD_OID=$(gh pr view --json headRefOid --jq .headRefOid)
gh pr edit "$PR_NUMBER" --add-reviewer "@copilot"
```

Poll both review request state and review history:

```bash
gh pr view "$PR_NUMBER" --json headRefOid,reviewRequests,reviews,latestReviews
```

Use `reviewRequests` to detect a pending Copilot review request. Use `reviews` or `latestReviews` to find the newest Copilot review, preferring known Copilot logins: `copilot-pull-request-reviewer`, `copilot-pull-request-reviewer[bot]`, or `github-copilot[bot]`. If GitHub changes the reviewer login, accept another author login containing `copilot` only when the review body or metadata identifies it as GitHub Copilot code review. A settled Copilot review for the current cycle is one submitted after `$REQUESTED_AT` and preferably attached to `$HEAD_OID`. Some GitHub API responses omit the review commit OID; in that case, accept the timestamp plus a fresh review-thread query as evidence.

If `headRefOid` differs from `$HEAD_OID` during polling, the PR changed while waiting. Stop the current wait, record a new `REQUESTED_AT` and `HEAD_OID` for the new head, re-request Copilot, and restart settle polling for that head.

After each poll, gather review threads again. If new unresolved Copilot threads appear, stop polling and score them. Otherwise, continue polling while any of these are true:

- Copilot is still listed in `reviewRequests`.
- No Copilot review newer than `$REQUESTED_AT` is visible yet.

Also inspect the newest Copilot review body after each poll. If the review body says Copilot generated comments, includes a `Suppressed comments` section, or contains file-and-line feedback that is not represented as an unresolved review thread, treat those body comments as Copilot feedback for the current cycle. Score and handle them the same way as review-thread comments. If a body comment has no thread id, make the code/doc change, then reply on the PR with a short audit note that quotes the file/line summary and validation command; do not try to resolve a non-thread comment.

Recommended polling cadence: wait 30 seconds, then 60 seconds, then 120 seconds between checks; repeat the 120-second interval until Copilot settles or the timeout is reached. Do not wait forever. If Copilot has not settled after about 10 minutes, report the PR URL, the pending state, and the last observed review request/review timestamps.

## Gather Copilot Comments

Get the current PR number and review threads:

```bash
OWNER_REPO=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
OWNER=${OWNER_REPO%/*}
REPO=${OWNER_REPO#*/}
PR_NUMBER=$(gh pr view --json number --jq .number)
gh api graphql --paginate -f owner="$OWNER" -f repo="$REPO" -F number="$PR_NUMBER" -f query='
query($owner:String!, $repo:String!, $number:Int!, $endCursor:String) {
  repository(owner:$owner, name:$repo) {
    pullRequest(number:$number) {
      reviewThreads(first:100, after:$endCursor) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id
          isResolved
          path
          line
          comments(first:100) {
            pageInfo { hasNextPage endCursor }
            nodes {
              id
              author { login }
              bodyText
              url
              createdAt
            }
          }
        }
      }
    }
  }
}'
```

If any returned thread has `comments.pageInfo.hasNextPage: true`, fetch the remaining comments for that thread before scoring it:

```bash
gh api graphql --paginate -f threadId="THREAD_ID" -f query='
query($threadId:ID!, $endCursor:String) {
  node(id:$threadId) {
    ... on PullRequestReviewThread {
      comments(first:100, after:$endCursor) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id
          author { login }
          bodyText
          url
          createdAt
        }
      }
    }
  }
}'
```

Treat comments from `copilot-pull-request-reviewer`, `copilot-pull-request-reviewer[bot]`, or `github-copilot[bot]` as Copilot comments. If GitHub changes the comment author login, accept another author login containing `copilot` only when the surrounding review metadata identifies it as GitHub Copilot code review. Only act on unresolved threads unless the user explicitly asks to revisit resolved history.

For review-body feedback, inspect Copilot-authored reviews from the current cycle in addition to review threads. Treat bullets under `Suppressed comments`, `Previously missed`, or any Copilot review body that says it generated comments as actionable unless the text is clearly obsolete after later commits. Preserve the file path and line number from the review body in your notes and PR reply because these comments may not have a resolvable thread id.

Copilot does not read replies added to its review comments. Replies are for human auditability, not for continuing a conversation with Copilot.

## Score Comments

Score each unresolved Copilot thread before changing code:

- `0 - no action`: demonstrably incorrect, obsolete after later commits, duplicate of another comment, or purely optional style that conflicts with repo policy.
- `1 - direct fix`: localized and clearly correct, with low regression risk.
- `2 - fix with validation`: likely correct but affects behavior, public output, persistence, concurrency, security controls, tests, CI, build, or generated artifacts.
- `3 - human input required`: ambiguous product intent, API contract, migration, rollout, data deletion, auth/permissions, billing, legal/compliance, security tradeoff, secret handling, or any fix that requires choosing between materially different designs.

Reply on every Copilot thread before resolving or stopping, including ignored comments. For every `0`, reply with the reason it is not being changed. For every `1` or `2`, implement the fix and reply with what changed plus the validation command. For every `3`, reply that human input is required, include the specific choice needed, then stop the cycle and ask the user for a decision with the thread URL.

## Fix And Verify

For score `1` and `2` comments:

1. Read the referenced files and surrounding code.
2. Make the smallest coherent fix.
3. Add or update tests when behavior changes or regression risk is meaningful.
4. Run targeted validation for score `1`; run broader relevant validation for score `2`.
5. Keep notes mapping each thread id to the fix and validation result.

Before pushing, check:

```bash
git status --short
git diff --check
```

Push fixes:

```bash
git push
```

## Reply And Resolve Threads

Reply to every Copilot thread, even when no code change is made:

```bash
gh api graphql -f threadId=THREAD_ID -f body='Handled in the latest push. Validation: COMMAND.' -f query='
mutation($threadId:ID!, $body:String!) {
  addPullRequestReviewThreadReply(input:{pullRequestReviewThreadId:$threadId, body:$body}) {
    comment { url }
  }
}'
```

Resolve a thread only after the code fix is pushed or after a `0` reply explains why no change is needed. Do not resolve score `3` threads while waiting for human input.

```bash
gh api graphql -f threadId=THREAD_ID -f query='
mutation($threadId:ID!) {
  resolveReviewThread(input:{threadId:$threadId}) {
    thread { id isResolved }
  }
}'
```

For Copilot review-body feedback that has no review thread, reply on the PR instead of a thread:

```bash
gh pr comment "$PR_NUMBER" --body 'Handled Copilot review-body feedback for PATH:LINE. Validation: COMMAND.'
```

## Check CI After Each Push

After every push:

```bash
gh pr checks --watch
```

If checks fail, inspect the failing logs, fix the root cause, run relevant validation locally, push, and re-check before requesting another Copilot review.

## Re-Request Copilot

After comments are handled and CI is green or pending with no known failures, re-request Copilot unless the cycle limit is reached:

```bash
PR_NUMBER=$(gh pr view --json number --jq .number)
REQUESTED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
HEAD_OID=$(gh pr view --json headRefOid --jq .headRefOid)
gh pr edit "$PR_NUMBER" --add-reviewer "@copilot"
```

Wait for Copilot to settle using the polling procedure above, then gather comments again. If no unresolved Copilot threads remain after the settled review, report the PR URL, cycle count, validation commands, CI state, current head commit, latest Copilot review timestamp, and whether any Copilot review request remains pending.

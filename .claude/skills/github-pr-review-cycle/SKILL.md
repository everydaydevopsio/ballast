---
name: github-pr-review-cycle
description: >
  Manage a GitHub pull request feedback loop with an automated code review
  agent. Use when asked to create or update a PR, request an AI code review,
  collect review comments, score whether comments need human input, fix
  actionable feedback, reply or resolve comments, push fixes, check CI, and
  repeat the review cycle until no unresolved comments remain or three cycles
  have completed. Works with GitHub Copilot, Bosun, or a reviewer the
  repository defines as its own skill.
---

<!-- Created by [Ballast](https://github.com/everydaydevopsio/ballast) v5.21.3. Do not edit this section. -->

# GitHub PR Review Cycle Skill

Drive a PR from local branch to a bounded review loop with whichever code review agent the repository has configured. Use `gh` for GitHub operations and keep the PR branch as the single source of truth.

The cycle, the scoring rubric, and the stopping rules are identical for every reviewer. Only three things vary: how a review is requested, how you know it has settled, and how findings are read back. Those three are isolated in the reviewer sections below.

## Resolve The Reviewer First

Before anything else, decide which reviewer this run uses. Precedence:

1. An explicit argument to this skill (`copilot`, `bosun`, or a skill name).
2. The `codeReviewer` key in `.rulesrc.json` at the repository root.
3. `copilot`, the default.

Read `.rulesrc.json` yourself rather than shelling out to a parser. Resolution has three
distinct outcomes, and **only one of them defaults**:

| Situation | Resolution |
| --- | --- |
| No `.rulesrc.json`, or the file has no `codeReviewer` key | `copilot` |
| `codeReviewer` is present and valid | that value |
| The file exists but cannot be read or parsed, or `codeReviewer` holds an unrecognized value | **stop and report — do not default** |

That third row is the one that matters. Defaulting on a read failure turns "this repository
must not use Copilot" into a Copilot review request, which is the exact outcome the setting
exists to prevent. A repository that cannot be read is not a repository that chose Copilot.
Say what failed and ask, rather than guessing.

For the same reason, do not pipe the lookup through `2>/dev/null || echo copilot`, and do not
make the resolution depend on a tool that may be absent — a missing `python3` or `jq` is a
read failure, not a vote for Copilot.

Valid values are `copilot`, `bosun`, `none`, and `skill:<skill-name>` where `<skill-name>` is
lowercase alphanumeric with hyphens. Anything else is invalid: stop, name the value you found,
and ask.

If `codeReviewer` is `none`, do not run a review cycle. Say that automated review is disabled for this repository, finish any PR work that was asked for, and stop.

If the value starts with `skill:`, follow **Custom Reviewer** below.

State the resolved reviewer in your first message so the user can correct it before any model credits are spent.

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

4. Create the PR:

```bash
gh pr create --fill
```

Then request the review using the reviewer-specific step below.

## Cycle Limit

Run at most three review cycles. A cycle is:

1. Record `PR_NUMBER`, `REQUESTED_AT`, and `HEAD_OID`, then request or re-request review.
2. Wait until review activity for the current head commit has settled.
3. Score unresolved comments.
4. Fix, reply, resolve, test, push.
5. Check PR CI.

**Record `REQUESTED_AT` before anything that can trigger a review, not after.** With a
reviewer that starts automatically on push — Bosun does — the push in step 4 *is* the next
cycle's request. Stamping `REQUESTED_AT` at step 1 of the following cycle, after CI has been
waited on in step 5, can place the timestamp *after* a review that already finished. That
review is then discarded as stale, no new one is requested because the push already
triggered it, and the cycle waits until timeout for a review that has already been
published. So for auto-triggering reviewers, re-stamp `REQUESTED_AT` and `HEAD_OID`
immediately before the push, and treat a commit match as authoritative over the timestamp.

Stop before three cycles only when a settled review for the current head produced no unresolved findings, confirmed by a fresh query after that review settled.

Do not treat a single immediate "nothing found" poll as completion. Every reviewer here can accept a request and publish its output later. If a timeout is reached without a settled review, report the PR as blocked or pending rather than complete.

Stop immediately and ask the user when any unresolved comment needs human input.

```bash
PR_NUMBER=$(gh pr view --json number --jq .number)
REQUESTED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
HEAD_OID=$(gh pr view --json headRefOid --jq .headRefOid)
```

A review that identifies the commit it covers is settled evidence for that commit whatever
its timestamp says. Use `REQUESTED_AT` only to disambiguate reviews that carry no commit,
never to reject one whose commit matches `$HEAD_OID`.

If `headRefOid` changes while you are waiting, the PR moved. Stop the current wait, record a new `REQUESTED_AT` and `HEAD_OID`, re-request, and restart the wait for that head.

## Score Comments

Score each unresolved finding before changing code. This rubric is reviewer-independent.

- `0 - no action`: demonstrably incorrect, obsolete after later commits, duplicate of another comment, or purely optional style that conflicts with repo policy.
- `1 - direct fix`: localized and clearly correct, with low regression risk.
- `2 - fix with validation`: likely correct but affects behavior, public output, persistence, concurrency, security controls, tests, CI, build, or generated artifacts.
- `3 - human input required`: ambiguous product intent, API contract, migration, rollout, data deletion, auth/permissions, billing, legal/compliance, security tradeoff, secret handling, or any fix that requires choosing between materially different designs.

Reply to every finding before resolving or stopping, including ignored ones. For every `0`, reply with the reason it is not being changed. For every `1` or `2`, implement the fix and reply with what changed plus the validation command. For every `3`, reply that human input is required, include the specific choice needed, then stop the cycle and ask the user for a decision with the finding's URL.

Reviewers do not read replies. Replies are for human auditability, not for continuing a conversation with the reviewer.

## Fix And Verify

For score `1` and `2` findings:

1. Read the referenced files and surrounding code.
2. Make the smallest coherent fix.
3. Add or update tests when behavior changes or regression risk is meaningful.
4. Run targeted validation for score `1`; run broader relevant validation for score `2`.
5. Keep notes mapping each finding to the fix and validation result. When the reviewer provides no resolvable thread, these notes are the only record that a finding was handled — write them down.

Before pushing:

```bash
git status --short
git diff --check
```

Re-stamp the cycle markers immediately before the push, because for an auto-triggering
reviewer the push is the request:

```bash
REQUESTED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
git push
HEAD_OID=$(git rev-parse HEAD)
```

## Check CI After Each Push

```bash
gh pr checks --watch
```

If checks fail, inspect the failing logs, fix the root cause, run relevant validation locally, push, and re-check before requesting another review.

## Reviewer: Copilot

**Request.** Use `@copilot` only with `--reviewer` or `--add-reviewer`. Do not use `--add-assignee`, `copilot-pull-request-reviewer[bot]`, `github-copilot[bot]`, or the requested-reviewers API as a substitute.

```bash
gh pr edit "$PR_NUMBER" --add-reviewer "@copilot"
```

On PR creation, `gh pr create --fill --reviewer "@copilot"` does both at once.

**Settle.** Poll both review request state and review history:

```bash
gh pr view "$PR_NUMBER" --json headRefOid,reviewRequests,reviews,latestReviews
```

Use `reviewRequests` to detect a pending Copilot request. Use `reviews` or `latestReviews` to find the newest Copilot review, preferring the logins `copilot-pull-request-reviewer`, `copilot-pull-request-reviewer[bot]`, or `github-copilot[bot]`. Accept another login containing `copilot` only when the review body or metadata identifies it as GitHub Copilot code review. A settled review is one submitted after `$REQUESTED_AT` and preferably attached to `$HEAD_OID`; some responses omit the review commit, in which case the timestamp plus a fresh thread query is sufficient evidence.

Keep polling while Copilot is still listed in `reviewRequests`, or while no review newer than `$REQUESTED_AT` is visible. Cadence: 30s, then 60s, then 120s repeating. Timeout at about 10 minutes and report the pending state.

**Read findings.** Query review threads:

```bash
OWNER_REPO=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
gh api graphql --paginate -f owner="${OWNER_REPO%/*}" -f repo="${OWNER_REPO#*/}" -F number="$PR_NUMBER" -f query='
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
            nodes { id author { login } bodyText url createdAt }
          }
        }
      }
    }
  }
}'
```

If a thread reports `comments.pageInfo.hasNextPage`, page that thread's comments before scoring it. Only act on unresolved threads unless the user asks to revisit resolved history.

Also inspect the newest Copilot review body. If it says Copilot generated comments, includes a `Suppressed comments` section, or carries file-and-line feedback with no matching unresolved thread, treat those as findings for this cycle. Preserve `path:line` in your notes, because such comments have no thread id to resolve.

**Reply and resolve.**

```bash
gh api graphql -f threadId=THREAD_ID -f body='Handled in the latest push. Validation: COMMAND.' -f query='
mutation($threadId:ID!, $body:String!) {
  addPullRequestReviewThreadReply(input:{pullRequestReviewThreadId:$threadId, body:$body}) { comment { url } }
}'

gh api graphql -f threadId=THREAD_ID -f query='
mutation($threadId:ID!) { resolveReviewThread(input:{threadId:$threadId}) { thread { id isResolved } } }'
```

Resolve only after the fix is pushed, or after a `0` reply explains why no change is needed. Never resolve a score `3` thread while waiting for human input. For review-body feedback with no thread, reply on the PR instead:

```bash
gh pr comment "$PR_NUMBER" --body 'Handled review-body feedback for PATH:LINE. Validation: COMMAND.'
```

## Reviewer: Bosun

Bosun is a self-hosted reviewer that authenticates as a GitHub App and runs the review in a Kubernetes Job. It differs from Copilot in four ways that matter here.

**Request.** Bosun has no reviewer-request API. It reviews automatically on `opened`, `reopened`, `synchronize`, and `ready_for_review`, so **every push already triggers the next review** — do not request one after pushing fixes, just wait. Request explicitly only when no automatic review is coming, for example on a draft PR or a PR whose head predates the App install:

```bash
gh pr comment "$PR_NUMBER" --body '@bridgectl review'
```

The trigger phrase is configurable in the Bosun deployment (`BOSUN_REVIEW_COMMAND`); `@bridgectl review` is the default.

Two constraints on the trigger, both enforced by Bosun's webhook:

- The comment author must be `OWNER`, `MEMBER`, or `COLLABORATOR` on the repository.
- Comments whose sender is a bot are ignored, so Bosun cannot trigger itself. A local `gh` session qualifies; a GitHub Actions `GITHUB_TOKEN` does not and will be silently dropped.

**Settle.** Bosun publishes as the App's bot account, on **two** surfaces. Findings arrive as
a pull-request review with inline comments; a review with nothing anchorable to the diff, and
every failure, arrive as a conversation comment. Poll both — polling only one is how a
settled review looks like a timeout:

```bash
OWNER_REPO=$(gh repo view --json nameWithOwner --jq .nameWithOwner)

gh api "repos/$OWNER_REPO/pulls/$PR_NUMBER/reviews" --paginate \
  --jq '[.[] | select(.body | startswith("## Bosun code review"))]
        | .[-3:] | .[] | {id, url: .html_url, login: .user.login, submitted_at, commit_id, body}'

gh api "repos/$OWNER_REPO/issues/$PR_NUMBER/comments" --paginate \
  --jq '[.[] | select(.body | startswith("## Bosun code review"))]
        | .[-3:] | .[] | {id, url: .html_url, login: .user.login, created_at, body}'
```

Filter for Bosun **before** taking the last few, not after: a busy PR can carry enough
unrelated bot comments (Dependabot, coverage, CI bots) to push the review out of a
bot-filtered tail. And return the whole `body`, not its first line — the first line is only
the `## Bosun code review` heading, and the commit stamp you need sits on the metadata line
below it.

A settled review is output headed `## Bosun code review` whose reviewed commit is
`$HEAD_OID`. **That commit, not the timestamp, decides whether the review covers the current
head.** Read it from `commit_id` on a review object, which Bosun sets to the commit it read;
on a conversation comment, read it from the metadata line directly under the heading, which
both surfaces carry. A Bosun job started by your push can finish before `gh pr checks`
returns, so a review for the right commit may legitimately predate `$REQUESTED_AT`. Fall
back to `submitted_at` / `created_at > $REQUESTED_AT` only for output that carries no commit
stamp at all.

Older Bosun deployments publish every review as a conversation comment and stamp no commit.
Both shapes are handled below, so a version check is not needed — but on an unstamped review
the timestamp is all you have, and two pushes in quick succession are genuinely ambiguous.

Timeout must be far longer than Copilot's. A Bosun review runs to `BOSUN_REVIEW_TIMEOUT_SECONDS`, 1800 seconds by default, plus queue time. Poll at 60s, then 120s repeating, and allow about 35 minutes.

Bosun has three terminal outcomes, not two:

| Outcome | Signal | Action |
| --- | --- | --- |
| Review published | a review **or** a comment headed `## Bosun code review` | Score the findings |
| Review failed | a comment headed `## Bosun code review failed` | Infrastructure failure, not findings. Do not score it. Report the quoted reason and stop the cycle |
| No review arrived | timeout with neither, on either surface | Bosun may have rejected the webhook at capacity, or is not installed on this repository. Report the PR as blocked and say the delivery may need redelivering |

Never score a failure comment as if it were review feedback.

**Read findings.** The normal case is a pull-request review with inline comments: read it exactly as the Copilot section describes — the thread query, replies, and `resolveReviewThread` all apply unchanged, with the author login being the App's bot (`<app-slug>[bot]`, for example `bosun-reviewer[bot]`). Each inline comment opens with its severity, `**high** — <title>`, so the rubric below applies per thread.

A review body can also carry findings that are not threads, under `### Findings outside the diff` or `### Findings` — the first when a finding names a file the PR does not touch, the second when GitHub rejected the inline positions and Bosun republished everything in the body. Score those from the body and record them in the audit comment described below; there is nothing to resolve.

Where Bosun publishes a single conversation comment instead, there are no threads to resolve. Read the comment body, which lists findings in severity order with file and line. Map severity to the scoring rubric:

| Bosun severity | Starting score |
| --- | --- |
| `blocker`, `high` | `2 - fix with validation` |
| `medium` | `1 - direct fix` |
| `low` | `1` or `0`, by judgment |

Severity is the reviewer's view of impact, not a decision about what to do; override it when the code says otherwise, and raise anything ambiguous to `3`. Because nothing can be resolved, post one audit comment per cycle recording every finding and its disposition:

```bash
gh pr comment "$PR_NUMBER" --body 'Bosun cycle N: handled PATH:LINE (fixed, validation: COMMAND); PATH:LINE (no action, reason).'
```

That comment is the only durable record that a finding was handled, so write it before starting the next cycle.

## Custom Reviewer

When `codeReviewer` is `skill:<skill-name>`, the repository supplies its own reviewer. Invoke that skill and let it produce the findings; this skill still owns the cycle, the scoring, and the stopping rules.

Contract the named skill is expected to honor:

1. **It reviews, it does not fix.** It must not edit, commit, push, merge, or approve. Fixes are made here, under the scoring rubric.
2. **It returns findings, each with a file path, a line or range, a severity, and a concrete remediation.** Prose is acceptable; structure is better.
3. **It reports when it found nothing,** explicitly, so an empty result is distinguishable from a failed run.
4. **It is bounded.** It terminates on its own, and reports failure rather than hanging.

Run the cycle like this:

1. Resolve the skill name and confirm it exists. If it does not, stop and tell the user that `.rulesrc.json` names a reviewer skill that is not installed — do not silently fall back to Copilot, because the repository may have chosen its reviewer specifically to avoid one.
2. Invoke the skill, passing the PR number, the head SHA, and the base branch.
3. Treat its findings as the cycle's findings and score them with the rubric above.
4. Publish the audit trail. If the custom reviewer created resolvable review threads, reply and resolve them as in the Copilot section. Otherwise post one audit comment per cycle, as in the Bosun conversation-comment path.
5. Apply the same three-cycle limit and the same stop-and-ask rule for score `3`.

If the skill returns something that does not meet the contract — no file references, or no way to tell "clean" from "failed" — say so plainly and treat the cycle as blocked rather than inventing findings or declaring success.

## Report

When the loop ends, report: the PR URL, the reviewer used, the cycle count, validation commands run, CI state, the current head commit, the latest review timestamp, and whether any review request or Bosun job remains pending.

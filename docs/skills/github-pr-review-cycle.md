# github-pr-review-cycle

Drive a GitHub pull request through a bounded review loop with whichever code review agent
the repository has configured: GitHub Copilot, [Bosun](https://github.com/everydaydevopsio/bosun),
or a reviewer the repository defines as its own skill.

Supersedes [`github-pr-copilot-cycle`](github-pr-copilot-cycle.md), which is deprecated and
drives Copilot only.

## Install

```bash
ballast install --skill github-pr-review-cycle --code-reviewer bosun
```

Interactive installs prompt for the reviewer when this skill is selected and `.rulesrc.json`
has no saved value. `--yes` and CI mode default to `copilot`.

## Configuration

The reviewer is repository policy, saved as `codeReviewer` in `.rulesrc.json`:

```json
{
  "skills": ["github-pr-review-cycle"],
  "codeReviewer": "bosun"
}
```

| Value | Reviewer |
| --- | --- |
| `copilot` | GitHub Copilot code review. The default, and the behavior of the deprecated skill. |
| `bosun` | A self-hosted Bosun deployment authenticating as a GitHub App. |
| `none` | Automated review is disabled; the skill reports that and stops. |
| `skill:<skill-name>` | A reviewer the repository implements as its own skill. |

`ballast doctor` reports the saved value. The skill reads it at run time, so changing the
reviewer takes effect immediately — no reinstall or regeneration needed.

Resolution precedence when the skill runs:

1. An explicit argument to the skill.
2. `codeReviewer` in `.rulesrc.json`.
3. `copilot`.

## What stays the same across reviewers

The cycle, the scoring rubric, and the stopping rules are reviewer-independent:

- At most three review cycles.
- Findings are scored `0` (no action), `1` (direct fix), `2` (fix with validation), or
  `3` (human input required).
- A score `3` finding stops the cycle and asks the user.
- Every finding gets a reply before the cycle closes, including ones not acted on.
- CI is checked after every push.

## What varies

Only three things: how a review is requested, how you know it settled, and how findings are
read back.

**Copilot** is requested with `gh pr edit --add-reviewer "@copilot"`, settles within about ten
minutes, and publishes review threads that can be replied to and resolved.

**Bosun** has no reviewer-request API. It reviews automatically on `opened`, `reopened`,
`synchronize`, and `ready_for_review`, so every push already triggers the next review; an
explicit `@bridgectl review` comment is only needed when no automatic review is coming. Its
timeout is far longer — `BOSUN_REVIEW_TIMEOUT_SECONDS` defaults to 1800 plus queue time, so
allow about 35 minutes. It has three terminal outcomes rather than two: a published review, a
`## Bosun code review failed` comment, and no review at all (the webhook may have been rejected
at capacity, or the App is not installed). A failure comment is infrastructure failure and must
never be scored as review feedback.

Two Bosun constraints are worth knowing before wiring this into automation: the triggering
comment's author must be `OWNER`, `MEMBER`, or `COLLABORATOR`, and comments from bots are
ignored so Bosun cannot trigger itself. A local `gh` session qualifies; a GitHub Actions
`GITHUB_TOKEN` does not, and is dropped silently.

## Defining your own reviewer

Set `codeReviewer` to `skill:<skill-name>` and implement that skill. The review cycle still
owns the loop, the scoring, and the stopping rules; the named skill only produces findings.

The contract it must honor:

1. **Review only** — no edits, commits, pushes, merges, or approvals. Fixes are made by the
   cycle, under the scoring rubric.
2. **Return findings** with a file path, a line or range, a severity, and a concrete
   remediation. Prose is accepted; structure is better.
3. **Report "nothing found" explicitly**, so a clean result is distinguishable from a failed
   run.
4. **Terminate on its own**, reporting failure rather than hanging.

If the named skill is not installed, the cycle stops and says so rather than falling back to
Copilot — a repository that named its own reviewer may have done so specifically to avoid one.

## Migrating from github-pr-copilot-cycle

```bash
ballast install --skill github-pr-review-cycle --code-reviewer copilot
```

That reproduces the old behavior exactly. The deprecated skill keeps working until its removal
version; both can be installed at once during a transition.

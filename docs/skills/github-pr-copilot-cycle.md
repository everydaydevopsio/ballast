# GitHub PR Copilot Cycle

Use `github-pr-copilot-cycle` to run a pull request feedback loop with GitHub Copilot review.

## Install

```bash
ballast install --target codex --skill github-pr-copilot-cycle
ballast install --target claude --skill github-pr-copilot-cycle
```

It is also included by `--all-skills`.

## What It Covers

- Creating or updating a pull request.
- Requesting `@copilot` as a reviewer.
- Reading Copilot review comments and unresolved review threads.
- Fixing actionable comments while escalating ambiguous or risky feedback.
- Checking CI after each push.
- Re-requesting Copilot review until there are no unresolved Copilot comments or the cycle limit is reached.

The source of truth for this skill is `skills/common/github-pr-copilot-cycle/SKILL.md`.

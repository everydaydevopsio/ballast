# Plans

Active implementation and process plans for non-trivial Ballast work.

| Plan | Status | Purpose |
| --- | --- | --- |
| [plan-code-review-agent-selection.md](plan-code-review-agent-selection.md) | Implemented | Make the PR review cycle reviewer-agnostic: Copilot, Bosun, or a user-defined skill, selected by a `codeReviewer` key in `.rulesrc.json`. |

New plans are added here as `plan-<feature-name>.md` with a Status and a one-line
purpose, per the plan-lifecycle rule.

Completed plans graduate to an ADR in [`adr/`](../adr/README.md) and are removed from this
index; the ADR becomes the durable record of the decision.

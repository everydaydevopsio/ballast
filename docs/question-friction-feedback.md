# Agent Performance Feedback

Ballast owns durable agent policy: rules, skills, and repository guidance. It does not own runtime telemetry.

`orchael/bridgectl` wraps Claude Code, Codex, OpenCode, Gemini, and other coding agents. bridgectl therefore owns runtime observation, telemetry schemas, persistence, aggregation, and performance analysis.

## Boundary

```text
Claude / Codex / OpenCode / Gemini
              |
          bridgectl
 runtime observation + telemetry
 storage + aggregation + analysis
              |
              v
       distilled findings
              |
              v
          Ballast review
 rules / skills / repo guidance
              |
              v
       measure again in bridgectl
```

Ballast should receive findings, not raw telemetry. A finding may include category, expected impact, confidence, supporting evidence, likely root cause, affected providers/projects, a recommended action, and a measurable success criterion.

When the finding belongs in Ballast:

1. Find the narrowest existing rule or skill that owns the behavior.
2. Prefer a specific instruction over provider-wide bypass modes.
3. Add a regression example where useful.
4. Regenerate checked-in provider outputs with `ballast upgrade --patch`.
5. Roll the change out deliberately.
6. Let bridgectl measure whether the change improved runtime behavior.

Protected production, destructive, publishing, credential, billing, and permission boundaries remain human-gated regardless of historic approval rates.

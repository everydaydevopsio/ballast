# Question Friction Feedback

Ballast should treat unnecessary agent questions as a measurable policy defect.

`orchael/bridgectl` emits redacted schema-version 1 question events. Ballast can collect them centrally with `services/telemetry-aggregator`, then use aggregate results to improve source rules.

## Architecture

```text
Claude / Codex / OpenCode / Gemini
              |
          bridgectl
   detect + redact + classify
              |
              v
 Ballast Telemetry Aggregator
     local JSONL or S3
              |
              v
   fingerprint summaries
              |
              v
        Ballast review
              |
              v
   source-rule improvement
              |
              +----> measure again
```

The aggregator runs locally in Docker or in Kubernetes via `charts/telemetry-aggregator`.

Use local storage for a single collector. Use S3 for distributed collectors and multiple replicas; every event is stored as a separate immutable object so collectors do not need shared locks.

## Candidate rule

A recurring question is a candidate for a Ballast rule change when:

- it has been observed at least five times;
- at least 90% of answered occurrences were accepted without modification;
- no recent occurrence was rejected;
- removing the question does not cross a high-risk boundary.

The following categories require human input even when historic acceptance is high:

- production mutations;
- destructive or difficult-to-reverse operations;
- publishing, releases, merges, or sending external messages;
- credentials, secrets, identity, or permission broadening;
- spending money or changing billing;
- choices where alternatives materially change user-visible behavior.

## Review workflow

For each high-confidence candidate:

1. Query `GET /v1/summary` for the review window.
2. Rank recurring fingerprints by frequency, unchanged acceptance rate, and human response time.
3. Find the narrowest existing Ballast rule that owns the behavior.
4. Prefer an instruction that lets the agent act without asking.
5. If the question comes from a provider permission system, prefer a scoped allow-list over an unprotected/bypass mode.
6. Add a regression example showing what the agent should do instead of asking.
7. Regenerate checked-in agent outputs with `ballast upgrade --patch`.
8. Roll the change out to a small set of repositories first.
9. Compare aggregator telemetry before and after the rollout.

Track questions per agent-hour, questions per completed session, unchanged acceptance rate, changed/rejected rate, and human response latency. A rule that suppresses useful questions is a regression even if total prompt count falls.

## Example

If telemetry reports "Should I run the unit tests?" 27 times with 27 acceptances, Ballast should add or strengthen a rule that verification commands run automatically after implementation.

If it reports "Should I deploy to production?" 27 times with 27 acceptances, Ballast should **not** infer blanket deployment approval. Production remains an explicit decision boundary unless the repository has a separately reviewed deployment policy granting that authority.

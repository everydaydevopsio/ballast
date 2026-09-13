# Question Friction Feedback

Ballast should treat unnecessary agent questions as a measurable policy defect.

`orchael/bridgectl` exports an aggregate JSON document with `schema_version: 1`. Each entry describes one recurring question fingerprint and how the human answered it: accepted, rejected, changed, or unknown.

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

1. Find the narrowest existing Ballast rule that owns the behavior.
2. Prefer an instruction that lets the agent act without asking.
3. If the question comes from a provider permission system, prefer a scoped allow-list over an unprotected/bypass mode.
4. Add a regression example showing what the agent should do instead of asking.
5. Regenerate checked-in agent outputs with `ballast upgrade --patch`.
6. Roll the change out to a small set of repositories first.
7. Compare bridgectl telemetry before and after the rollout.

The success metric is fewer questions per agent-hour while keeping rejected and changed answers flat or lower. A rule that suppresses useful questions is a regression even if total prompt count falls.

## Example

If bridgectl reports "Should I run the unit tests?" 27 times with 27 acceptances, Ballast should add or strengthen a rule that verification commands run automatically after implementation.

If it reports "Should I deploy to production?" 27 times with 27 acceptances, Ballast should **not** infer blanket deployment approval. Production remains an explicit decision boundary unless the repository has a separately reviewed deployment policy granting that authority.

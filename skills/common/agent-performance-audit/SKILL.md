---
name: agent-performance-audit
description: use distilled bridgectl agent-performance findings to audit Ballast rules and skills for evidence-backed improvements
---
# Agent Performance Audit Skill

Use this skill when `orchael/bridgectl` has produced a distilled agent-performance analysis and you want to determine whether Ballast rules or skills should change.

This skill does **not** query raw telemetry, know the telemetry schema, or access telemetry storage. Treat bridgectl's report as the evidence input and keep all telemetry collection, persistence, aggregation, and runtime analysis inside bridgectl.

## Expected input

Accept a bridgectl report containing some or all of:

- finding category;
- expected impact;
- confidence;
- supporting metrics or representative evidence;
- affected providers, models, repositories, or projects;
- likely root cause;
- recommended action;
- recommended owner;
- automation/review status;
- measurable success criterion.

The report may describe unnecessary human interruptions, retries, failures, latency, redundant tool use, weak verification, repeated context discovery, provider/model differences, token or execution cost, or other runtime behavior.

## Goal

Audit Ballast's **source rules and source skills**, plus the generated provider-specific outputs, to find the narrowest evidence-backed improvement that could address the bridgectl finding.

Do not assume every bridgectl finding belongs in Ballast. Runtime orchestration, provider configuration, environment problems, and bridgectl behavior should remain outside Ballast.

## Procedure

### 1. Triage ownership

Classify each finding as one of:

- `ballast-rule` — persistent guidance that should be present on most relevant turns;
- `ballast-skill` — specialized or lengthy guidance that should be loaded only when needed;
- `repository-guidance` — behavior specific to one repository rather than Ballast globally;
- `bridgectl` — runtime, orchestration, telemetry, session, or wrapper behavior;
- `provider-config` — Claude/Codex/OpenCode/Gemini configuration or permissions;
- `environment-tooling` — CI, shell, dependency, build, credential, or developer-environment issue;
- `human-workflow` — a deliberate decision boundary that should remain human-owned;
- `no-change` — evidence does not justify a change yet.

Only continue with a Ballast audit for `ballast-rule` or `ballast-skill` findings.

### 2. Inspect Ballast source first

Search Ballast's source content before generated outputs:

- `agents/common/**`
- language- or framework-specific source rules;
- `skills/common/**`
- language- or framework-specific skills;
- manifest/configuration files that control generated outputs.

Identify:

- an existing rule or skill that already owns the behavior;
- overlapping or contradictory guidance;
- missing guidance;
- overly broad guidance causing unnecessary questions or retries;
- guidance that belongs in a skill instead of persistent context;
- duplicated instructions that should be consolidated.

Prefer modifying an existing owning rule/skill over creating a new one.

### 3. Audit generated outputs

Inspect generated Claude, Codex, Gemini, OpenCode, Cursor, or other provider files to verify whether the source guidance is:

- present where expected;
- missing from a provider target;
- materially different across providers;
- duplicated excessively;
- being rendered in a way that weakens the intended instruction.

Generated files are evidence of rendering/parity problems, not the preferred place to author permanent fixes.

### 4. Map evidence to the smallest change

For every proposed Ballast change, state:

- **Finding** — the bridgectl observation being addressed;
- **Evidence** — metrics or examples from the supplied report;
- **Current owner** — the Ballast source file or skill responsible today;
- **Problem** — why current guidance fails or wastes agent effort;
- **Proposed change** — exact behavior to add, remove, strengthen, narrow, or move;
- **Scope** — common, provider-specific, language-specific, framework-specific, or repository-specific;
- **Risk** — what useful behavior could regress;
- **Success criterion** — the bridgectl metric that should improve after rollout.

Prefer narrow changes that solve the measured behavior. Do not generalize from one provider or repository unless the evidence supports it.

### 5. Check rule-versus-skill placement

Use a persistent rule when the guidance is:

- short;
- broadly applicable;
- needed on most relevant turns;
- important for safety, verification, autonomy, or repository conventions.

Use a skill when the guidance is:

- lengthy or procedural;
- only relevant for a subset of tasks;
- tool-specific or workflow-specific;
- better loaded on demand to reduce persistent context.

If runtime evidence shows agents repeatedly rediscovering the same procedure, consider turning that procedure into a Ballast skill rather than expanding a permanent rule.

### 6. Preserve protected decision boundaries

Never recommend eliminating human approval solely because approval is historically common for:

- production mutations;
- destructive or difficult-to-reverse operations;
- credential, identity, or permission broadening;
- billing or spending;
- publishing, releases, merges, or external communication where policy requires approval;
- material product or architectural choices.

Historical acceptance can justify reducing friction only when the underlying action is already safe to delegate.

### 7. Validate the proposed improvement

Before considering a Ballast change complete:

1. update the Ballast source rule or skill, not only generated files;
2. regenerate provider outputs with the normal Ballast upgrade/generation workflow;
3. verify provider parity where applicable;
4. run Ballast tests and linting relevant to the change;
5. preserve the bridgectl finding identifier or report reference in the PR/commit description when available;
6. record the before/after success criterion so bridgectl can measure the rollout.

## Output format

Return a prioritized audit with this structure for each recommendation:

```text
Priority: high | medium | low
Owner: ballast-rule | ballast-skill | repository-guidance | bridgectl | provider-config | environment-tooling | human-workflow | no-change
Confidence: high | medium | low

Finding:
<what bridgectl observed>

Evidence:
<metrics/examples supplied by bridgectl>

Ballast audit:
<relevant source rules/skills and generated outputs inspected>

Recommendation:
<smallest proposed change, or why Ballast should not change>

Success criterion:
<metric bridgectl should compare after rollout>
```

Rank by expected user-time saved, reliability improvement, and confidence rather than by raw event count alone.

## Examples

### Repeated test confirmation

If bridgectl reports that agents repeatedly ask whether to run repository-defined tests and humans almost always approve unchanged, inspect the Ballast verification/autonomy rules. Prefer strengthening the existing rule so relevant tests run automatically after implementation.

### Repeated repository discovery

If bridgectl reports agents repeatedly spending time locating the same deployment or release procedure, inspect whether that knowledge belongs in a Ballast skill or repository-specific guidance. Do not add a broad global rule when the procedure is repository-specific.

### Slow provider runtime

If bridgectl reports that one provider is slower because of session startup or wrapper retries, classify the finding as `bridgectl` or `provider-config`. Do not force a Ballast rule change when instructions are not the root cause.

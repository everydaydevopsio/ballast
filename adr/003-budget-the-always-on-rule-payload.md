# ADR-003: Budget the always-on rule payload, and enforce the budget in CI

- **Status:** Accepted
- **Date:** 2026-09-29 (recording work completed 2026-07-09 – 2026-09-15)
- **Branch:** `docs/retire-stale-plans`
- **PRs:** #305, #309, #310, #311, #312, #313, #314, #315, #331, #332, #333, #334, #335
- **Issues:** #286, #287, #288, #289, #290, #291, #292, #293, #294, #295, #296, #297
- **Supersedes:** none
- **Superseded by:** none

## Context

Ballast's generated rules are loaded eagerly into every agent session, in every managed
repository. A review of the emitted `.claude/rules/` payload in this repository measured
**133.3 KB — roughly 33k tokens — on every turn**, and found that 60–70% of it was
duplication, inactive content, or reference material that only mattered at the moment an
agent was doing one specific thing.

That cost is paid by every downstream repository, on every session, whether or not the
content applies. It is also invisible: nothing in the tool reported how much context it
was consuming, so the payload had grown without anyone deciding it should.

Three distinct kinds of waste were present:

- **Repetition** — the Repository Tool Policy was injected into every rule file rather
  than stated once in the manifest; testing guidance repeated the same TDD block per
  language; persona preambles and stacked H1/description headers recurred throughout.
- **Inactive content** — rules emitted for opt-in publishing variants nobody had
  configured, deployment-model-gated sections rendered with an "inactive" banner over
  their full body, and a tombstone rule for a removed feature.
- **Reference material** — the inline MIT licence text, a badge example gallery, a
  Dependabot tutorial config, and plan/ADR templates. All of it is needed occasionally
  and none of it is needed every turn.

## Decision

**Treat always-on context as a budgeted resource with an enforced ceiling, and put
anything procedural or reference-shaped behind on-demand loading instead.**

Four rules follow, and were applied across all backends:

1. **State repo-wide policy once, in the manifest.** `CLAUDE.md` / `AGENTS.md` is the
   single home for policy that applies to the whole repository; rules do not restate it.
   Targets with no manifest keep it inline, which is why the split must be tested from
   both sides.
2. **Do not emit content that is inactive.** A rule gated off by configuration is not
   rendered with a disabled banner — it is not written at all, and `doctor` cleans up
   rules that become inactive.
3. **Rules carry policy; skills carry procedure.** The boundary is 5 KB per emitted rule.
   Procedural and reference-heavy material moves to skills, which load on demand.
4. **Enforce it in CI, not by review.** A size gate fails the build when any emitted rule
   exceeds 5 KB or a target's total payload exceeds its budget, with an explicit
   `ruleBudget` override in `.rulesrc.json` for deliberate exceptions.

`ruleProfile` was added so the ceiling can differ by consumer: `minimal` compiles a single
~1.8 KB core rule for small-context agents, with `full` preserving prior behaviour.

## Alternatives Considered

- **Leave it and rely on model context windows growing.** Rejected: the cost is paid on
  every turn in every managed repository, and irrelevant rules do not merely cost tokens —
  they compete for attention with the ones that apply.
- **Trim by review discipline rather than a CI gate.** Rejected: the payload reached
  133 KB precisely because nothing measured it. A threshold nobody enforces is a
  preference, not a budget.
- **Convert rules to skills wholesale for a `standard` tier.** Deferred rather than
  rejected — scoped out of #295 by decision on the issue, and noted as follow-up.
- **Per-target scoped loading via path globs** (#297). Investigated and documented in
  ARCHITECTURE.md rather than implemented; target support is uneven, so the behaviour is
  recorded per target instead of assumed.

## Consequences

### Positive

- Always-on payload fell **133.3 KB → 71.2 KB per target, a 47% reduction**, with no
  applicable guidance lost.
- `ruleProfile: minimal` gives small-context agents a ~1.8 KB core rule.
- The manifest is now the unambiguous home for repo-wide policy, which removed a whole
  class of "which copy is authoritative" questions.
- Regression is prevented mechanically: the CI gate starts green and fails on growth.

### Negative

- The 5 KB per-rule ceiling is a real constraint on rule authors, and has already forced
  a wording change to keep a rule inside it during unrelated work.
- Splitting policy between manifest and rules means a target's behaviour now depends on
  whether it ships a manifest — a distinction that must be tested from both sides, and
  that a stale test script got wrong (#360).
- Moving reference material out of rules introduced the risk of pointing at content that
  consumers never receive; that risk materialised as #363 and is addressed by ADR-002.

## Implementation Notes

- Delivered in four phases across twelve issues: quick wins in the build pipeline, dedupe
  via a shared-fragment include mechanism, restructuring the publishing family plus
  `ruleProfile`, then the CI guardrail last so it started green.
- `{{include:...}}` was built first as the enabler for the dedupe and publishing work, and
  had to land in the TypeScript, Python, and Go backends together.
- Every phase changed `agents/` sources or the build pipeline, so each PR regenerated and
  committed the local `.claude/` and `.codex/` outputs.

## Verification

- All emitted rules ≤ 5 KB with valid checksums, enforced by the CI gate (rule ≤ 5 KB,
  target total ≤ 80 KB, `ruleBudget` override) and surfaced in `doctor` recommendations.
- Payload measured before and after per target: 133.3 KB → 71.2 KB.
- `ballast-audit` reports the per-target always-on cost, so the budget stays observable
  rather than needing re-derivation.

## Lessons Learned

- **Unmeasured cost grows.** The payload reached 33k tokens per turn without any single
  decision to let it; the durable fix was making the number visible and enforced, not the
  one-time trim.
- **Land the guardrail after the cleanup.** Adding the CI gate last meant it started green
  and stayed meaningful, rather than being immediately overridden.
- **"Inactive" is not free.** Rendering gated content with a disabled banner still costs
  the full body on every turn — suppression has to be structural.

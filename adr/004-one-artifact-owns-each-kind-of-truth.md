# ADR-004: Sequence Spec Kit with Ballast's rules, one artifact owning each kind of truth

- **Status:** Accepted
- **Date:** 2026-09-29
- **Branch:** `docs/graduate-spec-kit-process-adr`
- **PRs:** #380, #381
- **Issues:** #340
- **Supersedes:** none
- **Superseded by:** none

## Context

Ballast ships a `spec-kit` agent, three Spec Kit skills, and rules for tasks, plan
lifecycle, testing, documentation, and PR review. Every one of those had its own guide.
What did not exist was the sequence between them.

The consequence is specific, not abstract. A single decision could plausibly be recorded
in Spec Kit's `spec.md`, Spec Kit's `tasks.md`, Ballast's `tasks/todo.md`, a file under
`plans/`, a GitHub issue, and a PR comment. Six homes, no rule about which one is
authoritative — so it gets written in several and updated in one, and the other five decay
into confident, contradictory statements about what the product is supposed to do.

Ballast had already been bitten by exactly this class of problem from a different angle:
ADR-002 records what happens when the tool's own reports stop matching reality. Duplicated
intent is the same failure at the level of process rather than tooling.

`AGENTS.md` carried a live instance while this was being written. It instructed reviewers
to *"Follow `docs/code_review.md` for code reviews"* — a file that did not exist (#340).
The review policy was real, but it was sitting inline in `AGENTS.md` directly beneath the
pointer to its own absent home.

## Decision

**Give each artifact exactly one kind of truth to own, and document the sequence that moves
a change between them.**

| Artifact | Owns |
| --- | --- |
| Spec Kit `spec.md` | Product intent and acceptance scenarios |
| Spec Kit `plan.md` | Technical design for one feature |
| Spec Kit `tasks.md` | Generated implementation work for that feature |
| `plans/plan-<name>.md` | Branch and session continuity for non-trivial work |
| `tasks/todo.md` | Branch-local execution tracking, test evidence, rollback notes |
| GitHub Issues | Durable follow-up, bugs, cross-branch work |
| `adr/NNN-*.md` | Decisions that outlive the change that made them |

Two invariants hold it together:

1. **Intent is written before implementation, not reconstructed after it.** When existing
   behaviour changes, `spec.md` changes first. Superseded requirements are preserved
   through git history rather than rewritten to match whatever the code now does.
2. **Work that will not finish on this branch leaves the branch.** It becomes a GitHub
   issue with the link recorded — not a widened plan, and not a todo that outlives the
   branch it belongs to.

The sequence lives at `docs/development-process.md` as a standalone document. It links to
the existing per-agent and per-skill guides rather than restating them.

**The process is scaled to the change, and says so.** A single-file fix runs three of its
eight steps. Only two rules never relax: a behaviour change gets a test that failed first,
and documentation changes alongside behaviour.

## Alternatives Considered

- **Fold the sequence into `docs/agents/spec-kit.md`.** Rejected: the connective tissue
  between nine guides does not belong inside any one of them, and burying it there makes
  the thing most people need hardest to find.
- **Keep the process in agent rules only.** Rejected: rules guide agents, but humans need
  GitHub-readable docs that do not require inspecting generated `.codex/` or `.claude/`
  output. It would also add to always-on context, against ADR-003.
- **Use Spec Kit as the only task system.** Rejected: it would displace the configured
  GitHub task system and lose durable cross-branch issue semantics.
- **Put implementation detail in `spec.md`.** Rejected: Spec Kit separates intent from
  design deliberately; detail belongs in `plan.md`, generated tasks, branch plans, or ADRs.
- **Skip `tasks/todo.md` when Spec Kit has `tasks.md`.** Rejected: they hold different
  things. `tasks.md` is implementation work; `tasks/todo.md` is branch evidence, rollback
  notes, and final triage.
- **Make `speckit-taskstoissues` part of the default path.** Rejected as duplication for
  most changes; kept as an optional handoff when tasks must be visible off-branch.
- **Require ADR graduation for every Spec Kit feature.** Rejected: most features make no
  architectural decision, and an ADR per feature produces records nobody consults.

## Consequences

### Positive

- There is one answer to "where does this belong", which is what stops the duplication.
- The sequence is reachable from any entry point: nine guides now carry a pointer naming
  the step they belong to.
- Saying explicitly which steps to skip makes the process survivable on small changes,
  where an all-or-nothing process would simply be abandoned.
- `docs/code_review.md` now exists, so review standards are a real gate rather than a
  dangling reference (#340).

### Negative

- The sequence is a seventh document describing process, and it can itself go stale. It is
  covered by the docs-link check, but a link check verifies that references resolve, not
  that guidance is still true.
- Splitting ownership across seven artifacts is more to learn than "put it in the issue",
  and the skip table is a judgement call that can be applied too liberally.
- The process is documented but **not yet exercised**: this repository has not bootstrapped
  Spec Kit, so steps 1–3 are untested here. That is #303.

## Implementation Notes

- #340 and the plan's open question about `docs/code_review.md` were the same question
  asked in two places. Resolved by creating the file rather than repointing the reference,
  because that line is written by an external Codex reviewer installer — an orphaned
  `END CODEX REVIEWER INSTALLER` marker sits beneath it — so supplying the target is
  durable where editing the pointer is not. It was repo-local and never shipped to
  consumers.
- `docs-links.test.ts` was added as part of this work: every relative markdown link must
  resolve, and every `docs/*.md` path named in backticks must exist. A link check alone
  would have missed #340, whose reference was inline code.
- That check excludes `plans/` and `tasks/`. It found two references while being written —
  both correct, because a plan or todo naming a file it intends to create is describing
  intent. Documents asserting current state must be true; documents describing future
  state must not be held to that.

## Verification

- 385/385 TypeScript tests, eslint and prettier clean.
- `docs-links.test.ts` validates every link the process doc and the nine cross-links add,
  and reproduces #340 when `docs/code_review.md` is removed.
- Claims in the process doc were checked against the repository rather than assumed:
  `.specify/` is genuinely absent and #303 is genuinely open, and the doc says so.

## Lessons Learned

- **The same open question in two places is a symptom of the problem being solved.** #340
  and the plan's question were one question; noticing that was what made the fix obvious.
- **Documenting a process you have not run is a partial deliverable.** Steps 1–3 describe
  bootstrapping Spec Kit, which this repository has not done. Saying so in the document is
  the minimum; #303 is the rest.
- **A process with no skip rule gets skipped entirely.** Naming which steps a one-line fix
  can drop is what makes the remaining steps enforceable.

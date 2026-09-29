# Development Process

How Spec Kit and Ballast's existing rules fit together, from product intent to merged code.

Ballast ships a `spec-kit` agent and three Spec Kit skills alongside rules for tasks, plan
lifecycle, testing, documentation, and PR review. Each is documented on its own. What was
missing is the sequence — which artifact owns which kind of truth, and in what order they
come into play.

This is a recommended workflow, not a replacement for those rules. It sequences the
existing guides rather than restating them; each step links to the guide that owns the
detail.

## Which artifact owns what

The main failure mode this process prevents is the same decision being recorded in five
places and drifting in four of them. Each artifact owns exactly one kind of truth:

| Artifact | Owns | Does not own |
| --- | --- | --- |
| Spec Kit `spec.md` | Product intent and acceptance scenarios | Technical design, implementation detail |
| Spec Kit `plan.md` | Technical design for one feature | Product intent, branch bookkeeping |
| Spec Kit `tasks.md` | Generated implementation work for that feature | Evidence, rollback notes, durable follow-up |
| `plans/plan-<name>.md` | Branch and session continuity for non-trivial work | Anything a single-file fix would need |
| `tasks/todo.md` | Branch-local execution tracking, test evidence, rollback notes | Durable backlog — it is deleted when the branch is done |
| GitHub Issues | Durable follow-up, bugs, cross-branch work | Branch-local checklists |
| `adr/NNN-*.md` | Decisions that outlive the change that made them | Work in progress |

Two rules keep it that way:

- **Intent is written before implementation, not reconstructed after it.** When existing
  behaviour changes, update `spec.md` first. Old requirements are preserved through git
  history rather than rewritten to match whatever the code now does.
- **Work that will not be finished on this branch leaves the branch.** It goes to a GitHub
  Issue with the link recorded, not into a widened plan or a todo that outlives its branch.

## The sequence

### 1. Set up the repository

Use [`speckit-bootstrap`](skills/speckit-bootstrap.md) when `.specify/` is missing,
incomplete, or questionable.

Done when `.specify/` is valid, the native `speckit-*` skills are installed for the active
target, a project constitution exists if cross-cutting constraints are needed, and the
Ballast managed outputs list the `spec-kit` rule and the `speckit-*` skills.

> This repository has not bootstrapped Spec Kit yet — there is no `.specify/` directory.
> That work is tracked as [#303](https://github.com/everydaydevopsio/ballast/issues/303).

### 2. Establish a baseline, if the product already exists

Use [`speckit-reverse-engineer`](skills/speckit-reverse-engineer.md) before any forward
work when the product exists but reliable specs do not.

Evidence is weighted, strongest first: intentional specs and explicit product decisions,
then observable runtime behaviour, E2E and smoke tests, integration tests, unit tests,
source and configuration, and only then structural inference.

The discipline that matters here: **low-confidence behaviour stays an assumption or an
open question — it does not get promoted to a requirement** because it was easier to
write it as one. Capabilities are grouped by what the product does, not by route or file.

### 3. Run the feature

Use [`speckit-delivery`](skills/speckit-delivery.md) for bounded forward work. It
orchestrates the native skills in order:

```text
speckit-specify → speckit-clarify → speckit-checklist → speckit-plan
   → speckit-tasks → speckit-analyze → speckit-implement → speckit-converge
```

`speckit-taskstoissues` is an optional handoff for larger features where the generated
tasks need to be visible to people who are not on the branch. It is not part of the
default path — for most changes it duplicates tracking that `tasks.md` already does.

### 4. Track the branch

Create or update `tasks/todo.md` per the [tasks guide](agents/tasks.md): scope and
constraints, acceptance criteria, execution checklist, test strategy with
requirement-to-test mapping, rollback strategy, and outcome with command evidence.

It is branch-local. Before opening a PR, every unchecked item is resolved, promoted to a
GitHub Issue with the link recorded, or removed as no longer relevant.

### 5. Plan the implementation, and graduate decisions

Create a plan under `plans/` per the [plan lifecycle guide](agents/plan-lifecycle.md) when
the change touches more than two files, spans multiple sessions, carries real uncertainty,
or makes an architectural decision. Skip it for single-file fixes.

Keep it current: when the approach changes, the plan and its change log change with it. Do
not widen a plan to absorb out-of-scope work — that goes to `tasks/todo.md` or an issue.

Graduate to an ADR when the plan lifecycle rule calls for it — that is, when the decision
outlives the change. Not every Spec Kit feature produces an architectural decision, and
routine features should not generate ADRs for their own sake.

### 6. Test against the acceptance criteria

Start from the acceptance criteria in the Spec Kit spec, per the
[testing guide](agents/testing.md):

- Write the failing test first, and confirm it fails **for the expected reason**.
- Implement the smallest coherent change that makes it pass.
- Cover the failure paths — errors, edge cases, misuse — not only the happy one.
- Link tests back to requirement or issue IDs through names, comments, or PR evidence.
- Run targeted tests locally; rely on CI for the full matrix.

### 7. Update the documentation in the same change

Per the [docs guide](agents/docs.md), documentation changes alongside behaviour, CLI
commands, configuration, architecture, workflows, or operating assumptions — not after.

Docs should answer: what changed, why a user would care, how a new user gets started, how
an advanced user configures or operates it, and which commands, flags, config fields,
files, or APIs are affected.

### 8. Close the PR

Use [`github-pr-copilot-cycle`](skills/github-pr-copilot-cycle.md): push, request
`@copilot`, wait for review to settle, score every unresolved comment, fix the actionable
ones, reply and resolve each handled thread, and re-check CI after each push. Up to three
cycles, or until nothing actionable is unresolved.

Review standards — severity, what blocks a merge, and the evidence bar — are in
[code_review.md](code_review.md). Agent review findings are claims to verify, not verified
findings.

## Completion gates

A change is done when all of these hold:

- [ ] Spec and plan artifacts reflect the intended product change.
- [ ] Spec Kit tasks are implemented, or explicitly deferred with a reason.
- [ ] `speckit-converge` reports no actionable gaps.
- [ ] `tasks/todo.md` is complete, or its remainder is linked to GitHub Issues.
- [ ] Tests pass locally for the relevant scope, and CI passes for the PR.
- [ ] User-facing and operator-facing documentation is current.
- [ ] Copilot review has no unresolved actionable comments.
- [ ] Architectural decisions are captured in ADRs where the plan lifecycle rule requires it.

## When to skip steps

The full sequence is for a bounded product change. Most work is smaller:

| Change | Sequence |
| --- | --- |
| Single-file fix, obvious and contained | Steps 6–8. No plan, no spec. |
| Bug fix with a behaviour change | Steps 4, 6, 7, 8. Spec only if intent was wrong. |
| Non-trivial feature | All eight. |
| Architectural change | All eight, and step 5 ends in an ADR. |

Running the full process on a one-line fix produces ceremony, not quality. The rules that
never relax are the ones in steps 6 and 7: a behaviour change gets a test that failed
first, and docs change with behaviour.

## Related

- [`spec-kit` agent guide](agents/spec-kit.md) — when Spec Kit applies and how Ballast treats it
- [Code review policy](code_review.md)
- [Architecture decision records](../adr/README.md)

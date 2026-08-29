---
name: speckit-delivery
description: >
  Orchestrate GitHub Spec Kit's native skills for a bounded product change.
  Use after a Spec Kit baseline exists to move from product intent through
  clarification, planning, tasks, implementation, and convergence.
---

# Spec Kit Delivery

Use GitHub Spec Kit's native skills as the implementation lifecycle. Ballast coordinates the sequence but does not duplicate their internal prompts.

## Preconditions

- `.specify/` exists and is valid.
- Native `speckit-*` skills are installed for the active coding agent.
- The project constitution is populated when the project requires governing constraints.

Run `speckit-bootstrap` if these prerequisites are missing.

## New Feature or Product Change

Use the native workflow:

```text
speckit-specify
    ↓
speckit-clarify        when ambiguity matters
    ↓
speckit-checklist      when requirements need an explicit quality gate
    ↓
speckit-plan
    ↓
speckit-tasks
    ↓
speckit-analyze        before implementation for non-trivial changes
    ↓
speckit-implement
    ↓
speckit-converge
```

If `speckit-converge` appends remaining tasks, repeat:

```text
speckit-implement → speckit-converge
```

until the feature converges or a real product/technical decision requires human input.

## Existing Feature Change

When changing an existing feature, update the intentional `spec.md` first. Preserve requirement history through Git rather than rewriting the requirement to match current code.

Regenerate or reconcile downstream plan/tasks for the bounded change according to the repository's chosen Spec Kit persistence model.

## Requirements Before Implementation

Before implementation begins, confirm:

- user stories describe real user/business outcomes
- functional requirements are testable
- acceptance scenarios are observable
- assumptions and open questions are explicit
- technical design lives in the plan rather than the requirement specification
- tasks trace back to the specification and plan

## Runtime Verification

After implementation and automated tests, use runtime tooling when available. For web applications, use Pilot to exercise the implemented acceptance scenarios against the running app.

Runtime validation supplements `speckit-converge`; it does not replace it. Converge checks code against spec/plan/tasks, while Pilot demonstrates user-visible behavior.

## GitHub Issues

Use `speckit-taskstoissues` only when the team wants generated implementation tasks represented as GitHub issues. Do not duplicate long-lived normative requirements into issue bodies when the specification already exists in the repository; link back to the feature spec.

## Completion

A change is complete when:

- implementation and automated tests pass
- user-visible acceptance scenarios are verified where practical
- `speckit-converge` reports no actionable gaps
- documentation affected by the feature is current
- remaining intentional follow-up work is explicitly tracked

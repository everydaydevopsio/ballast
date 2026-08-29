# Spec Kit Delivery

Use `speckit-delivery` after a Spec Kit baseline exists to coordinate a bounded product change.

## Install

```bash
ballast install --target codex --skill speckit-delivery
ballast install --target claude --skill speckit-delivery
```

It is also included by `--all-skills`.

## What It Covers

- Confirming `.specify/` exists and native `speckit-*` skills are installed.
- Updating intentional `spec.md` first for existing feature changes.
- Using native Spec Kit skills for specification, clarification, planning, tasks, implementation, and convergence.
- Repeating implementation/convergence while `speckit-converge` reports actionable gaps.
- Using runtime validation where practical after automated tests.
- Creating GitHub issues from Spec Kit tasks only when the team wants that handoff.

## Workflow

```text
speckit-specify
speckit-clarify        when ambiguity matters
speckit-checklist      when requirements need an explicit quality gate
speckit-plan
speckit-tasks
speckit-analyze        before implementation for non-trivial changes
speckit-implement
speckit-converge
```

The source of truth for this skill is `skills/common/speckit-delivery/SKILL.md`.

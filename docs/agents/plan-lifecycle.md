# Plan Lifecycle Agent

The **plan-lifecycle** agent defines when agents should create implementation plans and how completed plans graduate into ADRs.

## What It Sets Up

- Plan creation guidance for non-trivial or multi-session work.
- A project-root `plans/` layout and `plans/plan-<feature-name>.md` naming convention.
- Rules for keeping a plan current while implementation changes.
- ADR graduation steps and an ADR template.
- A requirement that unresolved branch TODOs are completed or promoted to the configured task system before graduation.

## Install

```bash
ballast install --target codex --agent plan-lifecycle
ballast install --target claude --agent plan-lifecycle
ballast install --target cursor --agent plan-lifecycle
```

It is also included by `--all`.

## When To Use It

Use `plan-lifecycle` for changes that touch more than two files, span more than one session, include architectural decisions, or need durable context before merge.

The source of truth for this agent is `agents/common/plan-lifecycle/content.md`.

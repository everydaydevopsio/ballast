# Tasks Agent

The **tasks** agent keeps branch-local work tracking and durable task-system handoff consistent.

## What It Sets Up

- Task-system guidance for GitHub Issues, Jira, or Linear through `taskSystem`.
- MCP setup checks for the configured task system.
- `tasks/todo.md` as the branch-local execution, evidence, and triage artifact.
- `tasks/lessons.md` as durable learning after corrections or repeated failure patterns.

## Install

```bash
ballast install --target codex --agent tasks --task-system github
ballast install --target claude --agent tasks --task-system jira
ballast install --target cursor --agent tasks --task-system linear
```

It is also included by `--all`.

## Configuration

When `tasks` is selected and `.rulesrc.json` does not already include `taskSystem`, interactive installs prompt for a task system. Non-interactive installs use the default.

Valid `taskSystem` values are:

- `github`
- `jira`
- `linear`

The source of truth for this agent is `agents/common/tasks/`.

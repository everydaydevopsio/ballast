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

## Templates

The `tasks` rule points here rather than carrying these skeletons inline, so they cost nothing
until an agent actually writes one of these files. Copy the relevant block verbatim.

## `tasks/todo.md` Template

```markdown
# Task: <title>

## Context
- Owner:
- Date:
- Mode: <Autonomous|Approval-Required>
- PRD Section:
- Requirement IDs:

## Scope
- In scope:
- Out of scope:

## Acceptance Criteria
- AC1:
- AC2:

## Constraints
- Constraint 1

## Risks and Tradeoffs
- Risk:
- Tradeoff:

## Execution Checklist
- [ ] Step 1 with observable outcome
- [ ] Step 2 with observable outcome

## Test Strategy
- Unit:
- Integration:
- E2E:
- Failure-path tests:
- Requirement-to-test mapping:

## Rollback Strategy
- Trigger:
- Rollback steps:
- Validation after rollback:

## Outcome
- Result:
- Evidence links/commands:
- PRD updates:
```

## `tasks/lessons.md` Template

Use `tasks/lessons.md` for durable learning after corrections, regressions, or repeated failure patterns.

```markdown
# Lessons

## <YYYY-MM-DD> <Short Title>
- Incident/bug:
- Root cause pattern:
- Early signal missed:
- Preventative rule:
- Validation added (test/check/alert):
- Next trigger to detect sooner:
```

## Issue Output Template

Use this strict issue output format when presenting work that needs a decision or durable external tracking.

```markdown
### Issue #N: <Short Description>

**Severity:** <Critical|High|Medium|Low>
**User Impact:** <who is affected and how>
**Likelihood:** <High|Medium|Low>
**Time Sensitivity:** <Immediate|This sprint|Backlog>

**Problem**
Concrete explanation with file/line references and example behavior.

**Option A (Recommended)**
- Effort:
- Risk:
- Code Impact:
- Maintenance:

**Option B**
- Effort:
- Risk:
- Code Impact:
- Maintenance:

**Option C (Optional / Do Nothing)**
- Effort:
- Risk:
- Code Impact:
- Maintenance:

**Recommendation**
Explain why Option A is best based on correctness, risk, testability, and maintenance.

**Decision Request**
Proceed with: A (recommended), B, C, or alternate direction?
```

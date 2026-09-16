# Plan: Spec Kit Development Process

**Status:** Proposed
**Branch:** docs/spec-kit-development-process
**Created:** 2026-08-29
**Related ADRs:** _(none yet)_

## Problem

Ballast now includes a `spec-kit` agent and three Spec Kit wrapper skills, but the repository still needs a coherent software development process that explains how Spec Kit works with the existing task, plan lifecycle, documentation, testing, GitHub issue, and PR review rules.

Without an explicit process, teams can easily duplicate responsibilities across `spec.md`, Spec Kit `tasks.md`, Ballast `tasks/todo.md`, implementation plans, GitHub Issues, and PR comments. The goal is to make each artifact responsible for one kind of truth and create clear gates from product intent through merged code.

## Approach

Use Spec Kit as the product-intent and feature-lifecycle layer, then use existing Ballast rules and skills for execution discipline:

- Spec Kit owns product intent, acceptance scenarios, technical feature planning, generated implementation tasks, and convergence checks.
- Ballast `plans/` owns branch/session continuity for non-trivial implementation decisions and graduates durable decisions into ADRs.
- Ballast `tasks/todo.md` owns branch-local execution tracking, test evidence, rollback notes, and unresolved work triage.
- GitHub Issues own durable follow-up, cross-branch work, bugs, and feature requests because `.rulesrc.json` configures `taskSystem: github`.
- Documentation rules require user/operator-facing docs to change with behavior, CLI, configuration, architecture, or workflow changes.
- Testing rules require acceptance-driven TDD, failure-path coverage, and requirement-to-test traceability.
- `github-pr-copilot-cycle` closes the loop with Copilot review, CI checks, thread replies, and repeated review cycles.

The process should be documented as a recommended workflow, not a replacement for the existing rules. It should point to the existing rule and skill sources instead of embedding their full content.

## `.rulesrc.json` Configuration

Use this Ballast configuration shape to enable Spec Kit for this repository while preserving the existing task, docs, testing, and review workflow support:

```json
{
  "targets": ["claude", "codex"],
  "agents": [
    "local-dev",
    "docs",
    "cicd",
    "observability",
    "publishing",
    "git-hooks",
    "plan-lifecycle",
    "tasks",
    "spec-kit",
    "testing-process",
    "linting",
    "logging",
    "testing"
  ],
  "skills": [
    "owasp-security-scan",
    "aws-health-review",
    "aws-live-health-review",
    "aws-weekly-security-review",
    "github-health-check",
    "github-pr-copilot-cycle",
    "ballast-audit",
    "ballast-project-maintenance",
    "speckit-bootstrap",
    "speckit-reverse-engineer",
    "speckit-delivery"
  ],
  "ballastVersion": "5.18.3",
  "languages": ["typescript", "python", "go", "docker"],
  "paths": {
    "docker": ["."],
    "go": ["cli/ballast", "packages/ballast-go"],
    "python": ["packages/ballast-python"],
    "typescript": ["packages/ballast-typescript"]
  },
  "tools": {
    "docker": ["docker", "hadolint", "trivy"],
    "go": ["go", "gofumpt", "golangci-lint"],
    "python": ["uv", "pyenv"],
    "typescript": ["pnpm", "corepack"]
  },
  "discovery": {
    "excludePaths": ["examples"]
  },
  "taskSystem": "github",
  "deploymentModel": "none"
}
```

This example was verified against the repository's `.rulesrc.json` on 2026-09-16. When this config changes, run `ballast upgrade --patch` and commit the refreshed `.claude/`, `.codex/`, `AGENTS.md`, and `CLAUDE.md` managed outputs in the same PR.

## Files Affected

- `plans/plan-spec-kit-development-process.md` - process plan and adoption workflow.
- `plans/README.md` - active plan index.
- `.rulesrc.json` - already includes the required `spec-kit` agent and `speckit-*` skills; use the config above as the target shape.
- `docs/development-process.md` - proposed new home for the end-to-end workflow (does not exist yet; see Open Questions for placement).
- `docs/agents/spec-kit.md` - exists; add cross-links from the agent guide into this process.
- `docs/skills/speckit-bootstrap.md`, `docs/skills/speckit-reverse-engineer.md`, `docs/skills/speckit-delivery.md` - exist; each covers its own procedure, so this process should link them rather than restate them.
- `docs/skills/github-pr-copilot-cycle.md` - exists; link it as the PR closure gate.
- `docs/code_review.md` - **missing but referenced** by `AGENTS.md` ("Follow `docs/code_review.md` for code reviews"). Either create it or fix the reference before treating it as a process gate.

## Process Model

### 1. Repository Setup

Use `speckit-bootstrap` when `.specify/` is missing, incomplete, or questionable.

Expected outcomes:

- `.specify/` is present and valid.
- Native `speckit-*` skills are installed for the active target.
- Project constitution exists when cross-cutting constraints are needed.
- Ballast managed outputs list the `spec-kit` rule and `speckit-*` skills.

### 2. Brownfield Baseline

Use `speckit-reverse-engineer` before normal forward development when the product already exists but reliable specs do not.

Evidence order:

1. Existing intentional specifications and explicit product decisions.
2. Observable runtime behavior.
3. E2E and smoke tests.
4. Integration tests.
5. Unit tests.
6. Source code and configuration.
7. Structural inference.

Expected outcomes:

- Capability map grouped by product capability, not route or file.
- One feature spec per coherent capability.
- Optional `specs/BASELINE.md` for product map, confidence, assumptions, contradictions, and open questions.
- Low-confidence behavior remains an assumption or question, not a requirement.

### 3. New Feature or Product Change

Use `speckit-delivery` for bounded forward work:

```text
speckit-specify
speckit-clarify
speckit-checklist
speckit-plan
speckit-tasks
speckit-analyze
speckit-implement
speckit-converge
```

For existing behavior changes, update the intentional `spec.md` before implementation. Preserve old requirements through Git history rather than rewriting them to match current code after the fact.

### 4. Branch Execution Tracking

Create or update `tasks/todo.md` for branch-local work. It should hold:

- Scope and constraints.
- Acceptance criteria.
- Execution checklist.
- Test strategy and requirement-to-test mapping.
- Rollback strategy.
- Outcome and command evidence.

Do not use `tasks/todo.md` as durable backlog. Promote unresolved out-of-scope work to GitHub Issues and record the issue URL.

### 5. Implementation Planning and ADRs

Create a Ballast plan when the change touches more than two files, spans multiple sessions, has meaningful uncertainty, or includes architectural decisions.

During implementation:

- Keep the plan current when the approach changes.
- Record verification and alternatives.
- Do not widen the plan when out-of-scope work appears; move that to `tasks/todo.md` or GitHub Issues.

Before merge, graduate completed architectural decisions into ADRs when the plan lifecycle rule applies.

### 6. Testing and Verification

Use acceptance criteria from Spec Kit specs as the starting point for tests.

Expected testing behavior:

- Write a failing test first for behavior changes and bug fixes.
- Confirm the test fails for the expected reason.
- Implement the smallest coherent fix.
- Add failure-path coverage for errors, edge cases, and misuse paths.
- Link tests to requirements, acceptance criteria, or issue IDs through names, comments, or PR evidence.
- Run targeted local tests before push and rely on CI for the full matrix.

### 7. Documentation

Update documentation in the same change when behavior, CLI commands, configuration, architecture, workflows, or operating assumptions change.

Required documentation answers:

1. What changed?
2. Why would a user care?
3. How does a new user get started?
4. How does an advanced user configure, extend, debug, or operate it?
5. What commands, flags, configuration fields, files, or APIs are affected?

### 8. PR Closure

Use `github-pr-copilot-cycle` before considering the PR ready:

- Push the branch.
- Request `@copilot` as reviewer.
- Poll until Copilot settles.
- Score every unresolved Copilot comment.
- Fix actionable comments.
- Reply and resolve every handled thread.
- Check CI after each push.
- Repeat up to three cycles or until no unresolved Copilot comments remain.

## Completion Gates

A change is complete only when:

- Spec Kit spec and plan artifacts reflect the intended product change.
- Spec Kit tasks are implemented or explicitly deferred.
- `speckit-converge` reports no actionable gaps.
- Branch-local `tasks/todo.md` is complete or unresolved items are linked to GitHub Issues.
- Tests pass locally for the relevant scope.
- CI passes for the PR.
- User-facing and operator-facing documentation is current.
- Copilot review has no unresolved actionable comments.
- Architectural decisions are captured in ADRs when the plan lifecycle rule requires graduation.

## Phases

Per-agent and per-skill guides (`docs/agents/spec-kit.md`, `docs/skills/speckit-*.md`, `docs/skills/github-pr-copilot-cycle.md`) already exist from the Spec Kit merge, so the remaining work is the connective tissue between them, not new per-artifact docs.

- [ ] Phase 1: Resolve the placement question, then write the end-to-end process doc that sequences the existing guides.
- [ ] Phase 2: Fix the broken `docs/code_review.md` reference in `AGENTS.md` (create the doc or repoint the reference).
- [ ] Phase 3: Add cross-links from the existing Spec Kit guides to task, plan lifecycle, testing, docs, and PR review guidance.
- [ ] Phase 4: Add a docs-link check that prevents advertised-process drift (would have caught the `docs/code_review.md` break).
- [ ] Phase 5: Run focused validation and update this plan with evidence.

## Verification

Initial plan creation should verify:

- The plan exists and is linked from `plans/README.md`.
- The `.rulesrc.json` example matches the current repository configuration.
- Markdown formatting is valid and GitHub-readable.

Future implementation should verify:

- Documentation links resolve.
- Generated managed outputs remain consistent after `ballast upgrade --patch`.
- Relevant TypeScript, Python, Go, and wrapper tests pass for any behavior change.
- CI and Copilot review pass before merge.

## Alternatives Rejected

| Option                                            | Why rejected                                                                                                                                  |
| ------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| Use Spec Kit as the only task system              | It would duplicate or replace the configured GitHub task system and lose durable issue tracking semantics.                                    |
| Put all implementation details in `spec.md`       | Spec Kit separates product intent from technical design; implementation details belong in `plan.md`, generated tasks, branch plans, or ADRs.  |
| Keep process guidance only in agent rules         | Rules guide agents, but users need GitHub-readable docs that explain the workflow without inspecting generated `.codex/` or `.claude/` files. |
| Skip `tasks/todo.md` when Spec Kit has `tasks.md` | Spec Kit tasks describe implementation work; `tasks/todo.md` records branch evidence, rollback notes, and final triage.                       |

## Open Questions

- Should the final process live in a new `docs/development-process.md`, under `docs/agents/spec-kit.md`, or both?
- Should `docs/code_review.md` be created as a standalone review policy, or should `AGENTS.md` point to an existing review document?
- Should `speckit-taskstoissues` be part of the default process, or only an optional handoff for larger features?
- Should ADR graduation be required for all Spec Kit plans or only when the Ballast plan lifecycle rule is triggered?

## Change Log

| Date       | Change                                                                                                                                                                                                    |
| ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 2026-08-29 | Plan created with Spec Kit process model and target `.rulesrc.json` configuration.                                                                                                                        |
| 2026-09-16 | Merged main; corrected Files Affected (Spec Kit agent/skill docs already exist) and rescoped phases to the remaining connective work; confirmed the `.rulesrc.json` example still matches the repository. |

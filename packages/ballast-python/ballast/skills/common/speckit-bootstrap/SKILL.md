---
name: speckit-bootstrap
description: >
  Initialize or repair GitHub Spec Kit in an existing repository using native
  agent skills. Use when adopting Spec Kit, checking whether .specify is
  configured, or preparing a repository for spec-driven development.
---

# Spec Kit Bootstrap

Set up GitHub Spec Kit without duplicating its native workflow inside Ballast.

## Inspect

From the repository root, check for:

- `.specify/`
- `.specify/memory/constitution.md`
- `.specify/integration.json`
- `.specify/init-options.json`
- `specs/`
- the `specify` CLI
- native `speckit-*` skills for the active coding agent

Run `specify version` when the CLI is available.

## Initialize

If Spec Kit is not initialized, use the current CLI and native skill mode for the active integration. For Codex, prefer:

```bash
specify init . --here --integration codex --integration-options="--skills"
```

Do not vendor copies of Spec Kit's generated `speckit-*` skills into Ballast. Spec Kit owns those skills and should generate/update them.

If the repository already contains `.specify/`, preserve its configuration and use `specify` repair/update behavior rather than replacing intentional artifacts.

## Constitution

If the constitution is missing or still an unfilled template, use the native `speckit-constitution` skill to establish concise project-wide principles. Keep feature requirements out of the constitution.

Good constitution content includes:

- security and privacy constraints
- testing expectations
- compatibility requirements
- accessibility requirements
- architecture principles that apply across features

Do not use the constitution as a general project requirements document.

## Verify

Confirm the active integration exposes native skills including the core workflow:

```text
speckit-constitution
speckit-specify
speckit-plan
speckit-tasks
speckit-implement
speckit-converge
```

and optional quality skills when installed:

```text
speckit-clarify
speckit-checklist
speckit-analyze
speckit-taskstoissues
```

## Handoff

For a brownfield application with no reliable specifications, use `speckit-reverse-engineer` next.

For a new feature or intentional change, use `speckit-delivery`.

---
name: speckit-bootstrap
description: >
  Initialize or repair GitHub Spec Kit in an existing repository using native
  agent skills. Use when adopting Spec Kit, checking whether .specify is
  configured, or preparing a repository for spec-driven development.
---

<!-- Created by [Ballast](https://github.com/everydaydevopsio/ballast) v5.18.0. Do not edit this section. -->

# Spec Kit Bootstrap

Set up GitHub Spec Kit without duplicating its native workflow inside Ballast.

## Inspect

From the repository root, check for `.specify/`, `.specify/memory/constitution.md`, `.specify/integration.json`, `.specify/init-options.json`, `specs/`, the `specify` CLI, and native `speckit-*` skills for the active coding agent.

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

Good constitution content includes security and privacy constraints, testing expectations, compatibility requirements, accessibility requirements, and architecture principles that apply across features.

## Verify

Confirm the active integration exposes the core native skills: `speckit-constitution`, `speckit-specify`, `speckit-plan`, `speckit-tasks`, `speckit-implement`, and `speckit-converge`, plus optional quality skills such as `speckit-clarify`, `speckit-checklist`, `speckit-analyze`, and `speckit-taskstoissues` when installed.

## Handoff

For a brownfield application with no reliable specifications, use `speckit-reverse-engineer` next. For a new feature or intentional change, use `speckit-delivery`.

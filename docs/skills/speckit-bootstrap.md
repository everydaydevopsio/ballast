# Spec Kit Bootstrap

Use `speckit-bootstrap` when adopting or repairing GitHub Spec Kit in a repository.

## Install

```bash
ballast install --target codex --skill speckit-bootstrap
ballast install --target claude --skill speckit-bootstrap
```

It is also included by `--all-skills`.

## What It Covers

- Checking for `.specify/`, `.specify/memory/constitution.md`, `.specify/integration.json`, `.specify/init-options.json`, and `specs/`.
- Verifying the `specify` CLI when it is available.
- Initializing Spec Kit with native skill mode for the active integration.
- Preserving existing `.specify/` configuration instead of replacing intentional artifacts.
- Handing off to `speckit-reverse-engineer` for brownfield baselines or `speckit-delivery` for new changes.

For Codex, the preferred native initialization command is:

```bash
specify init . --here --integration codex --integration-options="--skills"
```

The source of truth for this skill is `skills/common/speckit-bootstrap/SKILL.md`.

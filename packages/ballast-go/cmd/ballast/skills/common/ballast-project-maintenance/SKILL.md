---
name: ballast-project-maintenance
description: >
  Inspect, bootstrap, and repair Ballast-managed repository state. Use this
  skill when a user asks whether a repo is Ballast-managed, why .ballast/ is
  missing, how to repair local Ballast CLIs, or how to refresh Ballast rules
  and skills from saved config.
---

# Ballast Project Maintenance Skill

Use this skill to inspect and repair Ballast-managed repository state without guessing.

## State Model

- `.rulesrc.json` is the saved Ballast configuration and should be committed when the repository is Ballast-managed.
- `.ballast/` is generated local tool state for project-local backend CLIs.
- `.ballast/` should be ignored by git and is safe to recreate.
- Target directories such as `.codex/`, `.claude/`, `.cursor/`, `.gemini/`, and `.opencode/` contain Ballast-managed rules and skills plus any tool-specific user files.

## Inspect State

From the repository root, run:

```bash
ballast doctor
```

Check:

- Whether `.rulesrc.json` exists and lists expected targets, agents, skills, languages, paths, and task system.
- Whether `.ballast/`, `.ballast/bin`, and `.ballast/tools` are present.
- Whether backend CLIs are found and match the expected Ballast version.
- Whether the doctor output recommends `ballast doctor --fix` or config refresh.

## Repair Missing Or Incomplete `.ballast/`

If `.ballast/` is missing or incomplete, recreate it with one of:

```bash
ballast doctor --fix
ballast install-cli
```

Use `ballast doctor --fix` when saved config should also be refreshed. Use `ballast install-cli` when only local backend CLIs need to be installed or upgraded.

For a specific backend:

```bash
ballast install-cli --language typescript
ballast install-cli --language python
ballast install-cli --language go
```

## Refresh Managed Rules And Skills

If `.rulesrc.json` exists and generated rules or skills are stale, run:

```bash
ballast install --refresh-config
```

For patch-based refresh that preserves supported user-managed sections:

```bash
ballast install --refresh-config --patch
```

For wrapper upgrades:

```bash
ballast upgrade
ballast upgrade --patch
```

## Agent Workflow

1. Read `.rulesrc.json` before changing Ballast-managed outputs.
2. Run `ballast doctor` and use its remediation text as the source of truth.
3. Prefer `ballast doctor --fix` for repair when saved config exists.
4. Prefer `ballast install-cli` when only `.ballast/` local tools are missing.
5. Do not commit `.ballast/`; it is generated local state.
6. When source `agents/`, `skills/`, sync/build scripts, or target config change, regenerate checked-in local `.claude/` and `.codex/` outputs in the same PR.

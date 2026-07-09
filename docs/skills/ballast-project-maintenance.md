# Ballast Project Maintenance

Use `ballast-project-maintenance` when an AI agent needs to inspect, bootstrap, or repair a Ballast-managed repository.

## Install

```bash
ballast install --target codex --skill ballast-project-maintenance
ballast install --target claude --skill ballast-project-maintenance
```

It is also included by `--all-skills`.

## What It Covers

- Confirming whether `.rulesrc.json` marks the repo as Ballast-managed.
- Inspecting generated local `.ballast/` tool state with `ballast doctor`.
- Recreating missing `.ballast/`, `.ballast/bin`, or `.ballast/tools` with `ballast doctor --fix` or `ballast install-cli`.
- Refreshing generated rules and skills from saved config with `ballast install --refresh-config`.
- Avoiding commits of `.ballast/`, which is generated local state.

The source of truth for this skill is `skills/common/ballast-project-maintenance/SKILL.md`.

# Ballast Architecture

This document describes the architecture of **@everydaydevopsio/ballast**, a CLI that installs AI agent rules (linting, local-dev, CI/CD, observability) into the correct locations for Cursor, Claude Code, or OpenCode.

---

## Overview

**One package, one command.** Add ballast as a dev dependency, run `ballast install`, and the right rule files are written for your chosen AI platform. Only agents shipped inside this package are installable; there is no discovery of external rule bundles.

---

## Repository Layout

```
ballast/
├── agents/                    # Agent content and per-target templates
│   ├── linting/
│   │   ├── content.md         # Shared rule content
│   │   └── templates/
│   │       ├── cursor-frontmatter.yaml
│   │       ├── claude-header.md
│   │       └── opencode-frontmatter.yaml
│   ├── local-dev/
│   ├── cicd/
│   └── observability/
├── src/                       # TypeScript source
│   ├── cli.ts                 # Entry: install command, --help, --version
│   ├── install.ts             # Install flow: resolve target/agents, install()
│   ├── config.ts              # .rulesrc.json load/save, findProjectRoot, isCiMode
│   ├── agents.ts              # Agent list, getAgentDir, resolveAgents
│   ├── build.ts               # buildContent(agentId, target), getDestination()
│   └── *.test.ts
├── bin/
│   └── ballast.js             # Shebang entry; requires dist/cli.js
├── dist/                      # Compiled output (from pnpm run build)
└── package.json
```

---

## Agents

Each **agent** is a named set of rules (e.g. linting, local-dev, cicd, observability). Agents are defined entirely inside this repo:

- **`agents/<id>/content.md`** — Shared markdown content for that agent.
- **`agents/<id>/templates/`** — Per-target wrappers:
  - **Cursor**: `cursor-frontmatter.yaml` — prepended as frontmatter; output is `.mdc`.
  - **Claude**: `claude-header.md` — prepended to content; output is `.md`.
  - **OpenCode**: `opencode-frontmatter.yaml` — prepended as frontmatter; output is `.md`.

The build layer concatenates template + content per target; there is no merge strategy (e.g. no marker-merge or append). Each install is a single generated file per agent.

---

## Targets and Destinations

Supported **targets** are: `cursor`, `claude`, `opencode`.

| Target   | Directory        | File pattern    |
| -------- | ---------------- | --------------- |
| cursor   | `.cursor/rules/` | `{agentId}.mdc` |
| claude   | `.claude/rules/` | `{agentId}.md`  |
| opencode | `.opencode/`     | `{agentId}.md`  |

Paths are relative to the **project root** (see Config and project root).

---

## Build Flow

1. **`buildContent(agentId, target)`** (in `build.ts`):
   - Reads `agents/<id>/content.md`.
   - Reads the appropriate template for the target.
   - Returns `template + content` (with a newline where applicable).

2. **`getDestination(agentId, target, projectRoot)`** (in `build.ts`):
   - Returns the directory and full file path where the built content should be written.

Install then writes that content to the destination path (creating the directory if needed).

---

## Install Flow

1. **Resolve project root** — `findProjectRoot()` walks up from the current working directory until it finds a directory containing `.rulesrc.json` or `package.json`; that directory is the project root.

2. **Resolve target and agents** — `resolveTargetAndAgents()`:
   - Loads `.rulesrc.json` from the project root (if present).
   - Uses CLI flags: `--target`, `--agent` / `--all`, `--yes`.
   - In **interactive** mode (no `--yes`, not CI): prompts for target and agents if not fully determined.
   - In **CI/non-interactive** mode (`CI=true` or `--yes`): if `.rulesrc.json` is missing, **requires** `--target` and `--agent` (or `--all`); otherwise exits with an error and usage hint.

3. **Persist config (optional)** — If the user did not pass target/agent flags, the resolved `target` and `agents` are saved to `.rulesrc.json` so future runs can be non-interactive.

4. **Install** — For each resolved agent:
   - Compute destination via `getDestination(agentId, target, projectRoot)`.
   - If the destination file already exists and `--force` is not set, **skip** (no overwrite).
   - Otherwise create the parent directory if needed, build content with `buildContent(agentId, target)`, and write the file.

**Overwrite policy:** Existing rule files are never overwritten unless the user passes `--force`.

---

## Config and Project Root

- **Config file:** `.rulesrc.json` in the project root.
- **Shape:** `{ "target": "cursor" | "claude" | "opencode", "agents": string[] }`.
- **When saved:** When running interactively (or without `--yes`) and the user did not pass `--target` / `--agent` / `--all`, so that the next run can reuse the same choices.
- **Project root:** First directory (walking up from the cwd) that contains `.rulesrc.json` or `package.json`; if none is found, the starting cwd is used.

---

## CI / Non-Interactive Mode

- **CI detection:** `isCiMode()` is true when `CI`, `TF_BUILD`, `GITHUB_ACTIONS`, or `GITLAB_CI` is set to a truthy value, or when the user passes `--yes`.
- **Requirement:** In CI/non-interactive mode, if `.rulesrc.json` is missing, the CLI **requires** `--target` and `--agent` (or `--all`). If they are missing, it prints an error and exits with code 1.

---

## CLI Surface

- **Entry:** `bin/ballast.js` (shebang) runs the compiled `dist/cli.js`.
- **Command:** `install` (default when no command is given). No other commands (e.g. no `doctor`, `diff`, `uninstall`).
- **Options:** `--target` / `-t`, `--agent` / `-a` (comma-separated), `--all`, `--force` / `-f`, `--yes` / `-y`, `--help` / `-h`, `--version` / `-v`.

---

## Design Notes

- **Copy only:** Files are always copied into the repo; there is no symlink mode.
- **No external bundles:** Only agents under `agents/` in this package are installed; the CLI does not scan dependencies for rule packages or manifests.
- **Platform first, then agents:** The user chooses target (platform), then which agents to install (or “all”).
- **Repeatability:** Storing `target` and `agents` in `.rulesrc.json` makes repeat runs (e.g. in CI or after `pnpm install`) non-interactive when config is present.

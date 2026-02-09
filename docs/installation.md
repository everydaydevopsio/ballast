# Ballast Installation Guide

This guide is intended for AI coding agents (Cursor, Claude Code, OpenCode, Codex). Follow these instructions to install and configure **ballast** in the user's project.

## What is Ballast?

**Ballast** (`@everydaydevopsio/ballast`) is a CLI that installs TypeScript AI agent rules (linting, local-dev, CI/CD, observability, logging, testing) into the correct locations for Cursor, Claude Code, OpenCode, or Codex. One package, one command—pick your platform and which agents to install.

## Prerequisites

- Node.js (the project may use nvm—check for `.nvmrc`)
- A package manager: npm, pnpm, or yarn

## Installation Steps

### 1. Install Ballast as a Dev Dependency

From the **project root** (where `package.json` lives), run:

```bash
pnpm add -D @everydaydevopsio/ballast
```

Or with npm:

```bash
npm install -D @everydaydevopsio/ballast
```

Or with yarn:

```bash
yarn add -D @everydaydevopsio/ballast
```

Use the package manager the project already uses (check for `pnpm-lock.yaml`, `package-lock.json`, or `yarn.lock`).

### 2. Run the Installer

```bash
npx ballast install
```

This will prompt the user for:

1. **AI platform**: `cursor`, `claude`, `opencode`, or `codex`
2. **Agents**: comma-separated (e.g. `linting, local-dev`) or `all`

### 3. Non-Interactive (CI or Scripted)

If the user wants to skip prompts, pass flags:

```bash
npx ballast install --target cursor --agent linting
```

For all agents:

```bash
npx ballast install --target cursor --all
```

For other platforms, replace `cursor` with `claude`, `opencode`, or `codex`.

### 4. Overwrite Existing Rules

By default, ballast does **not** overwrite existing rule files. To overwrite:

```bash
npx ballast install --target cursor --agent linting --force
```

## Config Persistence

After an interactive install, choices are saved to `.rulesrc.json`. Future runs reuse them (non-interactive). Commit this file so the team and CI get consistent installs.

## Install Locations

| Platform | Path             | File pattern  |
| -------- | ---------------- | ------------- |
| Cursor   | `.cursor/rules/` | `<agent>.mdc` |
| Claude   | `.claude/rules/` | `<agent>.md`  |
| OpenCode | `.opencode/`     | `<agent>.md`  |
| Codex    | `.codex/rules/`  | `<agent>.md`  |

## Agent Summary

| Agent             | Description                                                                    |
| ----------------- | ------------------------------------------------------------------------------ |
| **linting**       | ESLint, Prettier, Husky, lint-staged, GitHub Actions (full instructions)       |
| **local-dev**     | nvm, Docker, env, license setup (MIT default), MCP (optional), badges          |
| **cicd**          | CI/CD pipelines, quality gates, deployment (placeholder outline)               |
| **observability** | Logging, tracing, metrics, SLOs (placeholder outline)                          |
| **logging**       | Pino + Fluentd, pino-browser to /api/logs, window.onerror, unhandledrejection  |
| **testing**       | Jest (default) or Vitest for Vite, 50% coverage, test + test:coverage, CI step |

## Suggested Install Prompts

Users can paste one of these prompts into their AI coding agent:

- **Cursor**: "Install and configure ballast by following the instructions here: https://raw.githubusercontent.com/everydaydevopsio/ballast/refs/heads/master/docs/installation.md"
- **Generic**: "Install @everydaydevopsio/ballast as a dev dependency and run `npx ballast install` for Cursor with the linting and local-dev agents."
- **OpenCode**: "Set up ballast for OpenCode with all agents. Follow: https://raw.githubusercontent.com/everydaydevopsio/ballast/refs/heads/master/docs/installation.md"

## Help

```bash
npx ballast --help
npx ballast --version
```

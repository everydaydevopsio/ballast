# Ballast

[![CI](https://github.com/everydaydevopsio/ballast/actions/workflows/test.yml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/test.yml)
[![Lint](https://github.com/everydaydevopsio/ballast/actions/workflows/lint.yaml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/lint.yaml)
[![Release](https://github.com/everydaydevopsio/ballast/actions/workflows/publish.yml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/publish.yml)
[![License](https://img.shields.io/github/license/everydaydevopsio/ballast)](LICENSE)
[![GitHub Release](https://img.shields.io/github/v/release/everydaydevopsio/ballast)](https://github.com/everydaydevopsio/ballast/releases)
[![npm version](https://img.shields.io/npm/v/@everydaydevopsio/ballast.svg)](https://www.npmjs.com/package/@everydaydevopsio/ballast)
[![npm downloads](https://img.shields.io/npm/dm/@everydaydevopsio/ballast.svg)](https://www.npmjs.com/package/@everydaydevopsio/ballast)

CLI to install TypeScript AI agent rules for **Cursor**, **Claude Code**, **OpenCode**, and **Codex**. One package, one command—pick your platform and which agents to install.

## Prerequisites

- [nvm](https://github.com/nvm-sh/nvm) (Node Version Manager)

After cloning the repo, install and use the project's Node version:

```bash
nvm install   # installs the version from .nvmrc
nvm use       # switches to it (or run `nvm install` which does both)
```

Then install dependencies: `pnpm install` (or `npm install` / `yarn`).

## Agents

| Agent             | Description                                                                                      |
| ----------------- | ------------------------------------------------------------------------------------------------ |
| **linting**       | ESLint, Prettier, Husky, lint-staged, GitHub Actions (full instructions)                         |
| **local-dev**     | Local dev environment (nvm, Docker, env), license setup (MIT default), MCP (optional)            |
| **cicd**          | CI/CD pipelines, quality gates, deployment (placeholder outline)                                 |
| **observability** | Logging, tracing, metrics, SLOs (placeholder outline)                                            |
| **logging**       | Pino + Fluentd (Node/Next.js API), pino-browser to /api/logs, window.onerror, unhandledrejection |

## Using the agents

Once installed, rule files are loaded automatically by your AI platform (Cursor, Claude Code, or OpenCode). Use an agent by asking the AI for help in that area; the rule gives it the instructions.

| Agent             | How to use it                                                                                                                                                                        |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **linting**       | In any session: _"Help me set up linting for this project"_ or _"Fix lint errors per the linting rules."_ The agent will add ESLint, Prettier, Husky, lint-staged, and CI workflows. |
| **local-dev**     | Ask for help with local dev environment, license setup (LICENSE, package.json, README), or optional MCP integration.                                                                 |
| **cicd**          | Ask for help with CI/CD pipelines, quality gates, or deployment (placeholder).                                                                                                       |
| **observability** | Ask for help with logging, tracing, metrics, or SLOs (placeholder).                                                                                                                  |
| **logging**       | Ask for help with centralized logging: Pino + Fluentd for server, pino-browser to /api/logs for console, exceptions, window.onerror, unhandledrejection.                             |

## Installation

Install as a dev dependency in your project:

```bash
npm install -D @everydaydevopsio/ballast
# or
pnpm add -D @everydaydevopsio/ballast
# or
yarn add -D @everydaydevopsio/ballast
```

## Usage

### Interactive (recommended first time)

From your project root:

```bash
npx ballast install
```

You’ll be prompted for:

1. **AI platform**: `cursor`, `claude`, `opencode`, or `codex`
2. **Agents**: comma-separated (e.g. `linting, local-dev`) or `all`

Your choices are saved to `.rulesrc.json`. Future runs reuse them (non-interactive).

### With options

```bash
# Install linting agent for Cursor
npx ballast install --target cursor --agent linting

# Install all agents for Claude
npx ballast install --target claude --all

# Overwrite existing rule files
npx ballast install --target cursor --agent linting --force

# Non-interactive (CI): require --target and --agent/--all if no .rulesrc.json
npx ballast install --yes --target cursor --agent linting
ballast install --yes --target codex --all
```

### CI / non-interactive

In CI (or with `--yes`), if `.rulesrc.json` is not present you **must** pass `--target` and either `--agent` or `--all`:

```bash
ballast install --yes --target cursor --agent linting
ballast install --yes --target opencode --all
```

### Help and version

```bash
npx ballast --help
npx ballast --version
```

## Install locations

Rules are written under your project root:

| Platform | Path             | File pattern  |
| -------- | ---------------- | ------------- |
| Cursor   | `.cursor/rules/` | `<agent>.mdc` |
| Claude   | `.claude/rules/` | `<agent>.md`  |
| OpenCode | `.opencode/`     | `<agent>.md`  |
| Codex    | `.codex/rules/`  | `<agent>.md`  |

Codex installs a root `AGENTS.md` that references the `.codex/rules/` files so Codex CLI and Codex app can load the same guidance.

## Overwrite policy

Existing rule files are **never** overwritten unless you pass `--force`. Same behavior for all platforms.

## Config file

After an interactive install, `.rulesrc.json` in the project root stores:

```json
{
  "target": "cursor",
  "agents": ["linting", "local-dev"]
}
```

Commit this file to make installs repeatable for your team and in CI (or pass `--yes --target --agent` in CI when the file is not present).

## Development

Single package (no workspaces).

```bash
pnpm install
pnpm test
pnpm run test:coverage
pnpm run lint
pnpm run lint:fix
pnpm run prettier:fix
```

### Publishing

From the repo root:

```bash
pnpm publish --access public
```

**Version history:** v1 (OpenCode-only) is [opencode-typescript-linting-agent](https://github.com/everydaydevopsio/opencode-typescript-linting-agent); v2 (multi-platform) is this repo, [typescript-linting-agent](https://github.com/everydaydevopsio/typescript-linting-agent).

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Author

Mark C Allen ([@markcallen](https://github.com/markcallen))

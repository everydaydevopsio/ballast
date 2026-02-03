# Ballast

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

CLI to install TypeScript AI agent rules for **Cursor**, **Claude Code**, and **OpenCode**. One package, one command—pick your platform and which agents to install.

## Agents

| Agent             | Description                                                                           |
| ----------------- | ------------------------------------------------------------------------------------- |
| **linting**       | ESLint, Prettier, Husky, lint-staged, GitHub Actions (full instructions)              |
| **local-dev**     | Local dev environment (nvm, Docker, env), license setup (MIT default), MCP (optional) |
| **cicd**          | CI/CD pipelines, quality gates, deployment (placeholder outline)                      |
| **observability** | Logging, tracing, metrics, SLOs (placeholder outline)                                 |

## Using the agents

Once installed, rule files are loaded automatically by your AI platform (Cursor, Claude Code, or OpenCode). Use an agent by asking the AI for help in that area; the rule gives it the instructions.

| Agent             | How to use it                                                                                                                                                                        |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **linting**       | In any session: _"Help me set up linting for this project"_ or _"Fix lint errors per the linting rules."_ The agent will add ESLint, Prettier, Husky, lint-staged, and CI workflows. |
| **local-dev**     | Ask for help with local dev environment, license setup (LICENSE, package.json, README), or optional MCP integration.                                                                 |
| **cicd**          | Ask for help with CI/CD pipelines, quality gates, or deployment (placeholder).                                                                                                       |
| **observability** | Ask for help with logging, tracing, metrics, or SLOs (placeholder).                                                                                                                  |

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

1. **AI platform**: `cursor`, `claude`, or `opencode`
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

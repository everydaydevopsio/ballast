# Ballast Documentation

Documentation for users of **@everydaydevopsio/ballast**—the CLI that installs TypeScript AI agent rules for Cursor, Claude Code, OpenCode, and Codex.

## Quick Start

**[Installation Guide](installation.md)** — Install ballast from within your AI coding agent using a prompt.

## Agent Guides

| Agent                                    | Description                                              | Guide                                         |
| ---------------------------------------- | -------------------------------------------------------- | --------------------------------------------- |
| [linting](agents/linting.md)             | ESLint, Prettier, Husky, lint-staged, GitHub Actions     | [→ linting.md](agents/linting.md)             |
| [local-dev](agents/local-dev.md)         | nvm, Docker, env, license, badges, MCP (optional)        | [→ local-dev.md](agents/local-dev.md)         |
| [cicd](agents/cicd.md)                   | CI/CD pipelines, quality gates, deployment (placeholder) | [→ cicd.md](agents/cicd.md)                   |
| [observability](agents/observability.md) | Logging, tracing, metrics, SLOs (placeholder)            | [→ observability.md](agents/observability.md) |
| [logging](agents/logging.md)             | Pino + Fluentd, pino-browser to /api/logs                | [→ logging.md](agents/logging.md)             |
| [testing](agents/testing.md)             | Jest (default) or Vitest for Vite, 50% coverage, CI step | [→ testing.md](agents/testing.md)             |

Each guide includes:

- What the agent sets up
- What it provides
- Prompts you can use to improve your app

## Installation Prompt

Paste this into Cursor, Claude Code, OpenCode, or Codex to install ballast:

```
Install and configure ballast by following the instructions here:
https://raw.githubusercontent.com/everydaydevopsio/ballast/refs/heads/master/docs/installation.md
```

Replace `master` with `main` if your repo uses that branch.

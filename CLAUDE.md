# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a single-package CLI: **@everydaydevopsio/ballast**. The binary `ballast` installs TypeScript AI agent rules (linting, local-dev, CI/CD, observability) into the correct locations for Cursor, Claude Code, or OpenCode.

## Commands

```bash
# Install dependencies
pnpm install

# Build (compile TypeScript to dist/)
pnpm run build

# Run tests
pnpm test

# Run tests with coverage (80% threshold required)
pnpm run test:coverage

# Linting
pnpm run lint           # Check for linting errors
pnpm run lint:fix       # Auto-fix linting errors

# Formatting
pnpm run prettier       # Check formatting
pnpm run prettier:fix   # Auto-fix formatting
```

## Architecture

```
agents/                 # Agent content and per-target templates
├── linting/            # Full linting instructions
│   ├── content.md
│   └── templates/      # cursor, claude, opencode
├── local-dev/          # Placeholder (short outline)
├── cicd/               # Placeholder (short outline)
└── observability/      # Placeholder (short outline)

src/
├── cli.ts              # CLI entry: install command, --help, --version
├── install.ts          # Install flow: resolve target/agents, install(), runInstall()
├── config.ts           # .rulesrc.json load/save, findProjectRoot, isCiMode
├── agents.ts           # Agent list, getAgentDir, resolveAgents
├── build.ts            # buildContent(agentId, target), getDestination()
└── *.test.ts           # Unit tests

dist/                   # Compiled output (from pnpm run build)
bin/
└── ballast.js          # Shebang entry; requires dist/cli.js
```

See **AGENTS.md** for agent-facing project summary.

## Key Details

- Single overwrite policy: do not overwrite existing rule files unless `--force`.
- Platform first, then agents; user can choose "all" agents.
- In CI mode (`CI=true` or `--yes`), if `.rulesrc.json` is missing, `--target` and `--agent` (or `--all`) are required.
- Config is persisted in `.rulesrc.json` so repeat runs are non-interactive.
- CLI only installs agents that ship inside this package (no external bundle discovery).

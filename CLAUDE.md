# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Ballast is now a multi-language rules installer project:

- `@everydaydevopsio/ballast` (npm): TypeScript profile, backward compatible
- `ballast-python`: Python CLI package
- `ballast-go`: Go CLI package

All install rules target Cursor, Claude Code, OpenCode, and Codex.

## Commands

```bash
pnpm install
pnpm run build
pnpm test
pnpm run test:coverage
pnpm run lint
pnpm run lint:fix
pnpm run prettier
pnpm run prettier:fix
```

## Architecture

```text
agents/
├── common/                  # local-dev, cicd, observability
├── typescript/              # linting, logging, testing
├── python/                  # linting, logging, testing
└── go/                      # linting, logging, testing

src/                         # npm ballast TypeScript CLI implementation
packages/
├── ballast-python/          # Python package
└── ballast-go/              # Go package
```

## Key Details

- Single overwrite policy: do not overwrite existing rule files unless `--force`.
- Shared public agent IDs: `linting`, `local-dev`, `cicd`, `observability`, `logging`, `testing`.
- Common agents come from `agents/common/*`; language-specific agents come from `agents/<language>/*`.
- Config files:
  - TypeScript CLI: `.rulesrc.ts.json`
  - Python CLI: `.rulesrc.python.json`
  - Go CLI: `.rulesrc.go.json`
- In CI mode (`CI=true` or `--yes`), if config is missing, `--target` and `--agent` (or `--all`) are required.

## License

MIT

## Installed agent rules

Created by Ballast. Do not edit this section.

Read and follow these rule files in `.claude/rules/` when they apply:

- `.claude/rules/common/local-dev-badges.md` — Rules for common/local-dev-badges
- `.claude/rules/common/local-dev-env.md` — Rules for common/local-dev-env
- `.claude/rules/common/local-dev-license.md` — Rules for common/local-dev-license
- `.claude/rules/common/local-dev-mcp.md` — Rules for common/local-dev-mcp
- `.claude/rules/common/cicd.md` — Rules for common/cicd
- `.claude/rules/common/observability.md` — Rules for common/observability


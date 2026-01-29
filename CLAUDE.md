# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a monorepo containing TypeScript linting agents for multiple AI coding tools: OpenCode, Claude Code, and Cursor IDE. Each platform has its own package that shares core content.

## Commands

```bash
# Install dependencies (all packages)
npm install

# Run tests
npm test

# Run tests with coverage (80% threshold required)
npm run test:coverage

# Linting
npm run lint           # Check for linting errors
npm run lint:fix       # Auto-fix linting errors

# Formatting
npm run prettier       # Check formatting
npm run prettier:fix   # Auto-fix formatting
```

## Architecture

```
packages/
├── core/                    # @everydaydevops/typescript-linting-core (shared content)
│   ├── src/content.md       # Core linting instructions
│   ├── templates/           # Format-specific frontmatter
│   └── index.js             # Exports format builders
├── opencode/                # @everydaydevops/opencode-typescript-linting
│   └── install.js           # OpenCode-specific installer
├── claude/                  # @everydaydevops/claude-typescript-linting
│   └── install.js           # Claude Code-specific installer
└── cursor/                  # @everydaydevops/cursor-typescript-linting
    └── install.js           # Cursor-specific installer
```

## Key Details

- Uses npm workspaces for monorepo management
- Core package is published to npm as a dependency for platform packages
- Each platform package depends on core and has its own install.js
- OpenCode overwrites existing files; Claude/Cursor preserve user customizations
- Cursor global install is skipped (requires Settings UI)

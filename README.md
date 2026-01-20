# TypeScript Linting Agents

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

TypeScript linting agents for **OpenCode**, **Claude Code**, and **Cursor IDE** following Everyday DevOps best practices.

## Packages

This monorepo contains separate packages for each AI coding tool:

| Package                                                            | npm                                                                                                                                                             | Description       |
| ------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------- |
| [@everydaydevops/opencode-typescript-linting](./packages/opencode) | [![npm](https://badge.fury.io/js/@everydaydevops%2Fopencode-typescript-linting.svg)](https://www.npmjs.com/package/@everydaydevops/opencode-typescript-linting) | OpenCode agent    |
| [@everydaydevops/claude-typescript-linting](./packages/claude)     | [![npm](https://badge.fury.io/js/@everydaydevops%2Fclaude-typescript-linting.svg)](https://www.npmjs.com/package/@everydaydevops/claude-typescript-linting)     | Claude Code rules |
| [@everydaydevops/cursor-typescript-linting](./packages/cursor)     | [![npm](https://badge.fury.io/js/@everydaydevops%2Fcursor-typescript-linting.svg)](https://www.npmjs.com/package/@everydaydevops/cursor-typescript-linting)     | Cursor IDE rules  |

## Installation

Install only the package for your preferred tool:

```bash
# For OpenCode
npm install @everydaydevops/opencode-typescript-linting

# For Claude Code
npm install @everydaydevops/claude-typescript-linting

# For Cursor IDE
npm install @everydaydevops/cursor-typescript-linting
```

Or install globally:

```bash
npm install -g @everydaydevops/opencode-typescript-linting
```

## Usage

### OpenCode

```
opencode
> @typescript-linting help me set up linting for this project
```

### Claude Code

Rules are automatically loaded when working in the project:

```
claude
> Help me set up linting for this project
```

### Cursor

Rules auto-attach when working with TypeScript/JavaScript files, or invoke manually:

```
@typescript-linting help me set up linting for this project
```

## What It Does

The agents will:

- Install and configure ESLint with TypeScript support
- Set up Prettier for code formatting
- Configure Husky for Git hooks
- Set up lint-staged for pre-commit checks
- Create GitHub Actions workflow for CI linting
- Add helpful npm scripts for linting and formatting

## Installation Paths

| Tool        | Global                      | Local            |
| ----------- | --------------------------- | ---------------- |
| OpenCode    | `~/.config/opencode/agent/` | `.opencode/`     |
| Claude Code | `~/.claude/rules/`          | `.claude/rules/` |
| Cursor      | N/A (Settings UI)           | `.cursor/rules/` |

## Overwrite Behavior

- **OpenCode**: Always overwrites existing agent files
- **Claude Code**: Never overwrites existing rules (preserves customizations)
- **Cursor**: Never overwrites existing rules (preserves customizations)

## Learn More

- [TypeScript Linting Guide](https://www.markcallen.com/typescript-linting/)
- [OpenCode Documentation](https://opencode.ai/docs)
- [Claude Code Documentation](https://docs.anthropic.com/en/docs/claude-code)
- [Cursor Documentation](https://cursor.com/docs)

## Development

This is a monorepo using npm workspaces.

```bash
# Install dependencies
npm install

# Run tests
npm test

# Run tests with coverage
npm run test:coverage

# Lint
npm run lint
```

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Author

Mark C Allen ([@markcallen](https://github.com/markcallen))

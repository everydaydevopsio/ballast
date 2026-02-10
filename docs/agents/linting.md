# Linting Agent

The **linting** agent provides comprehensive TypeScript/JavaScript linting and code formatting for your project.

## What It Sets Up

- **ESLint** — Flat config format, TypeScript support via typescript-eslint, recommended rulesets
- **Prettier** — Code formatting with `.prettierrc` and `.prettierignore`
- **Husky** — Git hooks for pre-commit
- **lint-staged** — Run lint/format only on staged files
- **GitHub Actions** — Lint workflow on pull requests

## What It Provides

- Consistent code style across the team
- Automatic fixing of lint and format issues
- Pre-commit checks so bad code doesn't get committed
- CI enforcement so PRs must pass lint before merge

## Prompts to Improve Your App

Use these prompts with your AI coding agent (Cursor, Claude Code, OpenCode, Codex) after installing the linting agent:

### Setup and Configuration

- **"Help me set up linting for this project"** — Full setup from scratch
- **"Fix lint errors per the linting rules"** — Auto-fix existing issues
- **"Add a no-console rule that warns in development but errors in production"** — Custom rule
- **"Configure ESLint to ignore our generated API client in `src/generated/`"** — Ignore patterns

### Formatting

- **"Run prettier:fix on the whole codebase"** — Format all files
- **"Add Prettier config for 100 character line width"** — Adjust formatting
- **"Ensure JSON and YAML files are also formatted by Prettier"** — Extend lint-staged

### Hooks and CI

- **"Verify the pre-commit hook runs lint-staged correctly"** — Test hooks
- **"Add a pre-push hook that runs tests before pushing"** — Extend Husky
- **"Our GitHub Actions lint workflow is failing—help debug"** — CI troubleshooting

### General

- **"Show me what the linting rules would flag in this file"** — Explain rules
- **"We use CommonJS—create eslint.config.cjs instead of .mjs"** — Module format

# Git Hooks Agent

The **git-hooks** agent owns local Git hook orchestration for Ballast-managed repos.

## What It Sets Up

- `pre-commit` for standalone repos
- Husky + `lint-staged` for TypeScript monorepos
- `pre-push` hooks that run unit tests
- maintenance guidance such as `pre-commit autoupdate` and executable hook scripts

## Ownership Model

- `linting` owns lint, format, and type-check policy
- `testing` owns test strategy and commands
- `git-hooks` owns how local Git hooks invoke those commands

Ballast auto-installs `git-hooks` whenever `linting` is selected so existing install commands keep working.

## Hook Strategy

- TypeScript standalone repos use `pre-commit`
- TypeScript monorepos use Husky at the repo root
- Python, Go, Ansible, and Terraform use `pre-commit`
- `pre-push` runs the repo's unit test command

## Prompts to Improve Your App

- **"Set up pre-commit and pre-push for this Python repo"** — standalone hook baseline
- **"Use Husky for commit and push hooks in this monorepo"** — TypeScript monorepo hook setup
- **"Move our unit tests to pre-push and keep commit hooks fast"** — hook split by cost

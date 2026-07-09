# Git Hooks Agent

The **git-hooks** agent owns local Git hook orchestration for Ballast-managed repos.

## What It Sets Up

- Husky + `lint-staged` for TypeScript-only repos
- `pre-commit` for multi-language repos and non-TypeScript language profiles
- `pre-push` hooks that run unit tests
- maintenance guidance such as `pre-commit autoupdate` and executable hook scripts

## Ownership Model

- `linting` owns lint, format, and type-check policy
- `testing` owns test strategy and commands
- `git-hooks` owns how local Git hooks invoke those commands

Ballast auto-installs `git-hooks` whenever `linting` is selected so existing install commands keep working.

## Hook Strategy

- TypeScript-only repos use Husky at the repo root
- TypeScript-only Husky pre-commit hooks stay fast with `lint-staged` or the repo formatter/linter, including explicit `.yaml` and `.yml` formatting checks
- TypeScript-only `.husky/pre-push` hooks run the detected or canonical package-manager test command, with build or typecheck first when the repo convention requires it
- Multi-language repos use `pre-commit` at the repo root
- Python, Go, Ansible, and Terraform use `pre-commit`
- `pre-push` runs the repo's unit test command

## Prompts to Improve Your App

- **"Set up pre-commit and pre-push for this multi-language repo"** — mixed-language hook baseline
- **"Use Husky for commit and push hooks in this TypeScript repo"** — TypeScript-only hook setup
- **"Move our unit tests to pre-push and keep commit hooks fast"** — hook split by cost

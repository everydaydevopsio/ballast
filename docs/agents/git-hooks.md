# Git Hooks Agent

The **git-hooks** agent owns local Git hook orchestration for Ballast-managed repos.

## What It Sets Up

- Husky with `lint-staged` or the repo formatter/linter for TypeScript-only repos
- `pre-commit` (or `prek`) for multi-language repos and non-TypeScript language profiles
- secret detection declared as a `pre-commit` hook, using Gitleaks by default
- `pre-push` hooks that run unit tests
- maintenance guidance such as `pre-commit autoupdate` (or `prek update`) and executable hook scripts only when a hook backend requires scripts

## Ownership Model

- `linting` owns lint, format, and type-check policy
- `testing` owns test strategy and commands
- `git-hooks` owns how local Git hooks invoke those commands

Ballast auto-installs `git-hooks` whenever `linting` is selected so existing install commands keep working.

## Hook Strategy

- TypeScript-only repos use Husky at the repo root
- TypeScript-only Husky pre-commit hooks stay fast with `lint-staged` or the repo formatter/linter, including explicit formatting checks for `.yaml` and `.yml` files
- TypeScript-only `.husky/pre-push` hooks run the detected or canonical package-manager test command, with build or typecheck first when the repo convention requires it
- Multi-language repos use `pre-commit` at the repo root
- Python, Go, Ansible, and Terraform use `pre-commit`
- Any of those may use `prek` instead; see **Hook Runners** below
- `pre-commit` repos should declare Gitleaks in `.pre-commit-config.yaml` for secret detection and should not generate a repo-local no-secrets shell script
- `pre-push` runs the repo's unit test command
- CI and security-review workflows own heavier language-aware checks such as `govulncheck`, fuzzing, `go test -race`, Bandit, `pip-audit`, package-manager audits, `ansible-lint --profile=safety`, deeper IaC static analysis, and cloud/runtime posture scans unless a repo explicitly opts into running them from hooks

## Hook Runners

The hook *policy* is the same whichever runner you use: fast checks at commit time, unit tests on pre-push, and Gitleaks for secret detection. Only the commands differ.

| | `pre-commit` (default) | `prek` |
| --- | --- | --- |
| Install hooks | `pre-commit install` and `pre-commit install --hook-type pre-push` | `prek install` (covers every configured stage) |
| Update pinned hooks | `pre-commit autoupdate` | `prek update` |
| Run everything | `pre-commit run --all-files` | `prek run --all-files` |
| Validate the config | — | `prek validate-config` |
| Config file | `.pre-commit-config.yaml` | `.pre-commit-config.yaml` or `prek.toml` |
| Nested configs | `sub-pre-commit` hooks | discovered automatically |

[prek](https://prek.j178.dev/) reads the same `.pre-commit-config.yaml`, so switching runners does not mean rewriting hook definitions. With either runner the **root** config is what installs the Git hooks, even when nested workspace configs exist.

### Choosing a runner

Ballast picks the runner in this order:

1. **An explicit choice** in `.rulesrc.json`:

   ```json
   {
     "hookRunner": "prek"
   }
   ```

2. **An existing prek setup**, detected from a `prek.toml` at the repository root. A repository already standardized on prek is not told to migrate.

3. **`pre-commit`**, the default.

The saved choice survives reinstalls and refreshes, and every backend preserves it, so a later install cannot silently migrate a prek repository back to pre-commit.

TypeScript-only repositories use Husky instead, which is a different tool rather than a `.pre-commit-config.yaml` runner, so `hookRunner` does not apply there.

## Prompts to Improve Your App

- **"Set up pre-commit and pre-push for this multi-language repo"** — mixed-language hook baseline
- **"Use Husky for commit and push hooks in this TypeScript repo"** — TypeScript-only hook setup
- **"Move our unit tests to pre-push and keep commit hooks fast"** — hook split by cost

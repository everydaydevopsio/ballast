# Linting Agent

The **linting** agent provides language-appropriate linting, formatting, and CI checks for TypeScript, Python, Go, Ansible, and Terraform projects.

## What It Sets Up by Language

- **TypeScript/JavaScript**
  - ESLint (flat config)
  - Prettier
  - `pre-commit` for single-repo installs
  - Husky + lint-staged for unified multi-language monorepos
  - A unit-test `pre-push` hook
- **Python**
  - Ruff for linting and formatting
  - Optional Black when explicitly required
  - mypy for type checking when type hints are used
  - `pre-commit` with a maintained `.pre-commit-config.yaml`
  - A `pre-push` test hook managed by `pre-commit`
- **Go**
  - `gofmt` for formatting
  - `golangci-lint` for static analysis
  - `pre-commit` with sub-config support for nested Go packages when needed
  - A `pre-push` test hook managed by `pre-commit`
- **Ansible**
  - `ansible-lint` for role and playbook validation
  - `yamllint` for YAML formatting and style
  - `pre-commit` hooks for lint, syntax-check, and vault-safe workflows
  - Guidance for inventories, role layout, and idempotent task design
- **Terraform**
  - `terraform fmt -check -recursive` for formatting
  - `terraform validate` and `tflint` for static validation
  - `tfsec` or `trivy config` security checks
  - `tfenv` guidance with `.terraform-version` and `pre-commit` hooks

## What It Provides

- Consistent code style and lint policy per language
- Fast local commands for lint/format/type checks
- CI enforcement so lint failures block merges

## Monorepo Usage

In a TypeScript + Python + Go monorepo, apply linting standards per language area and keep each tool scoped to its files.

Hook strategy:

- TypeScript at the monorepo root uses Husky + lint-staged.
- Python, Go, Ansible, and Terraform use `pre-commit` with root or package-level `.pre-commit-config.yaml` files as needed.
- Use `pre-commit install --hook-type pre-push` for `pre-commit` repos, and use `.husky/pre-push` for TypeScript monorepos.
- Keep every `.pre-commit-config.yaml` current with `pre-commit autoupdate` whenever hook versions change.

Recommended command set:

- TypeScript: `pnpm run lint` (or project ESLint command)
- Python: `ruff check .` and `ruff format .`
- Go: `gofmt -w .` and `golangci-lint run`
- Ansible: `ansible-lint`, `yamllint .`, and `ansible-playbook --syntax-check site.yml`
- Terraform: `tfenv install && tfenv use`, `terraform fmt -check -recursive`, `terraform validate`, `tflint --recursive`, and `tfsec .`

## Prompts to Improve Your App

- **"Set up linting for all three languages in this monorepo"** — Multi-language baseline
- **"Set up linting for our Ansible playbooks and roles"** — Playbook baseline
- **"Set up linting, formatting, tfenv, and security checks for our Terraform repo"** — Terraform baseline
- **"Fix lint errors in this package according to its language rules"** — Targeted cleanup
- **"Add CI jobs so TypeScript, Python, Go, Ansible, and Terraform lint checks run independently"** — Monorepo CI
- **"Add ignore patterns for generated code in each language"** — Noise reduction

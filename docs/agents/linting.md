# Linting Agent

The **linting** agent provides language-appropriate linting, formatting, and CI checks for TypeScript, Python, Go, Ansible, and Terraform projects.

## What It Sets Up by Language

- **TypeScript/JavaScript**
  - ESLint (flat config)
  - Prettier
- **Python**
  - Ruff for linting and formatting
  - Optional Black when explicitly required
  - mypy for type checking when type hints are used
- **Go**
  - `gofmt` for formatting
  - `golangci-lint` for static analysis
- **Ansible**
  - `ansible-lint` for role and playbook validation
  - `yamllint` for YAML formatting and style
  - Guidance for inventories, role layout, and idempotent task design
- **Terraform**
  - `terraform fmt -check -recursive` for formatting
  - `terraform validate` and `tflint` for static validation
  - `tfsec` or `trivy config` security checks
  - `tfenv` guidance with `.terraform-version`

## What It Provides

- Consistent code style and lint policy per language
- Fast local commands for lint/format/type checks
- CI enforcement so lint failures block merges

## Monorepo Usage

In a TypeScript + Python + Go monorepo, apply linting standards per language area and keep each tool scoped to its files. Use the separate `git-hooks` agent for hook orchestration.

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

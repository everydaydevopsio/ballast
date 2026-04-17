# Testing Agent

The **testing** agent sets up and maintains test workflows for TypeScript, Python, Go, Ansible, and Terraform projects with sensible defaults and CI integration.

## What It Sets Up by Language

- **TypeScript/JavaScript**
  - Jest by default
  - Vitest for Vite-based projects
  - Coverage thresholds, CI test steps, smoke tests, and optional E2E for runnable apps
- **Python**
  - pytest for test execution
  - pytest-cov for coverage reporting and enforced coverage test commands
  - Smoke tests and optional E2E for runnable apps
- **Go**
  - `go test ./...` baseline
  - Coverage checks as part of the default test workflow, including CI coverage gates
  - Smoke tests and optional E2E for runnable apps
- **Ansible**
  - `ansible-playbook --syntax-check` baseline
  - `--check --diff` validation for change preview and idempotence
  - Molecule or localhost playbook smoke tests for roles and inventories
- **Terraform**
  - `terraform fmt -check -recursive` and `terraform validate`
  - `tflint` and `tfsec`/`trivy config` for lint and security coverage
  - `terraform init -backend=false` smoke setup plus plan-review guidance

## What It Provides

- Repeatable test commands for local development
- CI checks for each language profile
- Coverage visibility with configurable thresholds
- Smoke tests that use the repo Dockerfile and `docker-compose.yaml` for runnable apps
- Explicit smoke-test pass/fail output
- A smoke-test GitHub Action and matching README badge
- Narrow end-to-end coverage for one critical workflow when the app has a real user flow

## Monorepo Usage

In multi-language monorepos, run tests per language package and report failures independently.

Recommended baseline commands:

- TypeScript: `pnpm test`
- Python: `pytest`
- Go: `go test ./...`
- Ansible: `ansible-playbook --syntax-check site.yml` and `ansible-playbook --check --diff site.yml`
- Terraform: `tfenv install && tfenv use`, `terraform init -backend=false`, `terraform validate`, `tflint --recursive`, and `tfsec .`

## Prompts to Improve Your App

- **"Set up tests for TypeScript, Python, Go, and Ansible packages in this monorepo"** — Full multi-language setup
- **"Add separate CI jobs for each language test suite"** — Better isolation
- **"Raise coverage thresholds and make CI fail when they drop"** — Quality gate
- **"Find and fix flaky tests in this package"** — Stability
- **"Add smoke tests that build our app with Docker Compose and run in GitHub Actions"** — Deployability check
- **"Add a smoke-test badge to the README"** — Status visibility
- **"Add one stable end-to-end test for the login flow"** — Critical-path E2E
- **"Add Ansible syntax-check and check-mode validation to CI"** — Infrastructure safety
- **"Add Terraform validate, tflint, and tfsec to CI with tfenv version pinning"** — Terraform safety

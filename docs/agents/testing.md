# Testing Agent

The **testing** agent sets up and maintains test workflows for TypeScript, Python, Go, Ansible, and Terraform projects with sensible defaults and CI integration.

## What It Sets Up by Language

- **TypeScript/JavaScript**
  - Jest by default
  - Vitest for Vite-based projects
  - Coverage thresholds, CI test steps, smoke tests, and optional E2E for runnable apps
- **Python**
  - pytest for test execution
  - pytest-cov for coverage reporting and fail-under thresholds
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
  - `tflint` and `trivy config` for lint and security coverage, with `tfsec` only for legacy-compatible pipelines
  - `terraform init -backend=false` smoke setup plus plan-review guidance
  - Native `terraform test` for Terraform 1.6+ module assertions and Terratest for Go-backed/live integration coverage
  - OpenTofu equivalents such as `tofu test` when the repo standardizes on `tofu`

## What It Provides

- Repeatable test commands for local development
- CI checks for each language profile
- Coverage visibility with configurable thresholds
- Web smoke tests that use the repo Dockerfile and `docker-compose.yaml` for runnable apps and verify a live route or health endpoint
- Explicit smoke-test pass/fail output
- A smoke-test GitHub Action and matching README badge
- Narrow end-to-end coverage for one critical workflow when the app has a real user flow, keeping an existing browser E2E framework when present and preferring Playwright only when Playwright markers already exist or the repo has a real browser application surface with no existing browser E2E framework

## Framework Detection

- TypeScript/JavaScript: check package and config markers for Jest, Vitest, Cypress, Playwright, WebdriverIO, Selenium, Puppeteer, and Testing Library before adding or replacing test tooling.
- Python: check markers for pytest, unittest, tox, nox, Robot Framework, Selenium, Playwright or pytest-playwright, FastAPI TestClient, Django test client, Flask test client, and existing API/service test clients.
- Go: check markers for `go test`, integration build tags, `_integration_test.go`, `httptest`, API/service tests, Selenium, chromedp, rod, agouti, Playwright, and existing browser harnesses.
- Preserve an existing browser E2E framework unless the user explicitly asks to migrate.
- Prefer Playwright only when Playwright markers already exist, or when the repo has a real browser application surface and no existing browser E2E framework.
- Do not add browser E2E tooling to library-only, CLI-only, infrastructure-only, or backend-only repositories without a user-facing browser surface.

## Smoke and E2E Placement

- Local: run fast unit tests and targeted smoke checks while developing.
- Pre-push: run deterministic build/typecheck checks plus smoke tests that do not require long-lived external services.
- CI: run the full smoke and E2E gate for runnable web apps before merge or release.

## Monorepo Usage

In multi-language monorepos, run tests per language package and report failures independently.

Recommended baseline commands:

- TypeScript: `pnpm test`
- Python: `pytest`
- Go: `go test ./...`
- Ansible: `ansible-playbook --syntax-check site.yml` and `ansible-playbook --check --diff site.yml`
- Terraform: `tfenv install && tfenv use`, `terraform fmt -check -recursive`, `terraform init -backend=false`, `terraform validate`, `tflint --init`, `tflint --recursive`, `trivy config .`, and `terraform test`

## Prompts to Improve Your App

- **"Set up tests for TypeScript, Python, Go, and Ansible packages in this monorepo"** — Full multi-language setup
- **"Add separate CI jobs for each language test suite"** — Better isolation
- **"Raise coverage thresholds and make CI fail when they drop"** — Quality gate
- **"Find and fix flaky tests in this package"** — Stability
- **"Add smoke tests that build our app with Docker Compose and run in GitHub Actions"** — Deployability check
- **"Add a smoke-test badge to the README"** — Status visibility
- **"Add one stable end-to-end test for the login flow"** — Critical-path E2E
- **"Add Ansible syntax-check and check-mode validation to CI"** — Infrastructure safety
- **"Add Terraform validate, tflint, Trivy config scanning, and terraform test to CI with tfenv version pinning"** — Terraform safety

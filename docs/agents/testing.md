# Testing Agent

The **testing** agent sets up and maintains test workflows for TypeScript, Python, and Go projects with sensible defaults and CI integration.

## What It Sets Up by Language

- **TypeScript/JavaScript**
  - Jest by default
  - Vitest for Vite-based projects
  - Coverage thresholds and CI test steps
- **Python**
  - pytest for test execution
  - pytest-cov for coverage reporting
- **Go**
  - `go test ./...` baseline
  - Coverage checks via `go test ./... -cover`

## What It Provides

- Repeatable test commands for local development
- CI checks for each language profile
- Coverage visibility with configurable thresholds

## Monorepo Usage

In multi-language monorepos, run tests per language package and report failures independently.

Recommended baseline commands:

- TypeScript: `pnpm test`
- Python: `pytest`
- Go: `go test ./...`

## Prompts to Improve Your App

- **"Set up tests for TypeScript, Python, and Go packages in this monorepo"** — Full multi-language setup
- **"Add separate CI jobs for each language test suite"** — Better isolation
- **"Raise coverage thresholds and make CI fail when they drop"** — Quality gate
- **"Find and fix flaky tests in this package"** — Stability

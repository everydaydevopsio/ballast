# Testing Agent

The **testing** agent sets up and maintains a test suite for TypeScript and JavaScript projects with sensible defaults and CI integration.

## What It Sets Up

- **Test runner** — Jest by default for TS/JS; Vitest for Vite projects
- **Coverage** — Default 50% threshold (lines, functions, branches, statements)
- **NPM scripts** — `test` and `test:coverage`
- **GitHub Actions** — A testing step in the build (or main CI) workflow

## What It Provides

- Consistent test and coverage setup across the team
- CI that runs tests (and optionally coverage) on every push/PR
- Fail CI when coverage drops below the configured threshold (default 50%)

## Prompts to Improve Your App

Use these prompts with your AI coding agent after installing the testing agent:

### Setup and Configuration

- **"Help me set up testing for this project"** — Full setup (Jest or Vitest, coverage, CI step)
- **"Add a test step to our build workflow"** — Ensure CI runs tests
- **"We use Vite—switch to Vitest"** — Use Vitest instead of Jest
- **"Raise coverage threshold to 80%"** — Stricter coverage

### Writing and Running Tests

- **"Write unit tests for this module"** — Add tests following project conventions
- **"Run tests with coverage and fix any failures"** — Run and fix
- **"Our test workflow is failing—help debug"** — CI troubleshooting

### General

- **"Show me how to run tests and coverage locally"** — Explain scripts
- **"Add coverage reporting to our existing GitHub Action"** — Add or update coverage step

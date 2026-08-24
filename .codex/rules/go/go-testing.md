<!-- ballast:rule id="go/testing" version="5.16.5" checksum="d41e060516206ed4749ad7a825cc10ac7b185f5f1f3174ff9fc72320cd68eee6" -->
# Go Testing Rules

These rules provide Go Testing Rules guidance for projects in this repository.

---
You are a Go testing specialist. Your role is to set up effective and maintainable tests.

## Repository Tool Policy

- Check `.rulesrc.json` `tools` before adding, installing, or running language tooling.
- Configured tools: go=go,gofumpt,golangci-lint; python=uv,pyenv; typescript=pnpm,corepack.
- For Python commands, prefer `uv run <command>` and `uv add ...` over bare `python`, `pip`, `pytest`, `ruff`, or `mypy` when the command is project-scoped.
- For TypeScript commands, prefer `pnpm`/`pnpm exec` over `npm`/`npx` when the command is project-scoped.

## TDD Process Discipline

Tooling setup and process discipline are separate responsibilities: configure Go test and coverage, and also use TDD for behavioral changes.

TDD is required for bug fixes, new features, refactors with behavioral impact, and contract changes:

1. Start from acceptance criteria in `PRD.md`, the linked issue, or the current task.
2. Write a failing test first that proves the requirement is not yet met.
3. Confirm the test fails for the right reason before implementation.
4. Implement the minimum change needed to make the failing test pass.
5. Refactor only after the relevant tests are green.
6. Proof of completion: record the previously failing test and the passing command.
7. Failure-path coverage: include error, edge, and misuse paths, not only the happy path.
8. Traceability: link tests to requirement IDs, issue IDs, or acceptance criteria in test names, comments, or PR evidence.

## Your Responsibilities

1. Use `go test` as the baseline test runner.
2. Add table-driven tests for core logic.
3. Make coverage part of the default test workflow, not an optional follow-up check.
4. Include coverage checks in CI and fail when coverage requirements are not met.
5. Detect existing unit, integration, API/service, and browser E2E frameworks before adding or replacing test tooling.
6. Keep tests deterministic and isolated.
7. When the project ships a runnable app or service, add smoke tests that build with the repo Dockerfile and run via `docker-compose.yaml`.
8. Make smoke tests emit explicit pass/fail output and exit non-zero on failure.
9. Add a GitHub Actions smoke-test workflow and a README badge for its status.
10. Add narrow end-to-end coverage for one critical workflow when the app exposes a real end-user path.

## Commands

- `go test ./...`
- `go test ./... -cover`
- Coverage gate (example): `go test ./... -covermode=atomic -coverprofile=coverage.out` plus a threshold check in CI
- a smoke-test command or script that validates the built container and prints explicit success/failure output

## Framework Detection

- Check markers for `go test`, integration build tags, `_integration_test.go` files, `httptest`, API/service tests, Selenium, chromedp, rod, agouti, Playwright, and existing browser harnesses.
- Extend the repo's established integration-test pattern before introducing a new framework.
- Preserve an existing browser E2E framework unless the user explicitly asks to migrate.

## Smoke and End-to-End Testing

- Use the repository's actual Dockerfile for the application under test.
- Use `docker-compose.yaml` to build and run the app with required services for smoke validation.
- Keep `docker-compose.local.yaml` for watch-mode local development, not CI smoke validation.
- For a web app, make the web smoke test start the real app and verify a live route or health endpoint.
- Ensure the smoke command clearly prints success or failure and exits non-zero when the smoke test fails.
- Add a dedicated GitHub Actions workflow such as `.github/workflows/smoke.yml` that builds with Docker Compose, runs the smoke command, and fails the workflow on errors.
- Add a README badge for the smoke workflow.
- For apps with real user-facing or API workflows, add one stable E2E path that validates a critical flow without making the suite flaky.
- Prefer Playwright only when Playwright markers already exist, or when the repo has a real browser application surface and no existing browser E2E framework.
- Do not add browser E2E tooling to library-only, CLI-only, infrastructure-only, or backend-only repositories without a user-facing browser surface.
- Run fast unit tests and targeted smoke checks during local work, put deterministic build/typecheck plus smoke checks in pre-push, and run full smoke/E2E gates in CI.

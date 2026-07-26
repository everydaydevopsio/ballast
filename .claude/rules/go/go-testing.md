# Go Testing Rules

These rules provide Go Testing Rules guidance for projects in this repository.

---
You are a Go testing specialist. Your role is to set up effective and maintainable tests.

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
- Ensure the smoke command clearly prints success or failure and exits non-zero when the smoke test fails.
- Add a dedicated GitHub Actions workflow such as `.github/workflows/smoke.yml` that builds with Docker Compose, runs the smoke command, and fails the workflow on errors.
- Add a README badge for the smoke workflow.
- For apps with real user-facing or API workflows, add one stable E2E path that validates a critical flow without making the suite flaky.
- Prefer Playwright only when Playwright markers already exist, or when the repo has a real browser application surface and no existing browser E2E framework.
- Do not add browser E2E tooling to library-only, CLI-only, infrastructure-only, or backend-only repositories without a user-facing browser surface.
- Run fast unit tests and targeted smoke checks during local work, put deterministic build/typecheck plus smoke checks in pre-push, and run full smoke/E2E gates in CI.

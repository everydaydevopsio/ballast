<!-- ballast:rule id="go/testing" version="5.21.0" checksum="0f3be24e8be7dd33cd4521e2076b5e3c18b8d49f1b1df88248171d47ace9b305" -->
# Go Testing Rules

Follow the shared `testing-process` rules for TDD discipline, framework detection policy, and smoke/E2E expectations. This rule owns only the language-specific concerns: runner selection, commands, framework markers, and the coverage gate.

## Your Responsibilities

1. Use `go test` as the baseline test runner.
2. Add table-driven tests for core logic.
3. Make coverage part of the default test workflow, not an optional follow-up check.
4. Include coverage checks in CI and fail when coverage requirements are not met.
5. Keep tests deterministic and isolated.

## Commands

- `go test ./...`
- `go test ./... -cover`
- Coverage gate (example): `go test ./... -covermode=atomic -coverprofile=coverage.out` plus a threshold check in CI
- a smoke-test command or script that validates the built container and prints explicit success/failure output

## Framework Markers

- Check markers for `go test`, integration build tags, `_integration_test.go` files, `httptest`, API/service tests, Selenium, chromedp, rod, agouti, Playwright, and existing browser harnesses.

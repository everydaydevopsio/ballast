<!-- ballast:rule id="go/testing" version="5.18.3" checksum="89209a13c630ae1b0d486bfa46f12805a91faf34343f8c1e71a4dd4d639b3a50" -->
# Go Testing Rules

These rules provide Go Testing Rules guidance for projects in this repository.

---
You are a Go testing specialist. Your role is to set up effective and maintainable tests.

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

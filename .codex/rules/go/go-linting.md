<!-- ballast:rule id="go/linting" version="5.18.3" checksum="488db029ad98d9aaa1f7c4b098eb19bce3c9f686b3e8e2751715e832d6374897" -->
# Go Linting Rules

These rules provide Go Linting Rules guidance for projects in this repository.

---
You are a Go linting specialist. Your role is to implement consistent linting and formatting for Go projects.

## Your Responsibilities

1. Enforce formatting with `gofmt`.
2. Configure `golangci-lint` with sane defaults.
3. Add CI lint checks.
4. Keep lint rules strict enough to prevent regressions while avoiding excessive noise.
5. Coordinate with the `git-hooks` rules when the repo should enforce local hook checks.

## Commands

- `gofmt -w .`
- `golangci-lint run`
- `go test ./...`

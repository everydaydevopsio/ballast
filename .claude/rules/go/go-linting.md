<!-- ballast:rule id="go/linting" version="5.18.3" checksum="6d168717b7339cb9d5848cc6ddceb9d0c234bfab545d5dfde3d18348ea9e81a4" -->
# Go Linting Rules

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

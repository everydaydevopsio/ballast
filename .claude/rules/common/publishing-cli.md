<!-- ballast:rule id="typescript/publishing/cli" version="5.19.0" checksum="3ffbcbd0bfe95a7677ffacf1473054ca49cb9cbe68eb3652eec2620bd06a387f" -->
# CLI Publishing Agent

## Goals

- Publish CLI binaries from validated release tags using the bump-and-tag pattern.
- Support Go CLIs via GoReleaser (binary archives + checksums), TypeScript/Node CLIs via npmjs, and Python CLIs via PyPI.

Follow the shared publishing release pattern (`publishing` rule) for the bump-and-tag workflow, version and tag rules, concurrency, and per-registry publish guidance. This rule adds only the artifact-specific requirements.

## Go CLIs: GoReleaser

Use GoReleaser to produce binary archives and checksums attached to GitHub Releases. Configure `.goreleaser.yaml` at the repo root (or the CLI subdirectory) with: top-level `version: 2` (required by GoReleaser v2), `project_name` and per-build `binary`/`main`, `ldflags` embedding the version (`-s -w -X main.version={{ .Version }}`), `CGO_ENABLED=0` for portable binaries, `goos` linux/darwin/windows and `goarch` amd64/arm64 (ignore windows/arm64), tar.gz archives with zip overrides for windows, and a distinct `checksum.name_template` when multiple GoReleaser configs coexist in one repo.

In the publish job: `actions/setup-go`, verify the binary builds, run `go test ./...`, then run `goreleaser/goreleaser-action@v7` with `distribution: goreleaser`, `version` pinned to an explicit stable release (e.g. `'v2.14.0'`, not `'~> v2'`), `args: release --clean`, and `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}` in the step env — the job needs `contents: write` and the token must be passed explicitly.

## CLI-Specific Requirements

- Add a packaged-command smoke test before publishing: install or execute the built artifact, check `<cli> --help` and `<cli> --version`, and run one representative command. Keep local packaged-command smoke checks fast, run them in pre-push when the packaged artifact can be built deterministically, and require them in CI before publish jobs.
- Ensure `<cli> --version` output matches the release tag.
- For Python CLIs, define console entry points in `pyproject.toml` under `[project.scripts]`; for Node CLIs, verify the packaged CLI starts from the built artifact.
- Publish checksums for downloadable binaries.
- Keep `README.md` installation instructions aligned with the actual release channel, and add a publish-workflow badge:
  `[![Release](https://github.com/OWNER/REPO/actions/workflows/publish-cli.yml/badge.svg)](https://github.com/OWNER/REPO/actions/workflows/publish-cli.yml)`

## When to Apply

- When a repository publishes a CLI or command-line tool for direct installation by end users.
- When the CLI is written in Go, TypeScript/Node, or Python and needs a turn-key release workflow with semver bumping.

#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export UV_CACHE_DIR="${TMPDIR:-/tmp}/ballast-uv-cache"
mkdir -p "${UV_CACHE_DIR}"

cd "${repo_root}/packages/ballast-typescript"
pnpm test

cd "${repo_root}/packages/ballast-python"
uv run python -m unittest discover -s tests

cd "${repo_root}/packages/ballast-go"
go test ./cmd/ballast-go

cd "${repo_root}/cli/ballast"
go test ./...

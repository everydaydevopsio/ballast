#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# shellcheck source=./e2e/helpers.sh
source "${REPO_ROOT}/scripts/e2e/helpers.sh"
export BALLAST_E2E_TYPESCRIPT=1
setup_ballast_e2e

assert_husky_rule() {
  local path="$1"
  assert_contains "Use Husky for TypeScript-only repositories." "${path}"
  assert_contains "lint-staged" "${path}"
  assert_contains ".yaml" "${path}"
  assert_contains ".yml" "${path}"
  assert_contains ".husky/pre-push" "${path}"
  assert_contains "package-manager test command" "${path}"
  assert_contains "build or typecheck" "${path}"
  assert_not_contains ".pre-commit-config.yaml" "${path}"
  assert_not_contains "pre-commit install" "${path}"
}

assert_pre_commit_rule() {
  local path="$1"
  assert_contains ".pre-commit-config.yaml" "${path}"
  assert_contains "pre-commit install --hook-type pre-push" "${path}"
  assert_not_contains "Use Husky for TypeScript-only repositories." "${path}"
  assert_not_contains "lint-staged" "${path}"
  assert_not_contains ".husky/pre-push" "${path}"
}

TS_ONLY_PROJECT="${WORKDIR}/typescript-only"
mkdir -p "${TS_ONLY_PROJECT}"
cat > "${TS_ONLY_PROJECT}/package.json" <<'JSON'
{
  "name": "typescript-only",
  "private": true
}
JSON

(
  cd "${TS_ONLY_PROJECT}"
  ballast-typescript install --target codex,claude --agent linting --yes >/dev/null
)

assert_husky_rule "${TS_ONLY_PROJECT}/.codex/rules/git-hooks.md"
assert_husky_rule "${TS_ONLY_PROJECT}/.claude/rules/git-hooks.md"

MULTI_LANGUAGE_PROJECT="${WORKDIR}/multi-language"
mkdir -p "${MULTI_LANGUAGE_PROJECT}"
cat > "${MULTI_LANGUAGE_PROJECT}/package.json" <<'JSON'
{
  "name": "multi-language",
  "private": true,
  "scripts": {
    "prepare": "husky"
  },
  "devDependencies": {
    "husky": "^9.1.7"
  }
}
JSON
cat > "${MULTI_LANGUAGE_PROJECT}/.rulesrc.json" <<'JSON'
{
  "targets": ["codex", "claude"],
  "agents": ["linting"],
  "skills": [],
  "languages": ["typescript", "python"],
  "paths": {
    "typescript": ["."],
    "python": ["services/api"]
  }
}
JSON

(
  cd "${MULTI_LANGUAGE_PROJECT}"
  ballast-typescript install --target codex,claude --agent linting --yes >/dev/null
)

assert_pre_commit_rule "${MULTI_LANGUAGE_PROJECT}/.codex/rules/git-hooks.md"
assert_pre_commit_rule "${MULTI_LANGUAGE_PROJECT}/.claude/rules/git-hooks.md"

echo "PASS: git-hooks-mode-policy-e2e"

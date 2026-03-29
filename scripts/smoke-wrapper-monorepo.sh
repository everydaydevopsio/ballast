#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

make_fixture() {
  local monorepo="$1"

  mkdir -p "${monorepo}/apps/frontend"
  mkdir -p "${monorepo}/services/api"
  mkdir -p "${monorepo}/tools/worker"

  cp -R "${REPO_ROOT}/examples/smoke/typescript-sample/." "${monorepo}/apps/frontend/"
  cp -R "${REPO_ROOT}/examples/smoke/python-sample/." "${monorepo}/services/api/"
  cp -R "${REPO_ROOT}/examples/smoke/go-sample/." "${monorepo}/tools/worker/"

  cat > "${monorepo}/package.json" <<'EOF'
{
  "name": "ballast-wrapper-monorepo",
  "private": true,
  "packageManager": "pnpm@10.27.0"
}
EOF
}

verify_common_rules() {
  local monorepo="$1"

  test -f "${monorepo}/.cursor/rules/common/local-dev-env.mdc"
  test -f "${monorepo}/.cursor/rules/common/cicd.mdc"
  test -f "${monorepo}/.cursor/rules/common/observability.mdc"
  test -f "${monorepo}/.claude/rules/common/local-dev-env.md"
  test -f "${monorepo}/.opencode/common/local-dev-env.md"
  test -f "${monorepo}/.codex/rules/common/local-dev-env.md"
}

verify_language_rules() {
  local monorepo="$1"

  test -f "${monorepo}/.cursor/rules/typescript/typescript-linting.mdc"
  test -f "${monorepo}/.cursor/rules/typescript/typescript-logging.mdc"
  test -f "${monorepo}/.cursor/rules/typescript/typescript-testing.mdc"

  test -f "${monorepo}/.cursor/rules/python/python-linting.mdc"
  test -f "${monorepo}/.cursor/rules/python/python-logging.mdc"
  test -f "${monorepo}/.cursor/rules/python/python-testing.mdc"

  test -f "${monorepo}/.cursor/rules/go/go-linting.mdc"
  test -f "${monorepo}/.cursor/rules/go/go-logging.mdc"
  test -f "${monorepo}/.cursor/rules/go/go-testing.mdc"

  test -f "${monorepo}/.claude/rules/typescript/typescript-linting.md"
  test -f "${monorepo}/.claude/rules/python/python-linting.md"
  test -f "${monorepo}/.claude/rules/go/go-linting.md"

  test -f "${monorepo}/.opencode/typescript/typescript-linting.md"
  test -f "${monorepo}/.opencode/python/python-linting.md"
  test -f "${monorepo}/.opencode/go/go-linting.md"

  test -f "${monorepo}/.codex/rules/typescript/typescript-linting.md"
  test -f "${monorepo}/.codex/rules/python/python-linting.md"
  test -f "${monorepo}/.codex/rules/go/go-linting.md"
}

verify_rulesrc() {
  local monorepo="$1"

  grep -q '"targets"' "${monorepo}/.rulesrc.json"
  grep -q '"cursor"' "${monorepo}/.rulesrc.json"
  grep -q '"claude"' "${monorepo}/.rulesrc.json"
  grep -q '"opencode"' "${monorepo}/.rulesrc.json"
  grep -q '"codex"' "${monorepo}/.rulesrc.json"
  grep -q '"languages"' "${monorepo}/.rulesrc.json"
  grep -q '"typescript"' "${monorepo}/.rulesrc.json"
  grep -q '"python"' "${monorepo}/.rulesrc.json"
  grep -q '"go"' "${monorepo}/.rulesrc.json"
  grep -q '"apps/frontend"' "${monorepo}/.rulesrc.json"
  grep -q '"services/api"' "${monorepo}/.rulesrc.json"
  grep -q '"tools/worker"' "${monorepo}/.rulesrc.json"
}

verify_support_files() {
  local monorepo="$1"

  test -f "${monorepo}/CLAUDE.md"
  test -f "${monorepo}/AGENTS.md"
  grep -q '`.claude/rules/typescript/typescript-linting.md`' "${monorepo}/CLAUDE.md"
  grep -q '`.codex/rules/typescript/typescript-linting.md`' "${monorepo}/AGENTS.md"
}

main() {
  local monorepo="${WORKDIR}/ballast-wrapper-monorepo"
  make_fixture "${monorepo}"

  (
    cd "${monorepo}"
    ballast install --target cursor,claude,opencode,codex --all --all-skills --yes
  )

  verify_common_rules "${monorepo}"
  verify_language_rules "${monorepo}"
  verify_rulesrc "${monorepo}"
  verify_support_files "${monorepo}"

  echo "Ballast wrapper monorepo smoke test passed."
}

main "$@"

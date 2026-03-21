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
}

verify_rulesrc() {
  local monorepo="$1"

  grep -q '"languages"' "${monorepo}/.rulesrc.json"
  grep -q '"typescript"' "${monorepo}/.rulesrc.json"
  grep -q '"python"' "${monorepo}/.rulesrc.json"
  grep -q '"go"' "${monorepo}/.rulesrc.json"
  grep -q '"apps/frontend"' "${monorepo}/.rulesrc.json"
  grep -q '"services/api"' "${monorepo}/.rulesrc.json"
  grep -q '"tools/worker"' "${monorepo}/.rulesrc.json"
}

main() {
  local monorepo="${WORKDIR}/ballast-wrapper-monorepo"
  make_fixture "${monorepo}"

  (
    cd "${monorepo}"
    ballast install --target cursor --all --yes
  )

  verify_common_rules "${monorepo}"
  verify_language_rules "${monorepo}"
  verify_rulesrc "${monorepo}"

  echo "Ballast wrapper monorepo smoke test passed."
}

main "$@"

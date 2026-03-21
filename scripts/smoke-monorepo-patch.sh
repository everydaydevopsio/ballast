#!/usr/bin/env bash
set -euo pipefail

EXAMPLES_ROOT="${1:-../ballast-examples}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ ! -d "${EXAMPLES_ROOT}/typescript-sample" ]]; then
  echo "ballast-examples repo not found at ${EXAMPLES_ROOT}" >&2
  exit 1
fi

WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

build_monorepo_fixture() {
  local monorepo="$1"

  mkdir -p "${monorepo}/apps/typescript-sample"
  mkdir -p "${monorepo}/services/python-sample"
  mkdir -p "${monorepo}/tools/go-sample"

  cp -R "${EXAMPLES_ROOT}/typescript-sample/." "${monorepo}/apps/typescript-sample/"
  cp -R "${EXAMPLES_ROOT}/python-sample/." "${monorepo}/services/python-sample/"
  cp -R "${EXAMPLES_ROOT}/go-sample/." "${monorepo}/tools/go-sample/"

  cat > "${monorepo}/package.json" <<'EOF'
{
  "name": "ballast-examples-monorepo",
  "private": true,
  "packageManager": "pnpm@10.27.0"
}
EOF

  cat > "${monorepo}/pnpm-workspace.yaml" <<'EOF'
packages:
  - apps/*
EOF
}

seed_existing_rule() {
  local monorepo="$1"
  local rule_name="$2"

  mkdir -p "${monorepo}/.cursor/rules"
  cat > "${monorepo}/.cursor/rules/${rule_name}" <<'EOF'
---
description: Team customized linting rules
alwaysApply: true
---

Team intro.

## Your Responsibilities

Keep team-specific wording.

## Team Overrides

Keep this note.
EOF
}

verify_rule() {
  local rule_file="$1"
  local expected_section="$2"

  grep -q "description: Team customized linting rules" "${rule_file}"
  grep -q "alwaysApply: true" "${rule_file}"
  grep -q "Keep team-specific wording." "${rule_file}"
  grep -q "## Team Overrides" "${rule_file}"
  grep -q "${expected_section}" "${rule_file}"
}

run_typescript_smoke() {
  local monorepo="${WORKDIR}/typescript-monorepo"
  build_monorepo_fixture "${monorepo}"
  seed_existing_rule "${monorepo}" "typescript-linting.mdc"

  (
    cd "${monorepo}"
    node "${REPO_ROOT}/packages/ballast-typescript/dist/cli.js" install --target cursor --agent linting --patch --yes
  )

  verify_rule "${monorepo}/.cursor/rules/typescript-linting.mdc" "## When Completed"
  echo "TypeScript monorepo patch smoke test passed."
}

run_python_smoke() {
  local monorepo="${WORKDIR}/python-monorepo"
  build_monorepo_fixture "${monorepo}"
  seed_existing_rule "${monorepo}" "python-linting.mdc"

  (
    cd "${monorepo}"
    PYTHONPATH="${REPO_ROOT}/packages/ballast-python" python3 -m ballast install --language python --target cursor --agent linting --patch --yes
  )

  verify_rule "${monorepo}/.cursor/rules/python-linting.mdc" "## Baseline Tooling"
  echo "Python monorepo patch smoke test passed."
}

run_go_smoke() {
  local monorepo="${WORKDIR}/go-monorepo"
  build_monorepo_fixture "${monorepo}"
  seed_existing_rule "${monorepo}" "go-linting.mdc"

  (
    cd "${monorepo}"
    "${REPO_ROOT}/.ci/bin/ballast-go" install --language go --target cursor --agent linting --patch --yes
  )

  verify_rule "${monorepo}/.cursor/rules/go-linting.mdc" "## Commands"
  echo "Go monorepo patch smoke test passed."
}

run_typescript_smoke
run_python_smoke
run_go_smoke

echo "Monorepo patch smoke tests passed for all package apps."

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
  local target="$2"
  local rule_name="$3"
  local rule_dir=""

  case "${target}" in
    claude) rule_dir="${monorepo}/.claude/rules" ;;
    codex) rule_dir="${monorepo}/.codex/rules" ;;
    cursor) rule_dir="${monorepo}/.cursor/rules" ;;
    gemini) rule_dir="${monorepo}/.gemini/rules" ;;
    opencode) rule_dir="${monorepo}/.opencode" ;;
    *) echo "Unsupported target: ${target}" >&2; exit 1 ;;
  esac

  mkdir -p "${rule_dir}"
  cat > "${rule_dir}/${rule_name}" <<'EOF'
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

seed_existing_support_file() {
  local monorepo="$1"
  local target="$2"
  local support_file=""
  local existing_path=""

  case "${target}" in
    claude)
      support_file="${monorepo}/CLAUDE.md"
      existing_path='.claude/rules/old.md'
      ;;
    codex)
      support_file="${monorepo}/AGENTS.md"
      existing_path='.codex/rules/old.md'
      ;;
    gemini)
      support_file="${monorepo}/GEMINI.md"
      existing_path='.gemini/rules/old.md'
      ;;
    *)
      return
      ;;
  esac

  cat > "${support_file}" <<EOF
# $(basename "${support_file}")


## Team Notes

Keep this section.

## Installed agent rules

Read and follow these rule files in \`${existing_path%/old.md}/\` when they apply:

- \`${existing_path}\` — Old rule
EOF
}

rule_path() {
  local monorepo="$1"
  local target="$2"
  local rule_name="$3"

  case "${target}" in
    claude) echo "${monorepo}/.claude/rules/${rule_name}" ;;
    codex) echo "${monorepo}/.codex/rules/${rule_name}" ;;
    cursor) echo "${monorepo}/.cursor/rules/${rule_name}" ;;
    gemini) echo "${monorepo}/.gemini/rules/${rule_name}" ;;
    opencode) echo "${monorepo}/.opencode/${rule_name}" ;;
    *) echo "Unsupported target: ${target}" >&2; exit 1 ;;
  esac
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

verify_force_overwrite() {
  local rule_file="$1"
  local expected_section="$2"

  ! grep -q "Team customized linting rules" "${rule_file}"
  ! grep -q "Keep team-specific wording." "${rule_file}"
  grep -q "${expected_section}" "${rule_file}"
}

verify_support_file() {
  local monorepo="$1"
  local target="$2"
  local rule_basename="$3"
  local support_file=""
  local expected_path=""
  local old_path=""

  case "${target}" in
    claude)
      support_file="${monorepo}/CLAUDE.md"
      expected_path=".claude/rules/${rule_basename}"
      old_path=".claude/rules/old.md"
      ;;
    codex)
      support_file="${monorepo}/AGENTS.md"
      expected_path=".codex/rules/${rule_basename}"
      old_path=".codex/rules/old.md"
      ;;
    gemini)
      support_file="${monorepo}/GEMINI.md"
      expected_path=".gemini/rules/${rule_basename}"
      old_path=".gemini/rules/old.md"
      ;;
    *)
      return
      ;;
  esac

  grep -q "## Team Notes" "${support_file}"
  grep -q "Keep this section." "${support_file}"
  grep -Fq "\`${expected_path}\`" "${support_file}"
  ! grep -Fq "\`${old_path}\`" "${support_file}"
}

verify_forced_support_file() {
  local monorepo="$1"
  local target="$2"
  local rule_basename="$3"
  local support_file=""
  local expected_path=""
  local old_path=""

  case "${target}" in
    claude)
      support_file="${monorepo}/CLAUDE.md"
      expected_path=".claude/rules/${rule_basename}"
      old_path=".claude/rules/old.md"
      ;;
    codex)
      support_file="${monorepo}/AGENTS.md"
      expected_path=".codex/rules/${rule_basename}"
      old_path=".codex/rules/old.md"
      ;;
    gemini)
      support_file="${monorepo}/GEMINI.md"
      expected_path=".gemini/rules/${rule_basename}"
      old_path=".gemini/rules/old.md"
      ;;
    *)
      return
      ;;
  esac

  grep -Fq "\`${expected_path}\`" "${support_file}"
  ! grep -Fq "\`${old_path}\`" "${support_file}"
}

run_typescript_smoke() {
  local target
  local monorepo
  local rule_name
  local rule_file
  for target in cursor opencode claude codex gemini; do
    monorepo="${WORKDIR}/typescript-monorepo-${target}"
    build_monorepo_fixture "${monorepo}"
    rule_name="typescript-linting.md"
    if [[ "${target}" == "cursor" ]]; then
      rule_name="typescript-linting.mdc"
    fi
    seed_existing_rule "${monorepo}" "${target}" "${rule_name}"
    seed_existing_support_file "${monorepo}" "${target}"

    (
      cd "${monorepo}"
      node "${REPO_ROOT}/packages/ballast-typescript/dist/cli.js" install --target "${target}" --agent linting --patch --yes
    )

    rule_file="$(rule_path "${monorepo}" "${target}" "${rule_name}")"
    verify_rule "${rule_file}" "## When Completed"
    verify_support_file "${monorepo}" "${target}" "${rule_name}"

    (
      cd "${monorepo}"
      node "${REPO_ROOT}/packages/ballast-typescript/dist/cli.js" install --target "${target}" --agent linting --force --yes
    )

    verify_force_overwrite "${rule_file}" "## When Completed"
    verify_forced_support_file "${monorepo}" "${target}" "${rule_name}"
  done
  echo "TypeScript monorepo patch smoke test passed for all targets."
}

run_python_smoke() {
  local target
  local monorepo
  local rule_name
  local rule_file
  for target in cursor opencode claude codex gemini; do
    monorepo="${WORKDIR}/python-monorepo-${target}"
    build_monorepo_fixture "${monorepo}"
    rule_name="python-linting.md"
    if [[ "${target}" == "cursor" ]]; then
      rule_name="python-linting.mdc"
    fi
    seed_existing_rule "${monorepo}" "${target}" "${rule_name}"
    seed_existing_support_file "${monorepo}" "${target}"

    (
      cd "${monorepo}"
      PYTHONPATH="${REPO_ROOT}/packages/ballast-python" python3 -m ballast install --language python --target "${target}" --agent linting --patch --yes
    )

    rule_file="$(rule_path "${monorepo}" "${target}" "${rule_name}")"
    verify_rule "${rule_file}" "## Baseline Tooling"
    verify_support_file "${monorepo}" "${target}" "${rule_name}"

    (
      cd "${monorepo}"
      PYTHONPATH="${REPO_ROOT}/packages/ballast-python" python3 -m ballast install --language python --target "${target}" --agent linting --force --yes
    )

    verify_force_overwrite "${rule_file}" "## Baseline Tooling"
    verify_forced_support_file "${monorepo}" "${target}" "${rule_name}"
  done
  echo "Python monorepo patch smoke test passed for all targets."
}

run_go_smoke() {
  local target
  local monorepo
  local rule_name
  local rule_file
  for target in cursor opencode claude codex; do
    monorepo="${WORKDIR}/go-monorepo-${target}"
    build_monorepo_fixture "${monorepo}"
    rule_name="go-linting.md"
    if [[ "${target}" == "cursor" ]]; then
      rule_name="go-linting.mdc"
    fi
    seed_existing_rule "${monorepo}" "${target}" "${rule_name}"
    seed_existing_support_file "${monorepo}" "${target}"

    (
      cd "${monorepo}"
      "${REPO_ROOT}/.ci/bin/ballast-go" install --language go --target "${target}" --agent linting --patch --yes
    )

    rule_file="$(rule_path "${monorepo}" "${target}" "${rule_name}")"
    verify_rule "${rule_file}" "## Commands"
    verify_support_file "${monorepo}" "${target}" "${rule_name}"

    (
      cd "${monorepo}"
      "${REPO_ROOT}/.ci/bin/ballast-go" install --language go --target "${target}" --agent linting --force --yes
    )

    verify_force_overwrite "${rule_file}" "## Commands"
    verify_forced_support_file "${monorepo}" "${target}" "${rule_name}"
  done
  echo "Go monorepo patch smoke test passed for all targets."
}

run_typescript_smoke
run_python_smoke
run_go_smoke

echo "Monorepo patch smoke tests passed for all package apps."

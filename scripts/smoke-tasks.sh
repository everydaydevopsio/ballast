#!/usr/bin/env bash
# smoke-tasks.sh
#
# Verifies that the tasks agent installs correctly and that:
#   - Installing with --yes defaults taskSystem to "github"
#   - Installing with --task-system <value> uses the specified system
#   - taskSystem is persisted in .rulesrc.json
#   - taskSystem is preserved when reinstalling other agents
#   - Rule files contain the resolved task system name
#
# Usage:
#   ./scripts/smoke-tasks.sh [<repo-root>]
#
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
CLI="node ${REPO_ROOT}/packages/ballast-typescript/dist/cli.js"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT
TARGETS=(cursor claude opencode codex)

# ─── helpers ─────────────────────────────────────────────────────────────────

pass() { echo "  PASS: $*"; }
fail() { echo "  FAIL: $*" >&2; exit 1; }

rulesrc_contents() {
  local rulesrc="$1"
  if [ -r "${rulesrc}" ]; then
    cat "${rulesrc}" 2>/dev/null || true
    return
  fi
  printf '<unreadable: %s>' "${rulesrc}"
}

assert_rulesrc_field() {
  local dir="$1"
  local field="$2"
  local value="$3"
  local rulesrc="${dir}/.rulesrc.json"
  grep -q "\"${field}\"" "${rulesrc}" || \
    fail "Expected field '${field}' in ${rulesrc}. Content: $(rulesrc_contents "${rulesrc}")"
  grep -q "\"${value}\"" "${rulesrc}" || \
    fail "Expected value '${value}' for field '${field}' in ${rulesrc}. Content: $(rulesrc_contents "${rulesrc}")"
}

assert_rule_file_contains() {
  local file="$1"
  local text="$2"
  grep -qF "${text}" "${file}" || \
    fail "Expected '${text}' in ${file}"
}

make_project() {
  local dir="$1"
  mkdir -p "${dir}"
  echo '{}' > "${dir}/package.json"
}

ballast() {
  local dir="$1"; shift
  (cd "${dir}" && ${CLI} "$@")
}

# ─── helper: resolve rule file path for tasks agent by target ─────────────────

tasks_rule_file() {
  local dir="$1"
  local target="$2"
  local suffix="$3"  # "task-system" or "todo"
  case "${target}" in
    cursor)   echo "${dir}/.cursor/rules/tasks-${suffix}.mdc" ;;
    claude)   echo "${dir}/.claude/rules/tasks-${suffix}.md" ;;
    opencode) echo "${dir}/.opencode/tasks-${suffix}.md" ;;
    codex)    echo "${dir}/.codex/rules/tasks-${suffix}.md" ;;
  esac
}

# ─── test: tasks agent with --yes defaults to github ─────────────────────────

test_tasks_default_github() {
  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/tasks-default-${target}"
    make_project "${dir}"

    echo ""
    echo "▶ test_tasks_default_github (${target})"

    ballast "${dir}" install --target "${target}" --agent tasks --yes
    assert_rulesrc_field "${dir}" "taskSystem" "github"
    pass "step 1 (${target}): taskSystem=github saved to .rulesrc.json"

    local rule_file
    rule_file="$(tasks_rule_file "${dir}" "${target}" "task-system")"
    [ -f "${rule_file}" ] || \
      fail "Expected task-system rule file at ${rule_file}"
    assert_rule_file_contains "${rule_file}" "github"
    pass "step 2 (${target}): task-system rule file contains 'github'"
  done

  echo "  ✓ test_tasks_default_github passed for all targets"
}

# ─── test: tasks agent with --task-system jira ───────────────────────────────

test_tasks_jira() {
  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/tasks-jira-${target}"
    make_project "${dir}"

    echo ""
    echo "▶ test_tasks_jira (${target})"

    ballast "${dir}" install --target "${target}" --agent tasks --task-system jira --yes
    assert_rulesrc_field "${dir}" "taskSystem" "jira"
    pass "step 1 (${target}): taskSystem=jira saved to .rulesrc.json"

    local rule_file
    rule_file="$(tasks_rule_file "${dir}" "${target}" "task-system")"
    [ -f "${rule_file}" ] || \
      fail "Expected task-system rule file at ${rule_file}"
    assert_rule_file_contains "${rule_file}" "jira"
    pass "step 2 (${target}): task-system rule file contains 'jira'"
  done

  echo "  ✓ test_tasks_jira passed for all targets"
}

# ─── test: tasks agent with --task-system linear ─────────────────────────────

test_tasks_linear() {
  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/tasks-linear-${target}"
    make_project "${dir}"

    echo ""
    echo "▶ test_tasks_linear (${target})"

    ballast "${dir}" install --target "${target}" --agent tasks --task-system linear --yes
    assert_rulesrc_field "${dir}" "taskSystem" "linear"
    pass "step 1 (${target}): taskSystem=linear saved to .rulesrc.json"

    local rule_file
    rule_file="$(tasks_rule_file "${dir}" "${target}" "task-system")"
    [ -f "${rule_file}" ] || \
      fail "Expected task-system rule file at ${rule_file}"
    assert_rule_file_contains "${rule_file}" "linear"
    pass "step 2 (${target}): task-system rule file contains 'linear'"
  done

  echo "  ✓ test_tasks_linear passed for all targets"
}

# ─── test: todo rule file is installed alongside task-system ─────────────────

test_tasks_todo_installed() {
  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/tasks-todo-${target}"
    make_project "${dir}"

    echo ""
    echo "▶ test_tasks_todo_installed (${target})"

    ballast "${dir}" install --target "${target}" --agent tasks --yes

    local todo_file
    todo_file="$(tasks_rule_file "${dir}" "${target}" "todo")"
    [ -f "${todo_file}" ] || \
      fail "Expected todo rule file at ${todo_file}"
    assert_rule_file_contains "${todo_file}" "TODO"
    pass "(${target}): todo rule file installed and contains 'TODO'"
  done

  echo "  ✓ test_tasks_todo_installed passed for all targets"
}

# ─── test: taskSystem preserved when reinstalling other agents ────────────────

test_task_system_preserved_on_reinstall() {
  local target
  for target in "${TARGETS[@]}"; do
    local dir="${WORKDIR}/tasks-preserved-${target}"
    make_project "${dir}"

    echo ""
    echo "▶ test_task_system_preserved_on_reinstall (${target})"

    ballast "${dir}" install --target "${target}" --agent tasks --task-system jira --yes
    assert_rulesrc_field "${dir}" "taskSystem" "jira"
    pass "step 1 (${target}): installed tasks with jira"

    ballast "${dir}" install --target "${target}" --agent linting --yes
    assert_rulesrc_field "${dir}" "taskSystem" "jira"
    pass "step 2 (${target}): reinstalling linting does not drop taskSystem=jira"
  done

  echo "  ✓ test_task_system_preserved_on_reinstall passed for all targets"
}

# ─── run all tests ────────────────────────────────────────────────────────────

main() {
  echo "Running tasks smoke tests..."
  echo "CLI: ${CLI}"
  echo ""

  test_tasks_default_github
  test_tasks_jira
  test_tasks_linear
  test_tasks_todo_installed
  test_task_system_preserved_on_reinstall

  echo ""
  echo "All tasks smoke tests passed."
}

main "$@"

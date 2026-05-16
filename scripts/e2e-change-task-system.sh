#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT
BALLAST_E2E_TYPESCRIPT=1

# shellcheck source=./e2e/helpers.sh
source "${REPO_ROOT}/scripts/e2e/helpers.sh"
setup_ballast_e2e

PROJECT="${WORKDIR}/change-task-system"
create_monorepo_fixture "${PROJECT}"
add_typescript_profile "${PROJECT}"

cat > "${PROJECT}/.rulesrc.json" <<'EOF'
{
  "targets": ["claude", "codex"],
  "agents": ["linting", "tasks"],
  "skills": [],
  "languages": ["typescript", "python", "go"],
  "paths": {
    "typescript": ["apps/web"],
    "python": ["services/api"],
    "go": ["tools/worker"]
  },
  "ballastVersion": "5.10.2",
  "taskSystem": "jira"
}
EOF

materialize_saved_install "${PROJECT}"

(
  cd "${PROJECT}"
  ballast install --agent tasks --task-system linear --yes >/dev/null
)

assert_contains '"taskSystem": "linear"' "${PROJECT}/.rulesrc.json"
assert_not_contains '"taskSystem": "jira"' "${PROJECT}/.rulesrc.json"
assert_contains 'linear' "${PROJECT}/.codex/rules/common/tasks-task-system.md"
assert_not_contains 'Use jira as the system of record for work items.' "${PROJECT}/.codex/rules/common/tasks-task-system.md"
assert_not_contains 'This repository uses **jira** as the system of record' "${PROJECT}/.codex/rules/common/tasks-task-system.md"
assert_file_exists "${PROJECT}/.codex/rules/python/python-linting.md"

echo "PASS: change-task-system-e2e"

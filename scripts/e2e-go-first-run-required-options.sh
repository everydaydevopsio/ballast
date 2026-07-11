#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# shellcheck source=./e2e/helpers.sh
source "${REPO_ROOT}/scripts/e2e/helpers.sh"
setup_ballast_e2e

PROJECT="${WORKDIR}/go-first-run-required-options"
mkdir -p "${PROJECT}"

cat > "${PROJECT}/go.mod" <<'EOF'
module example.com/ballast-go-required-options

go 1.24
EOF

OUTPUT="$(
  cd "${PROJECT}"
  printf 'linear\nserverless\n' | ballast-go install --target codex --language go --all
)"

assert_contains '"taskSystem": "linear"' "${PROJECT}/.rulesrc.json"
assert_contains '"deploymentModel": "serverless"' "${PROJECT}/.rulesrc.json"
assert_file_exists "${PROJECT}/.codex/rules/tasks-task-system.md"
assert_contains 'linear' "${PROJECT}/.codex/rules/tasks-task-system.md"
assert_not_contains '{{taskSystem}}' "${PROJECT}/.codex/rules/tasks-task-system.md"
assert_contains 'Serverless deployment model:' "${PROJECT}/.codex/rules/publishing-apps.md"
assert_doctor_contains "${OUTPUT}" "Task system for tasks"
assert_doctor_contains "${OUTPUT}" "Deployment model for publishing apps"

echo "PASS: go-first-run-required-options-e2e"

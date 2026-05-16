#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# shellcheck source=./e2e/helpers.sh
source "${REPO_ROOT}/scripts/e2e/helpers.sh"
setup_ballast_e2e

PROJECT="${WORKDIR}/add-agent-existing-install"
create_monorepo_fixture "${PROJECT}"

cat > "${PROJECT}/.rulesrc.json" <<'EOF'
{
  "targets": ["claude", "codex"],
  "agents": ["linting"],
  "skills": [],
  "languages": ["python", "go"],
  "paths": {
    "python": ["services/api"],
    "go": ["tools/worker"]
  },
  "ballastVersion": "5.10.2"
}
EOF

materialize_saved_install "${PROJECT}"

(
  cd "${PROJECT}"
  ballast install --agent docs --yes >/dev/null
)

assert_contains '"linting"' "${PROJECT}/.rulesrc.json"
assert_contains '"docs"' "${PROJECT}/.rulesrc.json"
assert_file_exists "${PROJECT}/.codex/rules/common/docs.md"
assert_file_exists "${PROJECT}/.claude/rules/common/docs.md"
assert_file_exists "${PROJECT}/.codex/rules/python/python-linting.md"
assert_contains '`.codex/rules/common/docs.md`' "${PROJECT}/AGENTS.md"
assert_contains '`.codex/rules/python/python-linting.md`' "${PROJECT}/AGENTS.md"

echo "PASS: add-agent-existing-install-e2e"

#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# shellcheck source=./e2e/helpers.sh
source "${REPO_ROOT}/scripts/e2e/helpers.sh"
setup_ballast_e2e

PROJECT="${WORKDIR}/remove-agent-existing-install"
create_monorepo_fixture "${PROJECT}"

cat > "${PROJECT}/.rulesrc.json" <<'EOF'
{
  "targets": ["claude", "codex"],
  "agents": ["linting", "docs"],
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

(
  cd "${PROJECT}"
  ballast install --refresh-config >/dev/null
)

assert_file_absent "${PROJECT}/.codex/rules/common/docs.md"
assert_file_absent "${PROJECT}/.claude/rules/common/docs.md"
assert_not_contains '`.codex/rules/common/docs.md`' "${PROJECT}/AGENTS.md"
assert_not_contains '`.claude/rules/common/docs.md`' "${PROJECT}/CLAUDE.md"
assert_file_exists "${PROJECT}/.codex/rules/python/python-linting.md"
assert_file_exists "${PROJECT}/.claude/rules/python/python-linting.md"

echo "PASS: remove-agent-existing-install-e2e"

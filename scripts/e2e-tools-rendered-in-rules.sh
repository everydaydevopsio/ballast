#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# shellcheck source=./e2e/helpers.sh
source "${REPO_ROOT}/scripts/e2e/helpers.sh"
setup_ballast_e2e

PROJECT="${WORKDIR}/tools-rendered-in-rules"
mkdir -p "${PROJECT}/src/example"

cat > "${PROJECT}/pyproject.toml" <<'EOF'
[project]
name = "tools-rendered-in-rules"
version = "0.0.0"
requires-python = ">=3.11"
EOF

cat > "${PROJECT}/src/example/__init__.py" <<'EOF'
"""Example package."""
EOF

cat > "${PROJECT}/.rulesrc.json" <<'EOF'
{
  "targets": ["claude", "codex"],
  "agents": ["testing"],
  "skills": [],
  "languages": ["python"],
  "paths": {
    "python": ["."]
  },
  "tools": {
    "python": ["uv", "pyenv"]
  },
  "ballastVersion": "5.16.5"
}
EOF

(
  cd "${PROJECT}"
  ballast-go install --language python --target claude --target codex --agent testing --yes >/dev/null
)

for rule in \
  "${PROJECT}/.codex/rules/python-testing.md" \
  "${PROJECT}/.claude/rules/python-testing.md"
do
  assert_file_exists "${rule}"
  assert_contains "## Repository Tool Policy" "${rule}"
  assert_contains "python=uv,pyenv" "${rule}"
  assert_contains 'uv run <command>' "${rule}"
done

assert_contains '"tools"' "${PROJECT}/.rulesrc.json"
assert_contains '"uv"' "${PROJECT}/.rulesrc.json"

echo "PASS: tools-rendered-in-rules-e2e"

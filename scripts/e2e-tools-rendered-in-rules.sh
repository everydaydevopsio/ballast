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
  "targets": ["claude", "codex", "cursor", "opencode"],
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
  ballast-go install --language python --target claude --target codex \
    --target cursor --target opencode --agent testing --yes >/dev/null
)

# Targets that ship a manifest carry the policy there, once, and deliberately
# keep it out of every rule file. See test_manifest_targets_omit_tools_policy_in_rules
# and test_manifests_include_tools_policy_once in the Python backend.
for rule in \
  "${PROJECT}/.codex/rules/python-testing.md" \
  "${PROJECT}/.claude/rules/python-testing.md"
do
  assert_file_exists "${rule}"
  assert_not_contains "Repository Tool Policy" "${rule}"
done

for manifest in \
  "${PROJECT}/AGENTS.md" \
  "${PROJECT}/CLAUDE.md"
do
  assert_file_exists "${manifest}"
  assert_contains "### Repository Tool Policy" "${manifest}"
  assert_contains "python=uv,pyenv" "${manifest}"
  assert_contains 'uv run <command>' "${manifest}"
done

# Targets with no manifest keep the policy inline in each rule instead.
for rule in \
  "${PROJECT}/.cursor/rules/python-testing.mdc" \
  "${PROJECT}/.opencode/python-testing.md"
do
  assert_file_exists "${rule}"
  assert_contains "## Repository Tool Policy" "${rule}"
  assert_contains "python=uv,pyenv" "${rule}"
  assert_contains 'uv run <command>' "${rule}"
done

assert_contains '"tools"' "${PROJECT}/.rulesrc.json"
assert_contains '"uv"' "${PROJECT}/.rulesrc.json"

echo "PASS: tools-rendered-in-rules-e2e"

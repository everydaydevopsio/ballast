#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# shellcheck source=./e2e/helpers.sh
source "${REPO_ROOT}/scripts/e2e/helpers.sh"
setup_ballast_e2e

PROJECT="${WORKDIR}/upgrade-refresh-skills"
mkdir -p "${PROJECT}/.codex/rules" "${PROJECT}/.claude/rules"

cat > "${PROJECT}/go.mod" <<'EOF'
module example.com/upgrade-refresh-skills

go 1.24
EOF

cat > "${PROJECT}/.rulesrc.json" <<'EOF'
{
  "targets": ["codex", "claude"],
  "agents": ["linting"],
  "skills": ["owasp-security-scan"],
  "languages": ["go"],
  "paths": {
    "go": ["."]
  },
  "ballastVersion": "0.0.1"
}
EOF

cat > "${PROJECT}/.codex/rules/owasp-security-scan.md" <<'EOF'
stale skill content
EOF

cat > "${PROJECT}/.codex/rules/go-linting.md" <<'EOF'
existing linting rule
EOF

(
  cd "${PROJECT}"
  ballast --language go upgrade >/dev/null
)

assert_contains "# OWASP Security Scan Skill" "${PROJECT}/.codex/rules/owasp-security-scan.md"
assert_not_contains "stale skill content" "${PROJECT}/.codex/rules/owasp-security-scan.md"
assert_contains '"owasp-security-scan"' "${PROJECT}/.rulesrc.json"
assert_not_contains '"ballastVersion": "0.0.1"' "${PROJECT}/.rulesrc.json"
assert_contains '`.codex/rules/owasp-security-scan.md`' "${PROJECT}/AGENTS.md"
assert_contains '`.claude/skills/owasp-security-scan.skill`' "${PROJECT}/CLAUDE.md"
assert_contains "existing linting rule" "${PROJECT}/.codex/rules/go-linting.md"

echo "PASS: upgrade-refresh-skills-e2e"

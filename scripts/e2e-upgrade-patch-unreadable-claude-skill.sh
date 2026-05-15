#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# shellcheck source=./e2e/helpers.sh
source "${REPO_ROOT}/scripts/e2e/helpers.sh"
setup_ballast_e2e

PROJECT="${WORKDIR}/upgrade-patch-unreadable-claude-skill"
mkdir -p "${PROJECT}/.claude/skills"

cat > "${PROJECT}/go.mod" <<'EOF'
module example.com/upgrade-patch-unreadable-claude-skill

go 1.24
EOF

cat > "${PROJECT}/.rulesrc.json" <<'EOF'
{
  "targets": ["claude"],
  "agents": ["linting"],
  "skills": ["owasp-security-scan"],
  "languages": ["go"],
  "paths": {
    "go": ["."]
  },
  "ballastVersion": "0.0.1"
}
EOF

printf 'not-a-zip-archive' > "${PROJECT}/.claude/skills/owasp-security-scan.skill"

(
  cd "${PROJECT}"
  ballast --language go upgrade --patch >/dev/null
)

assert_valid_claude_skill_archive "${PROJECT}/.claude/skills/owasp-security-scan.skill"
assert_contains '"owasp-security-scan"' "${PROJECT}/.rulesrc.json"
assert_contains '`.claude/skills/owasp-security-scan.skill`' "${PROJECT}/CLAUDE.md"

echo "PASS: upgrade-patch-unreadable-claude-skill-e2e"

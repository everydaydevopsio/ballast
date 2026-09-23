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

# A pre-directory bundle, and a corrupt one at that. Claude Code never read
# these; the upgrade must migrate it away rather than try to repair it.
printf 'not-a-zip-archive' > "${PROJECT}/.claude/skills/owasp-security-scan.skill"

(
  cd "${PROJECT}"
  ballast --language go upgrade --patch >/dev/null
)

assert_file_exists "${PROJECT}/.claude/skills/owasp-security-scan/SKILL.md"
assert_contains 'name: owasp-security-scan' "${PROJECT}/.claude/skills/owasp-security-scan/SKILL.md"
assert_file_absent "${PROJECT}/.claude/skills/owasp-security-scan.skill"
assert_contains '"owasp-security-scan"' "${PROJECT}/.rulesrc.json"
assert_contains '`/owasp-security-scan`' "${PROJECT}/CLAUDE.md"

echo "PASS: upgrade-patch-unreadable-claude-skill-e2e"

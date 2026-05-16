#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# shellcheck source=./e2e/helpers.sh
source "${REPO_ROOT}/scripts/e2e/helpers.sh"
setup_ballast_e2e

PROJECT="${WORKDIR}/remove-skill-existing-install"
create_monorepo_fixture "${PROJECT}"

cat > "${PROJECT}/.rulesrc.json" <<'EOF'
{
  "targets": ["claude", "codex"],
  "agents": ["linting"],
  "skills": ["owasp-security-scan", "github-health-check"],
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
  "skills": ["owasp-security-scan"],
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

assert_file_absent "${PROJECT}/.codex/rules/github-health-check.md"
assert_file_absent "${PROJECT}/.claude/skills/github-health-check.skill"
assert_not_contains '`.codex/rules/github-health-check.md`' "${PROJECT}/AGENTS.md"
assert_not_contains '`.claude/skills/github-health-check.skill`' "${PROJECT}/CLAUDE.md"
assert_file_exists "${PROJECT}/.codex/rules/owasp-security-scan.md"
assert_file_exists "${PROJECT}/.claude/skills/owasp-security-scan.skill"

echo "PASS: remove-skill-existing-install-e2e"

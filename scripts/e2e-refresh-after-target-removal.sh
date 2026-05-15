#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# shellcheck source=./e2e/helpers.sh
source "${REPO_ROOT}/scripts/e2e/helpers.sh"
setup_ballast_e2e

PROJECT="${WORKDIR}/refresh-after-target-removal"
create_monorepo_fixture "${PROJECT}"

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

materialize_saved_install "${PROJECT}"

(
  cd "${PROJECT}"
  ballast install --remove-target codex --yes >/dev/null
  ballast install --refresh-config >/dev/null
  ballast --language go upgrade >/dev/null
)

assert_not_contains '"codex"' "${PROJECT}/.rulesrc.json"
assert_file_absent "${PROJECT}/.codex/rules/python/python-linting.md"
assert_file_absent "${PROJECT}/.codex/rules/owasp-security-scan.md"
assert_file_exists "${PROJECT}/.claude/rules/python/python-linting.md"
assert_file_exists "${PROJECT}/.claude/skills/owasp-security-scan.skill"

echo "PASS: refresh-after-target-removal-e2e"

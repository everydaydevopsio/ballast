#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

if [[ "${SKIP_BUILD:-0}" != "1" ]]; then
  make -C "${REPO_ROOT}" build-go build-cli
fi

export PATH="${REPO_ROOT}/cli/ballast:${REPO_ROOT}/packages/ballast-go:${PATH}"

PROJECT="${WORKDIR}/ballast-upgrade-refresh-skills"
mkdir -p "${PROJECT}/.codex/rules"

cat > "${PROJECT}/go.mod" <<'EOF'
module example.com/upgrade-refresh-skills

go 1.24
EOF

cat > "${PROJECT}/.rulesrc.json" <<'EOF'
{
  "targets": ["codex"],
  "agents": ["linting"],
  "skills": ["owasp-security-scan"],
  "languages": ["go"],
  "paths": {"go": ["."]},
  "ballastVersion": "0.0.1"
}
EOF

cat > "${PROJECT}/.codex/rules/owasp-security-scan.md" <<'EOF'
stale skill content
EOF

(
  cd "${PROJECT}"
  ballast --language go upgrade
)

grep -q "# OWASP Security Scan Skill" "${PROJECT}/.codex/rules/owasp-security-scan.md"
! grep -q "stale skill content" "${PROJECT}/.codex/rules/owasp-security-scan.md"
grep -q '"owasp-security-scan"' "${PROJECT}/.rulesrc.json"
grep -q '`.codex/rules/owasp-security-scan.md`' "${PROJECT}/AGENTS.md"

echo "Ballast upgrade skill refresh smoke test passed."

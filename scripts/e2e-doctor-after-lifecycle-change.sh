#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# shellcheck source=./e2e/helpers.sh
source "${REPO_ROOT}/scripts/e2e/helpers.sh"
setup_ballast_e2e

PROJECT="${WORKDIR}/doctor-after-lifecycle-change"
create_monorepo_fixture "${PROJECT}"

cat > "${PROJECT}/.rulesrc.json" <<'EOF'
{
  "targets": ["claude", "codex"],
  "agents": ["linting", "docs", "tasks"],
  "skills": ["owasp-security-scan", "github-health-check"],
  "languages": ["python", "go"],
  "paths": {
    "python": ["services/api"],
    "go": ["tools/worker"]
  },
  "ballastVersion": "5.10.2",
  "taskSystem": "jira"
}
EOF

materialize_saved_install "${PROJECT}"

(
  cd "${PROJECT}"
  ballast install --remove-target codex --yes >/dev/null
)

cat > "${PROJECT}/.rulesrc.json" <<'EOF'
{
  "targets": ["claude"],
  "agents": ["linting", "tasks"],
  "skills": ["owasp-security-scan"],
  "languages": ["python", "go"],
  "paths": {
    "python": ["services/api"],
    "go": ["tools/worker"]
  },
  "ballastVersion": "5.10.2",
  "taskSystem": "linear"
}
EOF

(
  cd "${PROJECT}"
  ballast install --refresh-config >/dev/null
)

DOCTOR_OUTPUT="$(
  cd "${PROJECT}" &&
  ballast doctor
)"

assert_doctor_contains "${DOCTOR_OUTPUT}" "- targets: claude"
assert_doctor_contains "${DOCTOR_OUTPUT}" "- skills: owasp-security-scan"
assert_doctor_contains "${DOCTOR_OUTPUT}" "- agents: linting, tasks"
assert_doctor_contains "${DOCTOR_OUTPUT}" "- languages: python, go"
assert_doctor_contains "${DOCTOR_OUTPUT}" "- paths: python=services/api; go=tools/worker"
assert_doctor_contains "${DOCTOR_OUTPUT}" "- taskSystem: linear"
assert_doctor_not_contains "${DOCTOR_OUTPUT}" "codex"
assert_doctor_not_contains "${DOCTOR_OUTPUT}" "github-health-check"
assert_doctor_not_contains "${DOCTOR_OUTPUT}" "docs"

echo "PASS: doctor-after-lifecycle-change-e2e"

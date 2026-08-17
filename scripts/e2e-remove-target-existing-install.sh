#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# shellcheck source=./e2e/helpers.sh
source "${REPO_ROOT}/scripts/e2e/helpers.sh"
setup_ballast_e2e

run_codex_case() {
  local project="${WORKDIR}/remove-target-existing-install-codex"
  create_monorepo_fixture "${project}"

  cat > "${project}/.rulesrc.json" <<'EOF'
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

  materialize_saved_install "${project}"

  (
    cd "${project}"
    ballast install --remove-target codex --yes >/dev/null
  )

  assert_not_contains '"codex"' "${project}/.rulesrc.json"
  assert_contains '"claude"' "${project}/.rulesrc.json"
  assert_file_absent "${project}/.codex/rules/python/python-linting.md"
  assert_file_absent "${project}/.codex/rules/go/go-linting.md"
  assert_file_absent "${project}/.codex/skills/owasp-security-scan"
  assert_not_contains '`.codex/rules/' "${project}/AGENTS.md"
  assert_contains '`.claude/rules/python/python-linting.md`' "${project}/CLAUDE.md"
  assert_file_exists "${project}/.claude/skills/owasp-security-scan.skill"
}

run_opencode_case() {
  local project="${WORKDIR}/remove-target-existing-install-opencode"
  create_monorepo_fixture "${project}"

  cat > "${project}/.rulesrc.json" <<'EOF'
{
  "targets": ["claude", "opencode"],
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

  materialize_saved_install "${project}"
  mkdir -p "${project}/.opencode/rules/python" "${project}/.opencode/rules/go"
  cat > "${project}/.opencode/rules/python/python-linting.md" <<'EOF'
legacy managed python linting rule
EOF
  cat > "${project}/.opencode/rules/go/go-linting.md" <<'EOF'
legacy managed go linting rule
EOF
  cat > "${project}/.opencode/rules/go/manual.md" <<'EOF'
manual opencode rule
EOF

  (
    cd "${project}"
    ballast install --remove-target opencode --yes >/dev/null
  )

  assert_not_contains '"opencode"' "${project}/.rulesrc.json"
  assert_contains '"claude"' "${project}/.rulesrc.json"
  assert_file_absent "${project}/.opencode/python/python-linting.md"
  assert_file_absent "${project}/.opencode/go/go-linting.md"
  assert_file_absent "${project}/.opencode/rules/python/python-linting.md"
  assert_file_absent "${project}/.opencode/rules/go/go-linting.md"
  assert_file_exists "${project}/.opencode/rules/go/manual.md"
  assert_file_absent "${project}/.opencode/skills/owasp-security-scan.md"
  assert_file_exists "${project}/.claude/rules/python/python-linting.md"
  assert_file_exists "${project}/.claude/skills/owasp-security-scan.skill"
}

run_codex_case
run_opencode_case

echo "PASS: remove-target-existing-install-e2e"

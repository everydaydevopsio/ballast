#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

make_fixture() {
  local monorepo="$1"

  mkdir -p "${monorepo}/apps/frontend"
  mkdir -p "${monorepo}/services/api"
  mkdir -p "${monorepo}/tools/worker"

  cp -R "${REPO_ROOT}/examples/smoke/typescript-sample/." "${monorepo}/apps/frontend/"
  cp -R "${REPO_ROOT}/examples/smoke/python-sample/." "${monorepo}/services/api/"
  cp -R "${REPO_ROOT}/examples/smoke/go-sample/." "${monorepo}/tools/worker/"

  cat > "${monorepo}/package.json" <<'EOF'
{
  "name": "ballast-wrapper-monorepo",
  "private": true,
  "packageManager": "pnpm@10.27.0"
}
EOF
}

verify_common_rules() {
  local monorepo="$1"

  test -f "${monorepo}/.cursor/rules/common/local-dev-env.mdc"
  test -f "${monorepo}/.cursor/rules/common/cicd.mdc"
  test -f "${monorepo}/.cursor/rules/common/observability.mdc"
  test -f "${monorepo}/.cursor/rules/common/publishing-libraries.mdc"
  test -f "${monorepo}/.claude/rules/common/local-dev-env.md"
  test -f "${monorepo}/.opencode/common/local-dev-env.md"
  test -f "${monorepo}/.codex/rules/common/local-dev-env.md"
}

verify_language_rules() {
  local monorepo="$1"

  test -f "${monorepo}/.cursor/rules/typescript/typescript-linting.mdc"
  test -f "${monorepo}/.cursor/rules/typescript/typescript-logging.mdc"
  test -f "${monorepo}/.cursor/rules/typescript/typescript-testing.mdc"

  test -f "${monorepo}/.cursor/rules/python/python-linting.mdc"
  test -f "${monorepo}/.cursor/rules/python/python-logging.mdc"
  test -f "${monorepo}/.cursor/rules/python/python-testing.mdc"

  test -f "${monorepo}/.cursor/rules/go/go-linting.mdc"
  test -f "${monorepo}/.cursor/rules/go/go-logging.mdc"
  test -f "${monorepo}/.cursor/rules/go/go-testing.mdc"

  test -f "${monorepo}/.claude/rules/typescript/typescript-linting.md"
  test -f "${monorepo}/.claude/rules/python/python-linting.md"
  test -f "${monorepo}/.claude/rules/go/go-linting.md"

  test -f "${monorepo}/.opencode/typescript/typescript-linting.md"
  test -f "${monorepo}/.opencode/python/python-linting.md"
  test -f "${monorepo}/.opencode/go/go-linting.md"

  test -f "${monorepo}/.codex/rules/typescript/typescript-linting.md"
  test -f "${monorepo}/.codex/rules/python/python-linting.md"
  test -f "${monorepo}/.codex/rules/go/go-linting.md"
}

verify_rulesrc() {
  local monorepo="$1"

  grep -q '"targets"' "${monorepo}/.rulesrc.json"
  grep -q '"cursor"' "${monorepo}/.rulesrc.json"
  grep -q '"claude"' "${monorepo}/.rulesrc.json"
  grep -q '"opencode"' "${monorepo}/.rulesrc.json"
  grep -q '"codex"' "${monorepo}/.rulesrc.json"
  grep -q '"languages"' "${monorepo}/.rulesrc.json"
  grep -q '"typescript"' "${monorepo}/.rulesrc.json"
  grep -q '"python"' "${monorepo}/.rulesrc.json"
  grep -q '"go"' "${monorepo}/.rulesrc.json"
  grep -q '"apps/frontend"' "${monorepo}/.rulesrc.json"
  grep -q '"services/api"' "${monorepo}/.rulesrc.json"
  grep -q '"tools/worker"' "${monorepo}/.rulesrc.json"
}

verify_support_files() {
  local monorepo="$1"

  test -f "${monorepo}/CLAUDE.md"
  test -f "${monorepo}/AGENTS.md"
  grep -q '`.claude/rules/typescript/typescript-linting.md`' "${monorepo}/CLAUDE.md"
  grep -q '`.codex/rules/typescript/typescript-linting.md`' "${monorepo}/AGENTS.md"
}

verify_skills() {
  local monorepo="$1"

  test -f "${monorepo}/.cursor/rules/owasp-security-scan.mdc"
  test -f "${monorepo}/.claude/skills/owasp-security-scan.skill"
  test -f "${monorepo}/.opencode/skills/owasp-security-scan.md"
  test -f "${monorepo}/.codex/rules/owasp-security-scan.md"
}

verify_skill_patch_keeps_support_rules() {
  local monorepo="$1"

  grep -q '`.claude/rules/typescript/typescript-linting.md`' "${monorepo}/CLAUDE.md"
  grep -q '`.claude/skills/github-health-check.skill`' "${monorepo}/CLAUDE.md"
  grep -q '`.codex/rules/typescript/typescript-linting.md`' "${monorepo}/AGENTS.md"
  grep -q '`.codex/rules/github-health-check.md`' "${monorepo}/AGENTS.md"
}

verify_codex_removed() {
  local monorepo="$1"

  test ! -e "${monorepo}/.codex/rules/common/local-dev-env.md"
  test ! -e "${monorepo}/.codex/rules/typescript/typescript-linting.md"
  test ! -e "${monorepo}/.codex/rules/owasp-security-scan.md"
  ! grep -q '"codex"' "${monorepo}/.rulesrc.json"
  ! grep -q '`.codex/rules/' "${monorepo}/AGENTS.md"
  grep -q '"cursor"' "${monorepo}/.rulesrc.json"
  grep -q '"claude"' "${monorepo}/.rulesrc.json"
  grep -q '"opencode"' "${monorepo}/.rulesrc.json"
}

verify_refresh_preserves_removed_target() {
  local monorepo="$1"

  test ! -e "${monorepo}/.codex/rules/common/local-dev-env.md"
  ! grep -q '"codex"' "${monorepo}/.rulesrc.json"
}

run_wrapper_language_smoke() {
  local project="${WORKDIR}/ballast-wrapper-python"

  mkdir -p "${project}"
  cp -R "${REPO_ROOT}/examples/smoke/python-sample/." "${project}/"

  (
    cd "${project}"
    ballast --language python install --target codex --agent linting --yes
  )

  test -f "${project}/.codex/rules/python-linting.md"
  grep -q '"codex"' "${project}/.rulesrc.json"
}

main() {
  local monorepo="${WORKDIR}/ballast-wrapper-monorepo"
  make_fixture "${monorepo}"

  (
    cd "${monorepo}"
    ballast install --target cursor --target claude,opencode --target codex --all --skill owasp-security-scan --yes
  )

  verify_common_rules "${monorepo}"
  verify_language_rules "${monorepo}"
  verify_rulesrc "${monorepo}"
  verify_support_files "${monorepo}"
  verify_skills "${monorepo}"

  (
    cd "${monorepo}"
    ballast install --target claude --target codex --skill github-health-check --patch --yes
  )

  verify_skill_patch_keeps_support_rules "${monorepo}"

  (
    cd "${monorepo}"
    ballast install --remove-target codex --yes
  )

  verify_codex_removed "${monorepo}"

  (
    cd "${monorepo}"
    ballast install --refresh-config
  )

  verify_refresh_preserves_removed_target "${monorepo}"
  run_wrapper_language_smoke

  echo "Ballast wrapper monorepo smoke test passed."
}

main "$@"

#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

resolve_examples_root() {
  local requested="${1:-}"
  local candidates=()
  if [[ -n "${requested}" ]]; then
    candidates+=("${requested}")
  fi
  candidates+=("${REPO_ROOT}/.ci/ballast-examples" "${REPO_ROOT}/../ballast-examples")

  local candidate=""
  for candidate in "${candidates[@]}"; do
    if [[ -d "${candidate}/typescript-sample" && -d "${candidate}/python-sample" && -d "${candidate}/go-sample" ]]; then
      printf '%s\n' "${candidate}"
      return 0
    fi
  done

  printf '%s\n' "${requested:-${REPO_ROOT}/.ci/ballast-examples}"
}

EXAMPLES_ROOT="$(resolve_examples_root "${2:-}")"

if [[ ! -d "${EXAMPLES_ROOT}/typescript-sample" || ! -d "${EXAMPLES_ROOT}/python-sample" || ! -d "${EXAMPLES_ROOT}/go-sample" ]]; then
  echo "ballast-examples repo not found at ${EXAMPLES_ROOT}" >&2
  exit 1
fi

assert_file() {
  local path="$1"
  test -f "$path"
}

# Claude Code discovers skills as directories, so verify the SKILL.md and the
# reference material that ships beside it rather than a zip listing.
assert_claude_skill_dir() {
  local skill_md="$1"
  grep -q "^name: " "$skill_md"
  test -f "$(dirname "$skill_md")/references/owasp-mapping.md"
}

run_target() {
  local sample="$1"
  local language="$2"
  local target="$3"
  local mode="${4:-skill}"
  local dir="${WORKDIR}/${sample}-${target}-${mode}"
  mkdir -p "${dir}"
  cp -R "${EXAMPLES_ROOT}/${sample}/." "${dir}/"

  (
    cd "$dir"
    if [[ "$mode" == "all-skills" ]]; then
      ballast-go install --language "$language" --target "$target" --all-skills --yes
    else
      ballast-go install --language "$language" --target "$target" --skill owasp-security-scan --yes
    fi
  )

  case "$target" in
    cursor)
      assert_file "$dir/.cursor/rules/owasp-security-scan.mdc"
      grep -q "alwaysApply: false" "$dir/.cursor/rules/owasp-security-scan.mdc"
      ;;
    claude)
      assert_file "$dir/.claude/skills/owasp-security-scan/SKILL.md"
      assert_claude_skill_dir "$dir/.claude/skills/owasp-security-scan/SKILL.md"
      grep -q "## Installed skills" "$dir/CLAUDE.md"
      grep -q '`/owasp-security-scan`' "$dir/CLAUDE.md"
      ;;
    opencode)
      assert_file "$dir/.opencode/skills/owasp-security-scan.md"
      grep -q "# OWASP Security Scan Skill" "$dir/.opencode/skills/owasp-security-scan.md"
      ;;
    codex)
      assert_file "$dir/.codex/skills/owasp-security-scan/SKILL.md"
      grep -q "# OWASP Security Scan Skill" "$dir/.codex/skills/owasp-security-scan/SKILL.md"
      if [[ "$mode" == "all-skills" ]]; then
        assert_file "$dir/.codex/skills/github-health-check/SKILL.md"
        assert_file "$dir/.codex/skills/github-pr-copilot-cycle/SKILL.md"
        assert_file "$dir/.codex/skills/ballast-audit/SKILL.md"
        grep -q "# Ballast Audit Skill" "$dir/.codex/skills/ballast-audit/SKILL.md"
      fi
      grep -q "## Installed skills" "$dir/AGENTS.md"
      ;;
  esac
}

run_target "go-sample" "go" "cursor"
run_target "python-sample" "python" "claude"
run_target "typescript-sample" "typescript" "opencode"
run_target "typescript-sample" "typescript" "codex" "all-skills"

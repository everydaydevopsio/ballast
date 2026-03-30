#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-$(pwd)}"

assert_file() {
  local path="$1"
  test -f "$path"
}

assert_claude_skill_zip() {
  local path="$1"
  unzip -l "$path" | grep -q "SKILL.md"
  unzip -l "$path" | grep -q "references/owasp-mapping.md"
}

run_target() {
  local sample="$1"
  local language="$2"
  local target="$3"
  local mode="${4:-skill}"
  local dir="$ROOT/examples/smoke/$sample"

  rm -rf "$dir/.cursor" "$dir/.claude" "$dir/.opencode" "$dir/.codex" "$dir/AGENTS.md" "$dir/CLAUDE.md" "$dir/.rulesrc.json"

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
      assert_file "$dir/.claude/skills/owasp-security-scan.skill"
      assert_claude_skill_zip "$dir/.claude/skills/owasp-security-scan.skill"
      grep -q "## Installed skills" "$dir/CLAUDE.md"
      ;;
    opencode)
      assert_file "$dir/.opencode/skills/owasp-security-scan.md"
      grep -q "# OWASP Security Scan Skill" "$dir/.opencode/skills/owasp-security-scan.md"
      ;;
    codex)
      assert_file "$dir/.codex/rules/owasp-security-scan.md"
      grep -q "# OWASP Security Scan Skill" "$dir/.codex/rules/owasp-security-scan.md"
      grep -q "## Installed skills" "$dir/AGENTS.md"
      ;;
  esac
}

run_target "go-sample" "go" "cursor"
run_target "python-sample" "python" "claude"
run_target "typescript-sample" "typescript" "opencode"
run_target "typescript-sample" "typescript" "codex" "all-skills"

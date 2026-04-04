#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

run_language_smoke() {
  local sample="$1"
  local language="$2"
  local binary="$3"
  local expected_rule="$4"

  local sample_dir="${WORKDIR}/${sample}"
  mkdir -p "${sample_dir}"
  cp -R "${REPO_ROOT}/examples/smoke/${sample}/." "${sample_dir}/"

  "${binary}" --help >/dev/null
  "${binary}" --version >/dev/null
  "${binary}" doctor >/dev/null

  (
    cd "${sample_dir}"
    "${binary}" install --language "${language}" --target cursor,opencode,codex --agent linting --yes
  )

  test -f "${sample_dir}/.cursor/rules/${expected_rule}"
  test -f "${sample_dir}/.opencode/${expected_rule/.mdc/.md}"
  test -f "${sample_dir}/.codex/rules/${expected_rule/.mdc/.md}"
  grep -q '"targets"' "${sample_dir}/.rulesrc.json"
  grep -q '"cursor"' "${sample_dir}/.rulesrc.json"
  grep -q '"opencode"' "${sample_dir}/.rulesrc.json"
  grep -q '"codex"' "${sample_dir}/.rulesrc.json"
  grep -q '## Installed agent rules' "${sample_dir}/AGENTS.md"
}

main() {
  run_language_smoke "typescript-sample" "typescript" "ballast-typescript" "typescript-linting.mdc"
  run_language_smoke "python-sample" "python" "ballast-python" "python-linting.mdc"
  run_language_smoke "go-sample" "go" "ballast-go" "go-linting.mdc"
  run_language_smoke "ansible-sample" "ansible" "ballast-go" "ansible-linting.mdc"
  run_language_smoke "terraform-sample" "terraform" "ballast-go" "terraform-linting.mdc"

  "${REPO_ROOT}/scripts/smoke-wrapper-monorepo.sh"

  echo "Cross-language release validation passed."
}

main "$@"

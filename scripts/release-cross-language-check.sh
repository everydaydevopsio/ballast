#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

run_language_smoke() {
  local sample="$1"
  local language="$2"
  local binary="$3"

  local sample_dir="${WORKDIR}/${sample}"
  mkdir -p "${sample_dir}"
  cp -R "${REPO_ROOT}/examples/smoke/${sample}/." "${sample_dir}/"

  (
    cd "${sample_dir}"
    "${binary}" install --language "${language}" --target cursor --agent linting --yes
  )

  test -f "${sample_dir}/.cursor/rules/linting.mdc"
}

main() {
  run_language_smoke "typescript-sample" "typescript" "ballast-typescript"
  run_language_smoke "python-sample" "python" "ballast-python"
  run_language_smoke "go-sample" "go" "ballast-go"

  "${REPO_ROOT}/scripts/smoke-wrapper-monorepo.sh"

  echo "Cross-language release validation passed."
}

main "$@"

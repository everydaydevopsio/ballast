#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# shellcheck source=./e2e/helpers.sh
source "${REPO_ROOT}/scripts/e2e/helpers.sh"
export BALLAST_E2E_TYPESCRIPT=1
setup_ballast_e2e

PROJECT="${WORKDIR}/add-remove-language-existing-install"
create_monorepo_fixture "${PROJECT}"
add_typescript_profile "${PROJECT}"

cat > "${PROJECT}/.rulesrc.json" <<'JSON'
{
  "targets": ["codex"],
  "agents": ["linting"],
  "skills": [],
  "languages": ["python", "go"],
  "paths": {
    "python": ["services/api"],
    "go": ["tools/worker"]
  },
  "ballastVersion": "5.10.2"
}
JSON

materialize_saved_install "${PROJECT}"

(
  cd "${PROJECT}"
  ballast --language typescript install --target codex --agent linting --yes >/dev/null
)

assert_contains '"typescript"' "${PROJECT}/.rulesrc.json"
assert_contains '"apps/web"' "${PROJECT}/.rulesrc.json"
assert_file_exists "${PROJECT}/.codex/rules/typescript/typescript-linting.md"

(
  cd "${PROJECT}"
  ballast install --remove-language typescript --yes >/dev/null
)

assert_not_contains '"typescript"' "${PROJECT}/.rulesrc.json"
assert_not_contains '"apps/web"' "${PROJECT}/.rulesrc.json"
assert_file_absent "${PROJECT}/.codex/rules/typescript/typescript-linting.md"
assert_file_exists "${PROJECT}/.codex/rules/python/python-linting.md"
assert_file_exists "${PROJECT}/.codex/rules/go/go-linting.md"

echo "PASS: add-remove-language-existing-install-e2e"

#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# shellcheck source=./e2e/helpers.sh
source "${REPO_ROOT}/scripts/e2e/helpers.sh"
setup_ballast_e2e

PROJECT="${WORKDIR}/repository-facts-populated"
create_monorepo_fixture "${PROJECT}"
add_typescript_profile "${PROJECT}"

cat > "${PROJECT}/pnpm-lock.yaml" <<'LOCK'
lockfileVersion: '9.0'
LOCK

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

(
  cd "${PROJECT}"
  git init -q
  git config user.name "Ballast E2E"
  git config user.email "ballast-e2e@example.com"
  git remote add origin git@github.com:everydaydevopsio/ballast.git
  git symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/main
  ballast install --refresh-config --yes >/dev/null
)

assert_file_exists "${PROJECT}/AGENTS.md"
assert_contains 'Canonical GitHub repo: `everydaydevopsio/ballast`' "${PROJECT}/AGENTS.md"
assert_contains 'Default branch: `main`' "${PROJECT}/AGENTS.md"
assert_contains 'Primary package manager: `pnpm`' "${PROJECT}/AGENTS.md"
assert_not_contains 'Canonical GitHub repo: `<OWNER/REPO>`' "${PROJECT}/AGENTS.md"

echo "PASS: repository-facts-populated-e2e"

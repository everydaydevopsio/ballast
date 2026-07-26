#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# shellcheck source=./e2e/helpers.sh
source "${REPO_ROOT}/scripts/e2e/helpers.sh"
setup_ballast_e2e

PROJECT="${WORKDIR}/support-file-default-patch"
mkdir -p "${PROJECT}"

cat > "${PROJECT}/go.mod" <<'EOF'
module example.com/ballast-support-file-default-patch

go 1.24
EOF

cat > "${PROJECT}/AGENTS.md" <<'EOF'
# AGENTS.md

## Team Notes

Keep this section.

## Installed agent rules

Created by Ballast. Do not edit this section.

Read and follow these rule files in `.codex/rules/` when they apply:

- `.codex/rules/old.md` — Old rule
EOF

OUTPUT="$(
  cd "${PROJECT}"
  ballast-go install --target codex --language go --agent docs --yes
)"

assert_contains '## Team Notes' "${PROJECT}/AGENTS.md"
assert_contains 'Keep this section.' "${PROJECT}/AGENTS.md"
assert_contains '`.codex/rules/docs.md`' "${PROJECT}/AGENTS.md"
assert_not_contains '`.codex/rules/old.md`' "${PROJECT}/AGENTS.md"
assert_not_contains 'Skipped support files' <(printf '%s' "${OUTPUT}")

echo "PASS: support-file-default-patch-e2e"

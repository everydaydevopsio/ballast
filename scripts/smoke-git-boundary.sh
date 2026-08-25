#!/usr/bin/env bash
# smoke-git-boundary.sh
#
# E2E test for the project-root resolution fix that avoids unsafe ancestor
# writes. Reproduces the bug from issue #278:
#
#   - Parent directory contains project markers (playbook.yml, .rulesrc.json)
#   - Child directory has no project markers and is not yet a git repo
#   - Running `ballast install` from the child must install INTO the child,
#     not escape upward to the parent.
#
# Uses the empty-sample fixture from ../ballast-examples/ and installs the
# Python language pack.
#
# Usage:
#   ./scripts/smoke-git-boundary.sh [<examples-root>]
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

resolve_examples_root() {
  local requested="${1:-}"
  local candidates=()
  if [[ -n "${requested}" ]]; then
    candidates+=("${requested}")
  fi
  candidates+=("${REPO_ROOT}/.ci/ballast-examples" "${REPO_ROOT}/../ballast-examples")

  local candidate=""
  for candidate in "${candidates[@]}"; do
    if [[ -d "${candidate}/empty-sample" ]]; then
      printf '%s\n' "${candidate}"
      return 0
    fi
  done

  printf '%s\n' "${requested:-${REPO_ROOT}/.ci/ballast-examples}"
}

EXAMPLES_ROOT="$(resolve_examples_root "${1:-}")"
EMPTY_SAMPLE="${EXAMPLES_ROOT}/empty-sample"

if [[ ! -d "${EMPTY_SAMPLE}" ]]; then
  echo "empty-sample fixture not found at ${EMPTY_SAMPLE}" >&2
  exit 1
fi

WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

pass() { echo "  PASS: $*"; }
fail() { echo "  FAIL: $*" >&2; exit 1; }

snapshot_parent_state() {
  local target="$1"
  (
    cd "${PARENT}"
    find . -path ./empty-sample -prune -o -type f -exec sha256sum {} \; | sort
  ) > "${target}"
}

# ─── Set up parent with project markers ─────────────────────────────────────
PARENT="${WORKDIR}/parent-with-markers"
mkdir -p "${PARENT}"
echo '---' > "${PARENT}/playbook.yml"
cat > "${PARENT}/.rulesrc.json" <<'EOF'
{
  "targets": ["claude"],
  "agents": ["linting"],
  "languages": ["ansible"]
}
EOF

# ─── Set up child as an unmarked new project from empty-sample ──────────────
CHILD="${PARENT}/empty-sample"
mkdir -p "${CHILD}"
cp -R "${EMPTY_SAMPLE}/." "${CHILD}/"

# Sanity: child has only README.md, no project markers
[[ -f "${CHILD}/README.md" ]] || fail "expected ${CHILD}/README.md from empty-sample"
[[ ! -f "${CHILD}/pyproject.toml" ]] || fail "empty-sample should not have pyproject.toml"
[[ ! -f "${CHILD}/.rulesrc.json" ]] || fail "empty-sample should not have .rulesrc.json"
[[ ! -e "${CHILD}/.git" ]] || fail "child should not already be a git repo"

# Snapshot parent state before install so we can assert it was not touched,
# including in-place edits to files that already exist in the parent.
PARENT_BEFORE="${WORKDIR}/parent-before.txt"
snapshot_parent_state "${PARENT_BEFORE}"

# ─── Run the install from inside the child ──────────────────────────────────
echo "==> Running ballast install from ${CHILD}"
(
  cd "${CHILD}"
  PYTHONPATH="${REPO_ROOT}/packages/ballast-python" \
    python3 -m ballast install \
      --language python \
      --target claude \
      --agent linting \
      --yes \
    >/dev/null
)

# ─── Assert install landed in the CHILD ─────────────────────────────────────
[[ -f "${CHILD}/.claude/rules/python-linting.md" ]] \
  || fail "expected ${CHILD}/.claude/rules/python-linting.md to be created in child"
pass "python-linting.md installed inside unmarked child project"

[[ -f "${CHILD}/CLAUDE.md" ]] \
  || fail "expected ${CHILD}/CLAUDE.md to be created in child"
pass "CLAUDE.md created inside unmarked child project"

[[ -f "${CHILD}/.rulesrc.json" ]] \
  || fail "expected ${CHILD}/.rulesrc.json to be created in child"
grep -q '"python"' "${CHILD}/.rulesrc.json" \
  || fail "expected python language recorded in ${CHILD}/.rulesrc.json"
pass ".rulesrc.json created in child with python language"

# ─── Assert install did NOT escape into the PARENT ──────────────────────────
PARENT_AFTER="${WORKDIR}/parent-after.txt"
snapshot_parent_state "${PARENT_AFTER}"
if ! diff -q "${PARENT_BEFORE}" "${PARENT_AFTER}" >/dev/null; then
  echo "    Parent file contents changed:" >&2
  diff "${PARENT_BEFORE}" "${PARENT_AFTER}" >&2 || true
  fail "parent directory was modified by install (ancestor marker was inherited)"
fi
pass "parent directory untouched (ancestor marker not inherited)"

[[ ! -d "${PARENT}/.claude" ]] \
  || fail "parent must not have .claude directory created by install"
pass "no .claude/ created in parent"

[[ ! -f "${PARENT}/CLAUDE.md" ]] \
  || fail "parent must not have CLAUDE.md created by install"
pass "no CLAUDE.md created in parent"

echo "Ancestor root-selection smoke test PASSED."

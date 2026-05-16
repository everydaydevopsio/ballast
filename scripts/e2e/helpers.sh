#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

setup_ballast_e2e() {
  if [[ "${SKIP_BUILD:-0}" != "1" ]]; then
    make -C "${REPO_ROOT}" build-go build-cli >/dev/null
  fi

  E2E_BIN_DIR="${E2E_BIN_DIR:-${WORKDIR}/bin}"
  mkdir -p "${E2E_BIN_DIR}"

  cat > "${E2E_BIN_DIR}/ballast-python" <<EOF
#!/usr/bin/env bash
set -euo pipefail
PYTHONPATH="${REPO_ROOT}/packages/ballast-python\${PYTHONPATH:+:\$PYTHONPATH}" python3 -m ballast "\$@"
EOF
  chmod +x "${E2E_BIN_DIR}/ballast-python"

  if [[ "${BALLAST_E2E_TYPESCRIPT:-0}" == "1" ]]; then
    if [[ "${SKIP_BUILD:-0}" != "1" ]]; then
      pnpm -C "${REPO_ROOT}" run build >/dev/null
    fi
    cat > "${E2E_BIN_DIR}/ballast-typescript" <<EOF
#!/usr/bin/env bash
set -euo pipefail
node "${REPO_ROOT}/packages/ballast-typescript/dist/cli.js" "\$@"
EOF
    chmod +x "${E2E_BIN_DIR}/ballast-typescript"
  fi

  export BALLAST_REPO_ROOT="${REPO_ROOT}"
  export UV_CACHE_DIR="${UV_CACHE_DIR:-/tmp/uv-cache}"
  mkdir -p "${UV_CACHE_DIR}"
  export PATH="${REPO_ROOT}/cli/ballast:${REPO_ROOT}/packages/ballast-go:${E2E_BIN_DIR}:${PATH}"
}

create_monorepo_fixture() {
  local root="$1"
  mkdir -p "${root}/services/api/ballast_api" "${root}/tools/worker"

  cat > "${root}/package.json" <<'EOF'
{
  "name": "ballast-e2e-fixture",
  "private": true
}
EOF

  cat > "${root}/services/api/pyproject.toml" <<'EOF'
[project]
name = "ballast-api"
version = "0.0.0"
requires-python = ">=3.10"
EOF

  cat > "${root}/services/api/ballast_api/__init__.py" <<'EOF'
"""ballast fixture package."""
EOF

  cat > "${root}/tools/worker/go.mod" <<'EOF'
module example.com/ballast-worker

go 1.24
EOF

  cat > "${root}/tools/worker/main.go" <<'EOF'
package main

func main() {}
EOF
}

add_typescript_profile() {
  local root="$1"
  mkdir -p "${root}/apps/web/src"

  cat > "${root}/apps/web/package.json" <<'EOF'
{
  "name": "ballast-web",
  "private": true
}
EOF

  cat > "${root}/apps/web/tsconfig.json" <<'EOF'
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext"
  }
}
EOF

  cat > "${root}/apps/web/src/index.ts" <<'EOF'
export {};
EOF
}

materialize_saved_install() {
  local root="$1"
  (
    cd "${root}"
    ballast install --refresh-config >/dev/null
  )
}

assert_file_exists() {
  local path="$1"
  [[ -f "${path}" ]] || fail "expected file ${path}"
}

assert_file_absent() {
  local path="$1"
  [[ ! -e "${path}" ]] || fail "expected ${path} to be absent"
}

assert_dir_absent() {
  local path="$1"
  [[ ! -d "${path}" ]] || fail "expected directory ${path} to be absent"
}

assert_contains() {
  local needle="$1"
  local path="$2"
  grep -Fq "${needle}" "${path}" || fail "expected '${needle}' in ${path}"
}

assert_not_contains() {
  local needle="$1"
  local path="$2"
  if grep -Fq "${needle}" "${path}"; then
    fail "expected '${needle}' to be absent in ${path}"
  fi
}

assert_valid_claude_skill_archive() {
  local path="$1"
  unzip -tqq "${path}" >/dev/null || fail "expected valid zip archive at ${path}"
  unzip -l "${path}" | grep -Fq "SKILL.md" || fail "expected SKILL.md inside ${path}"
}

assert_doctor_contains() {
  local output="$1"
  local needle="$2"
  grep -Fq -- "${needle}" <<<"${output}" || fail "expected '${needle}' in doctor output"
}

assert_doctor_not_contains() {
  local output="$1"
  local needle="$2"
  if grep -Fq -- "${needle}" <<<"${output}"; then
    fail "expected '${needle}' to be absent from doctor output"
  fi
}

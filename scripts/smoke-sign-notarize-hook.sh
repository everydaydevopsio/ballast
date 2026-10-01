#!/usr/bin/env bash
# smoke-sign-notarize-hook.sh
#
# Covers the failure paths of scripts/sign-and-notarize-darwin.sh, the
# GoReleaser post-build hook that signs and notarizes darwin binaries before
# any artifact is uploaded.
#
# The case that matters most is "Invalid": `xcrun notarytool submit --wait`
# exits 0 as long as the submission itself completed, including when Apple
# rejects the binary. The hook must read the submission status and fail on
# anything other than Accepted, or a rejected binary ships in a release.
#
# codesign, ditto, and xcrun are stubbed on PATH so this runs without Apple
# credentials, a signing identity, or network access. plutil is the real one,
# which is why this smoke test is macOS-only.
#
# Usage:
#   ./scripts/smoke-sign-notarize-hook.sh
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HOOK="${REPO_ROOT}/scripts/sign-and-notarize-darwin.sh"

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "SMOKE TEST SKIPPED: requires macOS for plutil and the hook's toolchain"
  exit 0
fi

WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

mkdir -p "${WORKDIR}/bin" "${WORKDIR}/home/.private_keys"
touch "${WORKDIR}/home/.private_keys/AuthKey_TESTKEY.p8"
echo 'not a real mach-o' > "${WORKDIR}/ballast"

cat > "${WORKDIR}/bin/codesign" <<'STUB'
#!/bin/sh
exit 0
STUB

# ditto -c -k --keepParent SRC DEST: only the destination archive matters here.
cat > "${WORKDIR}/bin/ditto" <<'STUB'
#!/bin/sh
eval DEST=\${$#}
: > "$DEST"
STUB

# Returns whatever status STUB_STATUS asks for, with the exit code the real
# notarytool would use (0 even for Invalid, unless STUB_SUBMIT_EXIT overrides).
cat > "${WORKDIR}/bin/xcrun" <<'STUB'
#!/bin/sh
if [ "$1" = "notarytool" ] && [ "$2" = "submit" ]; then
  printf '{"id":"sub-smoke","status":"%s","message":"Processing complete"}' "${STUB_STATUS:-Accepted}"
  exit "${STUB_SUBMIT_EXIT:-0}"
fi
if [ "$1" = "notarytool" ] && [ "$2" = "log" ]; then
  echo '{"issues":[{"severity":"error","message":"stub notary log"}]}'
  exit 0
fi
exit 0
STUB

chmod +x "${WORKDIR}/bin/"*

failures=0

run_hook() {
  env \
    PATH="${WORKDIR}/bin:${PATH}" \
    HOME="${WORKDIR}/home" \
    APPLE_SIGNING_IDENTITY="Developer ID Application: Smoke Test" \
    APPLE_API_KEY_ID="TESTKEY" \
    APPLE_API_KEY_ISSUER_ID="smoke-issuer" \
    "$@" \
    bash "${HOOK}" darwin "${WORKDIR}/ballast"
}

expect_exit() {
  local label="$1"
  local want="$2"
  shift 2

  local output
  local got=0
  output="$("$@" 2>&1)" || got=$?

  if [[ "${got}" -eq "${want}" ]]; then
    echo "ok: ${label} (exit ${got})"
  else
    echo "FAIL: ${label} expected exit ${want}, got ${got}"
    printf '%s\n' "${output}"
    failures=$((failures + 1))
  fi
}

# A clean notarization must let the build continue.
expect_exit "Accepted submission succeeds" 0 \
  run_hook STUB_STATUS=Accepted

# The regression this smoke test exists for.
expect_exit "Invalid submission fails even though notarytool exits 0" 1 \
  run_hook STUB_STATUS=Invalid

expect_exit "Rejected submission with a non-zero notarytool exit fails" 1 \
  run_hook STUB_STATUS=Rejected STUB_SUBMIT_EXIT=1

# Missing credentials must abort rather than silently skip notarization.
expect_exit "Missing APPLE_API_KEY_ISSUER_ID fails" 1 \
  env PATH="${WORKDIR}/bin:${PATH}" HOME="${WORKDIR}/home" \
  APPLE_SIGNING_IDENTITY="Developer ID Application: Smoke Test" \
  APPLE_API_KEY_ID="TESTKEY" \
  bash "${HOOK}" darwin "${WORKDIR}/ballast"

expect_exit "Missing App Store Connect key file fails" 1 \
  env PATH="${WORKDIR}/bin:${PATH}" HOME="${WORKDIR}/empty-home" \
  APPLE_SIGNING_IDENTITY="Developer ID Application: Smoke Test" \
  APPLE_API_KEY_ID="TESTKEY" APPLE_API_KEY_ISSUER_ID="smoke-issuer" \
  bash "${HOOK}" darwin "${WORKDIR}/ballast"

# Non-darwin builds share the hook and must be a no-op.
expect_exit "Non-darwin build is a no-op" 0 \
  env PATH="${WORKDIR}/bin:${PATH}" bash "${HOOK}" linux "${WORKDIR}/ballast"

if [[ "${failures}" -ne 0 ]]; then
  echo "SMOKE TEST FAILED: ${failures} case(s)"
  exit 1
fi

echo "SMOKE TEST PASSED"

#!/usr/bin/env bash
# Sign and notarize a single darwin binary as a GoReleaser post-build hook.
#
# Runs inside the build phase, before any artifact is uploaded, so a signing or
# notarization failure aborts the release instead of leaving signed-but-
# unnotarized archives on a published GitHub Release and in the Homebrew tap.
#
# Usage: sign-and-notarize-darwin.sh <goos> <binary-path>
set -euo pipefail

GOOS="${1:?goos required}"
BINARY="${2:?binary path required}"

if [ "$GOOS" != "darwin" ]; then
  exit 0
fi

: "${APPLE_SIGNING_IDENTITY:?APPLE_SIGNING_IDENTITY must be set to sign darwin builds}"
: "${APPLE_API_KEY_ID:?APPLE_API_KEY_ID must be set to notarize darwin builds}"
: "${APPLE_API_KEY_ISSUER_ID:?APPLE_API_KEY_ISSUER_ID must be set to notarize darwin builds}"

KEY_FILE="${HOME}/.private_keys/AuthKey_${APPLE_API_KEY_ID}.p8"
if [ ! -f "$KEY_FILE" ]; then
  echo "App Store Connect API key not found at ${KEY_FILE}" >&2
  exit 1
fi

echo "Signing ${BINARY}"
codesign --sign "$APPLE_SIGNING_IDENTITY" --options runtime --timestamp "$BINARY"
codesign --verify --strict "$BINARY"

# notarytool needs an archive, not a bare executable. A bare binary cannot be
# stapled, so Gatekeeper validates these online against Apple's service.
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT
ZIP="${WORKDIR}/$(basename "$BINARY").zip"
SUBMIT_JSON="${WORKDIR}/notarytool-submit.json"
ditto -c -k --keepParent "$BINARY" "$ZIP"

echo "Notarizing ${BINARY}"

# `notarytool submit --wait` exits 0 as long as the submission itself completed,
# including when Apple comes back `Invalid`. Read the status out of the JSON and
# assert it rather than trusting the exit code, or a rejected binary ships.
set +e
xcrun notarytool submit "$ZIP" \
  --key "$KEY_FILE" \
  --key-id "$APPLE_API_KEY_ID" \
  --issuer "$APPLE_API_KEY_ISSUER_ID" \
  --wait \
  --timeout 20m \
  --output-format json >"$SUBMIT_JSON"
SUBMIT_EXIT=$?
set -e

# plutil reads JSON and ships with macOS, so this needs no extra dependency.
SUBMISSION_ID="$(plutil -extract id raw -o - -- "$SUBMIT_JSON" 2>/dev/null || true)"
SUBMISSION_STATUS="$(plutil -extract status raw -o - -- "$SUBMIT_JSON" 2>/dev/null || true)"

if [ "$SUBMIT_EXIT" -ne 0 ] || [ "$SUBMISSION_STATUS" != "Accepted" ]; then
  echo "Notarization failed for ${BINARY}: status=${SUBMISSION_STATUS:-unknown} exit=${SUBMIT_EXIT}" >&2
  cat "$SUBMIT_JSON" >&2 || true
  if [ -n "$SUBMISSION_ID" ]; then
    echo "Notary log for submission ${SUBMISSION_ID}:" >&2
    xcrun notarytool log "$SUBMISSION_ID" \
      --key "$KEY_FILE" \
      --key-id "$APPLE_API_KEY_ID" \
      --issuer "$APPLE_API_KEY_ISSUER_ID" >&2 || true
  fi
  exit 1
fi

echo "Notarized ${BINARY} (submission ${SUBMISSION_ID}, status ${SUBMISSION_STATUS})"

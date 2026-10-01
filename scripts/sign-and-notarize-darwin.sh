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
ZIP="$(mktemp -d)/$(basename "$BINARY").zip"
ditto -c -k --keepParent "$BINARY" "$ZIP"

echo "Notarizing ${BINARY}"
xcrun notarytool submit "$ZIP" \
  --key "$KEY_FILE" \
  --key-id "$APPLE_API_KEY_ID" \
  --issuer "$APPLE_API_KEY_ISSUER_ID" \
  --wait \
  --timeout 20m

rm -f "$ZIP"
echo "Notarized ${BINARY}"

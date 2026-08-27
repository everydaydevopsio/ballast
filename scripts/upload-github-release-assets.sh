#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 3 ]; then
  echo "Usage: $0 <release-tag> <release-title> <asset> [<asset> ...]" >&2
  exit 2
fi

release_tag="$1"
release_title="$2"
shift 2

if ! gh release view "$release_tag" >/dev/null 2>&1; then
  gh release create "$release_tag" --title "$release_title" --generate-notes || {
    status=$?

    for _ in 1 2 3 4 5; do
      gh release view "$release_tag" >/dev/null 2>&1 && break
      sleep 2
    done

    gh release view "$release_tag" >/dev/null 2>&1 || exit "$status"
  }
fi

gh release upload "$release_tag" "$@" --clobber

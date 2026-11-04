#!/usr/bin/env bash
#
# Self-test for scripts/agent-rule-read-check.sh.
#
# The real check needs provider credentials, so CI cannot run it. The part worth
# protecting is the classification logic -- what counts as a pass, a failure, and
# an environmental skip -- and that can be exercised with stub CLIs and no keys.
#
# Also covers the Actions step contract: the step runs under errexit, so it must
# capture the script's exit status instead of dying on it.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHECK="$REPO_ROOT/scripts/agent-rule-read-check.sh"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
STUB_DIR="$WORK/stub"
mkdir -p "$STUB_DIR"

BALLAST_BIN="${BALLAST_BIN:-}"
if [ -z "$BALLAST_BIN" ]; then
  BALLAST_BIN="$WORK/ballast-go"
  # Stamp the version like every other source build: an unstamped backend emits
  # rules marked "dev", which is exactly the drift ci-workflow.test.ts guards.
  BALLAST_VERSION="$(node -p "require('$REPO_ROOT/packages/ballast-typescript/package.json').version")"
  go build -C "$REPO_ROOT/packages/ballast-go" -ldflags "-X main.ballastVersion=${BALLAST_VERSION}" -o "$BALLAST_BIN" ./cmd/ballast-go
fi
export BALLAST_BIN

failures=0

run_case() {
  local name="$1" expected="$2" agent="$3"
  local actual=0
  PATH="$STUB_DIR:$PATH" bash "$CHECK" "$agent" >"$WORK/out.log" 2>&1 || actual=$?
  if [ "$actual" -eq "$expected" ]; then
    printf '  ok    %-52s exit=%s\n' "$name" "$actual"
  else
    printf '  FAIL  %-52s exit=%s want=%s\n' "$name" "$actual" "$expected"
    sed 's/^/        /' "$WORK/out.log"
    failures=$((failures + 1))
  fi
}

stub() {
  local agent="$1"
  cat >"$STUB_DIR/$agent"
  chmod +x "$STUB_DIR/$agent"
}

echo "agent-rule-read-check self-test"

# Reads the rule and echoes the sentinel.
stub claude <<'EOF'
#!/usr/bin/env bash
grep -ohE 'BALLAST-RULE-READ-[A-Z0-9]+' .claude/rules/local-dev-env.md 2>/dev/null | head -1
EOF
run_case "agent reads rule and returns sentinel" 0 claude

# Answers without consulting the rules.
stub claude <<'EOF'
#!/usr/bin/env bash
echo "I could not find a verification code."
EOF
run_case "agent answers without reading the rule" 1 claude

# Environmental failures.
stub claude <<'EOF'
#!/usr/bin/env bash
echo "Error: 401 Unauthorized - invalid api key" >&2
exit 1
EOF
run_case "auth failure skips" 2 claude

stub claude <<'EOF'
#!/usr/bin/env bash
echo "429 rate limit exceeded" >&2
exit 1
EOF
run_case "rate limit skips" 2 claude

stub claude <<'EOF'
#!/usr/bin/env bash
echo "request to api failed, reason: ENOTFOUND" >&2
exit 1
EOF
run_case "network failure skips" 2 claude

stub claude <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
run_case "empty answer skips" 2 claude

# Regression: a correct answer whose random sentinel contains an HTTP-like digit
# run must pass, not be misread as an environmental failure.
stub claude <<'EOF'
#!/usr/bin/env bash
grep -ohE 'BALLAST-RULE-READ-[A-Z0-9]+' .claude/rules/local-dev-env.md 2>/dev/null | head -1
EOF
saved_sentinel="${BALLAST_RULE_READ_SENTINEL:-}"
export BALLAST_RULE_READ_SENTINEL="BALLAST-RULE-READ-ABC401DEF429GHI"
run_case "sentinel containing 401/429 still passes" 0 claude
if [ -n "$saved_sentinel" ]; then
  export BALLAST_RULE_READ_SENTINEL="$saved_sentinel"
else
  unset BALLAST_RULE_READ_SENTINEL
fi

# Codex uses a different invocation and rule directory.
stub codex <<'EOF'
#!/usr/bin/env bash
[ "$1" = "exec" ] || { echo "expected 'codex exec', got: $*" >&2; exit 9; }
grep -ohE 'BALLAST-RULE-READ-[A-Z0-9]+' .codex/rules/local-dev-env.md 2>/dev/null | head -1
EOF
run_case "codex exec reads rule and returns sentinel" 0 codex

# A missing CLI is an environment gap, not a Ballast defect.
rm -f "$STUB_DIR/codex"
env -i PATH="/usr/bin:/bin" BALLAST_BIN="$BALLAST_BIN" bash "$CHECK" codex >/dev/null 2>&1 && missing=0 || missing=$?
if [ "${missing:-0}" -eq 2 ]; then
  printf '  ok    %-52s exit=2\n' "missing agent CLI skips"
else
  printf '  FAIL  %-52s exit=%s want=2\n' "missing agent CLI skips" "${missing:-0}"
  failures=$((failures + 1))
fi

# The Actions step runs under errexit. Verify the capture idiom survives a
# non-zero script exit, because without it a skip would be reported as a failure.
for code in 1 2; do
  cat >"$WORK/step.sh" <<EOF
set -uo pipefail
status=0
bash -c 'exit $code' || status=\$?
echo "captured=\$status"
EOF
  captured="$(bash -e "$WORK/step.sh" 2>/dev/null || true)"
  if [ "$captured" = "captured=$code" ]; then
    printf '  ok    %-52s exit=%s\n' "actions step captures script exit $code" "$code"
  else
    printf '  FAIL  %-52s got=%s\n' "actions step captures script exit $code" "$captured"
    failures=$((failures + 1))
  fi
done

if [ "$failures" -ne 0 ]; then
  echo "SELF-TEST FAILED: $failures case(s)"
  exit 1
fi
echo "SELF-TEST PASSED"

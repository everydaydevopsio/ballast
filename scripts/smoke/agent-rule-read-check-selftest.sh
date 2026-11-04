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

# The check now refuses to run without credentials, so the stubbed cases need
# placeholder values. They are never sent anywhere: every CLI here is a stub.
export ANTHROPIC_API_KEY="selftest-anthropic-key"
export OPENAI_API_KEY="selftest-openai-key"
unset CODEX_API_KEY

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

# A sandbox refusal is the agent describing its own permissions, not a missing
# rule. It arrives on stdout with a zero exit, so it would otherwise read as a
# genuine assertion failure -- which is how it first surfaced in CI.
stub claude <<'EOF'
#!/usr/bin/env bash
echo "Unable to read the verification code: filesystem access failed."
EOF
run_case "sandbox refusal skips rather than fails" 2 claude

stub claude <<'EOF'
#!/usr/bin/env bash
echo "cat: .claude/rules/local-dev-env.md: Permission denied"
EOF
run_case "permission denied skips rather than fails" 2 claude

# codex prints its configuration banner -- including "sandbox: read-only" -- on
# stderr for every run. A classifier matching a bare "sandbox" therefore turned
# every codex result into a skip, so the check appeared to run while asserting
# nothing. A wrong answer alongside that banner must still FAIL.
stub codex <<'EOF'
#!/usr/bin/env bash
cat >&2 <<'BANNER'
OpenAI Codex v0.166.0
--------
workdir: /tmp/fixture
model: gpt-6-luna
provider: openai
approval: never
sandbox: read-only
reasoning effort: medium
--------
BANNER
echo "I could not find a verification code in this project."
EOF
run_case "codex banner does not mask a wrong answer" 1 codex

# The same banner must not mask a correct answer either.
stub codex <<'EOF'
#!/usr/bin/env bash
cat >&2 <<'BANNER'
OpenAI Codex v0.166.0
sandbox: read-only
approval: never
BANNER
grep -ohE 'BALLAST-RULE-READ-[A-Z0-9]+' .codex/rules/local-dev-env.md 2>/dev/null | head -1
EOF
run_case "codex banner does not mask a correct answer" 0 codex

# A real sandbox refusal still skips, banner and all.
stub codex <<'EOF'
#!/usr/bin/env bash
echo "sandbox: read-only" >&2
echo "Unable to read the verification code: filesystem access failed."
EOF
run_case "codex sandbox refusal still skips" 2 codex

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

# Codex uses a different invocation and rule directory, and authenticates from
# CODEX_API_KEY rather than OPENAI_API_KEY.
stub codex <<'EOF'
#!/usr/bin/env bash
[ "$1" = "exec" ] || { echo "expected 'codex exec', got: $*" >&2; exit 9; }
# Without a sandbox that permits reads, codex answers "filesystem access
# failed" and never reaches the rule file.
case " $* " in
  *" --sandbox read-only "*) ;;
  *) echo "codex was not given a readable sandbox: $*" >&2; exit 9 ;;
esac
[ -n "${CODEX_API_KEY:-}" ] || { echo "CODEX_API_KEY was not forwarded to codex" >&2; exit 9; }
grep -ohE 'BALLAST-RULE-READ-[A-Z0-9]+' .codex/rules/local-dev-env.md 2>/dev/null | head -1
EOF
run_case "codex exec runs read-only with CODEX_API_KEY" 0 codex

# OPENAI_API_KEY is the conventional secret name; it must reach codex as
# CODEX_API_KEY, which is what the non-interactive path actually reads.
run_case "OPENAI_API_KEY is forwarded as CODEX_API_KEY" 0 codex

# Missing credentials are an environment gap, not a rule regression.
saved_anthropic="$ANTHROPIC_API_KEY"
unset ANTHROPIC_API_KEY
stub claude <<'EOF'
#!/usr/bin/env bash
echo "stub should not have been reached"
EOF
run_case "missing ANTHROPIC_API_KEY skips" 2 claude
export ANTHROPIC_API_KEY="$saved_anthropic"

saved_openai="$OPENAI_API_KEY"
unset OPENAI_API_KEY
run_case "missing codex credentials skip" 2 codex
export OPENAI_API_KEY="$saved_openai"

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

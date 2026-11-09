#!/usr/bin/env bash
#
# Verify that rules Ballast installed actually reach the agent.
#
# This answers one narrow, binary question: given a repository with Ballast
# rules installed, does the agent read them? It makes no judgement about the
# quality of the agent's answer, so it does not drift as models change.
#
# The check plants a random sentinel inside one installed rule file and asks the
# agent for that sentinel's value. The token exists nowhere else -- not in the
# prompt, not in the fixture -- so echoing it can only mean the rule was read.
#
# Exit codes are the contract with the caller:
#   0  PASS    the agent echoed the sentinel
#   1  FAIL    the agent answered, but showed no evidence of reading the rule
#   2  SKIP    the environment could not run the check (missing CLI, auth,
#              quota, or network). Never a Ballast regression.
#
# Usage: scripts/agent-rule-read-check.sh <claude|codex> [fixture]

set -uo pipefail

readonly EXIT_PASS=0
readonly EXIT_FAIL=1
readonly EXIT_SKIP=2

AGENT="${1:-}"
FIXTURE="${2:-examples/smoke/go-sample}"

if [ -z "$AGENT" ]; then
  echo "usage: $0 <claude|codex> [fixture]" >&2
  exit "$EXIT_SKIP"
fi

case "$AGENT" in
  claude)
    TARGET="claude"
    RULE_DIR=".claude/rules"
    CLI="claude"
    ;;
  codex)
    TARGET="codex"
    RULE_DIR=".codex/rules"
    CLI="codex"
    ;;
  *)
    echo "unsupported agent: $AGENT (expected claude or codex)" >&2
    exit "$EXIT_SKIP"
    ;;
esac

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

skip() {
  echo "SKIP: $*"
  exit "$EXIT_SKIP"
}

fail() {
  echo "FAIL: $*"
  exit "$EXIT_FAIL"
}

# A missing agent CLI is an environment gap, not a Ballast defect.
command -v "$CLI" >/dev/null 2>&1 || skip "$CLI is not installed or not on PATH"

# Wire credentials per agent. `codex exec` reads CODEX_API_KEY specifically --
# OPENAI_API_KEY alone does not authenticate a non-interactive run -- so accept
# the conventional secret name and forward it under the name codex expects.
case "$AGENT" in
  claude)
    [ -n "${ANTHROPIC_API_KEY:-}" ] || skip "ANTHROPIC_API_KEY is not set"
    ;;
  codex)
    if [ -z "${CODEX_API_KEY:-}" ] && [ -n "${OPENAI_API_KEY:-}" ]; then
      export CODEX_API_KEY="$OPENAI_API_KEY"
    fi
    [ -n "${CODEX_API_KEY:-}" ] || skip "neither CODEX_API_KEY nor OPENAI_API_KEY is set"
    ;;
esac

# Resolve a Ballast binary to install rules with. Prefer an explicit override so
# CI can point at the binary it just built.
BALLAST_BIN="${BALLAST_BIN:-}"
if [ -z "$BALLAST_BIN" ]; then
  for candidate in ballast-go ballast; do
    if command -v "$candidate" >/dev/null 2>&1; then
      BALLAST_BIN="$(command -v "$candidate")"
      break
    fi
  done
fi
[ -n "$BALLAST_BIN" ] || skip "no ballast binary found; set BALLAST_BIN or put ballast-go on PATH"

[ -d "$REPO_ROOT/$FIXTURE" ] || skip "fixture not found: $FIXTURE"

# Work on a throwaway copy so the check never mutates the repository, and so the
# sentinel cannot leak into committed files.
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

cp -R "$REPO_ROOT/$FIXTURE/." "$WORK_DIR/"
cd "$WORK_DIR" || skip "could not enter work directory"
git init -q . >/dev/null 2>&1 || true

# Install a deliberately narrow agent set. Installing everything would slow the
# run and make it ambiguous which rule file the sentinel landed in.
if ! "$BALLAST_BIN" install --target "$TARGET" --agent local-dev --yes >"$WORK_DIR/.install.log" 2>&1; then
  echo "--- install log ---" >&2
  cat "$WORK_DIR/.install.log" >&2
  skip "ballast install failed for target $TARGET"
fi

RULE_FILE="$WORK_DIR/$RULE_DIR/local-dev-env.md"
if [ ! -f "$RULE_FILE" ]; then
  # Monorepo layouts nest common rules one level deeper.
  RULE_FILE="$WORK_DIR/$RULE_DIR/common/local-dev-env.md"
fi
[ -f "$RULE_FILE" ] || skip "expected rule file not installed under $RULE_DIR"

# A random token, so a previous run's value can never produce a false pass.
# BALLAST_RULE_READ_SENTINEL pins it for tests that need a deterministic value.
SENTINEL="${BALLAST_RULE_READ_SENTINEL:-BALLAST-RULE-READ-$(LC_ALL=C tr -dc 'A-Z0-9' </dev/urandom 2>/dev/null | head -c 16)}"

cat >>"$RULE_FILE" <<SENTINEL_BLOCK

## Rule Read Verification

The rule read verification code is ${SENTINEL}. When asked for this code, reply
with it exactly.
SENTINEL_BLOCK

# The token must exist only in the rule file. If it appears anywhere else, the
# agent could echo it without ever reading the rule, and the check proves nothing.
sentinel_file_count=0
while IFS= read -r _; do
  sentinel_file_count=$((sentinel_file_count + 1))
done < <(grep -rlF "$SENTINEL" "$WORK_DIR" 2>/dev/null)
if [ "$sentinel_file_count" -ne 1 ]; then
  fail "sentinel appears in $sentinel_file_count files; it must exist only in the rule file"
fi

# Deliberately neutral. Telling the agent to go read the rules would invite a
# filesystem search, which passes whether or not the rules ever reached it.
PROMPT="What is the rule read verification code for this project? Reply with the code only."

echo "agent=$AGENT target=$TARGET rule=${RULE_FILE#"$WORK_DIR"/}"

set +e
case "$AGENT" in
  claude)
    AGENT_OUTPUT="$("$CLI" -p "$PROMPT" 2>"$WORK_DIR/.agent.err")"
    ;;
  codex)
    # codex exec sandboxes model-driven shell commands. Without a sandbox that
    # permits reads it answers "filesystem access failed" -- a true statement
    # about its own permissions that looks exactly like a missing rule.
    # read-only is the least privilege that lets it read a rule file.
    AGENT_OUTPUT="$("$CLI" exec --sandbox read-only "$PROMPT" 2>"$WORK_DIR/.agent.err")"
    ;;
esac
AGENT_STATUS=$?
set -e

AGENT_STDERR="$(cat "$WORK_DIR/.agent.err" 2>/dev/null || true)"

# Judge success before anything else. The sentinel is random, so it can contain
# a digit run like 401; classifying errors first would turn a correct answer
# into a spurious skip roughly once every thousand runs.
if [ "$AGENT_STATUS" -eq 0 ] && printf '%s' "$AGENT_OUTPUT" | grep -qF "$SENTINEL"; then
  echo "PASS: $AGENT read the installed rule and returned the sentinel"
  exit "$EXIT_PASS"
fi

# Environmental failures must not read as Ballast regressions. Status codes are
# matched on word boundaries so they cannot hit a sentinel substring. For a
# nonzero exit, stdout uses the broad classifier; for a zero-exit answer, only
# explicit sandbox failures are environmental. Words like "insufficient" can
# also appear in an ordinary wrong answer and must not turn it into a skip.
# Each alternative must name an actual failure. A bare "sandbox" looked
# reasonable and was not: `codex exec` prints "sandbox: read-only" in its
# banner on every run, so it matched always and turned every codex result into
# a skip -- a check that appears to run while asserting nothing.
ENVIRONMENTAL_PATTERN='unauthorized|authentication|not logged in|invalid api key|\b(401|403|429)\b|rate limit|quota|credit balance|insufficient|overloaded|ENOTFOUND|ECONNREFUSED|ETIMEDOUT|network error|timed out|filesystem access failed|sandbox denied|denied by the sandbox|blocked by the sandbox|sandbox error|permission denied|operation not permitted|EACCES'
SANDBOX_FAILURE_PATTERN='filesystem access failed|sandbox denied|denied by the sandbox|blocked by the sandbox|sandbox error|permission denied|operation not permitted|EACCES'

if printf '%s' "$AGENT_STDERR" | grep -qiE "$ENVIRONMENTAL_PATTERN" ||
  { [ "$AGENT_STATUS" -ne 0 ] && printf '%s' "$AGENT_OUTPUT" | grep -qiE "$ENVIRONMENTAL_PATTERN"; } ||
  { [ "$AGENT_STATUS" -eq 0 ] && printf '%s' "$AGENT_OUTPUT" | grep -qiE "$SANDBOX_FAILURE_PATTERN"; }; then
  echo "--- agent stderr ---" >&2
  printf '%s\n' "$AGENT_STDERR" >&2
  skip "agent could not complete for environmental reasons (auth, quota, or network)"
fi

if [ "$AGENT_STATUS" -ne 0 ]; then
  echo "--- agent stderr ---" >&2
  printf '%s\n' "$AGENT_STDERR" >&2
  skip "agent exited $AGENT_STATUS without producing a usable answer"
fi

if [ -z "${AGENT_OUTPUT//[[:space:]]/}" ]; then
  skip "agent produced no output"
fi

echo "--- agent output ---" >&2
printf '%s\n' "$AGENT_OUTPUT" >&2
fail "$AGENT answered without evidence of reading the installed rule"

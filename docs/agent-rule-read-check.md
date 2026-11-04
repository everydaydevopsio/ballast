# Agent Rule Read Check

An advisory check that answers one question: when Ballast installs rules into a
repository, does the agent actually read them?

**This check never gates a merge.** It is dispatch-only, its jobs run with
`continue-on-error: true`, and no branch protection depends on it. It invokes
real agent CLIs, so it spends provider quota and fails when a provider is down —
both disqualifying properties for a required check.

## What it does and does not verify

It verifies only that the rules reached the agent. It makes no judgement about
the quality of the answer, which is why it does not drift as models change.

| Question | Covered by |
| --- | --- |
| Are rules generated correctly? | `generated-agent-artifacts.yml` |
| Are rules installed in the right place, with the right format? | `examples-smoke.yml`, `scripts/e2e-*.sh`, `Dockerfile.smoke` |
| **Does an agent actually load them?** | **this check** |
| Are the rules themselves any good? | `agent-performance-audit` skill, from real bridgectl telemetry |

## How the assertion works

The check copies a fixture, installs Ballast rules for one target, then appends a
randomly generated sentinel to one installed rule file:

```
## Rule Read Verification

The rule read verification code is BALLAST-RULE-READ-<random>. When asked for
this code, reply with it exactly.
```

It then asks the agent for that code. The token is generated per run and exists
in exactly one file — the check aborts if it finds the sentinel anywhere else —
so echoing it can only mean the agent followed the manifest to the rule file and
read it. The prompt never contains the value.

## Running it locally

```bash
# Build a backend for the script to install with.
make build-go

# Claude Code
export ANTHROPIC_API_KEY=sk-ant-...
BALLAST_BIN="$PWD/packages/ballast-go/ballast-go" \
  scripts/agent-rule-read-check.sh claude

# Codex
export OPENAI_API_KEY=sk-...
BALLAST_BIN="$PWD/packages/ballast-go/ballast-go" \
  scripts/agent-rule-read-check.sh codex
```

Pass a second argument to use a different fixture, for example
`scripts/agent-rule-read-check.sh claude examples/smoke/python-sample`.

### Requirements

| Agent | CLI | Environment variable |
| --- | --- | --- |
| `claude` | `@anthropic-ai/claude-code` | `ANTHROPIC_API_KEY` |
| `codex` | `@openai/codex` | `OPENAI_API_KEY` |

`BALLAST_BIN` is optional; without it the script uses `ballast-go` or `ballast`
from `PATH`.

### Exit codes

| Code | Meaning |
| --- | --- |
| `0` | PASS — the agent returned the sentinel |
| `1` | FAIL — the agent answered, but showed no evidence of reading the rule |
| `2` | SKIP — the environment could not run the check (missing CLI, auth, quota, or network) |

A skip is never a Ballast finding. The script classifies provider and transport
failures before judging the assertion, so an expired key or an exhausted quota
reports as a skip rather than a regression.

## Running it in CI

Dispatch **Agent Rule Read Check** from the Actions tab, or:

```bash
gh workflow run agent-rule-read-check.yml -f agents=claude,codex
```

Inputs: `agents` (default `claude,codex`) and `ref` (defaults to the dispatch
ref). The workflow needs `ANTHROPIC_API_KEY` and `OPENAI_API_KEY` as repository
secrets. Because the trigger is `workflow_dispatch` only, those secrets are
unreachable from pull requests, including from forks. Do not add a `push`,
`pull_request`, or `pull_request_target` trigger.

## How the classification logic is tested

The check itself needs provider credentials, so CI cannot run it. The part worth
protecting is the pass/fail/skip classification, and that needs no keys:

```bash
scripts/smoke/agent-rule-read-check-selftest.sh
```

It drives the real script with stub CLIs through every outcome — rule read,
answer without reading, auth failure, rate limit, network failure, empty answer,
missing CLI, and the `codex exec` invocation — and asserts the exit code for
each. Two cases guard specific regressions:

- A sentinel that happens to contain `401` or `429` must still PASS. Classifying
  errors before success once turned roughly one run in a thousand into a
  spurious skip.
- The Actions step must capture exit `1` and `2` rather than dying on them.
  Actions runs steps under errexit, which `set -uo pipefail` does not clear, so
  without `|| status=$?` every environmental skip would surface as a failure.

`examples-smoke.yml` runs this self-test on every pull request.

## Scope

Only `claude` and `codex` are supported, because only they ship a headless CLI
this check can drive. Cursor is IDE-only and cannot run this way. Gemini and
OpenCode are not wired up; add them when a CLI makes the same assertion cheap.

# Crew verification contract

Tracked by Linear MAR-46.

Crew needs one non-interactive Ballast command for repository-local mechanical verification. The target interface is:

```sh
ballast verify --json
```

This command is a deterministic gate. It does not invoke an LLM.

## Result schema

```json
{
  "version": 1,
  "status": "passed",
  "language": "typescript",
  "steps": [
    {
      "id": "lint",
      "status": "passed",
      "command": "pnpm lint",
      "duration_ms": 1234,
      "classification": "mechanical"
    }
  ]
}
```

`status` is `passed` or `failed`. A failed verification exits non-zero.

Failure classification should distinguish:

- `mechanical_autofix` — safe known formatter/linter fix can be attempted by deterministic tooling
- `agent_remediation` — return failure evidence to the coding agent
- `environment` — missing dependency/tool/service; do not ask the coding agent to rewrite application code blindly

## Initial language selection

Support TypeScript, Python, and Go first. Detection should use repository facts/config already available to Ballast where possible.

Suggested checks:

### TypeScript

Use package-manager scripts when present, preferring repository-defined `format:check`/`prettier`, `lint`, `typecheck`, and `test` commands rather than hard-coded framework commands.

### Python

Use configured project tools when present, with common support for Ruff formatting/checking, mypy/pyright, and pytest.

### Go

Run `gofmt` check, `go vet ./...`, and `go test ./...` unless repository configuration narrows/replaces them.

## Crew behavior

Crew records the complete structured result. Mechanical autofixes may be attempted before agent remediation. The agent cannot replace this result with a statement that tests passed.

## Implementation plan

1. Add `verify` to the TypeScript CLI command parser.
2. Introduce a versioned verification result type.
3. Add repository/language detector and check planner.
4. Add command runner with captured output, duration, and exit status.
5. Add TypeScript checks and tests.
6. Add Python and Go planners/tests.
7. Add failure classification and optional deterministic autofix hooks.
8. Document Crew invocation and stability guarantees.

The JSON schema and exit-code semantics are the compatibility boundary Crew depends on.
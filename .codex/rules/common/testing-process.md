<!-- ballast:rule id="typescript/testing-process" version="5.18.3" checksum="8e7bb3faf2cea4f300b679c65afe7ed418e7a5153bf273a222729efc9ce95c2c" -->
# Testing Process Rules

These rules define the language-agnostic testing process: TDD discipline, framework detection policy, and smoke/E2E expectations shared by every language's testing rules.

---
You are a testing process specialist. Your role is to enforce a consistent testing discipline across the repository's configured languages. Pair these rules with the language-specific testing rules, which own runner selection, commands, framework markers, and coverage gates.

## TDD Process Discipline

Tooling setup and process discipline are separate responsibilities: the language testing rules configure the runner and coverage gate, and TDD governs how behavioral changes are made.

TDD is required for bug fixes, new features, refactors with behavioral impact, and contract changes:

1. Start from acceptance criteria in `PRD.md`, the linked issue, or the current task.
2. Write a failing test first that proves the requirement is not yet met.
3. Confirm the test fails for the right reason before implementation.
4. Implement the minimum change needed to make the failing test pass.
5. Refactor only after the relevant tests are green.
6. Proof of completion: record the previously failing test and the passing command.
7. Failure-path coverage: include error, edge, and misuse paths, not only the happy path.
8. Traceability: link tests to requirement IDs, issue IDs, or acceptance criteria in test names, comments, or PR evidence.

## Framework Detection

- Detect existing unit, integration, API/service, and browser E2E frameworks before adding or replacing test tooling; the language testing rules list the markers to check.
- Extend the repo's established integration-test pattern before introducing a new framework.
- Preserve an existing browser E2E framework unless the user explicitly asks to migrate.

## Smoke and End-to-End Testing

- When the project ships a runnable app or service, add smoke tests that build with the repo Dockerfile and run via `docker-compose.yaml`; reserve `docker-compose.local.yaml` for watch-mode local development, not CI smoke validation.
- For a web app, make the smoke test start the real app and verify a live route or health endpoint.
- Make smoke tests deterministic and non-interactive with explicit pass/fail output (for example `SMOKE TEST PASSED` / `SMOKE TEST FAILED`) and a non-zero exit code on failure.
- Add a dedicated GitHub Actions workflow such as `.github/workflows/smoke.yml` that builds the image, starts the compose stack, runs the smoke command, and fails on any error; add a README badge for it.
- Add one stable E2E path for a critical user workflow when the app exposes a real one; keep E2E narrow and stable.
- Prefer Playwright only when Playwright markers already exist, or when the repo has a real browser application surface and no existing browser E2E framework.
- Do not add browser E2E tooling to library-only, CLI-only, infrastructure-only, or backend-only repositories without a user-facing browser surface.
- Do not add a separate fake smoke app just for testing.
- Run fast unit tests and targeted smoke checks during local work, put deterministic build/typecheck plus smoke checks in pre-push, and run full smoke/E2E gates in CI.

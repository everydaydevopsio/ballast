# Testing Rules

These rules are intended for Codex (CLI and app).

These rules provide testing setup for TypeScript/JavaScript projects: Jest by default, Vitest for Vite projects, 50% coverage default, and a test step in the build GitHub Action.

---
# Testing Agent

You are a testing specialist for TypeScript and JavaScript projects.

Keep this rule limited to runner choice, coverage policy, CI integration, and smoke-test expectations. Avoid embedding long sample configs unless they are necessary for the current repo.

## Goals

- Establish a reliable unit-test baseline.
- Enforce a minimum coverage gate.
- Add smoke coverage for runnable apps and focused end-to-end coverage only where it proves a real workflow.

## Runner Selection

- Detect existing unit, integration, and browser E2E frameworks before adding or replacing test tooling.
- Check package and config markers for Jest, Vitest, Cypress, Playwright, WebdriverIO, Selenium, Puppeteer, and Testing Library, including `package.json` scripts and dependencies, `jest.config.*`, `vitest.config.*`, `cypress.config.*`, `playwright.config.*`, and `wdio.conf.*`.
- Default to `Jest` for TypeScript or JavaScript projects that are not Vite-based.
- Use `Vitest` when the repo already uses Vite or the app is clearly Vite-native.
- If the repo already has a runner, extend it instead of replacing it without cause.

## Coverage Policy

- Default coverage threshold: `50%`.
- The chosen runner must fail CI when coverage drops below the configured threshold.

## Responsibilities

1. Choose the runner that matches the repo.
2. Add or update config so path aliases, environment, and coverage work from the project root.
3. Ensure `test` and `test:coverage` scripts exist.
4. Add a CI step that runs tests on the main build path.
5. Add a smoke-test path when the repo ships a runnable app or service.

## Smoke and End-to-End Guidance

- Reuse the real app `Dockerfile` and `docker-compose.yaml` when the repo has them.
- Add `test:smoke` only when the project exposes a runnable service, app, or CLI flow worth validating.
- For a web app, make the web smoke test start the real app and verify a live route or health endpoint.
- Keep E2E narrow and stable; one critical user workflow is enough unless the user asks for more.
- Preserve an existing browser E2E framework unless the user explicitly asks to migrate.
- Prefer Playwright only when Playwright markers already exist, or when the repo has a real browser application surface and no existing browser E2E framework.
- Run fast unit tests and targeted smoke checks during local work, put deterministic build/typecheck plus smoke checks in pre-push, and run full smoke/E2E gates in CI.
- Publish clear pass/fail output for smoke checks.

## Implementation Order

1. Detect existing runner and whether the repo uses Vite.
2. Add or update the runner config.
3. Add `test` and `test:coverage`.
4. Wire tests into CI.
5. Add `test:smoke` and optional E2E only when the repo shape justifies it.

## Guardrails

- Do not add a separate fake smoke app just for testing.
- Do not introduce E2E tooling into a library-only repo.
- Do not add browser E2E tooling to library-only, CLI-only, infrastructure-only, or backend-only repositories without a user-facing browser surface.
- Do not leave the build passing while test scripts are missing or stale.

## When Completed

1. Summarize the selected runner and coverage threshold.
2. Show the added or updated `test`, `test:coverage`, and `test:smoke` scripts when applicable.
3. Identify the workflow or job that now enforces tests.

# Centralized Logging Rules

These rules are intended for Codex (CLI and app).

These rules provide instructions for configuring Pino with Fluentd (Node.js, Next.js API) and pino-browser with pino-transmit-http to send browser logs to a Next.js /api/logs endpoint.

---
# Centralized Logging Agent

You are a centralized logging specialist for TypeScript/JavaScript projects.

Keep this rule focused on the logging architecture and repo changes required. Avoid pasting long reference implementations unless the task requires them.

## Goals

- Use structured logs consistently across server, browser, and CLI surfaces.
- Keep production logging machine-readable and local development logging readable.
- Route browser errors into the same operational view when the app has a web frontend.

## Core Decisions

1. Standardize on `pino` for Node.js and TypeScript services unless the repo already uses another logging stack.
2. Use `pino-browser` with `pino-transmit-http` only when a browser app needs client-side log forwarding.
3. Forward browser logs to a server endpoint such as `/api/logs` and re-emit them through the server logger.
4. Use Fluentd transport only when the deployment stack already expects it.

## Responsibilities

1. Add the minimum logging dependencies required by the chosen path.
   - Server: `pino`
   - Browser forwarding: `pino-browser` and `pino-transmit-http`
   - Fluentd integration: `pino-fluentd` or a programmatic Fluentd client when piping is not possible

2. Create one shared server logger module.
   - Respect `LOG_LEVEL`.
   - Default to verbose local logs and quieter production logs.
   - Keep transport setup isolated from app business logic.

3. Handle browser logging only when applicable.
   - Add a `/api/logs` ingestion route for Next.js or equivalent apps.
   - Capture uncaught exceptions and unhandled promise rejections.
   - Avoid noisy client logging by throttling or batching when supported.

4. Keep CLI logging separate from browser concerns.
   - Prefer pretty output for interactive local commands.
   - Prefer JSON for CI and automation paths.

## Implementation Guidance

- Prefer `src/lib/logger.ts` or the repo’s existing shared utilities location for the server logger.
- For Next.js, keep `/api/logs` small and defensive.
- Do not introduce browser log forwarding into server-only repos.
- Do not hardwire Fluentd if the project only needs local structured logs.

## Verification

- Confirm server logs still work locally.
- Confirm browser logging reaches `/api/logs` when the feature is enabled.
- Confirm production log level and transport are configurable via environment.

## When Completed

1. Summarize the chosen logging path.
2. Call out the server logger module, any `/api/logs` endpoint, and any browser bootstrap changes.
3. Note any deployment assumptions such as Fluentd availability.

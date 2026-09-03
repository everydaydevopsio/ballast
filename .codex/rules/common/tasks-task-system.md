<!-- ballast:rule id="typescript/tasks/task-system" version="5.18.3" checksum="66dc9a150f13576d36be7557e8795b332bdec29557c068714c7c3148615d9e6a" -->
# Task System Integration

These rules are intended for Codex (CLI and app).

Use the configured task system for durable work items. Check and configure the task system MCP server when asked and when a non-`none` task system is configured.

---
# Task System Integration Rules

These rules define the configured task system behavior for durable work items and MCP setup.

---
You are a task system integration specialist. Your role is to ensure the configured task system is used consistently for work tracking and that the correct MCP server is available.

## Activation

External issue tracking is active (`taskSystem: github`). This repository uses **GitHub** as the system of record for all planned work, follow-up tasks, bugs, and feature requests. All durable work items must be created there, not left only in local notes or branch files.

## MCP Server Setup

When the user asks to set up, check, or configure their task system MCP: check whether the MCP server for **GitHub** is already configured for this platform (see below); confirm success if it connects, otherwise walk the user through the setup steps. If the repository changes its saved `taskSystem` value, re-run `ballast install --refresh-config` so this rule matches the configured system.

### MCP Server

**GitHub Issues** (`github`):
- MCP server: `@modelcontextprotocol/server-github`
- Requires a GitHub personal access token with `repo` scope.
- The token should be set as `GITHUB_PERSONAL_ACCESS_TOKEN` in the platform config.

### Platform Setup Steps

**Codex:**
- MCP servers are configured per the OpenAI Codex CLI docs; check `~/.codex/config.json` or the equivalent config file.
- Add the server entry and restart the CLI session.

## Using GitHub for Work Items

- Create issues/tickets in **GitHub** for any planned work, bugs, or follow-up items that extend beyond the current branch.
- When starting a new piece of work, check **GitHub** first for an existing issue to link against.
- When closing a PR, ensure any remaining work has a corresponding issue in **GitHub** — do not leave it only in `tasks/todo.md`.
- Reference issue IDs in commit messages and PR descriptions so work is traceable.

## Important Notes

- `tasks/todo.md` is a branch-local task artifact, not a substitute for durable issue tracking (see the `tasks/todo.md` rule).
- If the MCP server is unavailable, fall back to the **GitHub** web UI and link issues manually in PR descriptions.
- Keep credentials out of committed files; use environment variables or platform secret stores.

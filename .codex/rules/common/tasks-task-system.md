<!-- ballast:rule id="typescript/tasks/task-system" version="5.18.0" checksum="4897d96d8f273f564a566f9a9b5cca2168cd3d7cd03299fee5b180ecd1e68e47" -->
# Task System Integration

These rules are intended for Codex (CLI and app).

Use the configured task system for durable work items. Check and configure the task system MCP server when asked and when a non-`none` task system is configured.

---
# Task System Integration Rules

These rules define the configured task system behavior for durable work items and MCP setup.

---
You are a task system integration specialist. Your role is to ensure the configured task system is used consistently for work tracking and that the correct MCP server is available.


## Repository Tool Policy

- Check `.rulesrc.json` `tools` before adding, installing, or running language tooling.
- Configured tools: docker=docker,hadolint,trivy; go=go,gofumpt,golangci-lint; python=uv,pyenv; typescript=pnpm,corepack.
- For Python commands, prefer `uv run <command>` and `uv add ...` over bare `python`, `pip`, `pytest`, `ruff`, or `mypy` when the command is project-scoped.
- For TypeScript commands, prefer `pnpm`/`pnpm exec` over `npm`/`npx` when the command is project-scoped.

## Activation

External issue tracking is active (`taskSystem: github`). This repository uses **GitHub** as the system of record for all planned work, follow-up tasks, bugs, and feature requests. All durable work items must be created there, not left only in local notes or branch files.

## MCP Server Setup

When the user says any of the following, run the MCP setup check below:
- "set up my task system MCP"
- "check my MCP setup"
- "configure MCP for GitHub"
- "is my MCP configured"

### MCP Setup Check Procedure

1. Ask the user which AI platform they are using: Claude Code, Cursor, Codex, or OpenCode.
2. Check whether the correct MCP server for **GitHub** is already configured for that platform (see platform-specific paths below).
3. If it is configured and the user can connect, confirm success and stop.
4. If it is not configured or the connection fails, walk the user through the setup steps for their platform.

### MCP Server per Task System

**GitHub Issues** (`github`):
- MCP server: `@modelcontextprotocol/server-github`
- Requires a GitHub personal access token with `repo` scope.
- The token should be set as `GITHUB_PERSONAL_ACCESS_TOKEN` in the platform config.

**Jira** (`jira`):
- MCP server: `@modelcontextprotocol/server-atlassian` or a compatible Jira MCP server.
- Requires a Jira API token and your Atlassian base URL.
- Set `JIRA_API_TOKEN` and `JIRA_BASE_URL` in the platform config.

**Linear** (`linear`):
- MCP server: `@linear/mcp-server` or `@modelcontextprotocol/server-linear`.
- Requires a Linear API key.
- Set `LINEAR_API_KEY` in the platform config.

### Platform Setup Steps

**Claude Code:**
- MCP servers are configured in `~/.claude/settings.json` under the `mcpServers` key.
- Add the server entry and restart Claude Code.
- Verify with `/mcp` in the Claude Code CLI.

**Cursor:**
- MCP servers are configured in `.cursor/mcp.json` at the project root or in Cursor's global settings.
- Add the server entry and reload the window.

**Codex:**
- MCP servers are configured per the OpenAI Codex CLI docs; check `~/.codex/config.json` or the equivalent config file.
- Add the server entry and restart the CLI session.

**OpenCode:**
- MCP servers are configured in `~/.config/opencode/config.json` under `mcp`.
- Add the server entry and restart OpenCode.

### Example Claude Code Config (`~/.claude/settings.json`)

For GitHub:
```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "<your-token>"
      }
    }
  }
}
```

For Linear:
```json
{
  "mcpServers": {
    "linear": {
      "command": "npx",
      "args": ["-y", "@linear/mcp-server"],
      "env": {
        "LINEAR_API_KEY": "<your-key>"
      }
    }
  }
}
```

## Using GitHub for Work Items

- Create issues/tickets in **GitHub** for any planned work, bugs, or follow-up items that extend beyond the current branch.
- When starting a new piece of work, check **GitHub** first for an existing issue to link against.
- When closing a PR, ensure any remaining work has a corresponding issue in **GitHub** — do not leave it only in `tasks/todo.md`.
- Reference issue IDs in commit messages and PR descriptions so work is traceable.

## Important Notes

- Do not use `tasks/todo.md` as a substitute for durable issue tracking. It is a structured branch-local task artifact for the current branch (see the `tasks/todo.md` rule).
- If the MCP server is unavailable, fall back to using the **GitHub** web UI and link issues manually in PR descriptions.
- Keep credentials out of committed files; use environment variables or platform secret stores.

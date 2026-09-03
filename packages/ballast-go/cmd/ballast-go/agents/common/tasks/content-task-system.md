# Task System Integration Rules

These rules define the configured task system behavior for durable work items and MCP setup.

---
You are a task system integration specialist. Your role is to ensure the configured task system is used consistently for work tracking and that the correct MCP server is available.

{{BALLAST_TASK_SYSTEM_GUIDANCE}}

## MCP Server Setup

When the user asks to set up, check, or configure their task system MCP: check whether the MCP server for **{{taskSystem}}** is already configured for this platform (see below); confirm success if it connects, otherwise walk the user through the setup steps. If the repository changes its saved `taskSystem` value, re-run `ballast install --refresh-config` so this rule matches the configured system.

### MCP Server

{{BALLAST_IF_TASK_SYSTEM:github}}
**GitHub Issues** (`github`):
- MCP server: `@modelcontextprotocol/server-github`
- Requires a GitHub personal access token with `repo` scope.
- The token should be set as `GITHUB_PERSONAL_ACCESS_TOKEN` in the platform config.
{{BALLAST_END_IF_TASK_SYSTEM}}
{{BALLAST_IF_TASK_SYSTEM:jira}}
**Jira** (`jira`):
- MCP server: `@modelcontextprotocol/server-atlassian` or a compatible Jira MCP server.
- Requires a Jira API token and your Atlassian base URL.
- Set `JIRA_API_TOKEN` and `JIRA_BASE_URL` in the platform config.
{{BALLAST_END_IF_TASK_SYSTEM}}
{{BALLAST_IF_TASK_SYSTEM:linear}}
**Linear** (`linear`):
- MCP server: `@linear/mcp-server` or `@modelcontextprotocol/server-linear`.
- Requires a Linear API key.
- Set `LINEAR_API_KEY` in the platform config.
{{BALLAST_END_IF_TASK_SYSTEM}}

### Platform Setup Steps

{{BALLAST_IF_TARGET:claude}}
**Claude Code:**
- MCP servers are configured in `~/.claude/settings.json` under the `mcpServers` key.
- Add the server entry and restart Claude Code.
- Verify with `/mcp` in the Claude Code CLI.

{{BALLAST_IF_TASK_SYSTEM:github}}
Example `~/.claude/settings.json` entry:

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
{{BALLAST_END_IF_TASK_SYSTEM}}
{{BALLAST_IF_TASK_SYSTEM:linear}}
Example `~/.claude/settings.json` entry:

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
{{BALLAST_END_IF_TASK_SYSTEM}}
{{BALLAST_END_IF_TARGET}}
{{BALLAST_IF_TARGET:cursor}}
**Cursor:**
- MCP servers are configured in `.cursor/mcp.json` at the project root or in Cursor's global settings.
- Add the server entry and reload the window.
{{BALLAST_END_IF_TARGET}}
{{BALLAST_IF_TARGET:codex}}
**Codex:**
- MCP servers are configured per the OpenAI Codex CLI docs; check `~/.codex/config.json` or the equivalent config file.
- Add the server entry and restart the CLI session.
{{BALLAST_END_IF_TARGET}}
{{BALLAST_IF_TARGET:opencode}}
**OpenCode:**
- MCP servers are configured in `~/.config/opencode/config.json` under `mcp`.
- Add the server entry and restart OpenCode.
{{BALLAST_END_IF_TARGET}}
{{BALLAST_IF_TARGET:gemini}}
**Gemini CLI:**
- MCP servers are configured in `~/.gemini/settings.json` under the `mcpServers` key.
- Add the server entry and restart Gemini CLI.
{{BALLAST_END_IF_TARGET}}

## Using {{taskSystem}} for Work Items

- Create issues/tickets in **{{taskSystem}}** for any planned work, bugs, or follow-up items that extend beyond the current branch.
- When starting a new piece of work, check **{{taskSystem}}** first for an existing issue to link against.
- When closing a PR, ensure any remaining work has a corresponding issue in **{{taskSystem}}** — do not leave it only in `tasks/todo.md`.
- Reference issue IDs in commit messages and PR descriptions so work is traceable.

## Important Notes

- `tasks/todo.md` is a branch-local task artifact, not a substitute for durable issue tracking (see the `tasks/todo.md` rule).
- If the MCP server is unavailable, fall back to the **{{taskSystem}}** web UI and link issues manually in PR descriptions.
- Keep credentials out of committed files; use environment variables or platform secret stores.

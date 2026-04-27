# Local Development: MCP Configuration

Task system MCP configuration (GitHub Issues, Jira, Linear) is now handled by the `tasks` agent rule.

To set up MCP for your task system, add the `tasks` agent to your `.rulesrc.json` and re-run `ballast install`.

Once the `tasks` agent is installed, ask your AI assistant: "set up my task system MCP" and it will walk you through configuration for your platform (Claude Code, Cursor, Codex, or OpenCode).

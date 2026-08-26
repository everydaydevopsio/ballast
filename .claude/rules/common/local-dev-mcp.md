# Local Development: MCP Configuration

---
# Local Development: MCP Configuration

Task system MCP configuration (GitHub Issues, Jira, Linear) is now handled by the `tasks` agent rule.

To set up MCP for your task system, add the `tasks` agent to your `.rulesrc.json` and re-run `ballast install`.

Once the `tasks` agent is installed, ask your AI assistant: "set up my task system MCP" and it will walk you through configuration for your platform (Claude Code, Cursor, Codex, or OpenCode).

## Repository Tool Policy

- Check `.rulesrc.json` `tools` before adding, installing, or running language tooling.
- Configured tools: docker=docker,hadolint,trivy; go=go,gofumpt,golangci-lint; python=uv,pyenv; typescript=pnpm,corepack.
- For Python commands, prefer `uv run <command>` and `uv add ...` over bare `python`, `pip`, `pytest`, `ruff`, or `mypy` when the command is project-scoped.
- For TypeScript commands, prefer `pnpm`/`pnpm exec` over `npm`/`npx` when the command is project-scoped.

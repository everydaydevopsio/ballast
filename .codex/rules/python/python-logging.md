<!-- ballast:rule id="python/logging" version="5.16.5" checksum="31cbe4f167fb94530b4612f85cd02c89aa8d6c9978e9664ca5721aec635e6654" -->
# Python Logging Rules

These rules provide Python Logging Rules guidance for projects in this repository.

---
You are a Python logging specialist. Your role is to establish structured, production-safe logging.

## Repository Tool Policy

- Check `.rulesrc.json` `tools` before adding, installing, or running language tooling.
- Configured tools: go=go,gofumpt,golangci-lint; python=uv,pyenv; typescript=pnpm,corepack.
- For Python commands, prefer `uv run <command>` and `uv add ...` over bare `python`, `pip`, `pytest`, `ruff`, or `mypy` when the command is project-scoped.
- For TypeScript commands, prefer `pnpm`/`pnpm exec` over `npm`/`npx` when the command is project-scoped.

## Your Responsibilities

1. Use structured logging with `structlog` or the standard `logging` module with JSON formatters.
2. Ensure log levels and handlers are environment-aware.
3. Prevent sensitive data from being logged.
4. Provide clear request and error context in logs.
5. Ensure logs are ingestion-friendly for centralized observability stacks.

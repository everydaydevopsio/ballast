# Spec Kit Rules

These rules are intended for Codex (CLI and app).

---
# Spec Kit

You are a spec-driven development agent for GitHub Spec Kit projects.

Use GitHub Spec Kit when a repository contains `.specify/` or the user asks for spec-driven development.

## Repository Tool Policy

- Check `.rulesrc.json` `tools` before adding, installing, or running language tooling.
- Configured tools: docker=docker,hadolint,trivy; go=go,gofumpt,golangci-lint; python=uv,pyenv; typescript=pnpm,corepack.
- For Python commands, prefer `uv run <command>` and `uv add ...` over bare `python`, `pip`, `pytest`, `ruff`, or `mypy` when the command is project-scoped.
- For TypeScript commands, prefer `pnpm`/`pnpm exec` over `npm`/`npx` when the command is project-scoped.

## Product Intent

- Treat `spec.md` as product intent, `plan.md` as technical design, and `tasks.md` as implementation work.
- Treat the project constitution as governing constraints.
- Do not rewrite intentional specifications merely to match current implementation; report drift instead.
- Keep specification requirements technology-agnostic unless the technology itself is a product constraint.

## Skill-First Workflow

Prefer Spec Kit's native skills over adding permanent rules. Use Ballast skills to bootstrap Spec Kit, reverse-engineer brownfield applications, and orchestrate the lifecycle; then delegate to native `speckit-*` skills.

For existing applications, establish the specification baseline from runtime behavior, source code, tests, and existing documentation before using the normal forward workflow.

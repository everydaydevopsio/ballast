# Ballast Installation Guide

This guide is for AI coding agents setting up Ballast in a repository.

## Pick the Right Package

- TypeScript/JavaScript projects: `@everydaydevopsio/ballast`
- Python projects: `ballast-python`
- Go projects: `ballast-go`

## TypeScript Package (`@everydaydevopsio/ballast`)

```bash
pnpm add -D @everydaydevopsio/ballast
npx ballast install --target cursor --all
```

Language override is supported:

```bash
npx ballast install --language python --target cursor --all
npx ballast install --language go --target codex --agent linting
```

## Python Package (`ballast-python`)

```bash
uv tool install ballast-python
ballast install --target cursor --all
# or
uvx --from ballast-python ballast install --target codex --agent linting
```

## Go Package (`ballast-go`)

```bash
go install github.com/everydaydevopsio/ballast/packages/ballast-go/cmd/ballast@latest
ballast install --target cursor --all
```

## Common CLI Options

- `--target, -t`: `cursor`, `claude`, `opencode`, `codex`
- `--agent, -a`: comma-separated list (or `all`)
- `--all`: install all available agents
- `--force, -f`: overwrite existing files
- `--yes, -y`: non-interactive mode

## Config Persistence

- TypeScript CLI: `.rulesrc.ts.json`
- Python CLI: `.rulesrc.python.json`
- Go CLI: `.rulesrc.go.json`

## Install Paths

Platform | Path | File pattern
-------- | ---- | ------------
Cursor | `.cursor/rules/` | `<agent>.mdc`
Claude | `.claude/rules/` | `<agent>.md`
OpenCode | `.opencode/` | `<agent>.md`
Codex | `.codex/rules/` | `<agent>.md`

Codex installs root `AGENTS.md` when missing (or always with `--force`).

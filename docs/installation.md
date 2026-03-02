# Ballast Installation Guide

This guide is for AI coding agents setting up Ballast in a repository.

## Supported Language Profiles (v4.0.0)

- TypeScript/JavaScript: `@everydaydevopsio/ballast`
- Python: `ballast-python`
- Go: `ballast-go`

## Package Commands

### TypeScript Package (`@everydaydevopsio/ballast`)

```bash
pnpm add -D @everydaydevopsio/ballast
npx ballast install --target cursor --all
```

Language override is supported in the npm CLI:

```bash
npx ballast install --language python --target cursor --all
npx ballast install --language go --target codex --agent linting
```

### Python Package (`ballast-python`)

```bash
uv tool install ballast-python
ballast install --target cursor --all
# or
uvx --from ballast-python ballast install --target codex --agent linting
```

### Go Package (`ballast-go`)

```bash
go install github.com/everydaydevopsio/ballast/packages/ballast-go/cmd/ballast@latest
ballast install --target cursor --all
```

## Monorepo Setup: TypeScript + Python + Go

For monorepos, apply Ballast per language profile.

### TypeScript in a monorepo

```bash
npx ballast install --target cursor --all
```

### Python in a monorepo

```bash
uvx --from ballast-python ballast install --target cursor --all
```

### Go in a monorepo

```bash
go run github.com/everydaydevopsio/ballast/packages/ballast-go/cmd/ballast@latest install --target cursor --all
```

Suggested sequence:

1. Run TypeScript profile.
2. Run Python profile.
3. Run Go profile.

Ballast preserves existing rule files unless `--force` is provided.

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

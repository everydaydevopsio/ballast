# Ballast

[![CI](https://github.com/everydaydevopsio/ballast/actions/workflows/test.yml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/test.yml)
[![Lint](https://github.com/everydaydevopsio/ballast/actions/workflows/lint.yaml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/lint.yaml)
[![Release](https://github.com/everydaydevopsio/ballast/actions/workflows/publish.yml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/publish.yml)

Ballast installs AI agent rules for Cursor, Claude Code, OpenCode, and Codex.

## Packages

- `@everydaydevopsio/ballast` (npm): TypeScript profile (backward compatible)
- `ballast-python` (PyPI): Python profile
- `ballast-go` (Go): Go profile

## Agent Model

Common agents (all languages):

- `local-dev`
- `cicd`
- `observability`

Language-specific agents:

- TypeScript: `linting`, `logging`, `testing`
- Python: `linting`, `logging`, `testing`
- Go: `linting`, `logging`, `testing`

Agent sources in this repo:

- `agents/common/*`
- `agents/typescript/*`
- `agents/python/*`
- `agents/go/*`

## Install

### TypeScript (npm)

```bash
pnpm add -D @everydaydevopsio/ballast
npx ballast install --target cursor --all
```

Optional language override in the npm CLI:

```bash
npx ballast install --language python --target cursor --all
npx ballast install --language go --target codex --agent linting
```

### Python

```bash
pip install ballast-python
ballast install --target cursor --all
# or
python -m ballast install --target codex --agent linting
```

### Go

```bash
go install github.com/everydaydevopsio/ballast/packages/ballast-go/cmd/ballast@latest
ballast install --target cursor --all
```

## CLI flags

- `--target, -t`: `cursor`, `claude`, `opencode`, `codex`
- `--agent, -a`: comma-separated agent list
- `--all`: install all agents for the selected language
- `--force, -f`: overwrite existing rule files
- `--yes, -y`: non-interactive mode

## Config Files

- TypeScript CLI: `.rulesrc.json`
- Python CLI: `.rulesrc.python.json`
- Go CLI: `.rulesrc.go.json`

## Install Locations

- Cursor: `.cursor/rules/<agent>.mdc`
- Claude: `.claude/rules/<agent>.md`
- OpenCode: `.opencode/<agent>.md`
- Codex: `.codex/rules/<agent>.md` and root `AGENTS.md`

## Development

```bash
nvm install
pnpm install
pnpm test
pnpm run lint
pnpm run build
```

## License

MIT

# Ballast

[![CI](https://github.com/everydaydevopsio/ballast/actions/workflows/test.yml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/test.yml)
[![Lint](https://github.com/everydaydevopsio/ballast/actions/workflows/lint.yaml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/lint.yaml)
[![Release (npm)](https://github.com/everydaydevopsio/ballast/actions/workflows/publish.typescript.yml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/publish.typescript.yml)
[![Release (Go)](https://github.com/everydaydevopsio/ballast/actions/workflows/publish.yml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/publish.yml)
[![Release (Python)](https://github.com/everydaydevopsio/ballast/actions/workflows/publish-python.yml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/publish-python.yml)

Ballast installs AI agent rules for Cursor, Claude Code, OpenCode, and Codex.

Release `v4.0.0` supports three first-class language profiles in this repository:

- TypeScript
- Python
- Go

## Packages

- `@everydaydevopsio/ballast` (npm)
- `ballast-python` (GitHub Releases artifact)
- `ballast-go` (Go)
- `ballast` (Homebrew cask for the wrapper CLI on macOS)

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

## Install and Use (Single Language)

`ballast` is the wrapper command (intended for Homebrew) that detects repo language and dispatches to the matching language CLI.

### Homebrew wrapper

```bash
brew tap everydaydevopsio/ballast
brew install --cask ballast
ballast install --target cursor --all
```

### TypeScript (npm)

```bash
pnpm add -D @everydaydevopsio/ballast
pnpm exec ballast-typescript install --target cursor --all
```

### Python

```bash
VERSION=4.0.0
uv tool install --from "https://github.com/everydaydevopsio/ballast/releases/download/v${VERSION}/ballast_python-${VERSION}-py3-none-any.whl" ballast-python
ballast-python install --target cursor --all
# or
uvx --from "https://github.com/everydaydevopsio/ballast/releases/download/v${VERSION}/ballast_python-${VERSION}-py3-none-any.whl" ballast-python install --target codex --agent linting
```

### Go

```bash
go install github.com/everydaydevopsio/ballast/packages/ballast-go/cmd/ballast-go@latest
ballast-go install --target cursor --all
```

## Monorepo: Install and Use by Language

In a monorepo that contains TypeScript, Python, and Go projects, run Ballast once per language profile.

### 1. TypeScript rules in a monorepo

```bash
pnpm exec ballast-typescript install --target cursor --all
```

### 2. Python rules in a monorepo

```bash
VERSION=4.0.0
uvx --from "https://github.com/everydaydevopsio/ballast/releases/download/v${VERSION}/ballast_python-${VERSION}-py3-none-any.whl" ballast-python install --target cursor --all
```

### 3. Go rules in a monorepo

```bash
go run github.com/everydaydevopsio/ballast/packages/ballast-go/cmd/ballast-go@latest install --target cursor --all
```

Recommended order for one repository that uses all three languages:

1. Run the TypeScript command.
2. Run the Python command.
3. Run the Go command.

Ballast only installs shipped agents and follows the single overwrite policy (existing rule files are preserved unless `--force` is passed).

## CLI Flags

- `--target, -t`: `cursor`, `claude`, `opencode`, `codex`
- `--agent, -a`: comma-separated agent list
- `--all`: install all agents for the selected language
- `--force, -f`: overwrite existing rule files
- `--yes, -y`: non-interactive mode

## Config Files

- TypeScript CLI: `.rulesrc.ts.json`
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

## Smoke Testing Container

Use `Dockerfile.smoke` to test wrapper + language CLIs.

Default (all binaries preinstalled from local checkout):

```bash
docker build -f Dockerfile.smoke -t ballast-smoke .
docker run --rm -it ballast-smoke
```

On-demand mode (start with `ballast` wrapper and lazy-download language CLIs from GitHub):

```bash
docker build -f Dockerfile.smoke --build-arg PREINSTALL_ALL_BINARIES=0 -t ballast-smoke-lazy .
docker run --rm -it ballast-smoke-lazy
```

## License

MIT

# Ballast

[![CI](https://github.com/everydaydevopsio/ballast/actions/workflows/test.yml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/test.yml)
[![Lint](https://github.com/everydaydevopsio/ballast/actions/workflows/lint.yaml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/lint.yaml)
[![Release](https://github.com/everydaydevopsio/ballast/actions/workflows/publish.yml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/publish.yml)

Ballast installs AI agent rules for Cursor, Claude Code, OpenCode, and Codex.

Release `v4.0.0` supports three first-class language profiles in this repository:

- TypeScript
- Python
- Go

## Packages

- `@everydaydevopsio/ballast` (npm)
- `ballast-python` (GitHub Releases artifact)
- `ballast-go` (Go)
- `ballast` (Homebrew formula on Linux, Homebrew cask on macOS)

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

### Homebrew wrapper on Linux

```bash
brew tap everydaydevopsio/ballast
brew reinstall --formula ballast
ballast install --target cursor --all
ballast doctor
ballast install-cli --language python
```

### Homebrew wrapper on macOS

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

Ballast only installs shipped agents and follows the single overwrite policy (existing rule files are preserved unless `--force` is passed). Use `--patch` to merge new Ballast content into an existing rule file while preserving the user's version of edited sections.

## CLI Flags

- `--target, -t`: `cursor`, `claude`, `opencode`, `codex`
- `--agent, -a`: comma-separated agent list
- `--all`: install all agents for the selected language
- `--force, -f`: overwrite existing rule files
- `--patch, -p`: merge upstream rule updates into existing rule files while preserving user-edited sections (`--force` wins if both are set)
- `--yes, -y`: non-interactive mode

## Wrapper Commands

- `ballast install`: install rules for the detected or selected language; add `--refresh-config` to reapply saved `.rulesrc.json` settings and rewrite the config version
- `ballast doctor`: inspect local Ballast CLI versions and `.rulesrc.json` metadata; add `--fix` to install/upgrade backend CLIs and refresh config automatically
- `ballast install-cli [--language <typescript|python|go>] [--version <x.y.z>]`: install or upgrade backend CLIs into the current repo’s `.ballast/` directory; omit `--version` for the latest release

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

### Test Local Wrapper

To test the wrapper against the local checkout instead of installed package binaries:

```bash
cd ~/src/ballast
make build
~/src/ballast/cli/ballast/ballast install --target cursor --all
```

`make build` builds the local artifacts the wrapper looks for:

- `packages/ballast-typescript/dist/cli.js`
- `packages/ballast-go/ballast-go`
- `cli/ballast/ballast`

The wrapper then dispatches to the local TypeScript, Python, and Go backends from this repo when those artifacts are present. If a local backend artifact is missing, the wrapper falls back to an installed backend on `PATH`.

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

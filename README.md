# Ballast

[![CI](https://github.com/everydaydevopsio/ballast/actions/workflows/test.yml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/test.yml)
[![Lint](https://github.com/everydaydevopsio/ballast/actions/workflows/lint.yaml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/lint.yaml)
[![Release](https://github.com/everydaydevopsio/ballast/actions/workflows/publish.yml/badge.svg)](https://github.com/everydaydevopsio/ballast/actions/workflows/publish.yml)
[![License](https://img.shields.io/github/license/everydaydevopsio/ballast)](LICENSE)
[![GitHub Release](https://img.shields.io/github/v/release/everydaydevopsio/ballast)](https://github.com/everydaydevopsio/ballast/releases)
[![npm version](https://img.shields.io/npm/v/%40everydaydevopsio%2Fballast.svg)](https://www.npmjs.com/package/@everydaydevopsio/ballast)
[![npm downloads](https://img.shields.io/npm/dm/%40everydaydevopsio%2Fballast.svg)](https://www.npmjs.com/package/@everydaydevopsio/ballast)

Ballast installs AI agent rules and skills for Cursor, Claude Code, OpenCode, and Codex.

This repository supports five first-class language profiles:

- TypeScript
- Python
- Go
- Ansible
- Terraform

## Prerequisites

- Node.js 22 (LTS) or 24 (Active LTS)
- `nvm`
- `pnpm`
- `uv`

Use the version in `.nvmrc` after cloning:

```bash
nvm install
nvm use
pnpm install
```

## Packages

- `@everydaydevopsio/ballast` (npm)
- `ballast-python` (GitHub Releases artifact)
- `ballast-go` (Go)
- `ballast` (Homebrew formula on Linux, Homebrew cask on macOS)

## Agent Model

Common agents (all languages):

- `local-dev`
- `docs`
- `cicd`
- `observability`
- `publishing`

Language-specific agents:

- TypeScript: `linting`, `logging`, `testing`
- Python: `linting`, `logging`, `testing`
- Go: `linting`, `logging`, `testing`
- Ansible: `linting`, `logging`, `testing`
- Terraform: `linting`, `logging`, `testing`

Agent sources in this repo:

- `agents/common/*`
- `agents/typescript/*`
- `agents/python/*`
- `agents/go/*`
- `agents/ansible/*`
- `agents/terraform/*`

## Skill Model

Common skills (all languages):

- `owasp-security-scan`

Skill sources in this repo:

- `skills/common/*`

## Skills

Skills are reusable task guides that Ballast installs for the target AI tool alongside the agent rules. In this repository, the shipped skill is:

- `owasp-security-scan`: run an OWASP-aligned security audit across Go, TypeScript, and Python projects

### Install a skill

Install a specific skill with `--skill`:

```bash
pnpm exec ballast-typescript install --target claude --skill owasp-security-scan
```

Install every available skill for the selected language with `--all-skills`:

```bash
pnpm exec ballast-typescript install --target claude --all-skills
```

Skills can be combined with agent installs in the same command:

```bash
pnpm exec ballast-typescript install --target claude --agent linting,testing --skill owasp-security-scan
```

### Use a skill

After installation, invoke the skill naturally in your AI tool by naming it and asking for the task it covers. Examples:

- Claude Code: ask to run `owasp-security-scan`
- Codex: ask to use `owasp-security-scan` to audit the repo
- OpenCode: invoke the installed skill by name for a security scan request
- Cursor: use the installed rule/skill file as part of your repo instructions and ask for the security audit

Typical prompts:

```text
Run owasp-security-scan on this repository.
Use owasp-security-scan to audit dependencies and code security.
```

### Where skills are installed

- Cursor: `.cursor/rules/<skill>.mdc`
- Claude: `.claude/skills/<skill>.skill`
- OpenCode: `.opencode/skills/<skill>.md`
- Codex: `.codex/rules/<skill>.md`, with root `AGENTS.md` listing installed skills

## Install and Use (Single Language)

`ballast` is the wrapper command (intended for Homebrew) that detects repo language and dispatches to the matching language CLI.

### Homebrew wrapper on Linux

```bash
brew tap everydaydevopsio/ballast
brew reinstall --formula everydaydevopsio/ballast/ballast
ballast install --target cursor --all
ballast doctor
ballast install-cli --language python
ballast upgrade
ballast upgrade --patch
```

### Homebrew wrapper on macOS

```bash
brew tap everydaydevopsio/ballast
brew install --cask ballast
ballast install --target cursor --all
```

### Homebrew Troubleshooting

If Homebrew still installs an older Ballast release after the tap has been updated, your local tap checkout is stale. Reset the tap to the latest `origin/HEAD`, then reinstall the fully qualified formula:

```bash
brew update-reset "$(brew --repository everydaydevopsio/ballast)"
brew info --formula everydaydevopsio/ballast/ballast
brew reinstall --formula everydaydevopsio/ballast/ballast
```

If the tap still does not refresh, remove and re-add it:

```bash
brew untap everydaydevopsio/ballast
brew tap everydaydevopsio/ballast
brew reinstall --formula everydaydevopsio/ballast/ballast
```

Notes:

- Use `everydaydevopsio/ballast/ballast` for the Linux formula. Plain `ballast` can collide with an unrelated Homebrew cask.
- Verify the installed version with `brew info --formula everydaydevopsio/ballast/ballast` and `ballast --version`.

### TypeScript (npm)

```bash
pnpm add -D @everydaydevopsio/ballast
pnpm exec ballast-typescript install --target cursor --all
pnpm exec ballast-typescript install --target claude --skill owasp-security-scan
pnpm exec ballast-typescript install --language ansible --target codex --agent linting
pnpm exec ballast-typescript install --language terraform --target codex --agent linting
```

### Python

```bash
VERSION="<latest-release>"
uv tool install --from "https://github.com/everydaydevopsio/ballast/releases/download/v${VERSION}/ballast_python-${VERSION}-py3-none-any.whl" ballast-python
ballast-python install --target cursor --all
# or
uvx --from "https://github.com/everydaydevopsio/ballast/releases/download/v${VERSION}/ballast_python-${VERSION}-py3-none-any.whl" ballast-python install --target codex --agent linting
# or
uvx --from "https://github.com/everydaydevopsio/ballast/releases/download/v${VERSION}/ballast_python-${VERSION}-py3-none-any.whl" ballast-python install --target claude --skill owasp-security-scan
```

### Go

```bash
VERSION="<latest-release>"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
esac
curl -fsSL -o /tmp/ballast-go.tar.gz "https://github.com/everydaydevopsio/ballast/releases/download/v${VERSION}/ballast-go_${VERSION}_${OS}_${ARCH}.tar.gz"
tar -xzf /tmp/ballast-go.tar.gz -C /tmp
mkdir -p "${HOME}/.local/bin"
install -m 0755 /tmp/ballast-go "${HOME}/.local/bin/ballast-go"
ballast-go install --target cursor --all
ballast-go install --target opencode --skill owasp-security-scan
ballast-go install --language ansible --target codex --agent testing
ballast-go install --language terraform --target codex --agent linting
```

## Monorepo: Install and Use by Language

In a repo that contains TypeScript, Python, Go, Ansible, or Terraform projects, run Ballast once per language profile. The `ballast` wrapper can auto-detect single-language repos for all five profiles and mixed repos that include them.

### 1. TypeScript rules in a monorepo

```bash
pnpm exec ballast-typescript install --target cursor --all
```

### 2. Python rules in a monorepo

```bash
VERSION="<latest-release>"
uvx --from "https://github.com/everydaydevopsio/ballast/releases/download/v${VERSION}/ballast_python-${VERSION}-py3-none-any.whl" ballast-python install --target cursor --all
```

### 3. Go rules in a monorepo

```bash
ballast-go install --target cursor --all
```

Recommended order for one repository that uses all five language profiles:

1. Run the TypeScript command.
2. Run the Python command.
3. Run the Go command.
4. If the repo also contains Ansible, run `ballast-go install --language ansible --target cursor --all`.
5. If the repo also contains Terraform, run `ballast-go install --language terraform --target cursor --all`.

Ballast only installs shipped agents and skills and follows the single overwrite policy (existing rule files are preserved unless `--force` is passed). Use `--patch` to merge new Ballast content into an existing rule file while preserving the user's version of edited sections.

## CLI Flags

- `--target, -t`: `cursor`, `claude`, `opencode`, `codex`; adds to saved targets in `.rulesrc.json`
- `--remove-target`: remove one or more saved targets and clean up Ballast-managed files for them
- `--agent, -a`: comma-separated agent list
- `--skill, -s`: comma-separated skill list
- `--all`: install all agents for the selected language
- `--all-skills`: install all available skills for the selected language
- `--force, -f`: overwrite existing rule files
- `--patch, -p`: merge upstream rule updates into existing rule files while preserving user-edited sections (`--force` wins if both are set)
- `--yes, -y`: non-interactive mode

## Wrapper Commands

- `ballast install`: install rules for the detected or selected language; `--target` merges into saved targets, `--remove-target` removes saved targets with Ballast-managed cleanup, and `--refresh-config` reapplies saved `.rulesrc.json` settings
- `ballast doctor`: inspect local Ballast CLI versions and `.rulesrc.json` metadata; add `--fix` to install/upgrade backend CLIs and refresh config automatically, and add `--patch` to merge backend file updates during that refresh
- `ballast upgrade [--patch]`: rewrite `.rulesrc.json` to the running Ballast wrapper version, then sync backend CLIs to match it; `--patch` forwards patch mode to the backend refresh
- `ballast install-cli [--language <typescript|python|go|ansible|terraform>] [--version <x.y.z>]`: install or upgrade backend CLIs into the current repo’s `.ballast/` directory; omit `--version` for the latest release. The `ansible` and `terraform` selections reuse the `ballast-go` backend.

## Config Files

- Canonical shared config for the wrapper and language CLIs: `.rulesrc.json`
- Legacy fallback configs still read for backward compatibility:
  - TypeScript CLI: `.rulesrc.ts.json`
  - Python CLI: `.rulesrc.python.json`
  - Go CLI: `.rulesrc.go.json`
- Saved settings include `target`, `agents`, and `skills`

## Install Locations

- Cursor: `.cursor/rules/<agent>.mdc`
- Claude: `.claude/rules/<agent>.md` and `.claude/skills/<skill>.skill`
- OpenCode: `.opencode/<agent>.md` and `.opencode/skills/<skill>.md`
- Codex: `.codex/rules/<agent>.md` and root `AGENTS.md`
- Cursor skills: `.cursor/rules/<skill>.mdc`
- Codex skills: `.codex/rules/<skill>.md`, with root `AGENTS.md` listing installed skills

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

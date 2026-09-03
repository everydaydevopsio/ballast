<p align="center">
  <img src=".github/assets/icon.svg" alt="Ballast project icon" width="128">
</p>

<h1 align="center">Ballast</h1>

<p align="center">
  <strong>Reusable rules and skills that keep AI coding agents aligned with your repository.</strong>
</p>

<p align="center">
  <a href="https://github.com/everydaydevopsio/ballast/actions/workflows/ci.yml"><img src="https://github.com/everydaydevopsio/ballast/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/everydaydevopsio/ballast/actions/workflows/publish.yml"><img src="https://github.com/everydaydevopsio/ballast/actions/workflows/publish.yml/badge.svg" alt="Release"></a>
  <a href="https://github.com/everydaydevopsio/ballast/releases"><img src="https://img.shields.io/github/v/release/everydaydevopsio/ballast" alt="GitHub release"></a>
  <a href="https://www.npmjs.com/package/@everydaydevopsio/ballast"><img src="https://img.shields.io/npm/v/%40everydaydevopsio%2Fballast.svg" alt="npm version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/everydaydevopsio/ballast" alt="License"></a>
</p>

Ballast installs versioned AI-agent rules and task skills into a codebase. It gives Cursor, Claude Code, OpenCode, Codex, and Gemini the same repository-specific guidance without copying instructions by hand for every tool.

Ballast does **not** run an agent or choose an AI model. It supplies the operating instructions that an agent loads from your repository.

## What Ballast provides

- **Agent rules** for recurring engineering concerns such as local development, documentation, CI/CD, observability, publishing, testing, logging, and linting.
- **Task skills** for bounded workflows such as OWASP reviews, AWS health checks, GitHub repository checks, Docker publishing, and GitHub Spec Kit delivery.
- **Native output for each AI tool**, including Cursor rules, Claude skills, Codex skills, OpenCode skills, Gemini rules, and managed support-file sections.
- **Seven language profiles**: TypeScript, Python, Go, Ansible, Terraform, Dart, and Docker.
- **Safe updates** that preserve existing files unless you explicitly request a patch or replacement.

## Quick start

The `ballast` wrapper detects the languages in a repository and dispatches to the matching installer.

### macOS

```bash
brew tap everydaydevopsio/ballast
brew trust everydaydevopsio/ballast
brew install --cask everydaydevopsio/ballast/ballast
```

### Linux

```bash
brew tap everydaydevopsio/ballast
brew trust everydaydevopsio/ballast
brew install --formula everydaydevopsio/ballast/ballast
```

Use the fully qualified Homebrew name. Another project also publishes a package named `ballast`.

Install all agent rules for Codex in the current repository:

```bash
cd /path/to/your/repository
ballast install --target codex --all
ballast doctor
```

Install selected guidance instead:

```bash
ballast install --target claude --agent local-dev,testing
ballast install --target claude --skill owasp-security-scan
```

For a mixed-language monorepo, the wrapper detects every supported profile:

```bash
ballast install --target codex --all --yes
```

## Package-specific installation

Use the language CLI directly when you do not need automatic detection.

### TypeScript and JavaScript projects

```bash
pnpm add -D @everydaydevopsio/ballast
pnpm exec ballast-typescript install --target cursor --all
pnpm exec ballast-typescript install --target claude --skill owasp-security-scan
```

### Python and Go projects

Ballast publishes the Python wheel and Go binaries with each GitHub release. See the [installation guide](docs/installation.md) for `uv`, `uvx`, and direct binary commands.

## Supported tools and languages

| AI tool | Installed surface |
| --- | --- |
| Cursor | Rules under `.cursor/rules/` |
| Claude Code | `CLAUDE.md` and skills under `.claude/skills/` |
| OpenCode | Target-native files under `.opencode/` |
| Codex | `AGENTS.md` and skills under `.codex/skills/` |
| Gemini | `GEMINI.md` and rules under `.gemini/rules/` |

Language profiles:

- TypeScript
- Python
- Go
- Ansible
- Terraform
- Dart
- Docker

Published entry points:

| Command | Distribution | Role |
| --- | --- | --- |
| `ballast` | Homebrew | Detects repository languages and dispatches to the matching implementation |
| `ballast-typescript` | npm | TypeScript implementation and npm integration |
| `ballast-python` | GitHub release wheel | Python implementation |
| `ballast-go` | GitHub release binary | Go implementation |

The wrapper is the simplest choice for most repositories. The separate implementations let a project use a package or binary that fits its toolchain.

## Agents and skills

An **agent rule** tells an AI tool how work should be performed in this repository. Examples include:

- `local-dev`
- `docs`
- `cicd`
- `observability`
- `publishing`
- `git-hooks`
- `testing`
- `linting`
- `logging`

A **skill** describes a focused task with a clear workflow. Examples include:

- `owasp-security-scan`
- `aws-health-review`
- `github-health-check`
- `github-pr-copilot-cycle`
- `ballast-audit`
- `docker-registry-publish`
- `speckit-bootstrap`
- `speckit-delivery`

After installation, name the skill in your request:

```text
Run owasp-security-scan on this repository.
Use github-health-check to review the repository.
Use speckit-delivery for this product change.
```

Use `--all-skills` when you want every shipped skill for the selected language:

```bash
ballast install --target codex --all-skills
```

## How Ballast manages repository files

Ballast records the selected languages, targets, agents, skills, and path mappings in `.rulesrc.json`.

By default, it preserves existing rule and skill files. Choose an update policy explicitly when upstream guidance changes:

```bash
ballast upgrade           # preserve existing managed files
ballast upgrade --patch   # merge updates while retaining local edits
ballast upgrade --force   # reset managed content to the canonical version
```

Support files such as `AGENTS.md`, `CLAUDE.md`, and `GEMINI.md` use marked, Ballast-managed sections. Ballast updates those sections without replacing unrelated project guidance. In non-interactive mode, it will not silently overwrite a customized support file.

Useful maintenance commands:

```bash
ballast doctor
ballast install --refresh-config
ballast setup-dev
```

## Documentation

| Guide | Purpose |
| --- | --- |
| [Documentation index](docs/README.md) | Agent and skill guide map |
| [Installation](docs/installation.md) | Homebrew, npm, Python, Go, and monorepo workflows |
| [Agent guides](docs/agents/) | Guidance installed by each agent family |
| [Skill guides](docs/skills/) | Inputs, outputs, and usage for shipped skills |
| [Publishing](docs/publish.md) | Package and release publishing |
| [Architecture](ARCHITECTURE.md) | Repository structure and implementation design |
| [Security policy](SECURITY.md) | Reporting security issues |

## Development

The monorepo contains TypeScript, Python, and Go implementations. Use:

- Node.js 22 or newer; `.nvmrc` pins the development version.
- The `pnpm` version declared in `package.json`.
- Python 3.10 or newer with `uv`.
- Go 1.24 or newer.

```bash
git clone https://github.com/everydaydevopsio/ballast.git
cd ballast

nvm install
nvm use
corepack enable
pnpm install

make build-all
pnpm test
pnpm lint
```

Run `ballast setup-dev` before agent-assisted repository work so local tooling and generated guidance match the project configuration.

## License

Ballast is available under the [MIT License](LICENSE).

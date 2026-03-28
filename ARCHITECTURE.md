# Ballast Architecture

This document describes the current architecture of the Ballast repository.

Ballast is a monorepo that ships AI rule installers for multiple language profiles and multiple target tools. The repository contains:

- a TypeScript package published to npm as `@everydaydevopsio/ballast`
- a Python package published as release artifacts
- a Go package published as release artifacts
- a Go-based `ballast` wrapper CLI for Homebrew-style installs and upgrades

## Overview

Ballast installs repo-local AI guidance into the locations expected by Cursor, Claude Code, OpenCode, and Codex.

The source-of-truth rule and skill content lives in this repository:

- `agents/common/*` for shared agents
- `agents/typescript/*`, `agents/python/*`, `agents/go/*` for language-specific agents
- `skills/common/*` for shared skills

The TypeScript package is the reference implementation for rule assembly and install behavior. The Python and Go packages ship the same content model for their own CLIs.

## Repository Layout

```text
ballast/
├── agents/
│   ├── common/                 # local-dev, cicd, observability
│   ├── typescript/             # linting, logging, testing
│   ├── python/                 # linting, logging, testing
│   └── go/                     # linting, logging, testing
├── skills/
│   └── common/
│       └── owasp-security-scan/
├── cli/
│   └── ballast/                # Go wrapper CLI used for install/upgrade/doctor flows
├── packages/
│   ├── ballast-typescript/     # npm package and reference installer implementation
│   ├── ballast-python/         # Python CLI package
│   └── ballast-go/             # Go CLI package
├── docs/                       # user-facing docs
└── .github/workflows/          # validation and publish workflows
```

Within `packages/ballast-typescript/`:

```text
packages/ballast-typescript/
├── src/
│   ├── cli.ts                  # install + doctor command surface
│   ├── install.ts              # target/agent/skill resolution and writes
│   ├── build.ts                # content assembly and destination mapping
│   ├── config.ts               # config load/save and project root detection
│   ├── agents.ts               # agent/skill registries and lookup
│   ├── patch.ts                # merge support for --patch
│   └── *.test.ts
├── agents/                     # packaged fallback copy of rule content
├── skills/                     # packaged fallback copy of skill content
├── bin/ballast.js              # shebang entrypoint
└── dist/                       # compiled output
```

## Content Model

### Agents

Ballast exposes these public agent IDs:

- Common: `local-dev`, `cicd`, `observability`
- Language-specific: `linting`, `logging`, `testing`

Each agent directory contains:

- `content.md` for the primary rule body
- optional `content-<suffix>.md` files for additional rule variants
- `templates/` files for per-target wrappers

Rule suffixes allow one logical agent to emit multiple installed files. The installer discovers `content.md` and any `content-*.md` variants automatically.

### Skills

Ballast currently ships one common skill:

- `owasp-security-scan`

Each skill directory contains `SKILL.md` and may include a `references/` directory. Claude installs the skill as a bundled `.skill` archive; the other targets install Markdown-based skill files.

## Target Formats and Destinations

Supported targets are:

- `cursor`
- `claude`
- `opencode`
- `codex`

Rule installation paths:

| Target | Directory | Extension |
| --- | --- | --- |
| Cursor | `.cursor/rules/` | `.mdc` |
| Claude | `.claude/rules/` | `.md` |
| OpenCode | `.opencode/` | `.md` |
| Codex | `.codex/rules/` | `.md` |

Skill installation paths:

| Target | Directory | Format |
| --- | --- | --- |
| Cursor | `.cursor/rules/` | `.mdc` |
| Claude | `.claude/skills/` | `.skill` zip bundle |
| OpenCode | `.opencode/skills/` | `.md` |
| Codex | `.codex/rules/` | `.md` |

Support files:

- Claude installs or patches a root `CLAUDE.md`
- Codex installs or patches a root `AGENTS.md`

If `BALLAST_RULE_SUBDIR` is set, installed rule files are scoped into subdirectories such as `.codex/rules/<subdir>/...`, and the emitted filenames are prefixed to avoid collisions.

## Build Pipeline

The TypeScript build layer assembles output by combining agent content with target-specific templates.

### Rule assembly

`build.ts` performs the following:

1. Resolve the source directory for the selected agent and language.
2. Load `content.md` or a `content-<suffix>.md` variant.
3. Load the target template:
   - Cursor: `cursor-frontmatter.yaml`
   - Claude: `claude-header.md`
   - OpenCode: `opencode-frontmatter.yaml`
   - Codex: `codex-header.md`, falling back to `claude-header.md`
4. Inject linting hook guidance for the selected language and repo shape when the content uses the `{{BALLAST_HOOK_GUIDANCE}}` token.
5. Emit the final target file content.

### Skill assembly

`build.ts` also builds skills per target:

- Cursor: frontmatter + skill body from `SKILL.md`
- Claude: stored zip archive containing `SKILL.md` and any `references/*`
- OpenCode: Markdown body from `SKILL.md`
- Codex: Markdown body from `SKILL.md`

### Support file assembly

`buildCodexAgentsMd()` and `buildClaudeMd()` generate the root support files that enumerate installed rules and skills for those tools.

## Install Flow

The reference install flow lives in `packages/ballast-typescript/src/install.ts`.

1. Detect the project root using the nearest directory containing `.rulesrc.json`, a legacy `.rulesrc.<lang>.json` file, or `package.json`.
2. Load config from `.rulesrc.json` when available. Legacy per-language config filenames are still accepted as inputs.
3. Resolve target, agents, and skills from flags, config, or interactive prompts.
4. In CI or `--yes` mode, require explicit install choices when no config is present.
5. Save config back to `.rulesrc.json`, including `target`, `agents`, optional `skills`, `ballastVersion`, and merged language/path metadata when relevant.
6. For each selected agent:
   - enumerate all rule suffixes
   - calculate destination paths
   - skip existing files unless `--force` or `--patch` is set
   - write generated content, or merge updates with `patch.ts` when `--patch` is used
7. For each selected skill:
   - write the target-specific skill format
   - skip existing files unless `--force` is set
8. For Claude and Codex:
   - create or update `CLAUDE.md` or `AGENTS.md`
   - optionally patch existing support files when patch mode is enabled

## Config Model

The canonical config file is `.rulesrc.json`.

Current saved shape:

```json
{
  "target": "codex",
  "agents": ["local-dev", "linting"],
  "skills": ["owasp-security-scan"],
  "ballastVersion": "5.2.0",
  "languages": ["typescript"],
  "paths": {
    "typescript": ["."]
  }
}
```

Notes:

- `.rulesrc.ts.json`, `.rulesrc.python.json`, and `.rulesrc.go.json` are legacy compatibility filenames.
- The installer reads legacy files, but writes the canonical `.rulesrc.json`.
- `findProjectRoot()` still treats either canonical or legacy config files as root markers.

## CLI Surface

The TypeScript CLI currently exposes:

- `install` as the default command
- `doctor` for local CLI/config inspection

Supported install flags include:

- `--target`, `-t`
- `--language`, `-l`
- `--agent`, `-a`
- `--skill`, `-s`
- `--all`
- `--all-skills`
- `--force`, `-f`
- `--patch`, `-p`
- `--yes`, `-y`
- `--help`, `-h`
- `--version`, `-v`

## Monorepo and Hook Behavior

For TypeScript installs, Ballast distinguishes between standalone and monorepo layouts to tailor linting hook guidance:

- standalone TypeScript repos get `pre-commit` guidance
- TypeScript workspace monorepos get Husky guidance
- Python and Go profiles get their own `pre-commit` guidance

The detection logic uses config metadata first and falls back to workspace manifest/package discovery.

## Release and Publishing Architecture

GitHub Actions publish and validate the repository.

Key workflows include:

- `test.yml`
- `lint.yaml`
- `cross-language-validate.yml`
- `examples-smoke.yml`
- `language-packs.yml`
- `publish.yml`

`publish.yml` is the top-level release workflow. It:

1. optionally bumps versions and creates a Git tag on manual dispatch
2. publishes the TypeScript package to npm
3. uploads Python build artifacts to the GitHub release
4. publishes Go binaries with GoReleaser
5. publishes the wrapper CLI with GoReleaser

## Design Constraints

- Ballast only installs content shipped in this repository.
- Existing files are preserved by default.
- `--force` overwrites generated files.
- `--patch` merges upstream updates into existing generated files where supported.
- Common agents stay language-agnostic at the public ID level; language-specific agents are namespaced internally by source directory and emitted filename.

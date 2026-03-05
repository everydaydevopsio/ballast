# Ballast Documentation

Ballast `v4.0.0` supports TypeScript, Python, and Go.

## Agent Families

Common agents (all language packs):

- `local-dev`
- `cicd`
- `observability`

Language-specific agents:

- `linting`
- `logging`
- `testing`

## Agent Guide Index

<!-- prettier-ignore -->
Agent | TypeScript | Python | Go | Guide
----- | ---------- | ------ | -- | -----
`local-dev` | Yes | Yes | Yes | [agents/local-dev.md](agents/local-dev.md)
`cicd` | Yes | Yes | Yes | [agents/cicd.md](agents/cicd.md)
`observability` | Yes | Yes | Yes | [agents/observability.md](agents/observability.md)
`linting` | Yes | Yes | Yes | [agents/linting.md](agents/linting.md)
`logging` | Yes | Yes | Yes | [agents/logging.md](agents/logging.md)
`testing` | Yes | Yes | Yes | [agents/testing.md](agents/testing.md)

## Installation and Monorepos

See [installation.md](installation.md) for package-specific commands and monorepo workflows for each language:

- npm (`@everydaydevopsio/ballast`)
- uv/uvx from GitHub Releases wheel (`ballast-python`)
- go install/go run (`ballast-go`)

## Publishing

See [publish.md](publish.md) for npmjs, Python GitHub release assets, and Go release publishing setup.

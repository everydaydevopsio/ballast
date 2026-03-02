# Ballast Documentation

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

## Installation

See [installation.md](installation.md) for package-specific commands:

- npm (`@everydaydevopsio/ballast`)
- uv (`ballast-python`)
- go install (`ballast-go`)

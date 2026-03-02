# Linting Agent

The **linting** agent provides language-appropriate linting, formatting, and CI checks for TypeScript, Python, and Go projects.

## What It Sets Up by Language

- **TypeScript/JavaScript**
  - ESLint (flat config)
  - Prettier
  - Optional Husky + lint-staged workflow
- **Python**
  - Ruff for linting and formatting
  - Optional Black when explicitly required
  - mypy for type checking when type hints are used
- **Go**
  - `gofmt` for formatting
  - `golangci-lint` for static analysis

## What It Provides

- Consistent code style and lint policy per language
- Fast local commands for lint/format/type checks
- CI enforcement so lint failures block merges

## Monorepo Usage

In a TypeScript + Python + Go monorepo, apply linting standards per language area and keep each tool scoped to its files.

Recommended command set:

- TypeScript: `pnpm run lint` (or project ESLint command)
- Python: `ruff check .` and `ruff format .`
- Go: `gofmt -w .` and `golangci-lint run`

## Prompts to Improve Your App

- **"Set up linting for all three languages in this monorepo"** — Multi-language baseline
- **"Fix lint errors in this package according to its language rules"** — Targeted cleanup
- **"Add CI jobs so TypeScript, Python, and Go lint checks run independently"** — Monorepo CI
- **"Add ignore patterns for generated code in each language"** — Noise reduction

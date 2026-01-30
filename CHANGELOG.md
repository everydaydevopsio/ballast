# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [3.0.0] - 2026-01-30

### Added

- New `ballast` CLI for managing AI agent rules across platforms
- New agent types: CI/CD, local-dev, and observability (alongside existing linting)
- TypeScript source code in `src/` with full type safety
- Interactive CLI with `--target`, `--agent`, `--all`, `--force`, and `--yes` options
- Configuration persistence via `.rulesrc.json` for non-interactive repeat runs
- CI mode support (`CI=true` or `--yes`) for automated pipelines
- Comprehensive test suite with Jest and 80% coverage threshold
- ARCHITECTURE.md and AGENTS.md documentation

### Changed

- **Breaking**: Migrated from monorepo to single-package CLI architecture
- **Breaking**: Renamed package to `@everydaydevopsio/ballast`
- **Breaking**: Node.js 22 is now required
- Consolidated all platform installers into unified `install.ts` module
- Moved agent content and templates to `agents/` directory structure
- Simplified build process with single TypeScript compilation

### Removed

- Removed separate packages (`packages/claude`, `packages/cursor`, `packages/opencode`, `packages/core`)
- Removed `pnpm-workspace.yaml` (no longer a monorepo)

## [2.0.0] - 2026-01-20

### Added

- Support for Claude Code as a target platform
- Support for Cursor as a target platform
- Monorepo structure with dedicated packages for each target (claude, cursor, opencode, core)
- Shared core package for common templates and content
- Platform-specific templates (claude-header.md, cursor-frontmatter.yaml, opencode-frontmatter.yaml)

### Changed

- Restructured project from single-package to monorepo architecture
- OpenCode remains supported as a target alongside new platforms

### Dependencies

- Bumped typescript-eslint to latest version
- Updated prettier group dependencies
- Updated eslint group dependencies

## [1.0.0] - 2025-12-24

### Added

- Initial release of OpenCode TypeScript Linting agent
- Automated setup for ESLint with TypeScript support
- Prettier configuration and integration
- Husky and lint-staged for Git hooks
- GitHub Actions workflow for CI linting
- Comprehensive test suite with Jest
- Complete documentation and examples

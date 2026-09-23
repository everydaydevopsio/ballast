# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Entries from `3.0.1` through `5.18.3` were not recorded here. From the next
release onward, `publish.yml` writes an entry here for every release cut
through its `workflow_dispatch` input; a missing `OPENAI_API_KEY` fails the
release rather than skipping the entry. Releases published from an existing
`v*` tag skip the bump job and are not recorded here.
[GitHub Releases](https://github.com/everydaydevopsio/ballast/releases) remains
the complete history.

## [Unreleased]

### Changed

- **Claude skills install as directories, so they are finally invocable.** Claude Code discovers project skills at `.claude/skills/<name>/SKILL.md` and exposes each as `/<name>`. Ballast wrote `.claude/skills/<name>.skill`, a zip in the claude.ai Agent Skills bundle format, which Claude Code never scans — so no Ballast skill was ever registered, and they worked only because the generated `CLAUDE.md` told the model to go read the files. Skills now install in the same directory layout the Codex target already used (which is why `$<name>` worked there), with reference material copied alongside `SKILL.md`. The generated manifest lists invocations (`/<name>`) instead of paths to read.
- **Existing installs migrate automatically.** The legacy `<name>.skill` bundle is removed whenever the Claude target is installed or refreshed, so a skill never appears twice in the directory, and a deselected skill's bundle is now removable for the `claude` target like the other directory-format targets. `buildClaudeSkill` remains for packaging claude.ai bundles; it is no longer what gets installed.

### Fixed

- **A pinned `.ballast/` backend older than the wrapper no longer emits stale output silently.** The version check in `ensureInstalled` was unreachable: both call sites ran it only when no local backend existed, so an outdated one was used as-is. `ballast install --refresh-config` would report installing every skill while writing the old backend's content and leaving `ballastVersion` behind. Backend resolution now distinguishes a version-pinned `.ballast/` backend from the ballast source tree, and verifies the former's version before forwarding.
- **`--patch` no longer corrupts skills.** Skills were section-merged like rules, which kept stale text for headings that still existed upstream and re-appended sections upstream had deleted — a patched skill could contain both the current guidance and the removed guidance it replaced. Skills are entirely Ballast-authored, so every write path now replaces them wholesale. This also fixes releases shipping skill files stamped with the previous version, since the release regenerates via `upgrade --patch`.
- **`doctor` now warns when the Homebrew cask token is hijacked.** `homebrew/cask` ships an unrelated app also called `ballast`, so the bare token resolves there and `brew upgrade --cask ballast` targets the wrong package. `ballast update` already self-healed this; `doctor` now surfaces it before a manual `brew` command hits it.

### Changed

- **`ballast-audit` is now installed by default.** Every install adds it, even when no skills are selected; an installation that has drifted cannot be detected by the rules it emits. Existing repositories pick it up on the next `ballast install` or `ballast install --refresh-config`.
- **Rewrote the `ballast-audit` skill** around the installed state rather than generic file heuristics. It now runs the language backend's rule-file census (the wrapper `ballast doctor` does not print rule-file status), clean-installs the repository's own `.rulesrc.json` into a scratch copy and diffs to find orphaned and stale rules, and checks each emitted rule and skill against evidence in the repository. It also documents that `unowned` files — those generated before the `<!-- ballast:rule -->` marker existed — are not repaired by `--refresh-config` or `--refresh-config --patch`, and must be removed and reinstalled.

## [5.21.0] - 2026-09-23

### Highlights

- **Claude skills now install as directories**, enabling them to register as slash commands. The new layout is supported by both the Go and Python backends.

### Fixes

- Made migration to the Claude skill directory layout safe to rerun, with improved handling of directories at the legacy path in the wrapper and Python backend.
- Improved skill-resource reconciliation and detection of missing generated resources. Migration errors are now surfaced rather than silently ignored.
- Corrected AWS skill script paths.
- Addressed silent staleness issues involving pinned backends, skill patching, and the Homebrew token.

### Changes

- Added tests covering skill-resource reconciliation.
- Clarified the manifest contract in `ARCHITECTURE.md` and documented multi-backend and `PATH` pitfalls.

**Full changelog:** [v5.20.0...v5.21.0](../../compare/v5.20.0...v5.21.0)

## [5.20.0] - 2026-09-22

### Highlights
- **`ballast-audit` is now installed by default**, making it available without a separate installation step. (#364)

### Fixes
- No standalone fixes in this release.

### Changes
- Reworked `ballast-audit` to base its auditing on installed state. (#364)

**Previous release:** v5.19.1

## [5.19.1] - 2026-09-22

### Highlights

- **Safer configuration refreshes:** `--refresh-config` no longer deletes generated rules.
- **Backend configuration preservation:** Go and Python configuration saves now retain `publishingProfiles`.

### Fixes

- Aligned `doctor` with `install` so both use the same default deployment model, and corrected the related help text.
- Scoped always-on rule context as part of the configuration-refresh and context-budget improvements.

### Changes

- Promoted the configuration-refresh and context-budget plan to architecture decision record **ADR-001**.
- Updated the development dependency `@types/node` from `22.20.2` to `22.20.3`.

**Full changelog:** [v5.19.0...v5.19.1](https://github.com/everydaydevopsio/ballast/compare/v5.19.0...v5.19.1)

## [5.19.0] - 2026-09-21

## v5.19.0

This release adds Spec Kit workflow support, reduces generated rule clutter, and improves configuration diagnostics and release reliability.

### Highlights

- **Spec Kit workflows:** Added bootstrap, reverse-engineering, and delivery-orchestration skills, plus a lightweight governance rule and supporting documentation.
- **Leaner generated context:** Added a minimal core-rule profile through `ruleProfile`. Inactive and reference-only rules are no longer emitted, task-system rules reflect the configured system and target, and repository tool policy is emitted once per target manifest.
- **Reusable rule content:** Added shared-fragment includes and consolidated common testing and publishing guidance. Trimmed repeated headings, persona preambles, and reference boilerplate.
- **Rule-size checks:** Added rule-size budget enforcement in CI and checks in `doctor` to help keep generated context manageable.
- **Smarter refresh and agent guidance:** Refresh now fills placeholder repository facts. Agent guidance reduces low-information questions, and a new performance-audit skill supports performance reviews.

### Fixes

- **Configuration and diagnostics:** Improved `doctor` detection and remediation for JavaScript profiles configured as TypeScript, minimal JavaScript setups, and removal of the final language. Language cleanup now clears stale tool selections without discarding saved profiles.
- **Profile handling:** Preserved `ruleProfile` when saving configuration through the Go backend, made TypeScript diagnostics profile-aware, tightened core-command language validation, and included Docker in language checks.
- **Cross-platform generation:** Treats pristine rules with CRLF line endings correctly, normalizes line endings before patching Python support files, and produces deterministic TypeScript rule ordering.
- **Include safety:** Hardened shared-fragment path validation against Windows-style paths and distinguished nesting-depth failures from recursive includes.
- **Tool-policy cleanup:** Restricted removal to sections matching the generated signature, protecting unrelated content.
- **Release pipeline:** Fixed the release gate and reusable-workflow concurrency collision, ensured generated outputs use the release version, and built the Go backend before regenerating release artifacts.

### Changes

- **Release notes automation:** Release notes and changelog entries are now generated with Castoff. Release-note generation requires `OPENAI_API_KEY` rather than silently skipping when it is missing.
- **Go toolchain:** Raised the Go toolchain to **1.26**, updated `x/term`, and expanded version checks to cover all modules, the smoke image, and workflow YAML.
- **Dependencies:** Updated YAML, Jest, ESLint, Prettier, lint-staged, globals, Node.js types, and `pnpm/action-setup`; removed duplicate lockfile entries.
- **Documentation and examples:** Refreshed the README and project icon, documented Spec Kit and setup/toolchain plans, clarified publishing guidance, and replaced an API-key-like value in an OWASP example.

## [3.0.0] - 2026-01-30

### Added

- New `ballast` CLI for managing AI agent rules across platforms
- New agent types: CI/CD, local-dev, and observability (alongside existing linting)
- TypeScript source code in `src/` with full type safety
- Interactive CLI with `--target`, `--agent`, `--all`, `--force`, and `--yes` options
- Configuration persistence via `.rulesrc.ts.json` for non-interactive repeat runs
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

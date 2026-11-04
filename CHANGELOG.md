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

### Added

- **`prek` is now a supported alternative to `pre-commit` for Git hooks.** The generated `git-hooks` rule mandated `pre-commit` and its commands, so a repository using [prek](https://prek.j178.dev/) was told it was noncompliant even with identical commit and push checks. prek reads the same `.pre-commit-config.yaml`, so the hook policy is unchanged — fast checks at commit time, unit tests on pre-push, and the official Gitleaks hook — and only the commands differ: `prek install` covers every configured stage, with `prek update`, `prek run --all-files`, and `prek validate-config` replacing their pre-commit equivalents. The prek path never tells you to run `pre-commit install` or `pre-commit autoupdate`. Guidance also recognizes prek's root `.pre-commit-config.yaml` or `prek.toml` and its automatic workspace discovery, while keeping the root config responsible for installing the Git hooks. Select it with `"hookRunner": "prek"` in `.rulesrc.json`, or let Ballast detect an existing `prek.toml`; all four backends preserve the choice, so a later install cannot silently migrate a prek repository back. `pre-commit` remains the default and its generated output is unchanged for TypeScript, Python, Go, and Ansible. TypeScript-only repositories continue to use Husky, which the runner choice does not affect.

### Changed

- **Terraform, Dart, and Docker hook guidance now says how to verify the configuration.** Those three were the only languages whose `git-hooks` rule omitted a `run --all-files` step, so they told you to keep hooks current without telling you how to check them.

### Fixed

- **`doctor`'s canonical rule comparison is now protected by tests.** `doctor` already compares each managed rule against the content the installed version would generate, not only against the checksum in the file's own marker, so a rule that is internally consistent but no longer canonical correctly reports `drifted`. Nothing covered that: the existing drift test edits a body without re-stamping its checksum, so deleting the canonical comparison entirely left all 25 doctor tests green. Added cases for a self-consistent but non-canonical rule, for an interrupted upgrade that leaves two targets on different content, and for an older `version=` stamp on canonical content staying `ok` — the stamp alone is not a finding, matching the `ballast-audit` skill.

- **Git submodules no longer decide the parent repository's language.** Profile detection scanned into submodule worktrees, so `markcallen/dotfiles` — which tracks `dotbot` as a submodule and has no first-party Python manifest — reported a Python profile belonging to third-party code. Submodules are now skipped by default, recognized both from `.gitmodules` (so an uninitialized gitlink is skipped too) and from a `.git` *file* in the directory (so a submodule not yet listed in `.gitmodules` is also caught). `ballast doctor` names each skipped submodule and the opt-in, so an ignored marker is never silent, and detection still only reports — it never writes or refreshes project files. Set `discovery.includeSubmodules: true` in `.rulesrc.json` to scan them anyway; all four backends preserve that flag so a backend rewrite cannot revert the choice.

- **One wrong-typed value in `.rulesrc.json` no longer discards the whole config.** Both Go surfaces decoded every scalar policy field into a `string`, so a single non-string value — `"codeReviewer": false`, say — failed `json.Unmarshal` for the entire document. `ballast-go` returned `nil`, which is indistinguishable from "no config", and an install with explicit selections then overwrote the file, silently destroying saved `paths`, `tools`, `publishingProfiles`, and `ruleProfile`; the wrapper instead aborted with the config reported as unreadable. Scalars now decode through `json.RawMessage` and normalize independently, so a mistyped value degrades to "that field is unset" and everything else survives. A structurally broken document is still an error. TypeScript and Python already behaved this way, so all four backends now agree.
- **The TypeScript backend now accepts `--flag=value` for every flag that takes one.** `parseArgs` recognized only the space-separated form, so `--task-system=jira` and `--deployment-model=kubernetes` were dropped without error and the install saved a default instead. Go's `flag` package and Python's `argparse` both accept the equals form natively, making this backend the lone outlier; `--code-reviewer=` had been special-cased, and that one-off is now replaced by generic handling that splits on the first `=` so values containing one still work.
- **An unrelated pull request can no longer go red because a macOS runner never arrived.** `sign-notarize-hook-smoke` ran on every pull request, and it is the only macOS job outside the release workflows. GitHub cancels a job that waits more than roughly 15 minutes for a hosted macOS runner, and reports that cancellation as a failed check with no steps and no logs — indistinguishable from a real failure at a glance. Observed queue times for this job ranged from 3 seconds to over 15 minutes. It now lives in its own `sign-notarize-smoke.yml`, which pull requests run only when the hook, its smoke test, or a GoReleaser config changes, while pushes to `main` still run it unconditionally so the guard executes continuously on the default branch. The job also fails fast if it ever lands on a non-macOS runner, because the smoke script exits 0 as a skip off Darwin and would otherwise report a pass having validated nothing.

### Changed

- **The released `ballast-go` backend now installs without any shell tooling.** `ballast install-cli --language go` and `ballast doctor --fix` built the install by handing a shell script to `sh -c` (or a command string to PowerShell), which required `curl`, `mktemp`, `grep`, `basename`, `openssl`, `tar`, and `install` to be on `PATH` — `openssl` and GNU `install` are absent from many minimal containers and locked-down CI images, and a missing tool surfaced as a generic install failure. The download, SHA-256 verification against `ballast-go_checksums.txt`, archive extraction, and binary placement now run in-process using the Go standard library. The binary is written through a temporary file and renamed, so a failed install never leaves a truncated backend behind, and reinstalling over a running backend succeeds. Unsupported platforms now report the published platform set and point at a source build instead of failing later on a download. Verified by installing a published release with `PATH` containing nothing but the wrapper itself.
- **Claude skills install as directories, so they are finally invocable.** Claude Code discovers project skills at `.claude/skills/<name>/SKILL.md` and exposes each as `/<name>`. Ballast wrote `.claude/skills/<name>.skill`, a zip in the claude.ai Agent Skills bundle format, which Claude Code never scans — so no Ballast skill was ever registered, and they worked only because the generated `CLAUDE.md` told the model to go read the files. Skills now install in the same directory layout the Codex target already used (which is why `$<name>` worked there), with reference material copied alongside `SKILL.md`. The generated manifest lists invocations (`/<name>`) instead of paths to read.
- **Existing installs migrate automatically.** The legacy `<name>.skill` bundle is removed whenever the Claude target is installed or refreshed, so a skill never appears twice in the directory, and a deselected skill's bundle is now removable for the `claude` target like the other directory-format targets. `buildClaudeSkill` remains for packaging claude.ai bundles; it is no longer what gets installed.

### Fixed

- **A pinned `.ballast/` backend older than the wrapper no longer emits stale output silently.** The version check in `ensureInstalled` was unreachable: both call sites ran it only when no local backend existed, so an outdated one was used as-is. `ballast install --refresh-config` would report installing every skill while writing the old backend's content and leaving `ballastVersion` behind. Backend resolution now distinguishes a version-pinned `.ballast/` backend from the ballast source tree, and verifies the former's version before forwarding.
- **`--patch` no longer corrupts skills.** Skills were section-merged like rules, which kept stale text for headings that still existed upstream and re-appended sections upstream had deleted — a patched skill could contain both the current guidance and the removed guidance it replaced. Skills are entirely Ballast-authored, so every write path now replaces them wholesale. This also fixes releases shipping skill files stamped with the previous version, since the release regenerates via `upgrade --patch`.
- **`doctor` now warns when the Homebrew cask token is hijacked.** `homebrew/cask` ships an unrelated app also called `ballast`, so the bare token resolves there and `brew upgrade --cask ballast` targets the wrong package. `ballast update` already self-healed this; `doctor` now surfaces it before a manual `brew` command hits it.

### Changed

- **`ballast-audit` is now installed by default.** Every install adds it, even when no skills are selected; an installation that has drifted cannot be detected by the rules it emits. Existing repositories pick it up on the next `ballast install` or `ballast install --refresh-config`.
- **Rewrote the `ballast-audit` skill** around the installed state rather than generic file heuristics. It now runs the language backend's rule-file census (the wrapper `ballast doctor` does not print rule-file status), clean-installs the repository's own `.rulesrc.json` into a scratch copy and diffs to find orphaned and stale rules, and checks each emitted rule and skill against evidence in the repository. It also documents that `unowned` files — those generated before the `<!-- ballast:rule -->` marker existed — are not repaired by `--refresh-config` or `--refresh-config --patch`, and must be removed and reinstalled.

## [5.22.0] - 2026-11-04

### Highlights

- **New `git-branch-cleanup` skill** for cleaning up Git branches. (#404)
- **Repository-specific code review agents:** choose which agent handles code reviews for each repository. (#402)

### Fixes

- Fixed installation of released `ballast-go` versions without relying on shell tooling. (#410)
- Updated Bosun review settling to check both review publication surfaces. (#403)
- Made release builds fail when Apple rejects notarization, preventing rejected builds from being treated as successful. (#385)

### Changes

- Removed the requirement for every pull request to pass through a scarce macOS runner. (#411)
- Updated development tooling, including ESLint, Prettier, `ts-jest`, Ruff, `@types/node`, and `globals`. (#405, #400, #399, #398, #397, #388, #387)
- Updated `golang.org/x/term` for `ballast-go`. (#396)

**Full changelog:** [v5.21.3...v5.22.0](../../compare/v5.21.3...v5.22.0)

## [5.21.3] - 2026-10-01

### Highlights
macOS builds are now notarized before any release artifacts are uploaded.

### Fixes
- Corrected the release workflow ordering to ensure macOS notarization completes before uploads begin. (#384)

### Changes
- Updated release sequencing only; no application behavior changes are included in this release.

**Full changelog:** [v5.21.2...v5.21.3](../../compare/v5.21.2...v5.21.3)

## [5.21.2] - 2026-10-01

Updates since **v5.21.0**, focused on reliability, rule integrity, and development workflow cleanup.

### Highlights

- Made `doctor --fix` safer by preserving rules for targets that have not been configured.
- Restored pre-commit’s ownership of Git hooks.
- Completed the September 21 reliability work covering #356, #357, #358, and #360. (#375)

### Fixes

- Removed a shipped rule reference to documentation that consumers do not receive. (#371)
- Added the review policy that `AGENTS.md` already referenced. (#380)
- Corrected `ballast-audit` state definitions and remediation commands. (#367)
- Prevented `doctor --fix` from stripping rules for an unconfigured target. (#372)
- Resolved Git hook ownership conflicts so pre-commit can manage hooks again. (#374)

### Changes

- Clarified how Spec Kit fits into the existing Ballast rules and documented its development process in **ADR-004**. (#381, #382)
- Consolidated completed plans into architecture decision records, including **ADR-002** and **ADR-003**, retired stale plans, and tracked triage of the remaining plan files. (#373, #377–#379)
- Updated development tooling: Prettier, four packages in the ESLint dependency group, and `ts-jest` from **29.4.12** to **29.4.14**. (#368–#370)

## [5.21.1] - 2026-10-01

### Highlights

- Improved reliability of rule handling: `doctor --fix` no longer removes rules for an unconfigured target, and shipped rules no longer reference documentation unavailable to consumers.
- Restored pre-commit’s ownership of Git hooks.
- Completed the September 21 reliability work and consolidated development-process guidance into architecture decision records.

### Fixes

- **Rule handling and distribution:** Preserved rules for unconfigured targets during `doctor --fix` and removed a pointer to documentation not included in the distributed package. (#372, #371)
- **Git hooks:** Returned hook management to pre-commit. (#374)
- **Audit guidance:** Corrected `ballast-audit` state definitions and remediation commands. (#367)
- **Review policy:** Added the review policy already referenced by `AGENTS.md`. (#380)
- **Reliability:** Closed out the September 21 reliability cluster covering #356, #357, #358, and #360. (#375)

### Changes

- **Development guidance:** Clarified how Spec Kit fits alongside existing Ballast rules and promoted its development-process plan to ADR-004. (#381, #382)
- **Documentation lifecycle:** Recorded the reliability work in ADR-002, retired two stale plans with one promoted to ADR-003, and documented triage of the remaining plans. (#373, #377, #378, #379)
- **Development dependencies:** Updated Prettier, upgraded `ts-jest` from 29.4.12 to 29.4.14, and refreshed four ESLint-related dependencies. (#369, #370, #368)

**Full changelog:** [v5.21.0…v5.21.1](../../compare/v5.21.0...v5.21.1)

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

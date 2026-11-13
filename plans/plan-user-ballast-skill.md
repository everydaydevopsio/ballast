# Plan: User-level `ballast` skill

- **Status:** In progress
- **Branch:** `user-ballast-skill`
- **Created:** 2026-11-14
- **Related ADRs:** none

## Problem

Ballast skills install per repository, so an agent cannot "install ballast in this repo" in a repository Ballast has never touched, and there is no single place to ask for a rule-compliance health check or to turn friction into upstream improvements.

## Approach

Ship a user-scope `ballast` skill embedded in the Go wrapper (`cli/ballast/userskills/ballast/SKILL.md`, `go:embed`) and add `ballast user-skill [install|remove|status]` to write it to `~/.claude/skills/ballast/` and `~/.codex/skills/ballast/`, honoring `CLAUDE_CONFIG_DIR` and `CODEX_HOME`. Ownership is tracked by a versioned HTML-comment marker so reinstall and remove never clobber a user's own `ballast` skill.

The feature is wrapper-only on purpose: the four-backend parity rule covers repository installs, and no backend writes user-scope files.

## Files Affected

- `cli/ballast/userskills/ballast/SKILL.md`: the skill (install, manage, health check, improve Ballast)
- `cli/ballast/userskill.go`, `cli/ballast/userskill_test.go`: command and tests
- `cli/ballast/main.go`: dispatch and usage
- `README.md`, `docs/README.md`, `docs/installation.md`, `docs/skills/ballast-user-skill.md`, `ARCHITECTURE.md`, `CHANGELOG.md`: docs

## Phases

- [x] Explore install surfaces and skill conventions
- [x] Write failing tests, implement `user-skill`
- [x] Write the skill content
- [x] Docs
- [x] Report user skill status in `ballast doctor` (informational; `--fix` does not touch user config)
- [x] `ballast doctor` prints explicit install/update commands; `ballast-audit` installs or updates the user skill
- [ ] Optional follow-up: refresh the user skill automatically from Homebrew post-install or `ballast update`

## Verification

- `go test ./...` in `cli/ballast`
- Build the wrapper, run `ballast user-skill install|status|remove` against a temporary `HOME`

## Alternatives Rejected

- **Add `ballast` to `skills/common` and the repo registries:** repo skills cannot help before Ballast is installed, which is the main use case.
- **Source in repo-root `skills/user/` with a sync step:** another packaged copy to keep in sync; `go:embed` cannot reach outside the module, and the wrapper is the only consumer.

## Open Questions

- Should `ballast update` refresh an installed user skill automatically? The running binary is the pre-upgrade one, so it would need to re-exec the new binary.

## Change Log

| Date | Change |
| --- | --- |
| 2026-11-14 | Created |
| 2026-11-14 | Added `ballast doctor` user-skill section |
| 2026-11-14 | Doctor names install vs update; `ballast-audit` installs or updates the user skill |

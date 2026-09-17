# Plan: Setup and Toolchain Reliability

- Status: Draft
- Branch: plan-setup-toolchain (implementation branches per phase)
- Created: 2026-09-15
- Related ADRs: none yet

## Problem

Three related gaps in first-run/setup reliability, from issues #128 and #94 plus a new blocker discovered during dependabot triage:

1. **Package-manager guidance drift (#128)** — generated rule content can still instruct agents to pin package-manager versions that drift from the repo's declared `package.json#packageManager` (e.g. "configure `pnpm/action-setup` with an explicit version"), and this repo's own CI hardcodes `version: 10.27.0` despite declaring `packageManager`.
2. **No tool prerequisite checking (#94)** — rules reference tools (`golangci-lint`, `hadolint`, `trivy`, `uv`, …) with no centralized check that they exist on `PATH`, and no Homebrew remediation guidance when missing.
3. **Go toolchain pin (new)** — dependabot PR #325 (`golang.org/x/term` 0.46.0) is blocked because the bump requires `go >= 1.26.0` while CI and go.mod pin 1.24; future `golang.org/x/*` updates will keep hitting this.

## What Is Already Built (review findings, 2026-09-15)

- The wrapper's `detectNodePackageManager` (cli/ballast/main.go) already implements #128's exact precedence: `package.json#packageManager` → `pnpm-lock.yaml` → `yarn.lock` → `package-lock.json` → npm default.
- `ballast setup-dev` already runs `corepack enable` when the manager is declared, then `<manager> install`.
- Repository-facts discovery (#288) already writes the detected package manager into CLAUDE.md/AGENTS.md.
- `.rulesrc.json` `tools` per language is rendered once into the manifest's Repository Tool Policy (#286); doctor prints the configured tools but performs no PATH checks.
- local-dev env content already prefers Node LTS for `.nvmrc`.

So #128 reduces to a content/CI alignment sweep, and #94 reduces to a doctor extension over the existing `tools` config.

## Approach

### Phase 1 — Go toolchain bump (#339; unblocks dependabot #325)

- Raise the Go toolchain to 1.26.x: `packages/ballast-go/go.mod`, `cli/ballast/go.mod`, `actions/setup-go` versions in workflows, and any GoReleaser config references.
- Re-run and merge dependabot #325 (x/term 0.46.0) after CI is green on 1.26.
- Consider `GOTOOLCHAIN=auto` guidance in docs so future x/* bumps do not re-block.

### Phase 2 — #128 content and CI alignment

- typescript/linting content: replace "configure `pnpm/action-setup` with an explicit version" with: omit `version` when `package.json#packageManager` is present (`pnpm/action-setup@v4+` reads it); pin explicitly only when no declaration exists.
- Sweep `agents/` for other stale package-manager pins or non-LTS Node examples; align with the detection order above and document it in `docs/`.
- Dogfood: drop the hardcoded pnpm `version:` from this repo's workflows (rely on `packageManager`).

### Phase 3 — #94 doctor tool checks with Homebrew remediation

- Extend the wrapper's `ballast doctor`: for each configured language's `tools` (plus wrapper-required basics), check executable presence on `PATH` and report present/missing.
- Add a tool → install-guidance map: Homebrew formula/cask where appropriate (`golangci-lint`, `gofumpt`, `hadolint`, `trivy`, `uv`, `pyenv`, `pnpm`, `tflint`, `tfenv`, `ansible-lint`, `flutter` cask, `fvm`), with non-brew alternatives where brew is wrong (`corepack` ships with Node, `molecule` via `uv tool install`).
- Classify mandatory (configured language linters/testers) vs optional; missing tools are recommendations, not failures; skip Homebrew guidance when `brew` is absent.
- `ballast setup-dev` surfaces the same missing-tool remediation before running commands.

## Files Affected

- `packages/ballast-go/go.mod`, `cli/ballast/go.mod`, `.github/workflows/*` (Phase 1)
- `agents/typescript/linting/content.md`, other `agents/` content with pins, `docs/installation.md` (Phase 2)
- `cli/ballast/main.go` (doctor + setup-dev), `cli/ballast/main_test.go`, docs (Phase 3)

## Phases

- [x] Phase 1 (#339): Go toolchain 1.26 bump; supersedes dependabot #325
- [ ] Phase 2 (#128): package-manager guidance sweep + repo CI dogfooding
- [ ] Phase 3 (#94): doctor PATH checks + Homebrew remediation map
- [ ] Docs and plan close-out

## Verification

- Phase 1: all Go CI jobs green on 1.26; #325 merges clean.
- Phase 2: no `pnpm@<major>`/`version: <major>` pins remain in generated content when `packageManager` governs; repo CI green without the explicit pnpm version.
- Phase 3: doctor on a machine missing a tool lists it with a copyable install command; wrapper tests cover present/missing/no-brew paths.

## Alternatives Rejected

- Rendering the detected package manager into rule bodies via a new token — repository facts already carry it per repo; rules stay generic.
- Making missing tools a doctor failure — too disruptive for CI/doctor consumers; recommendations suffice.

## Open Questions

- Should TS/Python backends gain doctor tool checks for parity, or is the wrapper the single front door? (Default: wrapper-only; note parity as follow-up.)

## Decisions

- **Go directive (Phase 1)**: both modules declare `go 1.26.0` exactly, with no `toolchain` line. The module graph already requires >= 1.26.0 through `golang.org/x/*`, so a lower directive plus `toolchain` would only re-introduce implicit toolchain downloads — which `actions/setup-go` disables by running with `GOTOOLCHAIN=local`. An explicit directive is the honest floor and keeps CI deterministic. CI pins `go-version: '1.26.x'`; workflows that already use `go-version-file` need no change.

## Change Log

| Date | Change |
| --- | --- |
| 2026-09-15 | Initial plan from the #128/#94 review plus the #325 Go toolchain finding |
| 2026-09-17 | Phase 1 complete: both modules on `go 1.26.0`, all pinned `go-version` bumped to `1.26.x`, `golang.org/x/term` 0.34.0 → 0.46.0 (and `x/sys` 0.35.0 → 0.48.0) applied directly, README prerequisite and toolchain rationale documented, regression guard added in `packages/ballast-typescript/src/go-toolchain.test.ts`. |
| 2026-09-16 | Re-verified against main: typescript/linting still advises an explicit `pnpm/action-setup` version, this repo's CI still pins `version: 10.27.0` despite declaring `packageManager`, both Go modules still pin `go 1.24`, and #325 still fails all three Go jobs. Plan stacked on the Spec Kit process plan so both share one `plans/README.md` index. |

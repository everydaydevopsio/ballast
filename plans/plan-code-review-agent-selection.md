# Plan: Selectable code review agent

Status: Implemented — ready to graduate to an ADR
Branch: `code-review-agent-selection`
Created: 2026-10-19
Related ADRs: none yet

## Problem

The `github-pr-copilot-cycle` skill hard-codes GitHub Copilot as the reviewer. Every
step — `gh pr edit --add-reviewer "@copilot"`, the `copilot-pull-request-reviewer[bot]`
author matching, the review-thread settle polling — assumes Copilot specifically. A
repository that wants a different reviewer has no supported path, and there is no way to
express "never use Copilot" as repository policy.

Two concrete reviewers need to work today: Copilot, and Bosun
(`everydaydevopsio/bosun`), a self-hosted reviewer that authenticates as a GitHub App.
Teams also need an escape hatch to plug in a reviewer Ballast has never heard of.

## Approach

Split the decision in two:

- **Policy lives in `.rulesrc.json`** as a new flat `codeReviewer` key, validated and
  plumbed exactly like `taskSystem` and `deploymentModel`. Durable, visible to
  `ballast doctor`, and survives `ballast install` — unknown keys are dropped, because
  config is rebuilt key-by-key from a known set (`config.ts` `writeRulesConfig`).
- **Dispatch happens at runtime inside the skill.** A new reviewer-agnostic skill,
  `github-pr-review-cycle`, reads `codeReviewer` from `.rulesrc.json` when it runs and
  drives the matching backend.

Runtime dispatch rather than generated-in config, because `buildSkillDirectoryMarkdown`
takes only a skill ID — skills are copied verbatim with no variable or conditional-token
substitution, unlike agent rules. Templating skills would mean changing that signature
and every call site in all four backends. Reading config at runtime also means switching
reviewers does not require regenerating skills.

### Configuration

```json
"codeReviewer": "copilot" | "bosun" | "none" | "skill:<skill-name>"
```

Default `copilot`, preserving today's behavior for every existing repository. `none`
disables the cycle, matching `taskSystem: none`. `skill:<name>` is the user-defined
escape hatch: the cycle skill invokes the named skill and expects it to honor the
reviewer contract documented in the skill body.

Resolution precedence at run time: explicit skill argument, then `.rulesrc.json`
`codeReviewer`, then `copilot`.

### Skill naming

Add `github-pr-review-cycle` under an accurate name. Mark `github-pr-copilot-cycle`
deprecated through the existing `statusDeprecated` + `deprecationNote` machinery in
`cli/ballast/registry.go`, leaving its content in place and working so current installs
keep running until a removal version is chosen. Renaming in place would fail validation
for every repository that lists the old ID in `.rulesrc.json`.

## Files affected

**Skill content**
- `skills/common/github-pr-review-cycle/SKILL.md` *(new)* — reviewer-agnostic cycle
- `skills/common/github-pr-review-cycle/claude-settings.json` *(new)* — permissions
- `skills/common/github-pr-copilot-cycle/SKILL.md` — deprecation banner pointing at the new skill

**Registries (four backends, must stay in parity)**
- `cli/ballast/registry.go` — new entry + description; deprecate the old entry
- `packages/ballast-typescript/src/agents.ts` — `COMMON_SKILL_IDS`
- `packages/ballast-go/cmd/ballast-go/main.go` — `commonSkills` + usage text
- `packages/ballast-python/ballast/cli.py` — `COMMON_SKILLS`

**Config plumbing (mirrors `deploymentModel`)**
- `packages/ballast-typescript/src/config.ts` — constants, type, `RulesConfig`, normalize, write
- `packages/ballast-typescript/src/cli.ts` — `--code-reviewer` flag
- `packages/ballast-typescript/src/doctor.ts` — report the configured reviewer
- `cli/ballast/main.go` — flag, validation, config read/write, manifest text
- `packages/ballast-go/cmd/ballast-go/main.go` — same
- `packages/ballast-python/ballast/cli.py` — same

**Tests**
- `packages/ballast-typescript/src/{config,cli,doctor,agents,build}.test.ts`
- `cli/ballast/main_test.go`, `packages/ballast-go/cmd/ballast-go/main_test.go`
- `packages/ballast-python/tests/test_cli.py`

**Generated + packaged output**
- `pnpm --filter @everydaydevopsio/ballast run sync:content` → all four packaged copies
- Regenerate `.claude/` and `.codex/`
- Root `.rulesrc.json` — add the new skill to this repo's own `skills` list

**Docs**
- `docs/skills/github-pr-review-cycle.md` *(new)*, `docs/installation.md`, `README.md`

## Phases

- [x] **Phase 1 — Config plumbing.** `codeReviewer` through TS config, CLI flag, doctor;
      then Go wrapper, Go backend, Python. Tests per backend.
- [x] **Phase 2 — Skill content.** Write `github-pr-review-cycle` with three dispatch
      paths and the custom-skill contract. Add the deprecation banner to the old skill.
- [x] **Phase 3 — Registries.** All four, plus parity tests and the deprecation note.
- [x] **Phase 4 — Sync and regenerate.** `sync:content`, regenerate `.claude/`/`.codex/`,
      update root `.rulesrc.json`.
- [x] **Phase 5 — Docs.**

## Verification

- `pnpm lint && pnpm test` (TypeScript)
- `go test ./...` in `cli/ballast` and `packages/ballast-go`
- `uv run pytest` in `packages/ballast-python`
- Parity tests comparing each registry to its packaged content directory must pass
- `repo-generated-artifacts.test.ts` must pass — it checks checked-in `.claude/`/`.codex/`
  against freshly generated output
- Install smoke: `ballast install --skill github-pr-review-cycle` writes the skill and
  records `codeReviewer`; `ballast doctor` reports it

## Alternatives rejected

**Template the reviewer into the skill at build time.** Would need variables threaded
into skill rendering across four backends, and would force a regenerate-and-commit cycle
to change reviewer. Runtime config read is smaller and more flexible.

**A separate committed config file (`.github/code-review.json`).** No backend changes,
but splits repository policy across two files and stays invisible to `ballast doctor`.

**Rename `github-pr-copilot-cycle` in place.** Cleanest tree, but breaks validation for
every repository that lists the old ID.

**Nested `codeReview: { reviewer, ... }` object.** Room for per-reviewer settings, but
every other policy knob in `.rulesrc.json` is a flat scalar; a flat string keeps
validation and four-backend plumbing simple. Revisit if per-reviewer options are needed.

## Implementation notes

Three things the plan did not anticipate, all found by smoke-testing rather than by the
test suites:

- **Python `install()` was called positionally.** Inserting `code_reviewer` before
  `refresh_task_rules` silently shifted the argument, disabling task-rule refresh. Caught
  by an existing test; the call site now passes keywords.
- **The Go backend dropped the value at four separate points** — two `rulesConfig`
  construction paths, the `saveConfig` preservation branch, and the `loadConfig`
  conversion from `rawRulesConfig`. Unit tests passed throughout; only an end-to-end
  install surfaced it.
- **The wrapper's own config handling only runs for true monorepos.** Single-language
  repositories return `nil, nil` from `resolveMonorepoPlan` (`main.go:3066`) and forward
  args verbatim to the backend, so that path depends entirely on backend-side
  persistence.

The lesson for the next config knob: a passing four-backend test suite does not prove
four-backend parity. Install into a scratch repo with each backend and read the resulting
`.rulesrc.json`.

## Open questions

1. Removal version for the deprecated `github-pr-copilot-cycle` entry — needs a
   `Remove:` semver, or leave TBD.
2. Should `codeReviewer: bosun` be auto-detected when a Bosun App install is present, or
   stay explicit? Starting explicit.
3. Does the custom `skill:<name>` path need a machine-checkable findings contract, or is
   a documented prose contract enough for the first version? Starting with prose.

## Change log

| Date | Change |
| --- | --- |
| 2026-10-19 | Initial plan |
| 2026-10-19 | All five phases implemented; verified across four backends plus end-to-end installs |

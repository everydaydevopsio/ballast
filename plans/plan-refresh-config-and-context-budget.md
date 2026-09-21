# Plan: Refresh-Config Data Loss and Always-On Context Budget

- **Status**: Complete — ready to graduate to an ADR
- **Branch**: `ballast-upgrade-context-audit`
- **Created**: 2026-09-21
- **Related ADRs**: none yet

## Problem

A `ballast upgrade` audit across the `claude` and `codex` targets surfaced one data-loss bug and
a set of context-budget problems.

### 1. `--refresh-config` deletes every rule file in single-language repos

`ballast install --refresh-config` prints a full "Installed" list, rewrites `CLAUDE.md` to
reference those rules, then removes them from disk. The result is a manifest instructing the
agent to read and follow files that do not exist.

Root cause, in `cli/ballast/main.go`:

- Single-language repos write rules in a **flat** layout: `.claude/rules/publishing.md`.
- Multi-language repos write a **nested** layout: `.claude/rules/common/publishing.md`.
- `managedRulePathsWithSuffixes` (line ~4067) only ever emits the nested layout.
- `removeUnlistedManagedRuleFiles` (line ~3914) walks the rules root and deletes any
  Ballast-owned file not in that expected set.
- `cleanupSingleLanguageManagedSelections` (line ~3149) runs this against a single-language
  repo, so every flat rule file is classified stale and removed.

The flat layout is not legacy — it is the current, correct output for single-language repos.
`legacyRootManagedRulePaths` already computes exactly those paths, but is only ever consulted
for _removal_, never added to the expected set.

Severity is raised by this being the command Ballast's own `tasks-task-system` rule tells agents
to run when `taskSystem` changes.

### 2. Install logs report files that are never written

Each language backend runs in sequence and prunes rules it does not own, so later passes remove
what earlier passes wrote while the log still claims success. The final state is correct and
idempotent, but the log cannot be trusted — which is what made problem 1 hard to spot.

### 3. Inactive deployment rules consume always-on context

With `deploymentModel: none`, `publishing-web.md` and `publishing-api.md` are still emitted in
full — Kubernetes, GitOps, Argo CD, health probes — behind a banner telling the agent the
content is inactive. That is 2,037 tokens per session in this repo, and the same waste in every
Ballast-managed repo without a deployment target.

### 4. `publishingProfiles` works but is undiscoverable

The config key correctly scopes which publishing rules are emitted (verified on a clean
fixture), but it appears in no CLI help output, so nothing prompts a repo to set it. The default
emits all seven publishing rules.

### 5. Every config-save path erases `publishingProfiles` (found during implementation and review)

`resolveMonorepoPlan` rebuilds `.rulesrc.json` from scratch into `configToSave`, which never
carried `PublishingProfiles` across. Any multi-language repository that set the key lost it on
the next install, and the wrapper's cleanup — which derives the current rule set from the saved
config — then treated all seven publishing rules as in scope. This is why setting the key in
this repo initially had no effect while a single-language fixture honoured it.

Copilot review surfaced the same omission in the Go and Python backends' own `saveConfig` /
`save_config`. Both merge forward `taskSystem`, `deploymentModel`, `ruleProfile`, `discovery`,
and `tools`, but not `publishingProfiles`. The wrapper masked it — `ballast install` re-saves
afterwards — but direct backend invocation is a supported entry point for single-language
repos, so the key this plan recommends as the main context lever was unreliable there.

## Measured Baseline

Token counts via `cl100k_base` over file contents.

| Surface                                   |     Tokens | Loaded                         |
| ----------------------------------------- | ---------: | ------------------------------ |
| `CLAUDE.md`                               |      1,617 | always                         |
| `.claude/rules` (32 files)                |     16,993 | always                         |
| **Claude always-on total**                | **18,610** |                                |
| `publishing-web.md` + `publishing-api.md` |      2,037 | always, self-declared inactive |
| `tasks-todo.md` fenced templates          |        403 | always, reference material     |

Literal duplication is not a problem: a scan for repeated normative sentences of eight or more
words found five across all 32 files, four of which are deliberate cross-reference pointers.
The issue is scope, not repetition.

## Approach

Fix correctness first, then reduce always-on context by scoping rather than by dropping agents.
Ballast genuinely uses publishing, Spec Kit, plan lifecycle, tasks and four languages; cutting
agents would remove guidance the repo depends on.

Layout-awareness is the core fix for problem 1. Rather than special-casing, thread the rule
layout (flat vs nested) through the cleanup path so the expected set always matches what the
backends actually wrote.

## Files Affected

| Path                                         | Reason                                                               |
| -------------------------------------------- | -------------------------------------------------------------------- |
| `cli/ballast/main.go`                        | Layout-aware cleanup; `publishingProfiles` help text                 |
| `cli/ballast/main_test.go`                   | Regression tests for flat-layout cleanup and manifest/disk invariant |
| `packages/ballast-typescript/src/build.ts`   | Gate publishing web/api emission on deployment model                 |
| `packages/ballast-typescript/src/install.ts` | Same, install path                                                   |
| `packages/ballast-python/ballast/cli.py`     | Backend parity for the same gating                                   |
| `packages/ballast-go/cmd/ballast-go/main.go` | Backend parity for the same gating                                   |
| `agents/common/tasks/content-todo.md`        | Move fenced templates behind a lazy reference                        |
| `docs/agents/tasks.md`                       | Destination for the moved templates                                  |
| `.rulesrc.json`                              | Set `publishingProfiles`; register `docker-registry-publish`         |
| `.claude/`, `.codex/`                        | Regenerated outputs                                                  |
| `.cursor/`                                   | Remove — not a configured target                                     |

## Phases

### Phase 1 — Fix `--refresh-config` data loss

- [x] Add a failing Go test reproducing flat-layout rule deletion
- [x] Make the cleanup path layout-aware
- [x] Confirm the test passes and nested-layout cleanup still prunes correctly

### Phase 2 — Manifest/disk invariant

- [x] Add a test asserting every rule referenced in a generated manifest exists on disk
- [x] Run against both flat and nested layouts, and after `--refresh-config`

### Phase 3 — Reduce this repo's always-on context

- [x] Carry `publishingProfiles` through the monorepo config rebuild (problem 5)

- [x] Set `publishingProfiles: ["cli", "libraries"]` in `.rulesrc.json`
- [x] Regenerate `.claude/` and `.codex/`; confirm four publishing rules disappear
- [x] Re-measure always-on tokens

### Phase 4 — Document `publishingProfiles`

- [x] Add it to `ballast --help` and the relevant docs

### Phase 5 — Gate deployment rules on deployment model

- [x] Suppress `publishing-web` / `publishing-api` emission when `deploymentModel` is `none`
- [x] Keep all three backends and their parity tests in sync

### Phase 6 — Move reference payloads out of always-on rules (reverted)

- [x] Relocate the three `tasks-todo` fenced templates behind a docs reference
- [x] Verify the rule still carries the decision and the trigger
- [x] **Reverted after review**: install never writes `docs/`, so the pointer dangles in every
      consuming repo. Templates are inline again, a guard test prevents regression, and #363
      tracks doing the move properly via the skill `references/` mechanism.

### Phase 7 — Hygiene

- [x] Remove the stale `.cursor/` target
- [x] Register `docker-registry-publish` in `.rulesrc.json`
- [x] File issues for anything deferred

## Verification

- `make build` and the full test suite pass for all three backends plus the wrapper.
- A clean single-language fixture survives `install` &rarr; `--refresh-config` with rules intact.
- A clean multi-language fixture still prunes genuinely stale rules.
- Every rule path referenced in generated `CLAUDE.md` / `AGENTS.md` exists on disk.
- Always-on token count re-measured and recorded in the Change Log.

## Alternatives Rejected

- **Skip cleanup entirely for single-language repos.** Would stop pruning genuinely removed
  agents and skills, trading one bug for another.
- **Drop the `publishing` agent from this repo** (&minus;26% context). Ballast is a publishing
  tool; the guidance applies.
- **Drop agents to reach a minimal rule set** (&minus;53% to &minus;77%). Removes guidance the
  repo actively uses. Scoping within agents achieves most of the benefit with no loss.
- **Deduplicate rule prose.** Measured: only five repeated sentences exist. Nothing to gain.

## Open Questions

- ~~Should `deploymentModel: none` suppress the deployment rules outright, or should the
  suppression be tied to `publishingProfiles` alone?~~ **Resolved 2026-09-21:** smart default
  with explicit override. When `deploymentModel` is `none` _and_ `publishingProfiles` is unset,
  `web` and `api` drop out of the default set; an explicit `publishingProfiles` entry always
  wins, so a repository can deliberately keep the deployment reference text.

## Review Outcomes

Copilot review on PR #362, two cycles, six findings — all verified before acting, all real.

| Finding                                                         | Verdict | Resolution                                     |
| --------------------------------------------------------------- | ------- | ---------------------------------------------- |
| `docs/agents/tasks.md` is never installed into consuming repos  | Real    | Phase 6 reverted; guard test added; #363 filed |
| Wrapper cleanup ignored `deploymentModel`, leaving orphan rules | Real    | Fixed, 2 tests                                 |
| Python tests placed after the `__main__` guard                  | Real    | Guard moved; 125 &rarr; 129 collected          |
| `docs/installation.md` contradicted the new default             | Real    | Corrected                                      |
| Go `saveConfig` drops `publishingProfiles`                      | Real    | Fixed, 1 test                                  |
| Python `save_config` drops `publishingProfiles`                 | Real    | Fixed, 1 test                                  |

The first finding invalidated a premise recorded in this plan. Phase 6 was justified by
`local-dev-env.md` already using a `docs/agents/*.md` pointer — but that pointer is itself
dangling in every consuming repository, so it was never a working pattern. The distinction that
matters: `local-dev`'s pointer is to supplementary examples and degrades gracefully, whereas the
tasks templates are content the rule instructs agents to copy _verbatim_, so a dead pointer
breaks the instruction rather than thinning it.

## Deferred

Filed rather than fixed here, to keep this change reviewable:

- #356 `ballast upgrade` aborts non-atomically, leaving targets on different content versions
- #357 Install log reports rule files that are never written
- #358 Skill bundle zips are packed nondeterministically (store vs deflate)
- #359 Python `resolve_project_root` tests fail on macOS due to `/tmp` symlink (pre-existing)
- #360 `e2e-tools-rendered-in-rules.sh` fails on a clean checkout (pre-existing)
- #361 Root pre-commit hook is broken for any TypeScript change (pre-existing)
- #363 Installed rules point at `docs/agents/*.md` files that are never shipped to consumers

## Change Log

| Date       | Change                                                                                                         |
| ---------- | -------------------------------------------------------------------------------------------------------------- |
| 2026-09-21 | Plan created from the `ballast upgrade` context audit                                                          |
| 2026-09-21 | Phase 1 done: layout-aware cleanup (`ruleLayout`), 3 regression tests                                          |
| 2026-09-21 | Phase 2 done: manifest/disk invariant test covering both layouts and the refresh entry point                   |
| 2026-09-21 | Problem 5 found and fixed: `configToSave` dropped `publishingProfiles`                                         |
| 2026-09-21 | Phase 3 done: always-on context 18,610 &rarr; 15,608 tokens (&minus;3,002, &minus;16.1%)                       |
| 2026-09-21 | Phase 4 done: `publishingProfiles` documented in `ballast --help` and `docs/installation.md`                   |
| 2026-09-21 | Phase 5 done: `deploymentModel: none` drops `web`/`api` from the default profile set across all three backends |
| 2026-09-21 | Phase 6 done: `tasks-todo` templates moved to `docs/agents/tasks.md`; rule 1,047 &rarr; 639 tokens             |
| 2026-09-21 | Phase 7 done: `.cursor/` removed, `docker-registry-publish` registered, 5 issues filed                         |
| 2026-09-21 | Final: always-on context 18,610 &rarr; 15,231 tokens (&minus;3,379, &minus;18.2%)                              |

# ADR-001: Scope generated rules to repository shape, and make cleanup layout-aware

- **Status**: Accepted
- **Date**: 2026-09-21
- **Branch**: `ballast-upgrade-context-audit`
- **PR**: [#362](https://github.com/everydaydevopsio/ballast/pull/362)
- **Supersedes**: none
- **Superseded by**: none

## Context

A `ballast upgrade` audit across the `claude` and `codex` targets measured what Ballast actually
loads into an agent's context on every session, and checked that the generated output was
internally consistent. Two classes of problem came out of it.

### Generated state could contradict itself

`ballast install --refresh-config` printed a full "Installed" list, rewrote `CLAUDE.md` to
reference those rules, then deleted them from disk:

```
$ ballast install --target claude --agent publishing --yes
  -> 7 rule files on disk, 7 referenced in CLAUDE.md      ok

$ ballast install --refresh-config --yes
  Skipped (already present; use --force to overwrite): publishing
  -> 0 rule files on disk, 7 still referenced in CLAUDE.md
```

Single-language repositories are forwarded straight to their backend, which writes rules **flat**
(`.claude/rules/publishing.md`). Multi-language repositories write them **nested**
(`.claude/rules/common/publishing.md`). `managedRulePathsWithSuffixes` only ever emitted the
nested layout, so `removeUnlistedManagedRuleFiles` classified every flat rule as stale.
`legacyRootManagedRulePaths` already computed exactly those paths, but was only ever consulted
for _removal_.

The severity came from the blast radius: `--refresh-config` is the command Ballast's own
`tasks-task-system` rule tells agents to run when `taskSystem` changes.

### Always-on rule context was wider than any repository needs

Measured with `cl100k_base` over `CLAUDE.md` plus all 32 generated rule files, this repository
loaded **18,610 tokens** into every session. Of that:

| Surface                                   | Tokens | Note                        |
| ----------------------------------------- | -----: | --------------------------- |
| `publishing` family (7 files)             |  4,812 | repo publishes a CLI + libs |
| `publishing-web.md` + `publishing-api.md` |  2,037 | self-declared inactive      |
| `tasks-todo.md` fenced templates          |    403 | reference, not decision     |

`publishing-web.md` and `publishing-api.md` opened by telling the agent the content was inactive
(`deploymentModel: none`) and then shipped their full Kubernetes, GitOps, Argo CD, and
health-probe bodies anyway — spending always-loaded context to say nothing applied.

Notably, literal duplication was **not** a problem. A scan for repeated normative sentences of
eight or more words found five across all 32 files, four of them deliberate cross-reference
pointers. The rules were well factored. The problem was scope, not repetition.

## Decision

**Generated state must be internally consistent, and its scope must follow the repository's
actual shape.**

Three concrete commitments:

1. **Cleanup expects the layout the backend actually wrote.** A `ruleLayout` (`nested` / `flat` /
   `either`) is threaded through the cleanup path. The single-language refresh uses `either`, so
   a repository still holding nested output from an earlier multi-language install is not damaged
   either.

2. **Config keys that scope generated output are preserved by every save path.** Every
   config-rebuild site now carries `publishingProfiles` forward alongside `taskSystem`,
   `deploymentModel`, `ruleProfile`, `discovery`, and `tools` — the wrapper's `configToSave`, the
   Go backend's `saveConfig`, and the Python backend's `save_config`.

3. **Defaults follow repository shape, and explicit configuration always overrides them.** With
   `deploymentModel: none` and no explicit `publishingProfiles`, the `web` and `api` variants
   leave the default set. Listing a profile explicitly wins over that default, so a repository can
   deliberately keep the deployment reference text.

Scope is reduced **within** agents rather than by dropping agents. Ballast genuinely uses
publishing, Spec Kit, plan lifecycle, tasks, and four languages.

## Alternatives Considered

- **Skip cleanup entirely for single-language repositories.** Would stop pruning genuinely
  removed agents and skills — trading one bug for another.
- **Drop the `publishing` agent from this repository** (−26% context). Ballast is a publishing
  tool; the guidance applies.
- **Cut agents to reach a minimal rule set** (−53% to −77%, modelled and one variant measured at
  6,007 tokens). Removes guidance the repository actively uses.
- **Deduplicate rule prose.** Measured: only five repeated sentences exist. Nothing to gain.
- **Hard-suppress `web`/`api` whenever `deploymentModel` is `none`,** ignoring explicit profiles.
  Simpler, but makes two config keys fight each other and removes the ability to keep the
  reference text.
- **Move reference payloads to `docs/agents/*.md`.** Attempted and reverted — see Lessons Learned.

## Consequences

### Positive

- `--refresh-config` no longer destroys generated rules; the generated manifest can no longer
  reference a file that cleanup removed.
- Always-on rule context for this repository: **18,610 → 15,639 tokens (−16.0%)**, with no
  guidance lost that applies.
- Every downstream repository with no deployment target gets ~2,000 tokens back automatically,
  without having to discover a config key.
- `publishingProfiles` is now reliable through every entry point, including direct backend
  invocation, and is documented in `ballast --help` and `docs/installation.md`.
- `ballast doctor` and `ballast install` agree on what the current rule set is, including for
  configs predating the `deploymentModel` key.

### Negative

- Repositories relying on `publishing-web.md` / `publishing-api.md` as reference material without
  owning a deployment target must now list those profiles explicitly. This is a behaviour change
  for existing consumers, mitigated by the override and by the documented escape hatch.
- The `ruleLayout` parameter adds a concept to the cleanup path that a reader must hold in mind.
  The `either` variant in particular encodes a tolerance rather than a fact.
- Scope-narrowing logic now lives in four places (wrapper plus three backends) and must stay in
  parity. Parity tests exist in each, but the duplication is real.

## Implementation Notes

- `ruleLayout` and the `*ForLayout` function variants keep the existing call sites on the nested
  default, so the change is additive rather than a signature sweep.
- `DEPLOYMENT_PUBLISHING_PROFILES` (TypeScript), `deploymentPublishingProfiles` (Go),
  `DEPLOYMENT_PUBLISHING_PROFILES` (Python), and `deploymentPublishingSuffixes` (wrapper) are the
  four places the `web`/`api` set is named. Changing one without the others will show up as a
  manifest/disk mismatch.
- `asDeploymentModel` in `doctor.ts` falls back to `DEFAULT_DEPLOYMENT_MODEL` for both missing and
  unrecognized values, matching `install()` exactly. Returning `undefined` there is what made
  doctor report a rule as healthy that install would never create.

## Verification

- Wrapper and `ballast-go`: `go test ./...` green. `ballast-typescript`: 376 tests passing.
  `ballast-python`: 123 passing (7 pre-existing macOS `/tmp` symlink failures, [#359]).
- `scripts/e2e-*.sh`: 17/18 (1 pre-existing failure, [#360]).
- Regression proof: reverting the `ruleLayout` fix makes all three refresh tests fail; restoring
  it makes them pass. Each cross-backend persistence test was confirmed failing before its fix.
- End-to-end across TypeScript, Go, and Python, in all three states:

| State                                | Emitted               | Manifest   |
| ------------------------------------ | --------------------- | ---------- |
| `deploymentModel: none`, no profiles | 5 rules, no web/api   | consistent |
| `publishingProfiles: ["cli", "web"]` | 3 rules, web restored | consistent |
| `deploymentModel: kubernetes`        | 7 rules               | consistent |

- `ballast doctor` reports no stale or drifted rules on this repository.

## Lessons Learned

**The invariant worth testing was not "did cleanup prune correctly" but "can the generated state
contradict itself".** The original bug and its inverse — cleanup keeping rules the backends had
stopped emitting — are the same defect seen from two directions, and a single assertion catches
both: every rule the manifest references must exist, and every rule on disk must be referenced.
`TestCleanupNeverOrphansManifestReferencedRules` encodes it.

**A precedent is not evidence that a pattern works.** Moving the `tasks` templates to
`docs/agents/tasks.md` was justified by `local-dev-env.md` already pointing at
`docs/agents/local-dev.md`. Review showed that pointer is itself dangling: `ballast install`
writes rules, skills, and support files, never `docs/`. The distinction that matters is what the
pointer carries — `local-dev` points at supplementary examples and degrades gracefully, whereas
the tasks templates are content the rule instructs agents to copy _verbatim_, so a dead pointer
breaks the instruction rather than thinning it. Reverted; [#363] tracks doing it properly through
the skill `references/` mechanism, which already ships to consumers.

**A misleading log hides a real bug.** Install reported writing files it never wrote ([#357],
cross-backend pruning). Those phantom lines were indistinguishable from the destructive case,
which is what made the `--refresh-config` deletion hard to spot. Reporting what survived the run,
rather than what each pass attempted, would have surfaced it immediately.

**The same omission recurred at four independent config-save sites.** The wrapper, the Go backend,
and the Python backend each merged forward every cross-cutting setting except
`publishingProfiles`; the TypeScript backend was the only one that had it right. Adding a config
key is not done until every rebuild path carries it, and a parity test per backend is cheaper than
finding out from a user whose setting vanished.

## Related Issues

Filed during this work and deliberately left out of scope:

- [#356] `ballast upgrade` aborts non-atomically, leaving targets on different content versions
- [#357] Install log reports rule files that are never written
- [#358] Skill bundle zips are packed nondeterministically (store vs deflate)
- [#359] Python `resolve_project_root` tests fail on macOS due to `/tmp` symlink (pre-existing)
- [#360] `e2e-tools-rendered-in-rules.sh` fails on a clean checkout (pre-existing)
- [#361] Root pre-commit hook is broken for any TypeScript change (pre-existing)
- [#363] Installed rules point at `docs/agents/*.md` files that are never shipped to consumers

[#356]: https://github.com/everydaydevopsio/ballast/issues/356
[#357]: https://github.com/everydaydevopsio/ballast/issues/357
[#358]: https://github.com/everydaydevopsio/ballast/issues/358
[#359]: https://github.com/everydaydevopsio/ballast/issues/359
[#360]: https://github.com/everydaydevopsio/ballast/issues/360
[#361]: https://github.com/everydaydevopsio/ballast/issues/361
[#363]: https://github.com/everydaydevopsio/ballast/issues/363

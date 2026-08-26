# Plan: Open Issue Review and Next Priorities

**Status:** Updated 2026-08-26 after the context-hygiene rules review.
**Created:** 2026-07-09
**Source:** Full review of generated rule output and the Ballast generation pipeline, plus live GitHub issue review for `everydaydevopsio/ballast`.

## Summary

The root-selection safety regression (#278) is fixed and closed. A full review of the generated rules found that the emitted `.claude/rules/` payload in this repo is ~133 KB (~33k tokens), all of it eagerly loaded into every Claude Code session, and that roughly 60–70% of it is duplication, inactive content, or reference material. Twelve issues (#286–#297) now track the reduction work.

The context-hygiene workstream is the new top priority: it improves every downstream agent interaction in every Ballast-managed repo, and the first phase is small, localized changes in the build pipeline with the largest per-byte payoff.

## Current Recommendation

Implement the context-hygiene phases in order (#286/#287/#288 first), then return to the setup/toolchain workstream (#128, #94).

| Priority | Issue(s) | Current intent |
| --- | --- | --- |
| 1 | #286, #287, #288 | Phase 1 quick wins: emit tool policy once in the manifest, stop emitting inactive/reference-only rules, populate Repository Facts from `.rulesrc.json`. ~40 KB saved per target, all localized in the build/install path. |
| 2 | #291, #290, #293, #292, #294 | Phase 2 dedupe: shared-fragment includes (enabler), then testing-rule dedupe, config-aware task-system rendering, reference-bloat trims, and framing cleanup. |
| 3 | #289, #295 | Phase 3 restructure: consolidate the publishing family onto the shared release pattern; add `ruleProfile` (`full`/`standard`/`minimal`) for small-context agents. |
| 4 | #296, #297 | Phase 4 guardrails: CI size gate from ballast-audit thresholds (land after reductions so it starts green); investigate per-target path/glob-scoped rule loading. |
| 5 | #128, #94 | Combined setup/toolchain workstream: package-manager detection, Node LTS alignment, local tool prerequisite checks, Homebrew remediation guidance. |
| 6 | #153, #149, #147 | GitHub health report workflow, then interactive remediation on top of it. |

## Active Workstream: Context Hygiene (#286–#297)

Goal: reduce the always-loaded rule payload from ~33k tokens to ~10k (standard) or ~1–2k (minimal profile) per session with zero loss of actual policy, and prevent regression.

### Phase 1 — Quick wins in the build pipeline (do first)

- **#286** Emit Repository Tool Policy once in the CLAUDE.md/AGENTS.md manifest; remove the per-rule injection (`insertRepositoryToolPolicy` in `packages/ballast-typescript/src/build.ts`). Saves ~15.5 KB per target.
- **#287** Skip emission of inactive/reference-only rules (apt/brew opt-ins, deployment-model-gated content, the `local-dev-mcp` tombstone); make `renderDeploymentModelGuidance` prune sections structurally. Saves ~24 KB per target here. Doctor must clean up rules that become inactive.
- **#288** Populate Repository Facts from `.rulesrc.json` and git metadata at install time; the section is currently the unfilled template placeholder.

Why first: largest savings per line of code changed, no content rewrites required, and #286/#288 together establish the manifest as the single home for repo-wide policy.

### Phase 2 — Dedupe shared content

- **#291** Shared-fragment include mechanism (`{{include:...}}`) generalizing the existing guidance tokens. Enabler for #290 and #289; implement across TypeScript, Python, and Go backends.
- **#290** Testing rules: single common TDD block, shared smoke/E2E and framework-detection fragments; language rules shrink to commands and markers.
- **#293** `tasks-task-system`: render only the configured task system and only the emitting target's platform setup.
- **#292** Trim reference bloat: drop the inline MIT text, badge example gallery, Dependabot tutorial config, and inline plan/ADR templates (move templates to `docs/`).
- **#294** Remove persona preambles and duplicated H1/description stacking from rule outputs.

### Phase 3 — Restructure and profiles

- **#289** Consolidate the publishing family (8 files / ~52 KB) into a shared release-pattern rule plus thin per-artifact deltas; move workflow/goreleaser YAML templates into an on-demand skill. Depends on #291.
- **#295** `ruleProfile` setting: `minimal` compiles a single ~3–5 KB core rule and converts the rest to skills; `standard` (new default) enforces rules-vs-skills at the 5 KB boundary; `full` preserves current behavior. Directly serves small-context agents (Codex, Gemini, Haiku-class subagents), with per-target overrides.

### Phase 4 — Guardrails and target research

- **#296** CI gate + doctor check enforcing the ballast-audit thresholds (no emitted rule > 5 KB; per-target payload budget). Land after Phases 1–3 so it starts green.
- **#297** Investigate per-target scoped loading (Cursor globs first); document eager vs on-demand loading behavior per target in ARCHITECTURE.md.

Cross-cutting note: every phase changes `agents/` sources or the build pipeline, so each PR must regenerate and commit the local `.claude/` and `.codex/` outputs, per repo policy.

## Up Next After Context Hygiene

### Implement #128 and #94 Together: Setup and Toolchain Reliability

Goal:

- Detect the Node package manager from `package.json#packageManager` first, then lockfiles.
- Align generated setup, local development, and CI guidance with the detected package manager.
- Prefer Node.js Active LTS or Maintenance LTS guidance rather than stale current-version examples.
- Remove hardcoded stale defaults such as generic `pnpm@9` or `version: 9` where they are no longer appropriate.
- Add local prerequisite detection for tools expected by installed Ballast rules.
- Report which required and optional tools are present on `PATH`.
- Provide Homebrew remediation guidance where Homebrew is the expected install path.

Why grouped:

- Both issues affect first-run/setup correctness and generated local-development guidance.
- Package-manager detection should feed directly into local tool prerequisite checks.
- A combined implementation avoids separate, conflicting setup metadata models.

Note: #288 (Repository Facts population) creates the natural landing spot for detected package-manager and tool facts, so doing Phase 1 first reduces rework here.

## Deferred Items

### GitHub Health and Repo Automation

- #153: Daily repo health check GitHub Action with structured report.
- #149: Interactive setup prompts in the `github-health-check` skill.
- #147: GitHub repo setup workflow and best practices skill.
- #90: GitHub Actions Slack notifications for successful and failed builds.

Recommendation:

- Design #153 first as a read-only scheduled report.
- Build #149 and #147 on top of the stable report/remediation model.
- Keep Slack notifications separate unless #153 defines a reusable notification contract.

### Platform and Agent Expansion

- #133: Create MCP server so AI agents can configure and use Ballast directly.
- #10: Add agent validation tests with Dockerfiles and rule validation per AI platform. Start from the generated artifact inventory gate in `.github/workflows/generated-agent-artifacts.yml`, which runs `src/repo-generated-artifacts.test.ts` so stale common-rule copies under language-specific rule directories fail in CI; promote it to strict `BALLAST_ENFORCE_REPO_GENERATED_ARTIFACTS=1` byte-for-byte drift enforcement after version-stamp drift is cleaned up.
- #99: Add Next.js-specific TypeScript rules.
- #124: Enhance Ballast toward a robust Agentic SDLC framework.

Recommendation:

- Defer #133 until setup and doctor behavior are stable enough to expose through MCP.
- Defer #10 until the expected generated outputs and prerequisite checks are explicit; the context-hygiene phases will churn generated outputs, so strict drift enforcement should wait until after Phase 3.
- Defer #99 until framework-specific rule selection has a clear design; new rules should be written against the post-#291 fragment model and the #296 size budget.
- Treat #124 as a strategic umbrella, not a single near-term implementation task. The `ruleProfile` work (#295) advances it.

### Product, Release, and Documentation Strategy

- #92: Define how product requirements documents are created, maintained, synced with the app, and presented over time.
- #81: Deploy `ballast-python` to the everydaydevopsio organization on PyPI.
- #65: Improve ballast-go release installer portability by removing shell tool assumptions.
- #11: First-run interview: collect project preferences.

Recommendation:

- Keep #92 as a planning/design task.
- Handle #81 only when release ownership and the exact PyPI organization name are confirmed.
- Revisit #65 with release/install work.
- Fold #11 into setup improvements after #128 and #94 define the data model.

## Completed Work Kept for Context

Recent completed work that shaped this priority order:

- #278: Safe root selection for unmarked nested projects, closed 2026-08-25.
- #285: Generated agent rule dedupe fix, merged 2026-08-25.
- #275 and #276: Dependabot cleanup, completed after CI passed.
- #274: Copilot review cycle completion criteria, completed via PR #274.
- #273: Ansible syntax-check pre-push guidance, completed via PR #273.
- #272 and #188: OpenCode target removal cleanup and verification, completed via PRs #268 and #272.
- #271 and #166: Doctor managed rule drift detection, completed via PR #271.
- #270, #158, and #159: Task templates and TDD testing guidance, completed via PR #270.
- #269 and #151: Ansible rule guidance cleanup, completed via PR #269.
- #154: Plan -> ADR lifecycle rule, closed on 2026-07-26.

Excluded from this plan:

- #61: Support native Windows executable detection in TypeScript doctor, because it is labeled `windows`.

## Notes

- If implementation changes repo-root `agents/`, `skills/`, Ballast sync/build scripts, or root target config, regenerate and commit the corresponding local Ballast-managed `.claude/` and `.codex/` outputs in the same PR.
- Do not edit checked-in generated `.claude/` or `.codex/` rule outputs directly. Change source templates/content under repo-root `agents/` and `skills/`, then regenerate.
- Context-hygiene baseline measured 2026-08-26: `.claude/rules/` = 133,310 bytes across 32 files; largest offenders publishing-cli (8.9 KB), publishing-apps (8.3 KB), publishing-api (7.9 KB), cicd (7.6 KB); "Repository Tool Policy" duplicated 31 times. Use this as the before-number when validating #286–#296.
- Keep the `windows` issue #61 out of this non-Windows priority plan unless the user explicitly asks to include Windows work.

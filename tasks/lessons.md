# Lessons

## 2026-09-29 A Tool That Reports Success Can Be Reporting Nothing
- Incident/bug: Every pre-commit hook in the repo was silently inert. `.husky/pre-commit` survived an old commit and `"prepare": "husky"` re-pointed `core.hooksPath` at `.husky/_` on every install, so git ran husky's `npx lint-staged` and never ran `.pre-commit-config.yaml` — gitleaks, detect-private-key, yamllint and the per-package gates included.
- Root cause pattern: Two hook systems installed, one of them capable of seizing `core.hooksPath` from a package `prepare` script. The losing system failed open and silently.
- Early signal missed: `tsc-files` failing to spawn had made `--no-verify` routine. The workaround was treated as the fix, so nobody asked what else `--no-verify` was switching off. A symptom everyone routes around is load-bearing.
- Preventative rule: A repository has exactly one hook system. When a hook fails, establish which system git is actually invoking (`git config core.hooksPath`) before repairing the command inside it.
- Validation added (test/check/alert): `pre-commit-config.test.ts` asserts no `.husky/` directory and no `prepare`/`husky`/`lint-staged` entry in any package manifest; confirmed to fail when `.husky/pre-commit` is restored.
- Next trigger to detect sooner: Any commit made with `--no-verify`, and any hook whose tooling cannot be resolved from the directory git runs it in.

## 2026-09-29 Verify the Claim Before Planning the Fix
- Incident/bug: A seven-issue cluster sat untriaged for a week because it looked like one large job. Checking each against the working tree turned it into four real fixes and two obsolete reports.
- Root cause pattern: Planning from issue text rather than from current behavior. One issue described nondeterministic zip output in code that no longer ships anything; another was a stale test script, not a broken renderer.
- Early signal missed: An earlier check concluded `doctor --fix` was safe in 5.21.0. It had only exercised the path where config still listed the target; the broken path was a target *leaving* the config. Writing the regression test proved the bug was still live.
- Preventative rule: Reproduce each reported defect against the current tree before sequencing work, and write the regression test before believing a bug is fixed. Order by blast radius of the failure, not issue number.
- Validation added (test/check/alert): Regression tests for the `doctor --fix` manifest invariant and the install-log path invariant; all 18 e2e scripts now run in CI, closing the gap that let one fail unnoticed.
- Next trigger to detect sooner: Any issue older than a few days, and any "this is fixed now" conclusion not backed by a test that failed first.

## 2026-07-09 Repository Facts Tests Must Isolate Git Discovery
- Incident/bug: A Go support-file test passed directly but failed under the pre-push hook because Git push environment allowed repository-fact discovery to return the real remote/default branch.
- Root cause pattern: Tests asserting placeholder repository facts did not stub the package-level command runner used for `git` discovery.
- Early signal missed: The test used a temp directory but still allowed external `git -C <temp>` behavior to inherit hook-time Git context.
- Preventative rule: Tests expecting placeholder repository facts must stub git command discovery or create an explicit isolated git fixture.
- Validation added (test/check/alert): `TestBuildMonorepoSupportFileIncludesPublishingAndSkillsForCodex` now stubs `git` command output, and `scripts/run-unit-tests-pre-push.sh` passes under the branch.
- Next trigger to detect sooner: When a test depends on absent repo metadata, audit package-level command hooks and Git environment inheritance.

## 2026-04-29 Managed Skill Files Must Refresh on Upgrade
- Incident/bug: `ballast upgrade` replayed saved skill selections but left existing skill files stale unless `--force` was passed.
- Root cause pattern: Generated skill artifacts reused agent-rule overwrite semantics even though skills are Ballast-managed shipped content.
- Early signal missed: Tests asserted existing skills were skipped instead of asserting refresh behavior for managed artifacts.
- Preventative rule: When adding generated managed assets, test refresh/upgrade semantics separately from user-editable rule patch semantics.
- Validation added (test/check/alert): Cross-backend unit tests plus wrapper upgrade smoke coverage for stale skill refresh.
- Next trigger to detect sooner: During PR review, verify whether an installed artifact is user-owned or managed before reusing skip/force behavior.

## 2026-03-02 Installer Asset Location Must Be Package-Safe
- Incident/bug: Installers assumed repo-relative `agents/` paths, breaking packaged installs.
- Root cause pattern: Runtime path resolution relied on monorepo layout instead of shipped assets.
- Early signal missed: Packaging manifests claimed assets but package-local directories were absent.
- Preventative rule: Any installer that reads templates/content must default to packaged resources and treat repo-root access as an explicit dev override.
- Validation added (test/check/alert): npm pack smoke check plus temp-dir runtime install smoke checks.
- Next trigger to detect sooner: During PR review, verify `npm pack`/wheel/go install outputs contain all runtime assets.

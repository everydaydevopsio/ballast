# Lessons

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

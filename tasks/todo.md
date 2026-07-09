# Tasks

- [x] Confirm issue #208 scope, operating mode, and governing PRD gap.
- [x] Add PRD requirements for generated `.ballast/` state and Ballast repair skill support.
- [x] Add failing coverage for missing `.ballast/` doctor/install-cli behavior and the new skill registry entry.
- [x] Implement wrapper state reporting/remediation and the Ballast maintenance skill.
- [x] Update docs, agent guidance, package mirrors, and generated local target outputs.
- [x] Run targeted verification and capture evidence.

## Previous Tasks

- [x] Confirm issue #211 scope and governing operating mode.
- [x] Add PRD requirements and acceptance criteria for `ballast setup-dev`.
- [x] Add failing wrapper tests for Corepack/package-manager setup behavior.
- [x] Implement the canonical wrapper `setup-dev` command.
- [x] Update local-dev agent guidance and regenerated Ballast-managed outputs.
- [x] Run targeted and full verification commands.
- [x] Push branch, open PR, and request Copilot review.

- [x] Confirm issue #144 root cause and affected backends.
- [x] Add PRD acceptance criteria for managed skill refresh.
- [x] Add failing unit coverage for TypeScript, Python, and Go skill refresh behavior.
- [x] Add failing wrapper smoke coverage for upgrade refreshing stale skill files.
- [x] Implement the minimal cross-backend fix while preserving agent rule overwrite semantics.
- [x] Run targeted and full verification commands.
- [x] Push branch, open PR, and request Copilot review.

- [x] Confirm the operator-visible behavior gap in `ballast doctor`.
- [x] Identify the governing requirements for the CLI output change.
- [x] Add failing tests for `languages` and `paths` in `doctor` output.
- [x] Implement the minimal reporting change across CLIs.
- [x] Run targeted tests and capture evidence.
- [x] Document the `novnc-desktop` language-detection root cause and recommended fix.
- [x] Add wrapper tests for JavaScript package warnings.
- [x] Implement JavaScript package warning logic in single-language and monorepo detection.
- [x] Add smoke coverage for JavaScript package warnings.
- [ ] File a GitHub issue for integration-test framework detection and Playwright guidance. Blocked here by GitHub token permissions and network access.

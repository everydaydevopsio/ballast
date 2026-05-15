# Product Requirements

## Generated Rule Context Hygiene

### Problem

Ballast-generated rule files for persistent agent context have accumulated large generic playbooks and repeated examples. This creates avoidable context bloat for installed targets such as Codex, especially in the `local-dev`, `linting`, `logging`, and `testing` rule families. The source templates need explicit size and density constraints so future generated rules stay concise without editing checked-in installed rule snapshots by hand.

### Requirements

1. Canonical rule templates under repo-root `agents/` must prefer concise persistent guidance over long example-heavy walkthroughs.
2. Deep reference material should live in documentation or skills, while persistent rules keep only the minimum instructions needed to route the agent correctly.
3. Source template updates must flow through the existing package-content sync path so TypeScript, Python, and Go package payloads can be refreshed from the same canonical sources.
4. The change must not require manual edits to installed repository rule snapshots such as `.codex/rules/*`.
5. Automated tests must enforce size budgets for the worst persistent Codex rule offenders.
6. Package-content sync must delete stale mirrored files when source files are removed or renamed.
7. Root `.rulesrc.json` target policy must include every checked-in Ballast-managed target surface that the repo expects to keep refreshed.
8. Repository guidance must require PRs that change Ballast generator inputs or target policy to include regenerated local Ballast-managed `.claude` and `.codex` outputs.

### Acceptance Criteria

1. The generated Codex `local-dev-env` rule built from source templates is smaller than 6 KB while still mentioning `.nvmrc`, `docker-compose.local.yaml`, `Makefile`, and `make up-local`.
2. The generated Codex TypeScript `logging` rule built from source templates is smaller than 6 KB while still mentioning `pino-browser` and `/api/logs`.
3. The generated Codex TypeScript `testing` rule built from source templates is smaller than 6 KB.
4. The generated Codex TypeScript `linting` rule built from source templates is smaller than 5 KB.
5. The content sync workflow can refresh package template mirrors from the repo-root `agents/` and `skills/` sources without editing installed target rule directories directly.
6. Content sync deletes stale mirrored files when a source file is removed or renamed.
7. Root `.rulesrc.json` includes `claude` and `codex` so tracked generated artifacts for both targets can be refreshed by config-driven installs.
8. `AGENTS.md` documents that PRs touching Ballast generator inputs or target policy must include regenerated local `.claude` and `.codex` artifacts.

## Skill Patch Support And Support File Force Confirmation

### Problem

Ballast does not apply the same overwrite decision matrix to installed skill files that it applies to agent rule files, and `--force` can silently replace support files such as `AGENTS.md`, `CLAUDE.md`, and `GEMINI.md`. Operators need a safe way to merge upstream skill updates without discarding local edits, and destructive support-file overwrites must require explicit confirmation.

### Requirements

1. Existing installed skill files must follow the same force/patch/skip decision matrix as agent rule files across the TypeScript, Python, Go, and wrapper install paths.
2. `--patch` must merge canonical skill content into an existing skill file using the existing patch logic for the selected target.
3. `--force` must overwrite an existing skill file without patching.
4. `--force` must prompt before replacing an existing support file (`AGENTS.md`, `CLAUDE.md`, or `GEMINI.md`) that would lose user customizations.
5. In non-interactive mode (`--yes` or CI environment variables), an attempted `--force` overwrite of an existing support file must fail with a clear error telling the operator to rerun interactively without `--yes`.
6. Creating a missing support file with `--force` must continue without prompting.
7. Support-file patch behavior and existing agent-rule overwrite semantics must remain unchanged.
8. README and installation documentation must describe the updated `--patch` and `--force` behavior.
9. If an existing Claude `.skill` archive is unreadable during `--patch`, install must recover by overwriting it with canonical packaged skill content instead of failing the run.

### Acceptance Criteria

1. Given an existing skill file and `force=false, patch=false`, install skips the file and leaves the existing content unchanged.
2. Given a missing skill file and `force=false, patch=true`, install creates the file with canonical content.
3. Given an existing skill file and `force=false, patch=true`, install merges canonical content into the existing file and preserves user-managed sections supported by the patcher.
4. Given an existing skill file and `force=true`, install overwrites the file with canonical content.
5. Given an existing support file and interactive `--force`, answering no skips the support file and prints a clear notice.
6. Given an existing support file and interactive `--force`, answering yes overwrites the support file with canonical content.
7. Given an existing support file and non-interactive `--force`, install exits with an error and does not overwrite the file.
8. Automated tests cover the skill-file decision matrix and support-file confirmation behavior in the TypeScript, Python, and Go backends.
9. README and `docs/installation.md` describe when to use `--patch` versus `--force`, including the support-file confirmation behavior.
10. Given an existing unreadable Claude `.skill` archive and `force=false, patch=true`, install replaces it with canonical content and completes without an install error.

## Ballast Upgrade Skill Refresh

### Problem

Ballast-installed skill files are generated managed artifacts, but the current refresh behavior treats existing skill files like user-owned agent rules. As a result, `ballast upgrade` replays saved `.rulesrc.json` skill selections without updating stale skill file content unless the operator also passes `--force`.

### Requirements

1. Normal install refreshes must rewrite selected managed skill files when they already exist.
2. `ballast upgrade` and `doctor --fix` must refresh saved skill selections through their existing `install --refresh-config` path without requiring `--force`.
3. Existing agent rule overwrite, patch, and force semantics must remain unchanged.
4. The behavior must stay consistent across the TypeScript, Python, Go, and wrapper CLIs.
5. Support files such as `AGENTS.md` and `CLAUDE.md` must continue reflecting the saved skill list after refresh.

### Acceptance Criteria

1. Given an existing installed skill file with stale content, running backend install with the same skill and `force=false` rewrites the file to current packaged skill content.
2. Given an existing installed agent rule and `force=false`, backend install still skips the rule unless patch mode or force mode is selected.
3. Given a repository with `.rulesrc.json` that declares a skill, running wrapper `upgrade` without `--force` invokes the refresh path that updates the existing managed skill file.
4. Automated unit coverage demonstrates the backend skill refresh behavior for TypeScript, Python, and Go.
5. Smoke coverage demonstrates the wrapper upgrade path refreshes stale managed skill content.

## Existing Install Refresh Reconciliation

### Problem

Ballast refreshes saved installs from `.rulesrc.json`, but removing managed agents or skills from saved config can leave stale managed files on disk for targets that remain installed. Operators need refresh behavior that reconciles the managed surface to the current saved config instead of only adding newly selected content.

### Requirements

1. Wrapper `install --refresh-config`, `upgrade`, and `doctor --fix` must remove stale managed agent-rule files for agents no longer present in saved `.rulesrc.json` while preserving remaining configured agents.
2. Wrapper `install --refresh-config`, `upgrade`, and `doctor --fix` must remove stale managed skill files for skills no longer present in saved `.rulesrc.json` while preserving remaining configured skills.
3. Reconciliation must apply only to Ballast-managed files for targets that remain installed; it must not remove unrelated user files.
4. Support-file managed sections such as `AGENTS.md` and `CLAUDE.md` must be refreshed so removed agents and skills are no longer referenced.
5. Target removal behavior must remain unchanged and continue deleting the full Ballast-managed surface for removed targets.

### Acceptance Criteria

1. Given an existing install with configured agents `linting, docs`, editing `.rulesrc.json` to keep only `linting` and running wrapper `install --refresh-config` deletes the managed `docs` rule files while leaving `linting` files intact.
2. Given an existing install with configured skills `owasp-security-scan, github-health-check`, editing `.rulesrc.json` to keep only `owasp-security-scan` and running wrapper `install --refresh-config` deletes the managed `github-health-check` files while leaving `owasp-security-scan` intact.
3. After either reconciliation flow, support files no longer list removed agent or skill references.
4. Refresh does not delete unmanaged user-authored files outside the Ballast-managed paths for the retained targets.

## Ballast Doctor Config Visibility

### Problem

Operators use `ballast doctor` to inspect the effective Ballast state for a repository, but the current report omits the saved `languages` and `paths` from `.rulesrc.json`. This makes it hard to confirm which language profiles Ballast considers installed in monorepos or mixed-language repos.

### Requirements

1. `ballast doctor` must display configured `languages` when `.rulesrc.json` contains them.
2. `ballast doctor` must display configured `paths` when `.rulesrc.json` contains them.
3. `ballast doctor` must display configured `taskSystem` when `.rulesrc.json` contains it.
4. The change must apply consistently across the TypeScript, Python, Go, and wrapper CLIs.
5. Existing `doctor` output for targets, agents, skills, installed CLIs, and recommendations must remain intact.

### Acceptance Criteria

1. Given a `.rulesrc.json` with `languages`, `ballast doctor` prints a `- languages: ...` line in the `Config:` section.
2. Given a `.rulesrc.json` with `paths`, `ballast doctor` prints a `- paths: ...` line in the `Config:` section.
3. Given a `.rulesrc.json` with `taskSystem`, `ballast doctor` prints a `- taskSystem: ...` line in the `Config:` section.
4. Given a `.rulesrc.json` without `languages`, `paths`, or `taskSystem`, `ballast doctor` does not print empty placeholder lines for those fields.
5. Automated tests cover the new output in each CLI implementation that renders `doctor` output.

## JavaScript Detection Warning

### Problem

Some repositories contain browser or Node.js components that are still JavaScript-first and therefore do not produce a reliable TypeScript profile for Ballast. This can hide real application components from Ballast's language/profile reporting, especially in mixed-language repos.

### Requirements

1. The `ballast` wrapper must warn when it detects a real `package.json`-based JavaScript component or app without a `tsconfig.json`.
2. The warning must apply in both single-language detection and monorepo planning paths.
3. The warning must tell the operator to convert the component to TypeScript or add `tsconfig.json` so Ballast can track it as a TypeScript profile.
4. The warning must not trigger for placeholder `package.json` files that do not look like an app or component.

### Acceptance Criteria

1. Given a repo with `package.json` containing app/component signals and no `tsconfig.json`, calling the wrapper detection path emits a warning on stderr.
2. Given a mixed-language repo where monorepo planning detects non-TypeScript profiles but the root still contains a JavaScript app/component, the wrapper emits the same warning on stderr.
3. Given a repo with `tsconfig.json`, the warning is not emitted.
4. Smoke coverage must exercise at least one single-language JavaScript package case and one mixed-language non-TypeScript monorepo case that emits the warning.

## Git Repository Boundary Root Resolution

### Problem

Running `ballast install` from a directory that is its own git repository but does not contain Ballast project markers can incorrectly install files into a parent directory when the parent does contain recognized markers. Root resolution currently walks upward looking for project markers and must not escape the active git repository.

### Requirements

1. Project-root resolution must check the current directory for recognized Ballast project markers before considering parent directories.
2. Upward traversal must stop at the first git repository boundary when the current directory does not contain project markers.
3. The change must apply consistently across the TypeScript, Python, and Go Ballast backends, and the `ballast` wrapper must treat Ballast config files as project-root markers.
4. Root resolution for nested directories inside a marked repository must continue to work when the traversal has not yet crossed a git boundary.
5. End-to-end smoke coverage must exercise the case where a child git repo has no project markers and its parent does.

### Acceptance Criteria

1. Given a child directory that is its own git repository and contains no recognized project markers, Ballast resolves the project root to that child directory rather than a marked parent directory.
2. Given a nested directory inside a repository whose root contains recognized project markers, Ballast resolves the project root to the marked repository root.
3. TypeScript, Python, and Go automated tests cover the git-boundary stop condition.
4. Wrapper tests cover root resolution when a repo is anchored by `.rulesrc.json` and legacy Ballast config files.
5. The examples smoke workflow runs an end-to-end git-boundary scenario that fails if install output is written to the parent directory.

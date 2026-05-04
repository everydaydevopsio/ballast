# Product Requirements

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

## Ballast Doctor Config Visibility

### Problem

Operators use `ballast doctor` to inspect the effective Ballast state for a repository, but the current report omits the saved `languages` and `paths` from `.rulesrc.json`. This makes it hard to confirm which language profiles Ballast considers installed in monorepos or mixed-language repos.

### Requirements

1. `ballast doctor` must display configured `languages` when `.rulesrc.json` contains them.
2. `ballast doctor` must display configured `paths` when `.rulesrc.json` contains them.
3. The change must apply consistently across the TypeScript, Python, Go, and wrapper CLIs.
4. Existing `doctor` output for targets, agents, skills, installed CLIs, and recommendations must remain intact.

### Acceptance Criteria

1. Given a `.rulesrc.json` with `languages`, `ballast doctor` prints a `- languages: ...` line in the `Config:` section.
2. Given a `.rulesrc.json` with `paths`, `ballast doctor` prints a `- paths: ...` line in the `Config:` section.
3. Given a `.rulesrc.json` without `languages` or `paths`, `ballast doctor` does not print empty placeholder lines for those fields.
4. Automated tests cover the new output in each CLI implementation that renders `doctor` output.

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

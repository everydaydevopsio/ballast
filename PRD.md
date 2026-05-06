# Product Requirements

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

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

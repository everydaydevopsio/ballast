# Product Requirements

## Ballast Local State Bootstrap And Repair

### Problem

Ballast stores project-local backend CLIs under `.ballast/`, but the product contract does not clearly say whether that directory is required source state, generated state, or disposable cache state. Operators and AI agents need deterministic behavior when `.ballast/` is absent or incomplete so they can inspect, initialize, and repair a Ballast-managed repository without guessing.

### Requirements

1. `.ballast/` must be treated as generated, repository-local tool state that is safe to recreate and should remain ignored by git.
2. `ballast install` must not require `.ballast/` to exist before installing agent rules or skills.
3. Wrapper backend dispatch must recreate missing local tool directories when it needs to install a backend CLI.
4. `ballast install-cli` must recreate `.ballast/bin` and `.ballast/tools` before installing backend CLIs.
5. `ballast doctor` must report whether `.ballast/`, `.ballast/bin`, and `.ballast/tools` exist.
6. `ballast doctor` must provide actionable remediation when `.ballast/` state is missing or incomplete.
7. `ballast doctor --fix` must recreate missing `.ballast/` directories through the same local CLI install path.
8. Agent guidance must explain how to check and repair Ballast local state.
9. Ballast must ship a dedicated common skill for AI agents that covers Ballast-managed repository status, bootstrap, and repair workflows.
10. Documentation must describe the dedicated Ballast skill and how to install it.

### Acceptance Criteria

1. Given a repository without `.ballast/`, `ballast doctor` reports `.ballast: missing` and recommends `ballast install-cli` or `ballast doctor --fix`.
2. Given a repository with `.ballast/` but missing `.ballast/bin` or `.ballast/tools`, `ballast doctor` reports the incomplete state and recommends repair.
3. Given a repository without `.ballast/`, `ballast install-cli --language go --version <version>` creates `.ballast/bin` and `.ballast/tools` before running the install command.
4. Given a repository without `.ballast/`, `ballast doctor --fix` creates `.ballast/bin` and `.ballast/tools` before running backend install commands.
5. Generated local-dev guidance tells agents to use `ballast doctor` to inspect Ballast local state and `ballast doctor --fix` or `ballast install-cli` to repair it.
6. The new Ballast skill is available through `--skill ballast-project-maintenance` and `--all-skills`.
7. README and installation docs document `.ballast/` as generated local state and list the Ballast project maintenance skill.

## Agent Development Environment Bootstrap

### Problem

AI agents need a deterministic first command that prepares a repository for local development before they inspect, edit, or test code. Without a canonical bootstrap path, agents may miss prerequisite setup such as enabling Corepack for a declared Node package manager, which can leave commands like `pnpm` unavailable even when the repository clearly declares them.

### Requirements

1. The Ballast wrapper must expose a canonical `setup-dev` command for agent startup.
2. `setup-dev` must resolve the project root using the same root-detection behavior as other wrapper commands.
3. For Node repositories with a declared `packageManager`, `setup-dev` must enable Corepack before running package-manager installs when the declared manager is managed through Corepack.
4. `setup-dev` must install or verify dependencies using the detected repository package manager.
5. Missing prerequisite and command failure output must name the failed command and provide actionable remediation.
6. Repositories without recognized dependency manifests must be skipped with clear output instead of failing.
7. Agent local-development guidance must tell agents to run `ballast setup-dev` as their first startup step when Ballast is available.

### Acceptance Criteria

1. Given a repository with `package.json` declaring `packageManager: "pnpm@..."`, `ballast setup-dev` runs `corepack enable` before `pnpm install`.
2. Given a repository with npm lockfile/package-manager signals, `ballast setup-dev` runs `npm install` without requiring Corepack.
3. Given a repository without recognized dependency manifests, `ballast setup-dev` exits successfully and prints that no setup steps were detected.
4. Given a setup command failure, `ballast setup-dev` exits non-zero and prints the command that failed plus manual remediation guidance.
5. Wrapper tests cover Corepack/package-manager behavior and the no-op path.
6. Generated local-dev rule output references `ballast setup-dev` as the first agent startup step.

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

1. Config-refresh flows (`install --refresh-config`, `upgrade`, and `doctor --fix`) must rewrite selected managed skill files when they already exist.
2. Ordinary backend install behavior outside those refresh flows must keep the existing skill decision matrix: skip on existing files unless `--patch` or `--force` is selected.
3. `ballast upgrade` and `doctor --fix` must refresh saved skill selections through their existing `install --refresh-config` path without requiring `--force`.
4. Existing agent rule overwrite, patch, and force semantics must remain unchanged.
5. The behavior must stay consistent across the TypeScript, Python, Go, and wrapper CLIs.
6. Support files such as `AGENTS.md` and `CLAUDE.md` must continue reflecting the saved skill list after refresh.

### Acceptance Criteria

1. Given an existing installed skill file with stale content, running backend install with the same skill and `force=false, patch=false` leaves the file unchanged outside config-refresh flows.
2. Given an existing installed skill file with stale content and refresh mode enabled through the wrapper config-refresh path, backend install rewrites the file to current packaged skill content without `--force`.
3. Given an existing installed agent rule and `force=false`, backend install still skips the rule unless patch mode or force mode is selected.
4. Given a repository with `.rulesrc.json` that declares a skill, running wrapper `upgrade` without `--force` invokes the refresh path that updates the existing managed skill file.
5. Automated unit coverage demonstrates the backend skill refresh behavior for TypeScript, Python, and Go.
6. Smoke coverage demonstrates the wrapper upgrade path refreshes stale managed skill content.

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

## Repository Facts Auto-Population

### Problem

Ballast scaffolds `AGENTS.md` and `CLAUDE.md` with a `Repository Facts` section, but currently leaves placeholder values. Agents then re-derive stable repository metadata repeatedly instead of using durable facts captured during install.

### Requirements

1. The `ballast` wrapper must discover repository facts once per invocation and pass them to backend installers through a temporary JSON file path provided in environment (`BALLAST_REPOSITORY_FACTS_FILE`) and optional backend CLI flag support (`--repository-facts-file`).
2. TypeScript, Python, and Go backends must consume the wrapper-provided facts section when present and valid.
3. On first-time support-file creation and on `--force` regeneration, generated `AGENTS.md`/`CLAUDE.md` content must include discovered values for detectable fields.
4. Monorepo support-file generation in the wrapper must render the same discovered repository facts content.
5. When a fact cannot be detected, the generated value must remain an explicit placeholder marker.
6. Discovery and rendering must remain non-destructive and read-only outside writing Ballast-managed outputs.

### Acceptance Criteria

1. Given a repository with detectable git origin, default branch, and package-manager signals, running install produces `AGENTS.md` with detected values instead of `<OWNER/REPO>`, `<main>`, and package-manager placeholders.
2. Given wrapper-driven monorepo install flows, generated support files include the same discovered facts.
3. Given missing signals, generated support files retain placeholder markers for undetected fields.
4. TypeScript, Python, and Go backends accept `--repository-facts-file` and honor `BALLAST_REPOSITORY_FACTS_FILE` when present.
5. E2E coverage verifies populated repository facts in generated support files.

## PR Review Loop Guidance

### Problem

Ballast-generated local-development rules treat PR hygiene as part of the agent workflow, but they do not explicitly require agents to keep checking Copilot review feedback after PR creation and subsequent pushes. Agents can miss follow-up Copilot comments or mark work complete without replying directly on addressed review threads, leaving unresolved PR feedback for operators to clean up manually.

### Requirements

1. Generated local-development PR workflow guidance must instruct agents to poll for Copilot review comments after PR creation.
2. Generated local-development PR workflow guidance must instruct agents to poll again after each push that updates an open PR.
3. Agents must summarize actionable Copilot review asks before making code changes.
4. Agents must reply directly on every Copilot review thread or comment they address.
5. The same review loop must remain compatible with human reviewer comments; Copilot-specific guidance must not cause agents to ignore human review feedback.
6. The stop condition for PR readiness must be explicit: required checks are green and there are no unresolved actionable Copilot or human review comments.
7. Generated guidance must include concrete command examples using `gh` or GitHub MCP tools where available.

### Acceptance Criteria

1. Generated local-development rule output tells agents to check Copilot comments repeatedly during PR readiness work.
2. Generated local-development rule output requires direct per-thread replies for addressed Copilot comments.
3. Generated local-development rule output defines the stop condition for the review loop.
4. Generated local-development rule output says the workflow also applies to human review comments.
5. Automated tests cover the generated PR workflow rule text.

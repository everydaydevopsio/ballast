# Product Requirements

## Terraform Rule Best-Practices Alignment

### Problem

Ballast's Terraform linting and testing rules still describe an older validation baseline centered on `tfsec` and Terratest as optional add-ons. Terraform 1.6+ ships native tests, TFLint uses explicit provider/plugin configuration, tfsec has moved under Trivy, and many teams now need OpenTofu-compatible command guidance. Agents need generated Terraform rules that reflect those current practices without losing the conservative validation path for infrastructure changes.

### Requirements

1. Terraform linting guidance must keep `terraform fmt -check -recursive`, `terraform init -backend=false`, `terraform validate`, and recursive `tflint` as the baseline local and CI validation path.
2. Terraform version guidance must keep `.terraform-version`/`tfenv` as the default Ballast path while allowing teams already standardized on `asdf` or `mise` to use those managers consistently.
3. TFLint guidance must require `.tflint.hcl` plugin blocks for the active providers and `tflint --init` before recursive linting.
4. Security scanning guidance must prefer `trivy config` for new work and describe `tfsec` as legacy-compatible because tfsec is now part of Trivy.
5. Terraform testing guidance must document native `terraform test` for Terraform 1.6+ module assertions and Terratest for Go-backed or live integration tests.
6. CI guidance for Terraform validation must include GitHub Actions `concurrency`, PR-time validation, plan/apply separation, and optional orchestration through Atlantis, Terraform Cloud, HCP Terraform, or OpenTofu-compatible platforms.
7. OpenTofu guidance must acknowledge `tofu` as a compatible alternative when the repository standardizes on it, including `tofu fmt`, `tofu init -backend=false`, `tofu validate`, and `tofu test` equivalents.
8. Terraform repo-layout guidance must prefer readable root files and independently testable modules without forcing a one-size-fits-all file split.

### Acceptance Criteria

1. Generated Terraform linting rules mention `trivy config` as the preferred new security scanner and `tfsec` as legacy-compatible.
2. Generated Terraform linting rules require `.tflint.hcl` plugin blocks and `tflint --init` before recursive linting.
3. Generated Terraform testing rules document native `terraform test` for Terraform 1.6+ and Terratest for Go-backed/live integration coverage.
4. Generated Terraform testing rules include a GitHub Actions `concurrency` block and state that PR validation is separate from merge-gated apply workflows.
5. Generated Terraform rules acknowledge OpenTofu command equivalents where relevant.
6. Backend git-hook guidance no longer presents `tfsec` as the only Terraform security scanner.

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

## TypeScript Husky Git Hook Guidance

### Problem

TypeScript-only repositories use Husky instead of `pre-commit`, but the generated Husky guidance does not explicitly require YAML formatting checks for both `.yaml` and `.yml` files or a tracked push-time test hook. YAML-heavy configuration such as GitHub Actions, Dependabot, Docker Compose, and Helm can drift outside TypeScript checks, and developers can push changes before the repo's canonical tests run.

### Requirements

1. TypeScript-only `git-hooks` guidance must use Husky and `lint-staged` or the repository's equivalent fast formatter/linter path for commit-time checks.
2. TypeScript-only Husky pre-commit guidance must explicitly include both `.yaml` and `.yml` formatting checks.
3. TypeScript-only Husky guidance must prefer the repository's existing formatter or linter command when one is already established.
4. TypeScript-only Husky guidance must configure `.husky/pre-push` to run the detected or canonical package-manager test command.
5. TypeScript-only Husky guidance must run the repository's required build or typecheck command before tests when that is the repo convention.
6. Pre-commit must remain fast; heavier build, typecheck, and unit test work belongs in `pre-push`.
7. Multi-language and non-TypeScript git-hook guidance must continue using `pre-commit` without inheriting Husky-specific instructions.
8. Documentation must explain the TypeScript-only split between Husky pre-commit formatting and Husky pre-push tests.

### Acceptance Criteria

1. Generated TypeScript-only Husky `git-hooks` content mentions `.yaml` and `.yml` explicitly.
2. Generated TypeScript-only Husky `git-hooks` content mentions `lint-staged` or the repo formatter/linter as the fast pre-commit path.
3. Generated TypeScript-only Husky `git-hooks` content mentions `.husky/pre-push`, the package-manager test command, and build/typecheck before tests when the repo convention requires it.
4. Multi-language TypeScript output continues to mention `.pre-commit-config.yaml` and `pre-commit install --hook-type pre-push`, and does not mention Husky or `lint-staged`.
5. Unit and E2E coverage assert the Husky YAML/YML and pre-push guidance.

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
7. Existing support files must be patched by default when `--force` is not set, updating only Ballast-managed installed-rule and installed-skill sections while preserving user-managed sections.
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
9. README and `docs/installation.md` describe when support files are patched by default and when to use `--patch` versus `--force`, including the support-file confirmation behavior.
10. Given an existing unreadable Claude `.skill` archive and `force=false, patch=true`, install replaces it with canonical content and completes without an install error.

## Required Agent Option Resolution

### Problem

Some agents require repo-level option values before rule content is generated. Wrapper-driven installs resolved publishing deployment model differently from the tasks task system, so first-run installs with `--all` could prompt for publishing while silently defaulting tasks when no `.rulesrc.json` existed.

### Requirements

1. Wrapper-driven installs must resolve required options through one shared code path.
2. When `tasks` is selected and `.rulesrc.json` has no `taskSystem`, interactive installs must prompt for the task system and non-interactive installs must use the default.
3. When `publishing` is selected and `.rulesrc.json` has no `deploymentModel`, interactive installs must prompt for the deployment model and non-interactive installs must use the default.
4. Explicit CLI flags must override saved config and prompted/default values.
5. Resolved values must be saved to `.rulesrc.json` and forwarded to backend invocations.

### Acceptance Criteria

1. Given a first-run multi-language install with `--all`, Ballast prompts for both task system and deployment model.
2. Given saved values in `.rulesrc.json`, Ballast does not prompt and reuses the saved values.
3. Given `--yes` or CI, Ballast uses defaults for missing selected-agent options.
4. Tests cover first-run prompt resolution and backend argument forwarding.

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

## Smoke And End-to-End Test Guidance

### Problem

Ballast testing and publishing rules mention smoke tests, but they do not consistently define the expected product-level coverage for web applications and installable CLIs. Agents need concise generated guidance that distinguishes fast local checks, pre-push confidence checks, and CI release gates without requiring broad integration-framework detection work.

### Requirements

1. Generated testing guidance must define a baseline web smoke test for runnable web applications that starts the real app and verifies a live route or health endpoint.
2. Generated testing guidance must define a narrow web end-to-end baseline for one critical user workflow when the repo has a browser application surface.
3. Browser end-to-end guidance must prefer Playwright when the repo already uses Playwright or the app shape calls for browser automation, while leaving broader framework detection to the integration-framework detection workstream.
4. Generated guidance must explain local, pre-push, and CI placement: fast unit and targeted smoke checks locally, deterministic smoke and required build/typecheck checks before push, and full smoke/E2E gates in CI.
5. Generated CLI publishing/testing guidance must require packaged-command smoke tests that install or execute the built artifact, verify `--help` and `--version`, and run at least one representative command.
6. Documentation must describe the same web smoke/E2E and CLI packaged-command smoke expectations.
7. Automated generated-content tests must cover the smoke/E2E placement guidance and CLI packaged-command smoke guidance.

### Acceptance Criteria

1. Generated web testing rules include smoke and end-to-end expectations for runnable web apps.
2. Generated testing rules prefer Playwright for browser E2E when appropriate without implementing the broader #145 framework-detection scope.
3. Generated guidance explains which checks belong locally, in pre-push, and in CI.
4. Generated CLI publishing/testing rules require packaged-command smoke tests for install/startup, help output, version output, and a representative command.
5. `docs/agents/testing.md` and `docs/agents/publishing.md` include the same operator-facing guidance.
6. Tests or snapshots fail if the generated guidance drops the required smoke/E2E or CLI packaged-command expectations.

## Integration Framework Detection Guidance

### Problem

Ballast testing rules now define smoke and E2E expectations, but agents still need explicit framework-detection discipline before adding or changing integration tests. Without language-aware detection markers, agents can replace established E2E stacks, add Playwright to library-only repositories, or miss browser app surfaces that should use Playwright when no browser E2E framework exists.

### Requirements

1. Generated TypeScript testing guidance must tell agents to detect existing unit, integration, and browser E2E frameworks before introducing new test tooling.
2. Generated Python and Go testing guidance must tell agents to detect existing unit, integration, API, service, and browser E2E frameworks before introducing new test tooling.
3. Browser E2E guidance must preserve an existing browser E2E framework such as Cypress, WebdriverIO, Selenium, Puppeteer, Robot Framework, pytest-playwright, Playwright Test, or Go browser harnesses when one is already present.
4. Browser E2E guidance must prefer Playwright only when Playwright markers already exist or when the repository has a real browser application surface and no existing browser E2E framework.
5. Generated guidance must warn agents not to add browser E2E tooling to library-only, CLI-only, infrastructure-only, or backend-only repositories without a user-facing browser surface.
6. Documentation must describe the same framework-detection and Playwright-selection expectations.
7. Automated generated-content tests must cover the framework markers, existing-framework preservation, Playwright preference, and non-browser guardrail.

### Acceptance Criteria

1. TypeScript generated testing rules mention package/config markers for Jest, Vitest, Cypress, Playwright, WebdriverIO, Selenium, Puppeteer, and Testing Library.
2. Python generated testing rules mention markers for pytest, unittest, tox/nox, Robot Framework, Selenium, Playwright or pytest-playwright, and API/service test clients.
3. Go generated testing rules mention `go test`, integration build tags or naming, API/service tests, Selenium/chromedp/rod/agouti, and Playwright-driven browser harnesses.
4. Generated testing rules for TypeScript, Python, and Go preserve existing browser E2E frameworks before adding Playwright.
5. Generated testing rules for TypeScript, Python, and Go prefer Playwright only for existing Playwright repos or browser apps with no existing browser E2E framework.
6. `docs/agents/testing.md` includes operator-facing framework-detection guidance aligned with generated rules.
7. Tests fail if generated guidance loses required detection markers, existing-framework preservation, Playwright preference, or non-browser guardrails.

## Deployment Model Configuration

### Problem

Ballast publishing guidance currently bakes in one Kubernetes deployment shape. Repositories use different app deployment models, and agents need a durable repository-level answer so generated publishing rules do not assume Kubernetes, serverless platforms, hosted platforms, or self-managed servers incorrectly. The same setup flow that captures the durable task system should capture deployment model when publishing rules are installed.

### Requirements

1. Ballast must support a repository-level `deploymentModel` config value with valid values `none`, `kubernetes`, `serverless`, `server`, and `hosted`.
2. Interactive install must prompt for `deploymentModel` only when the `publishing` agent is selected and no prior deployment model or explicit flag exists.
3. Non-interactive install must default `deploymentModel` to `none` when the `publishing` agent is selected without an explicit value or existing config.
4. CLI installs must accept `--deployment-model <model>` and reject invalid values with a clear list of valid options.
5. Wrapper monorepo installs must persist, validate, and forward `deploymentModel` to backend invocations that install the `publishing` agent.
6. `.rulesrc.json` must preserve an existing deployment model across installs that do not explicitly change it.
7. `ballast doctor` must display configured `deploymentModel` when present.
8. Generated publishing guidance must render model-specific deployment guidance:
   - `kubernetes`: application repo owns `charts/<app>/`; a separate GitOps repo owns ArgoCD `Application` or `ApplicationSet` configuration; CI publishes image tags or digests and updates the GitOps repo when image references are environment-specific there.
   - `serverless`: guidance covers managed function/container platforms, environment configuration, least-privilege deploy credentials, and preview/stage/prod promotion.
   - `server`: guidance covers self-managed VM or bare-metal deploys, service managers, artifact transfer, rollback, health checks, and secrets outside the repo.
   - `hosted`: guidance covers hosted app platforms such as Vercel, Netlify, Render, Railway, or Fly.io, including platform config ownership, environment variables, previews, and production promotion.
   - `none`: guidance avoids app deployment assumptions and keeps library/CLI publishing guidance intact.

### Acceptance Criteria

1. Given an interactive install selecting `publishing` with no prior config, Ballast prompts for `deploymentModel`.
2. Given `--deployment-model kubernetes`, `.rulesrc.json` stores `"deploymentModel": "kubernetes"` and generated publishing guidance uses the local Helm chart plus external ArgoCD GitOps model.
3. Given an invalid deployment model, install exits non-zero and prints the valid values.
4. Given a non-interactive install selecting `publishing` without a deployment model, `.rulesrc.json` stores `"deploymentModel": "none"`.
5. Given an existing `.rulesrc.json` with `deploymentModel`, later installs preserve it unless `--deployment-model` is provided.
6. `ballast doctor` prints `- deploymentModel: <value>` when configured.
7. Automated tests cover config parsing/persistence, CLI parsing, install prompt/default behavior, wrapper forwarding, doctor output, and generated publishing guidance.

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
4. Agents must reply directly on every Copilot review thread or comment they address, and must resolve addressed review threads when the review system supports thread resolution.
5. The same review loop must remain compatible with human reviewer comments; Copilot-specific guidance must not cause agents to ignore human review feedback.
6. The stop condition for PR readiness must be explicit: required checks are green and there are no unresolved actionable Copilot or human review comments.
7. Generated guidance must include concrete command examples using `gh` or GitHub MCP tools where available.

### Acceptance Criteria

1. Generated local-development rule output tells agents to check Copilot comments repeatedly during PR readiness work.
2. Generated local-development rule output requires direct per-thread replies and supported review-thread resolution for addressed Copilot comments.
3. Generated local-development rule output defines the stop condition for the review loop.
4. Generated local-development rule output says the workflow also applies to human review comments.
5. Automated tests cover the generated PR workflow rule text.

<!-- ballast:rule id="typescript/local-dev/env" version="5.18.3" checksum="667dbd55a164d504c300f62591417df21086f68a40cb4215702bcf3e1541eca5" -->
# Local Development Environment Rules

These rules help set up and maintain a consistent local development environment for the repository's configured languages and runtimes, including Dockerfile and Docker Compose when they fit the project.

---
Use this rule to set direction; the full playbook and examples live in `docs/agents/local-dev.md`.

## Goals

- Keep local setup reproducible, the first-run path short, and README/runbooks aligned with the actual developer workflow.

## Agent Startup

- If the `ballast` wrapper is available, run `ballast setup-dev` before inspecting or changing code.
- Use `ballast doctor` to inspect Ballast-managed local state; if `.ballast/` is missing or incomplete, use `ballast doctor --fix` or `ballast install-cli` to recreate it.
- Treat `.ballast/` as generated local tool state. Do not commit it.
- Treat setup output and `.rulesrc.json` `tools` as the source of truth for missing tools, skipped steps, manual remediation, and per-language tool policy.
- If `ballast setup-dev` is unavailable, fall back to the repository README setup path and document the gap.

## Apply This Rule When

- The task is about local setup, onboarding, `.nvmrc`, env files, Docker, Compose, or dev scripts.
- The user asks to prepare a repository for contributor use.
- The user asks to create, update, or land a PR as part of local-development workflow.

## Branch Before Code

Before modifying files, check `git branch --show-current` against the default branch (`gh repo view --json defaultBranchRef --jq .defaultBranchRef.name`, falling back to `git symbolic-ref --short refs/remotes/origin/HEAD`; if both fail, or the checkout is detached, branch first).

- If the current branch is `main`, `master`, `develop`, or the detected default branch, create or switch to a task branch first — named with the issue number when one exists (`issue-212-branch-before-code`), otherwise a short kebab-case task name.
- Do not make code, config, docs, or generated-output edits on the default branch unless the user explicitly requests an emergency direct change; read-only investigation needs no branch.
- Preserve any existing uncommitted work while creating the task branch.

## Core Responsibilities

1. Establish the local runtime baseline.
   - Check `.rulesrc.json` `tools` first (the Repository Tool Policy in the manifest lists the configured tools); follow overrides and keep docs/scripts consistent with them.
   - Add or update `.nvmrc` when the repo is Node-based.
   - Keep `package.json` `engines` aligned with the supported Node range.
   - Document prerequisites and setup commands in `README.md`.

2. Keep environment configuration explicit.
   - Add `.env.example` or equivalent non-secret config scaffolding when the app needs env vars.
   - Use `env-secrets` or the repo’s existing secret mechanism instead of committing raw secrets.

3. Containerize local development only when it helps the repo.
   - Prefer a production-style `Dockerfile`.
   - Use `docker-compose.yaml` for the base stack.
   - Use `docker-compose.local.yaml` and `Makefile` entrypoints such as `make up-local` for fast iteration when useful.

4. Keep developer commands coherent.
   - Ensure `build`, `start`, and `dev` scripts exist when the app needs them.
   - Prefer fast checks in local hooks and heavier checks in pre-push or CI.

5. Treat PR hygiene as part of local-dev workflow.
   - Verify expected reviewers are assigned.
   - Inspect failing checks with `gh`; summarize the failure.
   - Use `gh pr checks <pr-number>`, `gh pr view <pr-number> --json reviews,comments,reviewThreads`, or GitHub MCP tools for checks/review feedback.
   - Address review comments directly and stop only when required checks are green and actionable comments are resolved.

## Node Guidance

- Use the repo’s existing Node version when already declared; otherwise prefer the current LTS for `.nvmrc`.
- Document supported Node versions briefly instead of embedding a full installation tutorial.
- Tell contributors to run `nvm install` or `nvm use` before installing dependencies.

## Docker and Compose Guidance

- Do not overwrite an existing `Dockerfile`, `docker-compose.yaml`, `docker-compose.local.yaml`, or `Makefile` without checking the current workflow first.
- Keep `.dockerignore` tight.
- Prefer `develop.watch` or the repo’s existing hot-reload mechanism for local iteration.
- Document the happy-path commands in the README, including `make up-local` when that workflow exists.

## Documentation Bar

- README must explain prerequisites, install, local run, and the fastest successful path.
- Troubleshooting notes belong in docs or runbooks, not in the persistent rule body.

## When Completed

1. Summarize the local-dev workflow you added or preserved.
2. Call out any new entrypoints such as `.nvmrc`, `docker-compose.local.yaml`, `Makefile`, or `make up-local`.
3. Identify any remaining gaps in onboarding or PR workflow coverage.

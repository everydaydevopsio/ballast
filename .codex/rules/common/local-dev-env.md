# Local Development Environment Rules

These rules are intended for Codex (CLI and app).

These rules help set up and maintain a consistent local development environment for TypeScript/JavaScript projects, including Dockerfile and Docker Compose for local development following https://www.markcallen.com/dockerfile-for-typescript/

---
# Local Development Environment Agent

You are a local development environment specialist for TypeScript/JavaScript projects.

Keep this rule concise. Use it to set direction, then read implementation files or docs for details instead of dumping large boilerplate into the conversation.

For the full playbook and examples, use `docs/agents/local-dev.md`.

## Goals

- Keep local setup reproducible.
- Keep the first-run path short for new contributors.
- Keep README and runbooks aligned with the actual developer workflow.

## Agent Startup

- If the `ballast` wrapper is available, run `ballast setup-dev` before inspecting or changing code.
- Treat setup output as the source of truth for missing tools, skipped steps, and manual remediation.
- If `ballast setup-dev` is unavailable, fall back to the repository README setup path and document the gap.

## Apply This Rule When

- The task is about local setup, onboarding, `.nvmrc`, env files, Docker, Compose, or dev scripts.
- The user asks to prepare a repository for contributor use.
- The user asks to create, update, or land a PR as part of local-development workflow.

## Branch Before Code

Before modifying files, check the current branch with `git branch --show-current` and determine the default branch with `gh repo view --json defaultBranchRef --jq .defaultBranchRef.name`. If that command fails for any reason, use `git symbolic-ref --short refs/remotes/origin/HEAD` and strip the `origin/` prefix before comparing it to the current branch name. If both default-branch detection methods fail, assume the checkout is unsafe and create or switch to a task branch before editing files.

- If the current branch name is empty, treat the checkout as detached and create or switch to a task branch before editing files.
- If the current branch is `main`, `master`, `develop`, or the detected repository default branch, create or switch to a task branch first.
- Name task branches with the issue number when one exists, such as `issue-212-branch-before-code`; otherwise use a short kebab-case task name.
- Do not make code, config, docs, or generated-output edits on the default branch unless the user explicitly requests an emergency direct change.
- Read-only investigation, status checks, and answering questions do not require a new branch.
- If uncommitted work already exists, inspect it and preserve it; do not overwrite or discard user changes while creating the task branch.

## Core Responsibilities

1. Establish the local runtime baseline.
   - Add or update `.nvmrc` when the repo is Node-based.
   - Keep `package.json` `engines` aligned with the supported Node range.
   - Document prerequisites and setup commands in `README.md`.

2. Keep environment configuration explicit.
   - Add `.env.example` or equivalent non-secret config scaffolding when the app needs env vars.
   - Use `env-secrets` or the repo’s existing secret mechanism instead of committing raw secrets.

3. Containerize local development only when it helps the repo.
   - Prefer a production-style `Dockerfile`.
   - Use `docker-compose.yaml` for the base stack.
   - Use `docker-compose.local.yaml` for fast iteration and watch-mode overrides.
   - Add a `Makefile` with simple entrypoints such as `make up`, `make down`, `make logs`, and `make up-local`.

4. Keep developer commands coherent.
   - Ensure `build`, `start`, and `dev` scripts exist when the app needs them.
   - Prefer fast checks in local hooks and heavier checks in pre-push or CI.

5. Treat PR hygiene as part of local-dev workflow.
   - Verify expected reviewers are assigned.
   - Inspect failing checks with `gh`; summarize the failure.
   - After PR creation and every push, poll Copilot and human review comments until the PR is ready.
   - Before changes, summarize actionable Copilot asks and related human-review asks.
   - Use `gh pr checks <pr-number>`, `gh pr view <pr-number> --json reviews,comments,reviewThreads`, or GitHub MCP tools for checks/review feedback.
   - Reply directly on addressed Copilot threads/comments, then resolve them; do the same for human comments.
   - Stop only when required checks are green and no unresolved actionable Copilot or human review comments remain.

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

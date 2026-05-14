# Git Hooks Rules

These rules are intended for Claude Code.

These rules keep local Git hook orchestration consistent with the repository layout and testing strategy.

---
You are a Git hook specialist. Your role is to establish local Git hook orchestration that complements Ballast linting and testing rules without duplicating ownership.

## Your Responsibilities

1. Select the correct hook tool for the repository layout.
2. Configure fast checks for `pre-commit`.
3. Configure unit tests for `pre-push`.
4. Keep hook configuration current as commands and repo layout evolve.
5. Keep hook scripts executable and easy to audit.

## Hook Strategy

Use Husky for this monorepo.

- Install and initialize Husky.
- Create `.husky/pre-commit` with the repo's fast lint command, such as `npx lint-staged`.
- Create `.husky/pre-push` with the repo's unit test command, and for TypeScript monorepos run the build before the tests when the test command depends on generated output.
- Keep the hook file executable with `chmod +x .husky/pre-commit`.
- Keep `.husky/pre-push` executable with `chmod +x .husky/pre-push`.
- Keep the hook in sync with the repo's linting workflow whenever the command changes.

## Important Notes

- Keep commit-time hooks fast enough that developers do not bypass them.
- Keep `pre-push` focused on the repo's unit test command and required build step.
- Update hook commands when lint, format, build, or test scripts change.
- Verify the hook setup after changes before handing off the repo.

## When Completed

1. Show the user the hook files and commands you added or updated.
2. Explain how commit-time checks differ from push-time checks.
3. Explain how to verify the hook setup locally.

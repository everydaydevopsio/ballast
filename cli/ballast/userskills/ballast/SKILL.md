---
name: ballast
description: >
  Install, manage, and health-check Ballast (AI agent rules and skills for
  Claude Code, Codex, Cursor, Gemini CLI, and OpenCode) in any repository, and
  turn what you learn into improvement proposals for Ballast itself. Use this
  skill when the user says: install ballast, set up ballast in this repo, add
  ballast rules, upgrade or refresh ballast, add or remove a ballast target,
  skill, or language, is this repo following the ballast rules, ballast health
  check, check this repo against its rules, what is ballast missing, suggest
  ballast improvements, or file a ballast issue.
---

<!-- ballast-user-skill version={{version}}: managed by `ballast user-skill`; edits are overwritten on reinstall -->

# Ballast

Ballast installs AI agent rules and skills into a repository from a saved config, `.rulesrc.json`. This skill is installed for the user, not the repository, so it works in repositories that do not use Ballast yet.

Pick the workflow from the request:

| Request | Workflow |
| --- | --- |
| "install ballast in this repo", "set up ballast" | [1. Install](#1-install-ballast-in-a-repository) |
| "upgrade ballast", "add codex", "add a skill", "remove python" | [2. Manage](#2-manage-an-existing-installation) |
| "health check", "is this repo following the rules" | [3. Health check](#3-health-check-the-repository-against-its-rules) |
| "suggest fixes to ballast", "what should ballast do better" | [4. Improve Ballast](#4-propose-improvements-to-ballast) |

A health check normally ends with workflow 4: every finding where the *rule* was wrong, not the repository, is a Ballast improvement.

## Ground rules

- Run every command from the repository root (`git rev-parse --show-toplevel`).
- Before writing files, branch off the default branch if you are on it (`issue-<n>-<slug>` or a short kebab-case name). Preserve uncommitted work.
- `.rulesrc.json` and the generated `.claude/`, `.codex/`, `.cursor/`, `.gemini/`, `.opencode/` rule and skill files are committed. `.ballast/` is local tool state: never commit it, and make sure `.gitignore` lists it.
- Never hand-edit Ballast-generated rule or skill files. Change `.rulesrc.json` or the install flags and regenerate; content problems go upstream through workflow 4.
- Do not commit, push, open a PR, or file an issue without the user asking for it or approving it.

## 0. Preflight: make sure the CLI is usable

```bash
command -v ballast && ballast --version
ballast help | head -1   # must say "ballast installs AI agent rules ..."
```

- **Not installed**: tell the user and offer the install command; installing software system-wide needs their approval.
  - macOS: `brew tap everydaydevopsio/ballast && brew install --cask everydaydevopsio/ballast/ballast`
  - Linux: `brew tap everydaydevopsio/ballast && brew install --formula everydaydevopsio/ballast/ballast`
- **Wrong tool**: on macOS the core Homebrew cask `ballast` is an unrelated audio app. If `ballast help` does not print the line above, `ballast` is not this CLI, so do not run any other `ballast` command (including `ballast update`). With the user's approval, replace it through Homebrew directly, then repeat both preflight commands:
  - `brew uninstall --cask ballast && brew install --cask everydaydevopsio/ballast/ballast`
  - If `command -v ballast` points outside Homebrew, report the path and let the user decide.
- **Old version**: once `ballast help` is verified, compare `ballast --version` with the latest release (`gh release view -R everydaydevopsio/ballast --json tagName -q .tagName`). If it is behind, suggest `ballast update`.
- **Install requested in a managed repository**: if `.rulesrc.json` already exists, do not reinstall; use workflow 2 to upgrade or change it. This applies only to install requests. Health checks and improvement requests keep their own workflow (3 or 4) whether or not the repository is managed.

## 1. Install Ballast in a repository

### 1a. Detect, do not ask

Inspect the repository and choose each install option. Ask the user only when the evidence is genuinely split, and then ask once for everything.

| Option | Flag | How to decide |
| --- | --- | --- |
| Targets | `--target` | The agent you are running in (Claude Code → `claude`, Codex → `codex`), plus any of `.claude/`, `.codex/`, `.cursor/`, `.gemini/`, `.opencode/` already in the repo. Default `claude,codex`. |
| Languages | (auto) | Ballast detects TypeScript, Python, Go, Ansible, Terraform, Dart, and Docker per directory. Pass `--language` only to override a wrong detection. |
| Agents | `--all` or `--agent` | Default `--all`; workflow 3 narrows it afterwards with evidence. Use `--agent` lists only when the user asks for specific rules. |
| Task system | `--task-system` | `jira` if commit messages, branches, or PR titles carry `ABC-123` keys or docs link to Atlassian; `linear` for Linear links or `ENG-123`-style keys with Linear references; otherwise `github` for a GitHub remote. |
| Deployment model | `--deployment-model` | `none` for libraries, CLIs, and SDKs; `kubernetes` for Helm charts, kustomize, or k8s manifests; `serverless` for `serverless.yml`, SAM, or Lambda-only IaC; `docker` for Compose-deployed services; `hosted` for Vercel, Netlify, Fly, Render, Heroku config; `server` for VM or bare-host deploys. |
| Code reviewer | `--code-reviewer` | `copilot`, `bosun`, `none`, or `skill:<name>` for a repository-defined review skill. Omit it when there is no signal. |

### 1b. Install

```bash
ballast install --target claude,codex --all \
  --task-system github --deployment-model none --yes
```

`--yes` makes the run non-interactive; without the task-system and deployment-model flags a first install prompts for them. Every install adds the `ballast-audit` skill. Add other skills with `--skill <id>` (comma-separated) or `--all-skills`; run `ballast help` for the current list.

### 1c. Verify

```bash
ballast doctor
git status --short
git check-ignore -q .ballast && echo ".ballast ignored"
```

Confirm:

- `ballast doctor` reports no errors and the backend CLIs match the wrapper version.
- `.rulesrc.json` lists the targets, agents, languages, task system, and deployment model you chose.
- Rules exist under each target (for example `.claude/rules/`, `.codex/rules/`) and the manifest (`CLAUDE.md`, `AGENTS.md`) has the Ballast-managed sections.
- `.ballast/` is ignored. If not, add it to `.gitignore`.

Report what was installed, the options chosen and why, and offer to commit (`chore: install ballast rules`) and open a PR. Then offer workflow 3, since a fresh install usually surfaces gaps.

## 2. Manage an existing installation

Read `.rulesrc.json` first; it is the source of truth. Then use:

| Goal | Command |
| --- | --- |
| Inspect CLI versions and config | `ballast doctor` |
| Repair missing or stale `.ballast/` tools and refresh config | `ballast doctor --fix` |
| Install only the backend CLIs | `ballast install-cli` |
| Upgrade the wrapper itself (Homebrew) | `ballast update` |
| Move the repo to the running Ballast version | `ballast upgrade --patch` |
| Regenerate from saved config | `ballast install --refresh-config --patch` |
| Add a target, agent, or skill | `ballast install --target cursor`, `--agent publishing`, `--skill git-branch-cleanup` |
| Remove a target or language | `ballast install --remove-target gemini`, `--remove-language python` |
| Change task system or deployment model | `ballast install --task-system jira`, `--deployment-model kubernetes` |

Prefer `--patch`: rules keep user-authored sections, while skills are always replaced wholesale.

After any change:

1. `git diff --stat` and `git diff .rulesrc.json`. Confirm only the intended entries changed; a failed or partial run can silently drop `agents` entries or reorder keys.
2. Check backend freshness with `ballast doctor`: every backend under **Installed backends** should report the same version as `ballast --version`. If `ballastVersion` in `.rulesrc.json` trails `ballast --version`, run `ballast upgrade --patch`: it moves the pinned version forward and syncs the backends. Do not use `ballast install-cli` for this, because it installs the version pinned in `.rulesrc.json` and would reinstall the old backends. Reserve `install-cli` for repairing missing or broken tools at the version the config already pins. Do not judge freshness from the version stamped in generated rule headers: additive installs skip existing rules and `--patch` keeps an edited rule's existing marker, so an old stamp alone is not a stale backend. Content drift is the backend doctor's job (`ballast-audit` check 1).
3. Run `ballast doctor` again.

## 3. Health check the repository against its rules

Two layers. Do both, then report once.

### 3a. Is the Ballast installation itself healthy?

- `ballast doctor` output: errors, version drift, missing `.ballast/` tools.
- `ballastVersion` in `.rulesrc.json` against `ballast --version`; if behind, the fix is `ballast upgrade --patch`.
- Generated files modified by hand: do not refresh in place to find out. `--patch` deliberately keeps edited rule sections, so an empty diff proves nothing, and a refresh can create files (for example a missing `CLAUDE.md` or `AGENTS.md`) that a checkout will not remove. Use the language backend's checksum inspection instead, which is read-only:

  ```bash
  ballast doctor | sed -n 's/^- \(ballast-[a-z]*\): .*(\(.*\))$/\2/p'   # backend paths
  <backend-path> doctor 2>&1 | grep -E '\[(drifted|stale|unowned)'
  ```

  `drifted` is a hand edit, `stale` is no longer in the active config, and `unowned` predates ownership markers and no refresh will touch it.
- If the repository has the `ballast-audit` skill, run it: it covers stale, unowned, oversized, and irrelevant rules and skills, and the narrowest config.

### 3b. Is the repository following the rules?

The installed rule text is the specification. List the rule files for one target (`.claude/rules/**/*.md`, or `.codex/rules/**/*.md`, or the target the repo uses) and, for each rule, extract its concrete, checkable requirements, then check the repository. The table below is a starting point for common rules; when it disagrees with the installed rule text, the rule text wins.

| Rule | Check |
| --- | --- |
| `local-dev-license` | `LICENSE` exists; `package.json` `license` set; README ends with a License section |
| `local-dev-badges` | README badges after the title point at workflow files that exist |
| `local-dev-env` | `.nvmrc` and `engines` agree for Node repos; `.env.example` when code reads env vars; README has prerequisites, install, and run steps |
| `cicd` | Every workflow has a `concurrency` block (`cancel-in-progress: false` for publish); `.github/dependabot.yml` covers each ecosystem plus `github-actions` |
| `git-hooks` | Hook config exists (`.pre-commit-config.yaml`, husky, or lefthook as the rule says); fast checks on commit, tests on pre-push; gitleaks hook present |
| `linting` | Linter config present (ESLint flat config, `golangci-lint`, Ruff, etc.); lint runs in CI; lint command passes |
| `testing` | Test and coverage commands exist; CI enforces the coverage threshold; tests pass |
| `testing-process` | Smoke test and workflow exist when the repo ships a runnable app or service |
| `docs` | `docs/README.md` index exists; docs cover commands and config that the code exposes |
| `publishing-*` | Release workflow uses `workflow_dispatch` with `release_type`, `v`-prefixed tags, publish jobs check out the tag, concurrency never cancels |
| `plan-lifecycle` | `plans/README.md` and `adr/README.md` exist and index the files beside them |
| `tasks-*` | `tasks/todo.md` follows the template when present; open work links to the configured task system |

Run the repository's own fast checks (lint, unit tests, build) where they exist; a rule that says "CI runs tests" is only half-satisfied if the tests fail. Do not run deploys, publishes, or anything that writes outside the repository.

For every requirement, record one of: **pass**, **fail** (repo should change), **rule mismatch** (the rule does not fit this repository, is ambiguous, or contradicts another rule), or **n/a**.

### 3c. Report

Lead with a one-line verdict and counts, then:

```markdown
| # | Rule | Requirement | Status | Evidence | Fix |
| - | ---- | ----------- | ------ | -------- | --- |
| 1 | cicd | concurrency on every workflow | fail | `.github/workflows/lint.yml` has none | add the CI concurrency block |
| 2 | testing | coverage gate in CI | rule mismatch | Go repo, rule text assumes Jest | Ballast: language-aware coverage wording |
```

Order failures by impact: broken CI, security (secrets, hooks), release safety, then hygiene. Keep evidence to a file path and line or a command and its output.

Then offer two follow-ups, separately:

1. **Fix the repository**: apply the **fail** fixes the user picks, on a branch, verifying each with the same check that failed.
2. **Fix Ballast**: take every **rule mismatch** (and any Ballast CLI error you hit) into workflow 4.

If the narrowest config differs from `.rulesrc.json` (an agent no code path needs, a skill nothing uses, a missing language), propose the exact `ballast install` flags to change it.

## 4. Propose improvements to Ballast

Ballast lives at `everydaydevopsio/ballast`. Improvements come from evidence gathered in this repository, never from speculation.

### 4a. Collect and classify

Sources: workflow 3 **rule mismatch** rows, `ballast doctor` or install errors, generated output that needed manual fixing, rules that contradict each other or the repository's established tooling, repository practices Ballast has no rule for, and recurring agent mistakes the rules failed to prevent.

Classify each:

| Kind | Upstream location |
| --- | --- |
| Rule content wrong, vague, or too long | `agents/<common or language>/<agent>/content.md` |
| Skill content wrong or missing a step | `skills/common/<id>/SKILL.md` |
| CLI bug (install, doctor, upgrade, detection) | `cli/ballast/main.go` and the language backends under `packages/` |
| Missing rule, skill, language, or option | new issue, labelled as a feature request |
| Docs wrong or missing | `README.md`, `docs/` |

Drop anything you cannot back with a concrete example.

### 4b. Deduplicate

```bash
gh issue list -R everydaydevopsio/ballast --state all --search "<key terms>" --limit 20
```

If an issue exists, propose a comment with the new evidence instead of a new issue.

### 4c. Draft

One issue per independent problem:

```markdown
**Ballast version:** <ballast --version>
**Languages / targets:** <from .rulesrc.json>
**Kind:** rule content | skill content | CLI bug | feature | docs

### What happened
<observed behavior, with the rule or command involved>

### Expected
<what the rule or CLI should have done>

### Evidence
<minimal, sanitized snippet or command output>

### Proposed change
<file in the Ballast repo and the concrete wording or behavior change>
```

Sanitize before anything leaves the machine: no private repository names, internal hostnames, paths under the user's home directory, source code beyond a minimal generic snippet, customer data, or secrets. Describe the shape of the repository ("Go CLI monorepo with GoReleaser") instead of naming it.

### 4d. Hand off

Show all drafts together and ask which to file. Filing is outward-facing; do it only on approval:

```bash
gh issue create -R everydaydevopsio/ballast --title "<title>" --body-file - <<'EOF'
<body>
EOF
```

Report the created issue URLs. If the current repository *is* the Ballast repository, offer to implement the change on a branch instead of filing.

## Keeping this skill current

This skill ships with the `ballast` CLI. After `ballast update`, run `ballast user-skill install` to refresh it. `ballast user-skill status` and `ballast doctor` show which version is installed for each agent.

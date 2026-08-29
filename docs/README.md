# Ballast Documentation

Ballast supports TypeScript, Python, Go, Ansible, Terraform, Dart, and Docker.

## Agent Families

Common agents (all language packs):

- `local-dev`
- `docs`
- `cicd`
- `observability`
- `publishing`
- `git-hooks`
- `tasks`
- `plan-lifecycle`
- `spec-kit`

Language-specific agents:

- `linting`
- `logging`
- `testing`

## Agent Guide Index

<!-- prettier-ignore -->
Agent | TypeScript | Python | Go | Ansible | Terraform | Dart | Docker | Guide
----- | ---------- | ------ | -- | -------- | --------- | ---- | ------ | -----
`local-dev` | Yes | Yes | Yes | Yes | Yes | Yes | Yes | [agents/local-dev.md](agents/local-dev.md)
`docs` | Yes | Yes | Yes | Yes | Yes | Yes | Yes | [agents/docs.md](agents/docs.md)
`cicd` | Yes | Yes | Yes | Yes | Yes | Yes | Yes | [agents/cicd.md](agents/cicd.md)
`observability` | Yes | Yes | Yes | Yes | Yes | Yes | Yes | [agents/observability.md](agents/observability.md)
`publishing` | Yes | Yes | Yes | Yes | Yes | Yes | Yes | [agents/publishing.md](agents/publishing.md)
`git-hooks` | Yes | Yes | Yes | Yes | Yes | Yes | Yes | [agents/git-hooks.md](agents/git-hooks.md)
`tasks` | Yes | Yes | Yes | Yes | Yes | Yes | Yes | [agents/tasks.md](agents/tasks.md)
`plan-lifecycle` | Yes | Yes | Yes | Yes | Yes | Yes | Yes | [agents/plan-lifecycle.md](agents/plan-lifecycle.md)
`spec-kit` | Yes | Yes | Yes | Yes | Yes | Yes | Yes | [agents/spec-kit.md](agents/spec-kit.md)
`linting` | Yes | Yes | Yes | Yes | Yes | Yes | Yes | [agents/linting.md](agents/linting.md)
`logging` | Yes | Yes | Yes | Yes | Yes | Yes | Yes | [agents/logging.md](agents/logging.md)
`testing` | Yes | Yes | Yes | Yes | Yes | Yes | Yes | [agents/testing.md](agents/testing.md)

## Skill Guide Index

Common skills:

- `owasp-security-scan`
- `aws-health-review`
- `aws-live-health-review`
- `aws-weekly-security-review`
- `github-health-check`
- `github-pr-copilot-cycle`
- `ballast-audit`
- `ballast-project-maintenance`
- `docker-registry-publish`
- `speckit-bootstrap`
- `speckit-reverse-engineer`
- `speckit-delivery`

`github-health-check` covers CI status, pull request hygiene, Dependabot, code coverage, GitHub Code Quality findings, security feature enablement, security advisories, and alert listings.

`ballast-audit` audits AI rule and skill files for context density, duplication, and bloat.

`ballast-project-maintenance` covers Ballast-managed repository status, `.ballast/` local tool repair, and config refresh workflows.

The `speckit-*` skills coordinate GitHub Spec Kit adoption and delivery while leaving native Spec Kit workflow details to the generated native `speckit-*` skills.

Guide index:

- [skills/README.md](skills/README.md)
- [skills/aws-health-review.md](skills/aws-health-review.md)
- [skills/aws-live-health-review.md](skills/aws-live-health-review.md)
- [skills/aws-weekly-security-review.md](skills/aws-weekly-security-review.md)
- [skills/github-health-check.md](skills/github-health-check.md)
- [skills/github-pr-copilot-cycle.md](skills/github-pr-copilot-cycle.md)
- [skills/owasp-security-scan.md](skills/owasp-security-scan.md)
- [skills/ballast-audit.md](skills/ballast-audit.md)
- [skills/ballast-project-maintenance.md](skills/ballast-project-maintenance.md)
- [skills/docker-registry-publish.md](skills/docker-registry-publish.md)
- [skills/speckit-bootstrap.md](skills/speckit-bootstrap.md)
- [skills/speckit-reverse-engineer.md](skills/speckit-reverse-engineer.md)
- [skills/speckit-delivery.md](skills/speckit-delivery.md)

## Installation and Monorepos

See [installation.md](installation.md) for package-specific commands, skill installation, and the unified monorepo workflow:

- `ballast install --target cursor --all --yes` for TypeScript + Python + Go + Ansible + Terraform + Dart + Docker monorepos
- `git-hooks` owns `pre-commit`, Husky, and `pre-push` guidance; Ballast auto-installs it with `linting`
- npm (`@everydaydevopsio/ballast`)
- uv/uvx from GitHub Releases wheel (`ballast-python`)
- go install/go run (`ballast-go`)
- installed skills under target-specific skill locations such as `.claude/skills/` and `.opencode/skills/`

## Publishing

See [publish.md](publish.md) for npmjs, Python GitHub release assets, and Go release publishing setup.

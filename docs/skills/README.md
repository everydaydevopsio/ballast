# Ballast Skills

Ballast ships reusable skill guides alongside its agent rules.

## Available Skills

`owasp-security-scan`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go, Ansible, Terraform, Dart, Docker
- Installed by: `--skill owasp-security-scan` or `--all-skills`

`aws-health-review`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go, Ansible, Terraform, Dart, Docker
- Installed by: `--skill aws-health-review` or `--all-skills`

`aws-live-health-review`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go, Ansible, Terraform, Dart, Docker
- Installed by: `--skill aws-live-health-review` or `--all-skills`

`aws-weekly-security-review`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go, Ansible, Terraform, Dart, Docker
- Installed by: `--skill aws-weekly-security-review` or `--all-skills`

`github-health-check`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go, Ansible, Terraform, Dart, Docker
- Installed by: `--skill github-health-check` or `--all-skills`
- Focus: CI status, pull requests, Dependabot, code coverage, GitHub Code Quality findings, security feature enablement, security advisories, and alert listings

`github-pr-copilot-cycle`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go, Ansible, Terraform, Dart, Docker
- Installed by: `--skill github-pr-copilot-cycle` or `--all-skills`
- Focus: PR creation/update, Copilot review requests, actionable feedback fixes, CI checks, and repeated review cycles

`ballast-audit`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go, Ansible, Terraform, Dart, Docker
- Installed by: `--skill ballast-audit` or `--all-skills`

`ballast-project-maintenance`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go, Ansible, Terraform, Dart, Docker
- Installed by: `--skill ballast-project-maintenance` or `--all-skills`
- Focus: Ballast-managed repository status, `.ballast/` local tool repair, config refresh, and generated rule/skill maintenance

`docker-registry-publish`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go, Ansible, Terraform, Dart, Docker
- Installed by: `--skill docker-registry-publish` or `--all-skills`
- Focus: GHCR and Docker Hub image publishing, public/private visibility, registry credentials, image tags, and digest handoff

`speckit-bootstrap`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go, Ansible, Terraform, Dart, Docker
- Installed by: `--skill speckit-bootstrap` or `--all-skills`
- Focus: initializing or repairing GitHub Spec Kit setup and native `speckit-*` skills

`speckit-reverse-engineer`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go, Ansible, Terraform, Dart, Docker
- Installed by: `--skill speckit-reverse-engineer` or `--all-skills`
- Focus: creating a brownfield Spec Kit baseline from runtime behavior, source, tests, and existing docs

`speckit-delivery`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go, Ansible, Terraform, Dart, Docker
- Installed by: `--skill speckit-delivery` or `--all-skills`
- Focus: coordinating native Spec Kit specification, planning, tasking, implementation, and convergence for bounded changes

## Installation Targets

Skills install to the target tool's expected location:

- Cursor: `.cursor/rules/<skill>.mdc`
- Claude: `.claude/skills/<skill>.skill`
- Gemini: `.gemini/rules/<skill>.md`
- OpenCode: `.opencode/skills/<skill>.md`
- Codex: `.codex/skills/<skill>/SKILL.md`

Codex also records installed skills in the root `AGENTS.md`. Claude records them in the root `CLAUDE.md`. Gemini records them in the root `GEMINI.md`.

## Guides

- [owasp-security-scan.md](owasp-security-scan.md)
- [aws-health-review.md](aws-health-review.md)
- [aws-live-health-review.md](aws-live-health-review.md)
- [aws-weekly-security-review.md](aws-weekly-security-review.md)
- [github-health-check.md](github-health-check.md)
- [github-pr-copilot-cycle.md](github-pr-copilot-cycle.md)
- [ballast-audit.md](ballast-audit.md)
- [ballast-project-maintenance.md](ballast-project-maintenance.md)
- [docker-registry-publish.md](docker-registry-publish.md)
- [speckit-bootstrap.md](speckit-bootstrap.md)
- [speckit-reverse-engineer.md](speckit-reverse-engineer.md)
- [speckit-delivery.md](speckit-delivery.md)

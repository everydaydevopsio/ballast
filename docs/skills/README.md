# Ballast Skills

Ballast ships reusable skill guides alongside its agent rules.

## Available Skills

`owasp-security-scan`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go
- Installed by: `--skill owasp-security-scan` or `--all-skills`

`aws-health-review`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go, Ansible, Terraform
- Installed by: `--skill aws-health-review` or `--all-skills`

`aws-live-health-review`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go, Ansible, Terraform
- Installed by: `--skill aws-live-health-review` or `--all-skills`

`aws-weekly-security-review`

- Type: common skill
- Supported language profiles: TypeScript, Python, Go, Ansible, Terraform
- Installed by: `--skill aws-weekly-security-review` or `--all-skills`

## Installation Targets

Skills install to the target tool's expected location:

- Cursor: `.cursor/rules/<skill>.mdc`
- Claude: `.claude/skills/<skill>.skill`
- OpenCode: `.opencode/skills/<skill>.md`
- Codex: `.codex/rules/<skill>.md`

Codex also records installed skills in the root `AGENTS.md`. Claude records them in the root `CLAUDE.md`.

## Guides

- [owasp-security-scan.md](owasp-security-scan.md)
- [aws-health-review.md](aws-health-review.md)
- [aws-live-health-review.md](aws-live-health-review.md)
- [aws-weekly-security-review.md](aws-weekly-security-review.md)

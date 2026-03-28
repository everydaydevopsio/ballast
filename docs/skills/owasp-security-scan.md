# `owasp-security-scan`

The `owasp-security-scan` skill runs an OWASP-aligned security review across Ballast-supported language profiles.

## Scope

- Go: `gosec`, `govulncheck`, Semgrep
- TypeScript/JavaScript: `npm audit`, Semgrep
- Python: `bandit`, `pip-audit`, Semgrep
- Cross-language secrets scanning: `gitleaks`

The skill also uses its bundled references for:

- OWASP category mapping
- remediation guidance
- CI workflow examples
- tool configuration examples

## When To Use It

Use this skill when you want to:

- run a security audit
- scan for OWASP Top 10 issues
- review dependencies for known vulnerabilities
- check for hardcoded secrets
- generate a consolidated findings report with remediation guidance

## Install It

Examples:

```bash
pnpm exec ballast-typescript install --target claude --skill owasp-security-scan
pnpm exec ballast-typescript install --target codex --all-skills
ballast install --target opencode --skill owasp-security-scan --yes
```

## Installed Locations

- Cursor: `.cursor/rules/owasp-security-scan.mdc`
- Claude: `.claude/skills/owasp-security-scan.skill`
- OpenCode: `.opencode/skills/owasp-security-scan.md`
- Codex: `.codex/rules/owasp-security-scan.md`

## Output Expectations

The skill should:

1. detect which languages are present
2. run the relevant SAST, SCA, and secrets checks
3. consolidate findings by severity and OWASP category
4. avoid printing raw secret values
5. offer next-step remediation guidance

## Notes

- In large monorepos, run scans from each language root when needed.
- If a tool is unavailable, the skill should skip it gracefully and note the gap in the final report.
- The source of truth for this skill is `skills/common/owasp-security-scan/SKILL.md`.

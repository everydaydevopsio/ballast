# `github-health-check`

The `github-health-check` skill runs a comprehensive GitHub repository health audit using the `gh` CLI. It produces a structured report with status indicators and actionable items, auto-merges safe Dependabot PRs, and checks whether required GitHub security and code quality features are enabled.

## When To Use It

Use this skill when you want to:

- audit overall repository health
- check CI workflow status and failure trends
- review open pull requests and stale branches
- merge safe Dependabot dependency updates
- check whether GitHub Code Quality is turned on
- check security policy, security advisories, and private vulnerability reporting
- check Dependabot, code scanning, and secret scanning alerts
- list Code Quality findings, Dependabot malware/vulnerability alerts, code scanning alerts, and secret scanning alerts
- review branch protection and Snyk integration
- review public/private repository best practices
- get a prioritized list of items that need attention

## Install It

Examples:

```bash
pnpm exec ballast-typescript install --target codex --skill github-health-check
ballast-go install --target claude --skill github-health-check
ballast install --target opencode --skill github-health-check --yes
```

## Prerequisites

The skill requires the `gh` CLI to be authenticated:

```bash
gh auth status
```

## Output Expectations

The skill should:

1. produce a structured Markdown health report
2. use `PASS`, `WARN`, `FAIL`, and `ERROR` status indicators
3. include actionable remediation guidance for each finding
4. auto-merge Dependabot PRs that pass all required checks
5. assign `HIGH` priority to missing security advisories, Dependabot alerts, code scanning alerts, secret scanning alerts, and public-repo private vulnerability reporting
6. assign `MEDIUM` priority to missing security policy and Code Quality enablement/findings visibility
7. surface a prioritized list of items needing attention

## Notes

- The source of truth for this skill is `skills/common/github-health-check/SKILL.md`.
- All GitHub API access is read-only except for merging safe Dependabot PRs.

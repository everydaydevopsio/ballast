# `aws-weekly-security-review`

The `aws-weekly-security-review` skill runs a repeatable, read-only AWS security baseline review and writes a Markdown report with prioritized findings.

## When To Use It

Use this skill when you want to:

- run a weekly AWS security posture check
- review IAM, CloudTrail, S3, RDS, and GuardDuty baselines
- generate a lightweight security report under a specific AWS CLI profile

## Install It

Examples:

```bash
pnpm exec ballast-typescript install --target codex --skill aws-weekly-security-review
ballast-python install --target claude --skill aws-weekly-security-review
ballast install --target opencode --skill aws-weekly-security-review --yes
```

## Profile Selection

The skill supports explicit profile selection in two ways:

- `--profile <name>`
- `PROFILE=<name>` or `AWS_PROFILE=<name>`

If both are set, `--profile` wins. The default remains `wepro-readonly`.

## Output Expectations

The skill should:

1. keep all AWS access read-only
2. report clear severities and evidence
3. include cost impact notes for recommendations
4. surface permission gaps explicitly when a profile lacks read access
5. provide a comparable weekly baseline format

## Notes

- The source of truth for this skill is `skills/common/aws-weekly-security-review/SKILL.md`.

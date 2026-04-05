# `aws-live-health-review`

The `aws-live-health-review` skill generates a current-state AWS health snapshot for operational triage.

## When To Use It

Use this skill when you want to:

- check AWS health right now
- summarize current EC2, RDS, and ALB status
- inspect active CloudWatch alarms
- review recent CloudWatch logs using a specific AWS CLI profile

## Install It

Examples:

```bash
pnpm exec ballast-typescript install --target codex --skill aws-live-health-review
ballast-go install --target opencode --skill aws-live-health-review
ballast install --target claude --skill aws-live-health-review --yes
```

## Profile Selection

The skill supports explicit profile selection in two ways:

- `--profile <name>`
- `PROFILE=<name>` or `AWS_PROFILE=<name>`

If both are set, `--profile` wins. The default remains `wepro-readonly`.

## Output Expectations

The skill should:

1. report current state first
2. separate active issues from configuration risks
3. keep timestamps in UTC
4. use logs as supporting evidence rather than the sole basis for outage claims
5. remain read-only

## Notes

- The source of truth for this skill is `skills/common/aws-live-health-review/SKILL.md`.

# `aws-health-review`

The `aws-health-review` skill runs a repeatable, read-only AWS operational health review and writes a Markdown report with prioritized findings.

## When To Use It

Use this skill when you want to:

- run a weekly AWS health baseline
- review alarms, health events, backups, certificates, and billing signals
- append prioritized follow-up tasks to `TODO.md`
- run the checks under a specific AWS CLI profile

## Install It

Examples:

```bash
pnpm exec ballast-typescript install --target codex --skill aws-health-review
ballast-go install --target claude --skill aws-health-review
ballast install --target opencode --skill aws-health-review --yes
```

## Profile Selection

The skill supports explicit profile selection in two ways:

- `--profile <name>`
- `PROFILE=<name>` or `AWS_PROFILE=<name>`

If both are set, `--profile` wins. The default remains `wepro-readonly`.

## Output Expectations

The skill should:

1. generate a Markdown report in `reports/`
2. keep all AWS access read-only
3. distinguish `PASS`, `WARN`, `FAIL`, and `ERROR`
4. include remediation guidance and cost impact notes
5. add `HIGH` and `MEDIUM` follow-up items to `TODO.md` unless disabled

## Notes

- The source of truth for this skill is `skills/common/aws-health-review/SKILL.md`.
- This is a health baseline, not a full compliance audit.

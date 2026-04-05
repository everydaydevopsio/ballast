---
name: aws-live-health-review
description: Run a read-only AWS live health review for current EC2, RDS, ALB, CloudWatch alarms, and CloudWatch logs, then generate a Markdown status report with current health, risks, and evidence. Use when asked for the system's health right now or a current AWS operations snapshot.
---

# AWS Live Health Review

Use this skill for an on-demand, current-state AWS health snapshot.

## Profile Selection

- `--profile <name>`
- `PROFILE=<name>` or `AWS_PROFILE=<name>`

If both are set, `--profile` wins. The default remains `wepro-readonly`.

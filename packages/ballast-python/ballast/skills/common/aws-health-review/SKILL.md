---
name: aws-health-review
description: Run a weekly, read-only AWS health review covering configuration issues, performance problems, errors, and warnings. Generates a Markdown report and appends new P0/P1 tasks to TODO.md. Use when asked for AWS health checks, weekly infrastructure review, or configuration/performance triage.
---

# AWS Health Review

Use this skill to run a repeatable weekly AWS health review and generate a Markdown report.

## Profile Selection

- `--profile <name>`
- `PROFILE=<name>` or `AWS_PROFILE=<name>`

If both are set, `--profile` wins. The default remains `wepro-readonly`.

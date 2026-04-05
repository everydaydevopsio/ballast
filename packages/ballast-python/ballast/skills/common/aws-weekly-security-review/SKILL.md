---
name: aws-weekly-security-review
description: Run a weekly, read-only AWS security baseline review and generate a Markdown report with prioritized findings. Use when asked for recurring AWS security posture checks, quick risk triage, or a starting point for ongoing cloud hardening.
---

# AWS Weekly Security Review

Use this skill to run a simple, repeatable AWS security baseline review each week.

## Profile Selection

- `--profile <name>`
- `PROFILE=<name>` or `AWS_PROFILE=<name>`

If both are set, `--profile` wins. The default remains `wepro-readonly`.

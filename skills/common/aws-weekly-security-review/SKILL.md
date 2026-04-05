---
name: aws-weekly-security-review
description: Run a weekly, read-only AWS security baseline review and generate a Markdown report with prioritized findings. Use when asked for recurring AWS security posture checks, quick risk triage, or a starting point for ongoing cloud hardening.
---

# AWS Weekly Security Review

Use this skill to run a simple, repeatable AWS security baseline review each week.

## What It Does

The bundled script runs read-only checks against AWS and writes a Markdown report with:
- `PASS`, `WARN`, `FAIL`, or `ERROR` status per check.
- Severity-tagged findings (`HIGH`, `MEDIUM`, `LOW`, `INFO`).
- Cost impact note per finding (required), so recommended remediations account for billing impact.
- Raw evidence snippets for faster triage.

## Command

Run from repository root:

```bash
uv run skills/aws-weekly-security-review/scripts/aws_weekly_security_review.py
```

Optional flags:
- `PROFILE=wepro-readonly` or `AWS_PROFILE=wepro-readonly` (default profile if `--profile` is omitted; `PROFILE` takes precedence)
- `--profile wepro-readonly` (overrides `PROFILE` / `AWS_PROFILE`)
- `--region us-east-1` (default: `us-east-1`)
- `--output reports/aws-security-weekly-YYYYMMDD.md` (default auto-generated in `reports/`)

## Current Baseline Checks

1. Root account MFA enabled and IAM user MFA coverage reported (per-user evidence).
2. CloudTrail has at least one multi-region trail with logging enabled.
3. Security groups in `us-east-1` do not expose SSH (22), RDP (3389), MySQL (3306), PostgreSQL (5432), or MSSQL (1433) to the internet.
4. IAM users with `AdministratorAccess` either directly attached or inherited via group policy attachment.
5. S3 account-level Public Access Block is fully enabled (treats missing config as FAIL, not ERROR).
6. RDS instances are not publicly accessible.
7. IAM access key age — active keys >90 days flagged MEDIUM, >180 days flagged HIGH.
8. IAM account password policy — flags missing policy (HIGH) or policy missing min-length >= 14, symbols, or max-age <= 90 days (MEDIUM each).
9. S3 bucket-level Public Access Block — checks each bucket; treats missing config as all flags disabled.
10. GuardDuty threat detection is enabled and active in the scoped region.

## Guardrails

- Read-only only: no mutate/delete operations.
- Always run with an explicit AWS account context (`--profile` or `PROFILE` / `AWS_PROFILE`) and an explicit `--region`.
- This is a baseline review, not a full compliance audit.
- Every recommendation must include a cost impact statement.
- If a recommendation introduces a new AWS service or enables a paid feature, include expected cost drivers (for example: per-event, per-GB, per-request, storage, or data scan).
- Prefer AWS Pricing API-backed estimates when available; otherwise provide a clear qualitative range (`None`, `Low`, `Medium`, `High`) and list assumptions.
- For service-add/change recommendations, run AWS Pricing API lookups in-skill and include output under that finding's `Cost review` details in the check results.

## Extension Path

When extending this skill, keep checks small, deterministic, and mapped to one risk each. Add new checks in the same result format so weekly reports remain comparable over time.
- Every finding object must include `severity`, `message`, `recommendation`, and `cost_impact`.
- For service-add/change recommendations, include `cost_scope_services` and `cost_change_summary` in the finding so the skill can run automatic pricing review and add that output in the finding block.

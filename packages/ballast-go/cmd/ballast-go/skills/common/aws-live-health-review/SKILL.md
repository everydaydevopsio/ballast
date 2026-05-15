---
name: aws-live-health-review
description: Run a read-only AWS live health review for current EC2, RDS, ALB, CloudWatch alarms, and CloudWatch logs, then generate a Markdown status report with current health, risks, and evidence. Use when asked for the system's health right now or a current AWS operations snapshot.
---

# AWS Live Health Review

Use this skill for an on-demand, current-state AWS health snapshot.

It is narrower than `aws-health-review` and more operationally focused:
- current EC2 instance and status-check health
- current RDS availability and key risk flags
- current ALB and target-group health
- current CloudWatch alarms in `ALARM`
- recent CloudWatch log signals from active app and database log groups

## Command

Run from repository root:

```bash
uv run skills/aws-live-health-review/scripts/aws_live_health_review.py
```

Optional flags:
- `PROFILE=wepro-readonly` or `AWS_PROFILE=wepro-readonly` (default profile if `--profile` is omitted; `PROFILE` takes precedence)
- `--profile wepro-readonly` (overrides `PROFILE` / `AWS_PROFILE`)
- `--region us-east-1` (default: `us-east-1`)
- `--logs-hours 24` (how far back to inspect logs)
- `--output reports/aws-live-health-YYYYMMDDTHHMMSSZ.md`

## Guardrails

- Read-only only. Never use mutate or delete AWS operations.
- Always run with an explicit profile via `--profile` or `PROFILE`, plus `--region us-east-1`.
- Keep timestamps in UTC in the report and response.
- Separate current outages from configuration risks. Do not present a config warning as an active incident.

## Workflow

1. Run the script to generate a current status report in `reports/`.
2. Lead with current availability status first: EC2, ALB target health, RDS status, and active alarms.
3. Use CloudWatch log findings as supporting evidence, not as the sole basis for outage claims.
4. Call out important risks separately, especially:
   - public RDS exposure or repeated auth failures
   - slow-query volume
   - ALBs without access logging
   - large numbers of log groups without retention
   - stopped EC2 instances accumulating drift/cost
5. If the report shows a likely incident, escalate to `aws-incident-review` for a time-window analysis.

## Output Expectations

- Distinguish `healthy now` from `needs attention`.
- Treat autoscaling low-traffic alarms as scale signals unless other evidence shows impact.
- Be explicit about observability gaps when application logs are missing for EC2-hosted services.
- Keep the response concise and operational: current state, risks, and next actions.

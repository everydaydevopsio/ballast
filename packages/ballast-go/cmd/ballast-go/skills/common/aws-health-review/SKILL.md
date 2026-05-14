---
name: aws-health-review
description: Run a weekly, read-only AWS health review covering configuration issues, performance problems, errors, and warnings. Generates a Markdown report and appends new P0/P1 tasks to TODO.md. Use when asked for AWS health checks, weekly infrastructure review, or configuration/performance triage.
---

# AWS Health Review

Use this skill to run a repeatable weekly AWS health review across account health events, configuration, performance, reliability, observability, backups, certificates, slow queries, and billing. It produces a Markdown report and automatically adds high/medium findings as tasks in `TODO.md`.

## What It Does

The bundled script runs read-only checks and writes a Markdown report with:
- `PASS`, `WARN`, `FAIL`, or `ERROR` status per check.
- Severity-tagged findings (`HIGH`, `MEDIUM`, `LOW`, `INFO`).
- **Why** each issue matters (operational/cost rationale).
- **How to implement** the fix (concrete CLI steps).
- Cost impact note per finding.
- Raw evidence snippets for faster triage.
- New `P0`/`P1`/`P2` tasks appended to `TODO.md` for `HIGH`/`MEDIUM` findings.

## Command

Run from repository root:

```bash
uv run skills/aws-health-review/scripts/aws_health_review.py
```

Optional flags:
- `PROFILE=wepro-readonly` or `AWS_PROFILE=wepro-readonly` (default profile if `--profile` is omitted; `PROFILE` takes precedence)
- `--profile wepro-readonly` (overrides `PROFILE` / `AWS_PROFILE`)
- `--region us-east-1` (default: `us-east-1`)
- `--output reports/aws-health-weekly-YYYYMMDD.md` (default: auto-generated in `reports/`)
- `--no-todo-update` (skip writing tasks to `TODO.md`)
- `--todo-path TODO.md` (default: `TODO.md`)

## Current Checks

| # | Check | Category |
|---|-------|----------|
| 1 | CloudWatch alarms currently in ALARM state, excluding AWS-managed target-tracking alarms | Errors |
| 2 | AWS Health account events: open and upcoming account-specific events, action-required maintenance, affected resources | Reliability |
| 3 | CloudWatch log groups with no retention policy | Configuration |
| 4 | ALB target group unhealthy hosts | Performance / Errors |
| 5 | ALB access logging disabled | Configuration |
| 6 | ALB service signals: 24h ELB/target 5XX totals and target response time | Performance / Errors |
| 7 | RDS health: status, backup retention, storage utilization (≥75%/≥90%), CPU (≥70%/≥85%), Multi-AZ | Performance / Configuration |
| 8 | RDS deeper signals: freeable memory, connection count, read/write latency, disk queue depth | Performance |
| 9 | ASG health: InService count vs desired, unhealthy instances, recent failed instance refreshes | Errors |
| 10 | EC2 stopped instances | Cost / Configuration |
| 11 | EC2 runtime health: status-check impairment and scheduled AWS events | Reliability |
| 12 | App Runner service health | Errors |
| 13 | ElastiCache cluster health, eviction rate, and cache hit rate | Performance |
| 14 | CloudWatch metric alarms with no notification actions configured | Configuration |
| 15 | Backup coverage and restore readiness: AWS Backup plan/resource coverage, failed backup jobs, restore-job evidence | Resilience |
| 16 | Alarm coverage for critical resources: ALB, RDS, ASG, EC2 status checks | Observability |
| 17 | ACM certificate expiration for in-use certificates | Reliability |
| 18 | RDS slow-query count over the last 24 hours from CloudWatch slowquery log groups | DB Performance |
| 19 | Billing: MTD spend vs same period last month — flags ≥20% (MEDIUM) and ≥50% (HIGH) spikes; top 6 services by cost | Billing |
| 20 | Billing: Cost Anomaly Detection — checks monitors are configured; surfaces open anomalies with dollar impact | Billing |
| 21 | Billing: idle resource waste — unattached EBS volumes and unassociated Elastic IPs with estimated monthly waste | Billing |

## TODO.md Integration

For every `HIGH` or `MEDIUM` finding, the script appends a new row to the `TODO.md` priority table:
- `HIGH` findings → `P0`
- `MEDIUM` findings → `P1`
- `LOW` findings are reported in the Markdown report only and are not added to `TODO.md`

Tasks are inserted idempotently using a stable content-derived marker: re-running the same report date will not add duplicate rows.

## Guardrails

- Read-only only: no mutate/delete operations.
- Always run with an explicit profile via `--profile` or `PROFILE`, plus `--region us-east-1`.
- This is a health baseline review, not a full compliance audit.
- Every finding includes a cost impact statement.
- Do not act on recommendations without explicit human approval of the change plan.

## Weekly Schedule

This skill is intended to run weekly, typically at week closeout via the `weekly-weekfile-manager` skill (`end-week` step). It can also be run on demand for ad-hoc triage.

## Extension Path

When adding new checks:
- Keep each check small, deterministic, and mapped to one risk.
- Return results using `make_check()` and `make_finding()` helpers.
- Every finding must include `severity`, `message`, `recommendation`, `cost_impact`, `why`, and `how`.
- Add the check to the `checks` list in `main()` and update this SKILL.md table.

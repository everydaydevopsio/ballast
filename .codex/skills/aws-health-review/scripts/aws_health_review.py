#!/usr/bin/env python3
# /// script
# requires-python = ">=3.10"
# ///
"""Weekly read-only AWS health review: configuration, performance, errors, warnings, and billing."""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
import re
import subprocess
import sys
import time
from pathlib import Path
from typing import Any


# ---------------------------------------------------------------------------
# Shared helpers
# ---------------------------------------------------------------------------

class AwsCommandError(RuntimeError):
    """Raised when an AWS CLI command fails."""


def make_finding(
    severity: str,
    message: str,
    recommendation: str,
    cost_impact: str,
    why: str = "",
    how: str = "",
) -> dict[str, Any]:
    """Build a structured finding with severity, rationale, and implementation guidance."""
    finding: dict[str, Any] = {
        "severity": severity,
        "message": message,
        "recommendation": recommendation,
        "cost_impact": cost_impact,
    }
    if why:
        finding["why"] = why
    if how:
        finding["how"] = how
    return finding


def run_aws_json(profile: str, region: str, args: list[str]) -> Any:
    cmd = ["aws", "--profile", profile, "--region", region, *args, "--output", "json"]
    result = subprocess.run(cmd, capture_output=True, text=True, check=False)
    if result.returncode != 0:
        raise AwsCommandError(result.stderr.strip() or "AWS command failed")
    payload = result.stdout.strip()
    if not payload:
        return {}
    return json.loads(payload)


def run_aws_json_safe(profile: str, region: str, args: list[str]) -> Any:
    """Like run_aws_json but returns None on failure instead of raising."""
    try:
        return run_aws_json(profile, region, args)
    except (AwsCommandError, json.JSONDecodeError):
        return None


def utc_now() -> dt.datetime:
    return dt.datetime.now(dt.timezone.utc)


def parse_aws_datetime(value: str | None) -> dt.datetime | None:
    if not value:
        return None
    try:
        return dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None


def iso_z(value: dt.datetime) -> str:
    return value.strftime("%Y-%m-%dT%H:%M:%SZ")


def make_check(
    check_id: str,
    title: str,
    status: str,
    findings: list[dict[str, Any]],
    evidence: list[str],
) -> dict[str, Any]:
    return {
        "id": check_id,
        "title": title,
        "status": status,
        "findings": findings,
        "evidence": evidence,
    }


def _sort_datapoints(points: list[dict[str, Any]]) -> list[dict[str, Any]]:
    return sorted(points, key=lambda p: str(p.get("Timestamp", "")))


def get_metric_statistics(
    profile: str,
    region: str,
    namespace: str,
    metric_name: str,
    dimensions: list[tuple[str, str]],
    start: dt.datetime,
    end: dt.datetime,
    period: int,
    statistics: list[str],
) -> list[dict[str, Any]]:
    args = [
        "cloudwatch", "get-metric-statistics",
        "--namespace", namespace,
        "--metric-name", metric_name,
        "--start-time", iso_z(start),
        "--end-time", iso_z(end),
        "--period", str(period),
        "--statistics", *statistics,
    ]
    if dimensions:
        args.extend(["--dimensions", *[f"Name={name},Value={value}" for name, value in dimensions]])
    data = run_aws_json_safe(profile, region, args)
    if data is None:
        return []
    return _sort_datapoints(data.get("Datapoints", []))


def latest_metric_stat(
    profile: str,
    region: str,
    namespace: str,
    metric_name: str,
    dimensions: list[tuple[str, str]],
    hours: int,
    stat: str,
    period: int = 3600,
) -> float | None:
    end = utc_now()
    start = end - dt.timedelta(hours=hours)
    points = get_metric_statistics(profile, region, namespace, metric_name, dimensions, start, end, period, [stat])
    if not points:
        return None
    value = points[-1].get(stat)
    return float(value) if value is not None else None


def sum_metric_over_hours(
    profile: str,
    region: str,
    namespace: str,
    metric_name: str,
    dimensions: list[tuple[str, str]],
    hours: int,
    period: int = 3600,
) -> float | None:
    end = utc_now()
    start = end - dt.timedelta(hours=hours)
    points = get_metric_statistics(profile, region, namespace, metric_name, dimensions, start, end, period, ["Sum"])
    if not points:
        return None
    return sum(float(point.get("Sum", 0.0) or 0.0) for point in points)


def avg_metric_over_hours(
    profile: str,
    region: str,
    namespace: str,
    metric_name: str,
    dimensions: list[tuple[str, str]],
    hours: int,
    period: int = 3600,
) -> float | None:
    end = utc_now()
    start = end - dt.timedelta(hours=hours)
    points = get_metric_statistics(profile, region, namespace, metric_name, dimensions, start, end, period, ["Average"])
    if not points:
        return None
    values = [float(point.get("Average", 0.0) or 0.0) for point in points]
    return sum(values) / len(values) if values else None


def max_metric_over_hours(
    profile: str,
    region: str,
    namespace: str,
    metric_name: str,
    dimensions: list[tuple[str, str]],
    hours: int,
    period: int = 3600,
) -> float | None:
    end = utc_now()
    start = end - dt.timedelta(hours=hours)
    points = get_metric_statistics(profile, region, namespace, metric_name, dimensions, start, end, period, ["Maximum"])
    if not points:
        return None
    values = [float(point.get("Maximum", 0.0) or 0.0) for point in points]
    return max(values) if values else None


def aws_resource_suffix(arn: str, marker: str) -> str:
    if marker not in arn:
        return arn
    return arn.split(marker, 1)[1]


def is_target_tracking_alarm_name(name: str) -> bool:
    return name.startswith("TargetTracking-")


def alarm_metric_names(alarm: dict[str, Any]) -> set[str]:
    names: set[str] = set()
    metric_name = alarm.get("MetricName")
    if metric_name:
        names.add(str(metric_name))
    for metric_query in alarm.get("Metrics", []):
        metric_stat = metric_query.get("MetricStat", {})
        metric = metric_stat.get("Metric", {})
        inner_name = metric.get("MetricName")
        if inner_name:
            names.add(str(inner_name))
    return names


def alarm_dimension_values(alarm: dict[str, Any], dimension_name: str) -> set[str]:
    values: set[str] = set()
    for dim in alarm.get("Dimensions", []):
        if dim.get("Name") == dimension_name and dim.get("Value"):
            values.add(str(dim["Value"]))
    for metric_query in alarm.get("Metrics", []):
        metric_stat = metric_query.get("MetricStat", {})
        metric = metric_stat.get("Metric", {})
        for dim in metric.get("Dimensions", []):
            if dim.get("Name") == dimension_name and dim.get("Value"):
                values.add(str(dim["Value"]))
    return values


def stable_task_id(task_text: str) -> str:
    return hashlib.sha256(task_text.encode("utf-8")).hexdigest()[:8]


def start_logs_query(profile: str, region: str, log_group: str, start_time: int, end_time: int, query: str) -> str:
    data = run_aws_json(
        profile,
        region,
        [
            "logs", "start-query",
            "--log-group-name", log_group,
            "--start-time", str(start_time),
            "--end-time", str(end_time),
            "--query-string", query,
        ],
    )
    query_id = data.get("queryId", "")
    if not query_id:
        raise AwsCommandError("CloudWatch Logs Insights query did not return queryId")
    return str(query_id)


def wait_for_logs_query(profile: str, region: str, query_id: str, timeout_seconds: int = 60) -> list[dict[str, str]]:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        data = run_aws_json(profile, region, ["logs", "get-query-results", "--query-id", query_id])
        status = data.get("status", "Unknown")
        if status == "Complete":
            results: list[dict[str, str]] = []
            for row in data.get("results", []):
                results.append({item.get("field", ""): item.get("value", "") for item in row})
            return results
        if status in {"Failed", "Cancelled", "Timeout"}:
            raise AwsCommandError(f"CloudWatch Logs Insights query ended with status={status}")
        time.sleep(2)
    raise AwsCommandError(f"Timed out waiting for CloudWatch Logs Insights query {query_id}")


# ---------------------------------------------------------------------------
# Check 1: CloudWatch alarms in ALARM state
# ---------------------------------------------------------------------------

def check_cloudwatch_alarms(profile: str, region: str) -> dict[str, Any]:
    data = run_aws_json_safe(profile, region, [
        "cloudwatch", "describe-alarms",
        "--state-value", "ALARM",
        "--alarm-types", "MetricAlarm", "CompositeAlarm",
    ])
    alarms_in_alarm: list[str] = []
    evidence: list[str] = []

    if data is None:
        return make_check(
            "cloudwatch-alarms", "CloudWatch alarms in ALARM state",
            "ERROR", [make_finding("INFO", "Could not retrieve alarm state.", "", "None")], ["api_error=true"],
        )

    metric_alarms = data.get("MetricAlarms", [])
    composite_alarms = data.get("CompositeAlarms", [])

    ignored_managed = 0

    for alarm in metric_alarms:
        name = alarm.get("AlarmName", "unknown")
        if is_target_tracking_alarm_name(name):
            ignored_managed += 1
            evidence.append(f"IGNORED managed_target_tracking_alarm={name}")
            continue
        actions = alarm.get("AlarmActions", [])
        namespace = alarm.get("Namespace", "")
        metric = alarm.get("MetricName", "")
        alarms_in_alarm.append(name)
        evidence.append(f"ALARM metric={namespace}/{metric} alarm={name} actions={'yes' if actions else 'NONE'}")

    for alarm in composite_alarms:
        name = alarm.get("AlarmName", "unknown")
        alarms_in_alarm.append(name)
        evidence.append(f"ALARM composite={name}")

    if not alarms_in_alarm:
        return make_check(
            "cloudwatch-alarms", "CloudWatch alarms in ALARM state",
            "PASS", [], evidence or ["no_actionable_alarms_firing=true"],
        )

    return make_check(
        "cloudwatch-alarms", "CloudWatch alarms in ALARM state",
        "FAIL",
        [make_finding(
            severity="HIGH",
            message=f"{len(alarms_in_alarm)} CloudWatch alarm(s) are currently in ALARM state: {', '.join(alarms_in_alarm[:10])}{'...' if len(alarms_in_alarm) > 10 else ''}.",
            recommendation="Investigate each firing alarm. Resolve the underlying condition or suppress if expected noise.",
            cost_impact="None — reading alarm state is free. Underlying cause may have cost implications.",
            why="Active alarms signal unresolved infrastructure problems that can affect availability and performance.",
            how="For each alarm: (1) open CloudWatch > Alarms, (2) view the metric graph and recent state history, (3) correlate with service logs, (4) resolve root cause or adjust threshold if misconfigured.",
        )],
        evidence,
    )


# ---------------------------------------------------------------------------
# Check 1b: AWS Health events
# ---------------------------------------------------------------------------

def check_aws_health_events(profile: str, region: str) -> dict[str, Any]:
    data = run_aws_json_safe(profile, region, ["health", "describe-events"])
    if data is None:
        return make_check(
            "aws-health-events", "AWS Health account events",
            "WARN",
            [make_finding(
                "MEDIUM",
                "Could not query AWS Health events. This may indicate missing permission or unsupported support plan access.",
                "Ensure the read-only role can access AWS Health and review account-specific AWS Health events another way if needed.",
                "None for the check.",
                why="AWS Health surfaces account-specific outages, scheduled maintenance, and action-required notices that service metrics alone can miss.",
                how="Grant read-only AWS Health access (`health:DescribeEvents`, `health:DescribeEventDetails`, `health:DescribeAffectedEntities`) or review AWS Health Dashboard in the console.",
            )],
            ["aws_health_api_error=true"],
        )

    events = data.get("events", [])
    if not events:
        return make_check("aws-health-events", "AWS Health account events", "PASS", [], ["no_health_events_returned=true"])

    findings: list[dict[str, Any]] = []
    evidence: list[str] = []
    status = "PASS"
    actionable_arns: list[str] = []
    recent_cutoff = utc_now() - dt.timedelta(days=14)

    for event in events:
        arn = event.get("arn", "")
        service = event.get("service", "unknown")
        event_type = event.get("eventTypeCode", "unknown")
        event_status = event.get("statusCode", "unknown")
        scope = event.get("eventScopeCode", "unknown")
        start_time = parse_aws_datetime(event.get("startTime"))
        start_label = start_time.strftime("%Y-%m-%d") if start_time else "unknown"
        evidence.append(f"event service={service} type={event_type} status={event_status} scope={scope} start={start_label}")

        is_recent = start_time is None or start_time >= recent_cutoff
        if event_status in {"open", "upcoming"} and is_recent:
            actionable_arns.append(arn)

    if not actionable_arns:
        return make_check("aws-health-events", "AWS Health account events", "PASS", [], evidence[:20])

    details = run_aws_json_safe(profile, region, ["health", "describe-event-details", "--event-arns", *actionable_arns[:10]])
    entities = run_aws_json_safe(profile, region, ["health", "describe-affected-entities", "--filter", json.dumps({"eventArns": actionable_arns[:10]})])
    entities_by_event: dict[str, list[str]] = {}
    if entities is not None:
        for item in entities.get("entities", []):
            entities_by_event.setdefault(item.get("eventArn", ""), []).append(item.get("entityValue", "?"))

    detailed_events = (details or {}).get("successfulSet", [])
    if not detailed_events:
        detailed_events = [{"event": event, "eventDescription": {}} for event in events if event.get("arn") in actionable_arns]

    for detail in detailed_events:
        event = detail.get("event", {})
        event_arn = event.get("arn", "")
        event_type = event.get("eventTypeCode", "unknown")
        service = event.get("service", "unknown")
        event_status = event.get("statusCode", "unknown")
        affected = entities_by_event.get(event_arn, [])
        severity = "HIGH" if event_status == "open" else "MEDIUM"
        status = "FAIL" if severity == "HIGH" else ("WARN" if status == "PASS" else status)
        findings.append(make_finding(
            severity=severity,
            message=f"AWS Health event for {service} is {event_status}: {event_type}. Affected resources: {', '.join(affected[:5]) or 'not listed'}.",
            recommendation="Review the AWS Health event details and complete any required remediation or maintenance preparation.",
            cost_impact="None for the check. Unaddressed AWS Health events can lead to outages or forced maintenance windows.",
            why="AWS Health is AWS’s source of truth for account-specific degradation, maintenance, and action-required notices.",
            how="Open AWS Health Dashboard and read the event details, affected resources, and AWS guidance. Schedule any required maintenance before the deadline.",
        ))
        latest_desc = detail.get("eventDescription", {}).get("latestDescription", "")
        if latest_desc:
            evidence.append(f"detail type={event_type} desc={latest_desc[:180]}")

    return make_check("aws-health-events", "AWS Health account events", status, findings, evidence[:30])


# ---------------------------------------------------------------------------
# Check 2: CloudWatch log groups with no retention policy
# ---------------------------------------------------------------------------

def check_log_group_retention(profile: str, region: str) -> dict[str, Any]:
    paginator_token: str | None = None
    no_retention: list[str] = []
    evidence: list[str] = []

    while True:
        args = ["logs", "describe-log-groups"]
        if paginator_token:
            args += ["--next-token", paginator_token]

        data = run_aws_json_safe(profile, region, args)
        if data is None:
            break

        for lg in data.get("logGroups", []):
            name = lg.get("logGroupName", "")
            retention = lg.get("retentionInDays")
            if retention is None:
                no_retention.append(name)
                evidence.append(f"no_retention={name}")
            else:
                evidence.append(f"retention={retention}d group={name}")

        paginator_token = data.get("nextToken")
        if not paginator_token:
            break

    if not no_retention:
        return make_check(
            "log-group-retention", "CloudWatch log group retention policies",
            "PASS", [], evidence or ["all_groups_have_retention=true"],
        )

    severity = "HIGH" if len(no_retention) > 5 else "MEDIUM"
    return make_check(
        "log-group-retention", "CloudWatch log group retention policies",
        "WARN",
        [make_finding(
            severity=severity,
            message=f"{len(no_retention)} log group(s) have no retention policy (logs kept indefinitely): {', '.join(sorted(no_retention)[:10])}{'...' if len(no_retention) > 10 else ''}.",
            recommendation="Set explicit retention policies on all log groups. 30–90 days is typical for operational logs.",
            cost_impact="Medium. CloudWatch Logs charges $0.03/GB per month for storage. Unlimited retention accumulates unbounded cost over time.",
            why="Unlimited log retention drives unnecessary CloudWatch storage costs and violates data-lifecycle hygiene best practice.",
            how="Run: aws logs put-retention-policy --log-group-name <name> --retention-in-days 30 (or 60/90 per retention requirements). Apply to each group without a policy.",
        )],
        evidence[:30],
    )


# ---------------------------------------------------------------------------
# Check 3: ALB target group unhealthy hosts
# ---------------------------------------------------------------------------

def check_alb_target_health(profile: str, region: str) -> dict[str, Any]:
    tgs_data = run_aws_json_safe(profile, region, ["elbv2", "describe-target-groups"])
    if tgs_data is None:
        return make_check(
            "alb-target-health", "ALB target group health",
            "ERROR", [], ["api_error=true"],
        )

    unhealthy: list[str] = []
    evidence: list[str] = []

    for tg in tgs_data.get("TargetGroups", []):
        arn = tg.get("TargetGroupArn", "")
        name = tg.get("TargetGroupName", "unknown")
        health_data = run_aws_json_safe(profile, region, [
            "elbv2", "describe-target-health", "--target-group-arn", arn,
        ])
        if health_data is None:
            evidence.append(f"tg={name} health_check_error=true")
            continue

        targets = health_data.get("TargetHealthDescriptions", [])
        healthy = sum(1 for t in targets if t.get("TargetHealth", {}).get("State") == "healthy")
        total = len(targets)
        bad = [
            t for t in targets
            if t.get("TargetHealth", {}).get("State") not in ("healthy", "unused")
        ]
        evidence.append(f"tg={name} healthy={healthy}/{total}")

        if bad:
            for t in bad:
                state = t.get("TargetHealth", {}).get("State", "unknown")
                reason = t.get("TargetHealth", {}).get("Reason", "")
                target_id = t.get("Target", {}).get("Id", "?")
                unhealthy.append(f"{name}/{target_id}({state}:{reason})")

    if not unhealthy:
        return make_check(
            "alb-target-health", "ALB target group health",
            "PASS", [], evidence,
        )

    return make_check(
        "alb-target-health", "ALB target group health",
        "FAIL",
        [make_finding(
            severity="HIGH",
            message=f"{len(unhealthy)} ALB target(s) are unhealthy: {'; '.join(unhealthy[:8])}{'...' if len(unhealthy) > 8 else ''}.",
            recommendation="Investigate each unhealthy target: check EC2 instance status, application health check endpoint, and security group rules.",
            cost_impact="None for the check. Unhealthy targets reduce effective capacity, potentially increasing latency or error rates for real traffic.",
            why="Unhealthy targets mean the ALB has fewer backends to serve requests, increasing load on remaining hosts and risking elevated error rates.",
            how="(1) SSH to the unhealthy instance and verify the health-check endpoint responds correctly. (2) Check security group allows health-check traffic from the ALB. (3) Review application logs for startup errors or OOM signals.",
        )],
        evidence,
    )


# ---------------------------------------------------------------------------
# Check 4: ALB access logging disabled
# ---------------------------------------------------------------------------

def check_alb_access_logging(profile: str, region: str) -> dict[str, Any]:
    lbs_data = run_aws_json_safe(profile, region, ["elbv2", "describe-load-balancers"])
    if lbs_data is None:
        return make_check(
            "alb-access-logging", "ALB access logging",
            "ERROR", [], ["api_error=true"],
        )

    no_logging: list[str] = []
    evidence: list[str] = []

    for lb in lbs_data.get("LoadBalancers", []):
        arn = lb.get("LoadBalancerArn", "")
        name = lb.get("LoadBalancerName", "unknown")
        attrs_data = run_aws_json_safe(profile, region, [
            "elbv2", "describe-load-balancer-attributes", "--load-balancer-arn", arn,
        ])
        if attrs_data is None:
            evidence.append(f"lb={name} attrs_error=true")
            continue

        attrs = {a["Key"]: a["Value"] for a in attrs_data.get("Attributes", [])}
        logging_enabled = attrs.get("access_logs.s3.enabled", "false").lower() == "true"
        log_bucket = attrs.get("access_logs.s3.bucket", "")
        evidence.append(f"lb={name} access_log_enabled={logging_enabled} bucket={log_bucket or 'none'}")

        if not logging_enabled:
            no_logging.append(name)

    if not no_logging:
        return make_check(
            "alb-access-logging", "ALB access logging",
            "PASS", [], evidence,
        )

    return make_check(
        "alb-access-logging", "ALB access logging",
        "WARN",
        [make_finding(
            severity="MEDIUM",
            message=f"{len(no_logging)} ALB(s) do not have access logging enabled: {', '.join(no_logging)}.",
            recommendation="Enable access logging on each ALB and store logs in a dedicated S3 bucket with lifecycle rules.",
            cost_impact="Low. S3 storage for ALB logs is typically $0.023/GB per month. ALB log delivery itself is free.",
            why="Access logs are essential for security forensics, traffic analysis, and debugging latency/error patterns at the request level.",
            how="(1) Create an S3 bucket with an appropriate bucket policy. (2) Enable access logging: aws elbv2 modify-load-balancer-attributes --load-balancer-arn <arn> --attributes Key=access_logs.s3.enabled,Value=true Key=access_logs.s3.bucket,Value=<bucket>. (3) Set a lifecycle rule to expire logs after 90 days.",
        )],
        evidence,
    )


# ---------------------------------------------------------------------------
# Check 4b: ALB 5XX and latency signals
# ---------------------------------------------------------------------------

def check_alb_service_signals(profile: str, region: str) -> dict[str, Any]:
    lbs_data = run_aws_json_safe(profile, region, ["elbv2", "describe-load-balancers"])
    if lbs_data is None:
        return make_check(
            "alb-service-signals", "ALB 5XX and latency signals",
            "ERROR", [], ["api_error=true"],
        )

    findings: list[dict[str, Any]] = []
    evidence: list[str] = []
    status = "PASS"

    for lb in lbs_data.get("LoadBalancers", []):
        if lb.get("Type") != "application":
            continue
        name = lb.get("LoadBalancerName", "unknown")
        lb_dim = aws_resource_suffix(lb.get("LoadBalancerArn", ""), "loadbalancer/")
        dims = [("LoadBalancer", lb_dim)]

        elb_5xx = sum_metric_over_hours(profile, region, "AWS/ApplicationELB", "HTTPCode_ELB_5XX_Count", dims, 24)
        target_5xx = sum_metric_over_hours(profile, region, "AWS/ApplicationELB", "HTTPCode_Target_5XX_Count", dims, 24)
        resp_avg = avg_metric_over_hours(profile, region, "AWS/ApplicationELB", "TargetResponseTime", dims, 24)
        resp_max = max_metric_over_hours(profile, region, "AWS/ApplicationELB", "TargetResponseTime", dims, 24)

        if elb_5xx is not None:
            evidence.append(f"alb={name} elb5xx24h={elb_5xx:.0f}")
        if target_5xx is not None:
            evidence.append(f"alb={name} target5xx24h={target_5xx:.0f}")
        if resp_avg is not None:
            evidence.append(f"alb={name} targetResponseAvg24hMs={resp_avg * 1000:.1f}")
        if resp_max is not None:
            evidence.append(f"alb={name} targetResponseMax24hMs={resp_max * 1000:.1f}")

        total_5xx = (elb_5xx or 0.0) + (target_5xx or 0.0)
        if total_5xx >= 100:
            status = "FAIL"
            findings.append(make_finding(
                severity="HIGH",
                message=f"ALB {name} served {total_5xx:.0f} 5XX responses in the last 24h ({elb_5xx or 0:.0f} ELB, {target_5xx or 0:.0f} target).",
                recommendation=f"Investigate upstream failures and client-visible error spikes on ALB {name}.",
                cost_impact="None for the check. Customer-visible 5XXs usually indicate direct availability or revenue impact.",
                why="Healthy targets alone do not prove healthy request handling. 5XX totals capture real traffic failures experienced by clients.",
                how="Correlate ALB 5XXs with target logs, deployment history, and upstream service errors. Check whether failures are load balancer generated or target generated.",
            ))
        elif total_5xx > 0:
            if status == "PASS":
                status = "WARN"
            findings.append(make_finding(
                severity="MEDIUM",
                message=f"ALB {name} served {total_5xx:.0f} 5XX responses in the last 24h.",
                recommendation=f"Review recent 5XX patterns for ALB {name} and confirm they are expected or already addressed.",
                cost_impact="None for the check.",
                why="Even intermittent 5XXs are an early reliability signal that won’t appear in target-health checks.",
                how="Inspect ALB metrics and application logs around the highest-error periods to identify route-specific or deployment-specific failures.",
            ))

        if resp_avg is not None and resp_avg >= 1.0:
            status = "FAIL"
            findings.append(make_finding(
                severity="HIGH",
                message=f"ALB {name} has high target response time averaging {resp_avg:.2f}s over the last 24h.",
                recommendation=f"Investigate backend latency behind ALB {name}; review slow endpoints, database latency, and saturation.",
                cost_impact="None for the check. Elevated response time increases user abandonment and can trigger autoscaling or timeout cascades.",
                why="Target response time is a direct customer latency signal that target health and CPU-only checks often miss.",
                how="Correlate the latency window with RDS metrics, application logs, and deployment history. Identify the slowest routes and dependent queries.",
            ))
        elif resp_avg is not None and resp_avg >= 0.3:
            if status == "PASS":
                status = "WARN"
            findings.append(make_finding(
                severity="MEDIUM",
                message=f"ALB {name} target response time averaged {resp_avg:.2f}s over the last 24h.",
                recommendation=f"Review backend latency trends on ALB {name} before they become customer-visible incidents.",
                cost_impact="None for the check.",
                why="Latency regressions often appear before hard failures and should be corrected before autoscaling masks them.",
                how="Compare p95 endpoint latency, database timings, and queueing effects around the busiest hours.",
            ))

    if not evidence:
        evidence.append("no_application_load_balancers=true")
    return make_check("alb-service-signals", "ALB 5XX and latency signals", status, findings, evidence)


# ---------------------------------------------------------------------------
# Check 5: RDS storage and CPU via CloudWatch
# ---------------------------------------------------------------------------

def _get_rds_metric(profile: str, region: str, db_id: str, metric_name: str, stat: str = "Average") -> float | None:
    end = utc_now()
    start = end - dt.timedelta(hours=1)
    data = run_aws_json_safe(profile, region, [
        "cloudwatch", "get-metric-statistics",
        "--namespace", "AWS/RDS",
        "--metric-name", metric_name,
        "--dimensions", f"Name=DBInstanceIdentifier,Value={db_id}",
        "--start-time", start.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "--end-time", end.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "--period", "3600",
        "--statistics", stat,
    ])
    if data is None:
        return None
    datapoints = data.get("Datapoints", [])
    if not datapoints:
        return None
    return float(datapoints[-1].get(stat, 0.0))


def check_rds_health(profile: str, region: str) -> dict[str, Any]:
    instances_data = run_aws_json_safe(profile, region, ["rds", "describe-db-instances"])
    if instances_data is None:
        return make_check(
            "rds-health", "RDS instance health and performance",
            "ERROR", [], ["api_error=true"],
        )

    findings: list[dict[str, Any]] = []
    evidence: list[str] = []
    status = "PASS"

    for instance in instances_data.get("DBInstances", []):
        db_id = instance.get("DBInstanceIdentifier", "unknown")
        db_status = instance.get("DBInstanceStatus", "")
        allocated_gb = instance.get("AllocatedStorage", 0)
        backup_retention = instance.get("BackupRetentionPeriod", 0)
        multi_az = instance.get("MultiAZ", False)
        engine = instance.get("Engine", "")
        engine_version = instance.get("EngineVersion", "")

        evidence.append(
            f"rds={db_id} status={db_status} engine={engine} {engine_version} "
            f"storage={allocated_gb}GB multiAZ={multi_az} backupRetention={backup_retention}d"
        )

        # Check DB status
        if db_status not in ("available", "backing-up", "maintenance"):
            status = "FAIL"
            findings.append(make_finding(
                severity="HIGH",
                message=f"RDS instance {db_id} is in status '{db_status}' (expected 'available').",
                recommendation=f"Investigate RDS instance {db_id} immediately. Review recent Events in the RDS console.",
                cost_impact="None for the check. Degraded state may indicate downtime and business impact.",
                why="A non-available RDS instance means the database is unavailable or degraded, directly impacting application reliability.",
                how="(1) Open RDS console > {db_id} > Events tab. (2) Check recent maintenance or failover events. (3) If stuck, contact AWS Support.",
            ))

        # Backup retention
        if backup_retention == 0:
            if status == "PASS":
                status = "WARN"
            findings.append(make_finding(
                severity="HIGH",
                message=f"RDS instance {db_id} has automated backups disabled (BackupRetentionPeriod=0).",
                recommendation=f"Set backup retention to at least 7 days on {db_id}.",
                cost_impact="Low to medium. RDS automated backup storage is charged at $0.095/GB-month beyond the instance size.",
                why="Without automated backups, point-in-time recovery is impossible. A data-corruption event would require restoring from a manual snapshot.",
                how="Modify the instance: aws rds modify-db-instance --db-instance-identifier {db_id} --backup-retention-period 7 --apply-immediately",
            ))

        # Storage utilization (via CloudWatch FreeStorageSpace)
        free_bytes = _get_rds_metric(profile, region, db_id, "FreeStorageSpace")
        if free_bytes is not None:
            allocated_bytes = allocated_gb * 1024 ** 3
            used_pct = ((allocated_bytes - free_bytes) / allocated_bytes) * 100 if allocated_bytes > 0 else 0
            free_gb = free_bytes / 1024 ** 3
            evidence.append(f"rds={db_id} storageFreeGB={free_gb:.1f} storageUsedPct={used_pct:.1f}%")

            if used_pct >= 90:
                status = "FAIL"
                findings.append(make_finding(
                    severity="HIGH",
                    message=f"RDS instance {db_id} storage is {used_pct:.1f}% full ({free_gb:.1f} GB free of {allocated_gb} GB).",
                    recommendation=f"Increase allocated storage on {db_id} or purge unnecessary data immediately.",
                    cost_impact="Medium. Additional storage is $0.115/GB-month for gp2 or $0.10/GB-month for gp3 in us-east-1.",
                    why="RDS will enter read-only mode when storage is completely exhausted, causing immediate write failures and application outages.",
                    how="Enable storage autoscaling (recommended) or: aws rds modify-db-instance --db-instance-identifier {db_id} --allocated-storage <new_size> --apply-immediately",
                ))
            elif used_pct >= 75:
                if status == "PASS":
                    status = "WARN"
                findings.append(make_finding(
                    severity="MEDIUM",
                    message=f"RDS instance {db_id} storage is {used_pct:.1f}% full ({free_gb:.1f} GB free of {allocated_gb} GB).",
                    recommendation=f"Plan a storage increase for {db_id} within the next 1–2 weeks.",
                    cost_impact="Medium. Additional storage is ~$0.10–$0.115/GB-month depending on storage type.",
                    why="Storage at 75%+ is approaching the threshold where performance degrades and emergency action is needed.",
                    how="Enable RDS storage autoscaling to avoid future manual intervention, or schedule a storage increase during a maintenance window.",
                ))

        # CPU utilization
        cpu = _get_rds_metric(profile, region, db_id, "CPUUtilization")
        if cpu is not None:
            evidence.append(f"rds={db_id} cpuPct={cpu:.1f}%")
            if cpu >= 85:
                status = "FAIL"
                findings.append(make_finding(
                    severity="HIGH",
                    message=f"RDS instance {db_id} CPU is {cpu:.1f}% (last 1h average).",
                    recommendation=f"Identify top CPU-consuming queries on {db_id} and optimize or route to a read replica.",
                    cost_impact="None for the check. High CPU can cause query timeouts and degrade all application traffic.",
                    why="Sustained high CPU on RDS causes query queuing, timeout cascades, and can starve connection management overhead.",
                    how="(1) Enable Performance Insights and identify top wait events and SQL digests. (2) Add indexes for slow queries. (3) Route read-heavy traffic to a replica. (4) Consider scaling instance class if query patterns are already optimal.",
                ))
            elif cpu >= 70:
                if status == "PASS":
                    status = "WARN"
                findings.append(make_finding(
                    severity="MEDIUM",
                    message=f"RDS instance {db_id} CPU is elevated at {cpu:.1f}% (last 1h average).",
                    recommendation=f"Monitor query workload on {db_id} and identify any new slow queries.",
                    cost_impact="None for the check.",
                    why="CPU trending above 70% suggests growing query load that may exceed capacity at peak.",
                    how="Use Performance Insights to review top SQL by CPU. Correlate with application deploy history or traffic changes.",
                ))

        # Multi-AZ for production instances
        if not multi_az:
            if status == "PASS":
                status = "WARN"
            findings.append(make_finding(
                severity="MEDIUM",
                message=f"RDS instance {db_id} does not have Multi-AZ enabled.",
                recommendation=f"Enable Multi-AZ on {db_id} for automatic failover in the event of an AZ failure.",
                cost_impact="Medium. Multi-AZ roughly doubles RDS instance cost (~2x instance-hour + storage replication).",
                why="Single-AZ RDS has no automatic failover. An AZ-level failure causes an outage until manual recovery completes.",
                how="aws rds modify-db-instance --db-instance-identifier {db_id} --multi-az --apply-immediately (brief failover during modification).",
            ))

    return make_check("rds-health", "RDS instance health and performance", status, findings, evidence)


# ---------------------------------------------------------------------------
# Check 5b: RDS deeper performance signals
# ---------------------------------------------------------------------------

def check_rds_deep_metrics(profile: str, region: str) -> dict[str, Any]:
    instances_data = run_aws_json_safe(profile, region, ["rds", "describe-db-instances"])
    if instances_data is None:
        return make_check(
            "rds-deep-metrics", "RDS memory, connections, and latency",
            "ERROR", [], ["api_error=true"],
        )

    findings: list[dict[str, Any]] = []
    evidence: list[str] = []
    status = "PASS"

    for instance in instances_data.get("DBInstances", []):
        db_id = instance.get("DBInstanceIdentifier", "unknown")
        dims = [("DBInstanceIdentifier", db_id)]

        freeable_bytes = latest_metric_stat(profile, region, "AWS/RDS", "FreeableMemory", dims, 1, "Average")
        if freeable_bytes is not None:
            freeable_mb = freeable_bytes / (1024 ** 2)
            evidence.append(f"rds={db_id} freeableMemoryMB={freeable_mb:.0f}")
            if freeable_mb < 512:
                status = "FAIL"
                findings.append(make_finding(
                    severity="HIGH",
                    message=f"RDS instance {db_id} freeable memory is critically low at {freeable_mb:.0f} MiB.",
                    recommendation=f"Investigate memory pressure on {db_id}; optimize queries, reduce concurrency, or scale the instance class.",
                    cost_impact="Potentially high. Memory starvation can drive failovers, restarts, and emergency scaling.",
                    why="Low freeable memory causes swapping and instability, which frequently manifests as latency spikes and connection failures.",
                    how="Check Performance Insights, inspect top queries and waits, reduce expensive query concurrency, and scale the DB instance if demand is sustained.",
                ))
            elif freeable_mb < 1024:
                if status == "PASS":
                    status = "WARN"
                findings.append(make_finding(
                    severity="MEDIUM",
                    message=f"RDS instance {db_id} freeable memory is low at {freeable_mb:.0f} MiB.",
                    recommendation=f"Monitor memory pressure on {db_id} and review recent query workload changes.",
                    cost_impact="None for the check.",
                    why="Low freeable memory is an early warning for swap activity and degraded database performance.",
                    how="Review query patterns, connection pool settings, and whether a larger instance class is warranted.",
                ))

        connections = latest_metric_stat(profile, region, "AWS/RDS", "DatabaseConnections", dims, 1, "Average")
        if connections is not None:
            evidence.append(f"rds={db_id} dbConnections={connections:.1f}")
            if connections >= 1000:
                status = "FAIL"
                findings.append(make_finding(
                    severity="HIGH",
                    message=f"RDS instance {db_id} has very high connection count at {connections:.0f} (last 1h average).",
                    recommendation=f"Review application connection pooling and identify connection leaks against {db_id}.",
                    cost_impact="None for the check.",
                    why="Excessive open connections can exhaust database memory and thread capacity, even before CPU saturates.",
                    how="Inspect app pool sizes, close idle connections, and consider a proxy such as RDS Proxy if bursty connection churn is expected.",
                ))
            elif connections >= 500:
                if status == "PASS":
                    status = "WARN"
                findings.append(make_finding(
                    severity="MEDIUM",
                    message=f"RDS instance {db_id} has elevated connection count at {connections:.0f} (last 1h average).",
                    recommendation=f"Confirm the connection level is expected and review whether pooling can be improved for {db_id}.",
                    cost_impact="None for the check.",
                    why="High connection counts increase memory overhead and often signal inefficient application connection management.",
                    how="Review app connection pool configuration and database session inventory during peak traffic.",
                ))

        read_latency = latest_metric_stat(profile, region, "AWS/RDS", "ReadLatency", dims, 1, "Average")
        write_latency = latest_metric_stat(profile, region, "AWS/RDS", "WriteLatency", dims, 1, "Average")
        if read_latency is not None:
            evidence.append(f"rds={db_id} readLatencyMs={read_latency * 1000:.1f}")
        if write_latency is not None:
            evidence.append(f"rds={db_id} writeLatencyMs={write_latency * 1000:.1f}")
        peak_latency = max(v for v in [read_latency or 0.0, write_latency or 0.0])
        if peak_latency >= 0.1:
            status = "FAIL"
            findings.append(make_finding(
                severity="HIGH",
                message=f"RDS instance {db_id} has high storage latency with peak average read/write latency of {peak_latency * 1000:.1f} ms.",
                recommendation=f"Investigate storage pressure, slow queries, and instance sizing for {db_id}.",
                cost_impact="None for the check. High latency causes request slowdowns and timeout cascades.",
                why="Sustained RDS read/write latency usually indicates storage bottlenecks, saturation, or inefficient query patterns.",
                how="Correlate latency with slow queries, CPU, queue depth, and recent deploys. Scale storage or the instance only after query-level causes are understood.",
            ))
        elif peak_latency >= 0.02:
            if status == "PASS":
                status = "WARN"
            findings.append(make_finding(
                severity="MEDIUM",
                message=f"RDS instance {db_id} storage latency is elevated at {peak_latency * 1000:.1f} ms average.",
                recommendation=f"Review query load and storage pressure trends for {db_id}.",
                cost_impact="None for the check.",
                why="Latency trending upward often precedes application-visible slowdowns, especially under burst traffic.",
                how="Inspect Performance Insights waits and compare with read/write throughput and queue depth metrics.",
            ))

        queue_depth = latest_metric_stat(profile, region, "AWS/RDS", "DiskQueueDepth", dims, 1, "Average")
        if queue_depth is not None:
            evidence.append(f"rds={db_id} diskQueueDepth={queue_depth:.1f}")
            if queue_depth >= 64:
                status = "FAIL"
                findings.append(make_finding(
                    severity="HIGH",
                    message=f"RDS instance {db_id} has severe disk queue depth at {queue_depth:.1f}.",
                    recommendation=f"Investigate storage saturation and query pressure on {db_id} immediately.",
                    cost_impact="None for the check.",
                    why="High disk queue depth means IO requests are piling up faster than storage can service them, increasing latency for all queries.",
                    how="Correlate with read/write latency, storage throughput, and recent batch jobs. Reduce IO-heavy workloads or scale storage throughput.",
                ))
            elif queue_depth >= 16:
                if status == "PASS":
                    status = "WARN"
                findings.append(make_finding(
                    severity="MEDIUM",
                    message=f"RDS instance {db_id} has elevated disk queue depth at {queue_depth:.1f}.",
                    recommendation=f"Monitor IO saturation trends on {db_id} and review slow query patterns.",
                    cost_impact="None for the check.",
                    why="Elevated queue depth signals storage contention that can become application-visible under peak load.",
                    how="Review top IO-heavy queries, maintenance jobs, and whether storage throughput is appropriately sized.",
                ))

    return make_check("rds-deep-metrics", "RDS memory, connections, and latency", status, findings, evidence)


# ---------------------------------------------------------------------------
# Check 6: ASG instance refresh and scaling activity failures
# ---------------------------------------------------------------------------

def check_asg_health(profile: str, region: str) -> dict[str, Any]:
    asgs_data = run_aws_json_safe(profile, region, ["autoscaling", "describe-auto-scaling-groups"])
    if asgs_data is None:
        return make_check(
            "asg-health", "Auto Scaling Group health",
            "ERROR", [], ["api_error=true"],
        )

    findings: list[dict[str, Any]] = []
    evidence: list[str] = []
    status = "PASS"
    cutoff = utc_now() - dt.timedelta(days=7)

    for asg in asgs_data.get("AutoScalingGroups", []):
        name = asg.get("AutoScalingGroupName", "unknown")
        desired = asg.get("DesiredCapacity", 0)
        min_size = asg.get("MinSize", 0)
        max_size = asg.get("MaxSize", 0)
        instances = asg.get("Instances", [])
        in_service = [i for i in instances if i.get("LifecycleState") == "InService"]
        unhealthy_instances = [i for i in instances if i.get("HealthStatus") == "Unhealthy"]

        evidence.append(
            f"asg={name} desired={desired} inService={len(in_service)}/{len(instances)} "
            f"min={min_size} max={max_size} unhealthy={len(unhealthy_instances)}"
        )

        if len(in_service) < desired:
            severity = "HIGH" if len(in_service) == 0 else "MEDIUM"
            status = "FAIL" if severity == "HIGH" else ("WARN" if status == "PASS" else status)
            findings.append(make_finding(
                severity=severity,
                message=f"ASG {name} has {len(in_service)} InService instances vs desired {desired}.",
                recommendation=f"Investigate ASG {name} scaling activities and instance launch errors.",
                cost_impact="None for the check. Reduced capacity directly impacts application availability and latency.",
                why="An ASG with fewer InService instances than desired cannot serve full traffic, causing degraded performance or outages.",
                how="(1) aws autoscaling describe-scaling-activities --auto-scaling-group-name {name} to find launch failures. (2) Check launch template, AMI availability, and subnet capacity. (3) Review instance health check failures.",
            ))

        if unhealthy_instances:
            if status == "PASS":
                status = "WARN"
            ids = [i.get("InstanceId", "?") for i in unhealthy_instances]
            findings.append(make_finding(
                severity="MEDIUM",
                message=f"ASG {name} has {len(unhealthy_instances)} unhealthy instance(s): {', '.join(ids)}.",
                recommendation=f"Investigate why instances {', '.join(ids)} are failing health checks in ASG {name}.",
                cost_impact="None for the check. Unhealthy instances will be replaced, possibly incurring brief capacity reduction.",
                why="Unhealthy instances indicate failing health checks; the ASG will terminate and replace them, causing transient capacity fluctuation.",
                how="(1) Check EC2 instance system logs. (2) Verify the health check port/path is responding. (3) Review recent AMI or launch template changes.",
            ))

        # Check for recent failed instance refreshes
        refreshes = run_aws_json_safe(profile, region, [
            "autoscaling", "describe-instance-refreshes",
            "--auto-scaling-group-name", name,
        ])
        if refreshes:
            for refresh in refreshes.get("InstanceRefreshes", []):
                refresh_status = refresh.get("Status", "")
                start_time_str = refresh.get("StartTime", "")
                if refresh_status in ("Failed", "Cancelled") and start_time_str:
                    try:
                        start_time = dt.datetime.fromisoformat(start_time_str.replace("Z", "+00:00"))
                        if start_time > cutoff:
                            evidence.append(f"asg={name} instance_refresh status={refresh_status} started={start_time_str}")
                            if status == "PASS":
                                status = "WARN"
                            findings.append(make_finding(
                                severity="MEDIUM",
                                message=f"ASG {name} had a {refresh_status} instance refresh started {start_time_str}.",
                                recommendation=f"Review the failed instance refresh for ASG {name} and retry after resolving the root cause.",
                                cost_impact="Low. Failed refreshes may have partially launched/terminated instances that consumed instance-hours.",
                                why="Failed instance refreshes indicate the new AMI or configuration failed to pass health checks, leaving the fleet on the old config.",
                                how="(1) aws autoscaling describe-instance-refreshes --auto-scaling-group-name {name} for the failure reason. (2) Fix the launch template or AMI issue. (3) Retry the refresh.",
                            ))
                    except ValueError:
                        pass

    return make_check("asg-health", "Auto Scaling Group health", status, findings, evidence)


# ---------------------------------------------------------------------------
# Check 7: EC2 stopped instances (potential waste and config drift)
# ---------------------------------------------------------------------------

def check_ec2_stopped_instances(profile: str, region: str) -> dict[str, Any]:
    data = run_aws_json_safe(profile, region, [
        "ec2", "describe-instances",
        "--filters", "Name=instance-state-name,Values=stopped",
    ])
    if data is None:
        return make_check(
            "ec2-stopped", "EC2 stopped instances",
            "ERROR", [], ["api_error=true"],
        )

    stopped: list[str] = []
    evidence: list[str] = []

    for reservation in data.get("Reservations", []):
        for instance in reservation.get("Instances", []):
            iid = instance.get("InstanceId", "?")
            itype = instance.get("InstanceType", "?")
            name_tag = next(
                (t["Value"] for t in instance.get("Tags", []) if t["Key"] == "Name"), ""
            )
            launch_time = instance.get("LaunchTime", "")
            stopped.append(f"{iid}({name_tag or itype})")
            evidence.append(f"stopped_instance={iid} type={itype} name={name_tag} launched={launch_time}")

    if not stopped:
        return make_check("ec2-stopped", "EC2 stopped instances", "PASS", [], ["no_stopped_instances=true"])

    return make_check(
        "ec2-stopped", "EC2 stopped instances",
        "WARN",
        [make_finding(
            severity="LOW",
            message=f"{len(stopped)} EC2 instance(s) are stopped: {', '.join(stopped[:10])}{'...' if len(stopped) > 10 else ''}.",
            recommendation="Terminate stopped instances that are no longer needed, or document why they must remain stopped.",
            cost_impact="Low but non-zero. Stopped instances still accrue EBS storage charges ($0.08–$0.10/GB-month) and Elastic IP charges if attached.",
            why="Stopped instances accumulate EBS and Elastic IP charges and represent unmanaged configuration drift risk if forgotten.",
            how="(1) Confirm with the instance owner whether each stopped instance is still required. (2) If not needed: aws ec2 terminate-instances --instance-ids <id>. (3) If needed: document the reason and expected restart date.",
        )],
        evidence,
    )


# ---------------------------------------------------------------------------
# Check 7b: EC2 running instance health and scheduled events
# ---------------------------------------------------------------------------

def check_ec2_runtime_health(profile: str, region: str) -> dict[str, Any]:
    instances_data = run_aws_json_safe(profile, region, ["ec2", "describe-instances"])
    status_data = run_aws_json_safe(profile, region, ["ec2", "describe-instance-status", "--include-all-instances"])
    if instances_data is None or status_data is None:
        return make_check(
            "ec2-runtime-health", "EC2 status checks and scheduled events",
            "ERROR", [], ["api_error=true"],
        )

    states: dict[str, str] = {}
    names: dict[str, str] = {}
    for reservation in instances_data.get("Reservations", []):
        for instance in reservation.get("Instances", []):
            iid = instance.get("InstanceId", "?")
            states[iid] = instance.get("State", {}).get("Name", "unknown")
            names[iid] = next((t["Value"] for t in instance.get("Tags", []) if t["Key"] == "Name"), "")

    findings: list[dict[str, Any]] = []
    evidence: list[str] = []
    status = "PASS"

    for item in status_data.get("InstanceStatuses", []):
        iid = item.get("InstanceId", "?")
        if states.get(iid) != "running":
            continue
        instance_status = item.get("InstanceStatus", {}).get("Status", "unknown")
        system_status = item.get("SystemStatus", {}).get("Status", "unknown")
        ebs_status = item.get("AttachedEbsStatus", {}).get("Status", "n/a")
        label = f"{iid}({names.get(iid) or 'unnamed'})"
        evidence.append(f"instance={label} instanceStatus={instance_status} systemStatus={system_status} ebsStatus={ebs_status}")

        if instance_status != "ok" or system_status != "ok" or ebs_status not in {"ok", "not-applicable", "initializing", "insufficient-data", "n/a"}:
            severity = "HIGH" if system_status != "ok" else "MEDIUM"
            status = "FAIL" if severity == "HIGH" else ("WARN" if status == "PASS" else status)
            findings.append(make_finding(
                severity=severity,
                message=f"EC2 instance {label} has impaired status checks: instance={instance_status}, system={system_status}, ebs={ebs_status}.",
                recommendation=f"Investigate EC2 instance {iid} and review console output, underlying host health, and attached storage state.",
                cost_impact="None for the check. Impaired status checks often correlate directly with workload instability or downtime.",
                why="EC2 status checks are AWS’s primary signal for host-level and guest-level impairment that can break application traffic before alarms are tuned correctly.",
                how="Open EC2 > Instances > Status checks for the instance, inspect the failure reason, and remediate guest OS issues or stop/start the instance if host recovery is needed.",
            ))

        for event in item.get("Events", []):
            code = event.get("Code", "unknown")
            not_before = event.get("NotBefore", "")
            evidence.append(f"scheduled_event instance={label} code={code} notBefore={not_before}")
            if status == "PASS":
                status = "WARN"
            findings.append(make_finding(
                severity="MEDIUM",
                message=f"EC2 instance {label} has a scheduled AWS event: {code} at {not_before or 'unknown time'}.",
                recommendation=f"Review the scheduled event for {iid} and prepare maintenance or failover before AWS’s deadline.",
                cost_impact="None for the check.",
                why="AWS scheduled events often precede host retirement or reboot and should be planned instead of discovered during an outage.",
                how="Open the instance in the EC2 console, review the scheduled event details, and reschedule or replace the instance if needed.",
            ))

    if not evidence:
        evidence.append("no_running_instances_or_statuses=true")
    return make_check("ec2-runtime-health", "EC2 status checks and scheduled events", status, findings, evidence)


# ---------------------------------------------------------------------------
# Check 8: App Runner service health
# ---------------------------------------------------------------------------

def check_apprunner_health(profile: str, region: str) -> dict[str, Any]:
    data = run_aws_json_safe(profile, region, ["apprunner", "list-services"])
    if data is None:
        return make_check(
            "apprunner-health", "App Runner service health",
            "ERROR", [], ["api_error=true"],
        )

    services = data.get("ServiceSummaryList", [])
    if not services:
        return make_check(
            "apprunner-health", "App Runner service health",
            "PASS", [], ["no_apprunner_services=true"],
        )

    degraded: list[str] = []
    evidence: list[str] = []
    status = "PASS"

    for svc in services:
        svc_name = svc.get("ServiceName", "unknown")
        svc_status = svc.get("Status", "")
        svc_url = svc.get("ServiceUrl", "")
        evidence.append(f"apprunner={svc_name} status={svc_status} url={svc_url}")

        if svc_status not in ("RUNNING", "OPERATION_IN_PROGRESS"):
            status = "FAIL"
            degraded.append(f"{svc_name}({svc_status})")

    if degraded:
        findings = [make_finding(
            severity="HIGH",
            message=f"App Runner service(s) are not in RUNNING state: {', '.join(degraded)}.",
            recommendation="Investigate each degraded App Runner service in the console and review deployment logs.",
            cost_impact="None for the check. A non-running App Runner service means the application is unavailable.",
            why="App Runner services in a non-RUNNING state are not serving traffic, resulting in application downtime.",
            how="(1) Open App Runner console > select the degraded service. (2) Review the latest deployment logs under 'Deployment logs'. (3) Fix the build or runtime error and redeploy.",
        )]
        return make_check("apprunner-health", "App Runner service health", status, findings, evidence)

    return make_check("apprunner-health", "App Runner service health", "PASS", [], evidence)


# ---------------------------------------------------------------------------
# Check 9: ElastiCache cluster health and eviction rate
# ---------------------------------------------------------------------------

def _get_elasticache_metric(profile: str, region: str, cluster_id: str, metric_name: str) -> float | None:
    end = utc_now()
    start = end - dt.timedelta(hours=1)
    data = run_aws_json_safe(profile, region, [
        "cloudwatch", "get-metric-statistics",
        "--namespace", "AWS/ElastiCache",
        "--metric-name", metric_name,
        "--dimensions", f"Name=CacheClusterId,Value={cluster_id}",
        "--start-time", start.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "--end-time", end.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "--period", "3600",
        "--statistics", "Average",
    ])
    if data is None:
        return None
    points = data.get("Datapoints", [])
    if not points:
        return None
    return float(points[-1].get("Average", 0.0))


def check_elasticache_health(profile: str, region: str) -> dict[str, Any]:
    data = run_aws_json_safe(profile, region, ["elasticache", "describe-cache-clusters", "--show-cache-node-info"])
    if data is None:
        return make_check(
            "elasticache-health", "ElastiCache cluster health",
            "ERROR", [], ["api_error=true"],
        )

    clusters = data.get("CacheClusters", [])
    if not clusters:
        return make_check(
            "elasticache-health", "ElastiCache cluster health",
            "PASS", [], ["no_elasticache_clusters=true"],
        )

    findings: list[dict[str, Any]] = []
    evidence: list[str] = []
    status = "PASS"

    for cluster in clusters:
        cluster_id = cluster.get("CacheClusterId", "unknown")
        cluster_status = cluster.get("CacheClusterStatus", "")
        engine = cluster.get("Engine", "")
        engine_version = cluster.get("EngineVersion", "")
        node_type = cluster.get("CacheNodeType", "")

        evidence.append(f"cluster={cluster_id} status={cluster_status} engine={engine} {engine_version} type={node_type}")

        if cluster_status != "available":
            status = "FAIL"
            findings.append(make_finding(
                severity="HIGH",
                message=f"ElastiCache cluster {cluster_id} is in status '{cluster_status}' (expected 'available').",
                recommendation=f"Investigate ElastiCache cluster {cluster_id} in the console.",
                cost_impact="None for the check. A non-available cluster means cache is unavailable, increasing backend load.",
                why="An unavailable cache cluster means all cache reads fall through to the database, multiplying DB load and latency.",
                how="(1) Open ElastiCache console > {cluster_id} > Events. (2) Check for maintenance, failover, or node replacement events. (3) Contact AWS Support if status is stuck.",
            ))
            continue

        # Eviction rate
        evictions = _get_elasticache_metric(profile, region, cluster_id, "Evictions")
        if evictions is not None:
            evidence.append(f"cluster={cluster_id} evictions_per_min={evictions:.1f}")
            if evictions > 1000:
                if status == "PASS":
                    status = "WARN"
                findings.append(make_finding(
                    severity="MEDIUM",
                    message=f"ElastiCache cluster {cluster_id} has high eviction rate: {evictions:.0f}/min (last 1h avg).",
                    recommendation=f"Increase cluster memory or reduce TTLs on {cluster_id} to prevent cache churn.",
                    cost_impact="Medium. Scaling to a larger node type increases ElastiCache cost; exact amount depends on node type selected.",
                    why="High eviction rates mean the cache is full and discarding useful data, reducing cache effectiveness and increasing DB load.",
                    how="(1) Check CloudWatch CurrItems and BytesUsedForCache to confirm memory pressure. (2) Scale up node type or add shards/replicas. (3) Review application TTL settings and evict large/rarely-used keys first.",
                ))

        # Cache hit rate
        hits = _get_elasticache_metric(profile, region, cluster_id, "CacheHits")
        misses = _get_elasticache_metric(profile, region, cluster_id, "CacheMisses")
        if hits is not None and misses is not None:
            total = hits + misses
            if total > 0:
                hit_rate = (hits / total) * 100
                evidence.append(f"cluster={cluster_id} cacheHitRate={hit_rate:.1f}%")
                if hit_rate < 50:
                    if status == "PASS":
                        status = "WARN"
                    findings.append(make_finding(
                        severity="MEDIUM",
                        message=f"ElastiCache cluster {cluster_id} cache hit rate is {hit_rate:.1f}% (last 1h). Low hit rate means high cache miss load on DB.",
                        recommendation=f"Review caching strategy for {cluster_id}: check TTLs, key patterns, and whether the right data is being cached.",
                        cost_impact="None for the check. Low hit rate means more DB queries, increasing RDS compute/IO cost.",
                        why="A hit rate below 50% means the cache is not serving its purpose — most reads still reach the database, wasting cache resources.",
                        how="(1) Use Redis CLI MONITOR (carefully in low-traffic window) or Keyspace Statistics to find miss-prone key patterns. (2) Extend TTLs for stable data. (3) Ensure cache warming on application startup if applicable.",
                    ))

    return make_check("elasticache-health", "ElastiCache cluster health", status, findings, evidence)


# ---------------------------------------------------------------------------
# Check 10: CloudWatch alarms with no actions configured
# ---------------------------------------------------------------------------

def check_alarms_no_actions(profile: str, region: str) -> dict[str, Any]:
    data = run_aws_json_safe(profile, region, ["cloudwatch", "describe-alarms", "--alarm-types", "MetricAlarm"])
    if data is None:
        return make_check(
            "alarms-no-actions", "CloudWatch alarms without actions",
            "ERROR", [], ["api_error=true"],
        )

    no_action_alarms: list[str] = []
    evidence: list[str] = []

    for alarm in data.get("MetricAlarms", []):
        name = alarm.get("AlarmName", "unknown")
        alarm_actions = alarm.get("AlarmActions", [])
        ok_actions = alarm.get("OKActions", [])
        insufficient_actions = alarm.get("InsufficientDataActions", [])
        has_any = bool(alarm_actions or ok_actions or insufficient_actions)
        evidence.append(f"alarm={name} hasActions={has_any}")
        if not has_any:
            no_action_alarms.append(name)

    if not no_action_alarms:
        return make_check("alarms-no-actions", "CloudWatch alarms without actions", "PASS", [], evidence[:20])

    return make_check(
        "alarms-no-actions", "CloudWatch alarms without actions",
        "WARN",
        [make_finding(
            severity="LOW",
            message=f"{len(no_action_alarms)} CloudWatch alarm(s) have no actions configured (no SNS/Lambda/Auto Scaling): {', '.join(no_action_alarms[:10])}{'...' if len(no_action_alarms) > 10 else ''}.",
            recommendation="Add SNS notification actions to all alarms so on-call teams are paged when thresholds are breached.",
            cost_impact="Low. SNS charges $0.50/million API requests and $0.00002/notification (email), effectively free at typical alarm volumes.",
            why="Alarms without actions fire silently — they will appear in ALARM state but no team member is notified.",
            how="(1) Create or reuse an SNS topic for alarm notifications. (2) Subscribe the on-call email/PagerDuty endpoint. (3) aws cloudwatch put-metric-alarm ... --alarm-actions arn:aws:sns:<region>:<account>:<topic> for each alarm.",
        )],
        evidence[:20],
    )


# ---------------------------------------------------------------------------
# Check 11: Billing — month-to-date spend vs prior month + top services
# ---------------------------------------------------------------------------

def check_billing_mtd_spend(profile: str, region: str) -> dict[str, Any]:
    """Compare MTD spend to the same period last month and surface top services."""
    today = utc_now().date()
    first_of_month = today.replace(day=1)
    # Same period start last month
    prior_month_last_day = first_of_month - dt.timedelta(days=1)
    last_month_same_start = prior_month_last_day.replace(day=1)
    last_month_same_end = last_month_same_start.replace(
        day=min(today.day, prior_month_last_day.day)
    )

    fmt = "%Y-%m-%d"

    def _get_spend(start: dt.date, end: dt.date) -> float | None:
        data = run_aws_json_safe(profile, region, [
            "ce", "get-cost-and-usage",
            "--time-period", f"Start={start.strftime(fmt)},End={end.strftime(fmt)}",
            "--granularity", "MONTHLY",
            "--metrics", "UnblendedCost",
        ])
        if data is None:
            return None
        results = data.get("ResultsByTime", [])
        if not results:
            return None
        try:
            return float(results[0]["Total"]["UnblendedCost"]["Amount"])
        except (KeyError, IndexError, ValueError):
            return None

    def _get_top_services(start: dt.date, end: dt.date, top_n: int = 6) -> list[tuple[str, float]]:
        data = run_aws_json_safe(profile, region, [
            "ce", "get-cost-and-usage",
            "--time-period", f"Start={start.strftime(fmt)},End={end.strftime(fmt)}",
            "--granularity", "MONTHLY",
            "--metrics", "UnblendedCost",
            "--group-by", "Type=DIMENSION,Key=SERVICE",
        ])
        if data is None:
            return []
        results = data.get("ResultsByTime", [])
        if not results:
            return []
        groups = results[0].get("Groups", [])
        services: list[tuple[str, float]] = []
        for g in groups:
            try:
                svc = g["Keys"][0]
                amount = float(g["Metrics"]["UnblendedCost"]["Amount"])
                services.append((svc, amount))
            except (KeyError, IndexError, ValueError):
                continue
        services.sort(key=lambda x: x[1], reverse=True)
        return services[:top_n]

    findings: list[dict[str, Any]] = []
    evidence: list[str] = []
    status = "PASS"

    mtd = _get_spend(first_of_month, today)
    prior = _get_spend(last_month_same_start, last_month_same_end)

    if mtd is None:
        return make_check(
            "billing-mtd-spend", "Billing: month-to-date spend",
            "ERROR",
            [make_finding("INFO", f"Could not retrieve Cost Explorer data. Ensure Cost Explorer is enabled and the AWS profile {profile} has ce:GetCostAndUsage permission.", "", "None")],
            ["ce_api_error=true"],
        )

    evidence.append(f"mtd_spend=${mtd:.2f} period={first_of_month}..{today}")
    if prior is not None:
        evidence.append(f"prior_same_period_spend=${prior:.2f} period={last_month_same_start}..{last_month_same_end}")
        if prior > 0:
            pct_change = ((mtd - prior) / prior) * 100
            evidence.append(f"spend_change={pct_change:+.1f}%")
            if pct_change >= 50:
                status = "FAIL"
                findings.append(make_finding(
                    severity="HIGH",
                    message=f"MTD spend ${mtd:.2f} is {pct_change:.0f}% higher than the same period last month (${prior:.2f}).",
                    recommendation="Investigate the top services driving the spike and identify unexpected resource growth.",
                    cost_impact=f"Direct: ${mtd - prior:.2f} incremental spend vs same period last month.",
                    why="A >50% MTD spend spike vs the same period last month suggests unexpected resource growth, misconfigured autoscaling, or a runaway workload.",
                    how="(1) Review top services below. (2) Check for new EC2/RDS/data transfer charges. (3) Look for unintended resource launches or data egress events in CloudTrail.",
                ))
            elif pct_change >= 20:
                status = "WARN"
                findings.append(make_finding(
                    severity="MEDIUM",
                    message=f"MTD spend ${mtd:.2f} is {pct_change:.0f}% higher than the same period last month (${prior:.2f}).",
                    recommendation="Review top service drivers and confirm the increase is expected (e.g., planned capacity addition).",
                    cost_impact=f"Direct: ${mtd - prior:.2f} incremental spend vs same period last month.",
                    why="A 20–50% spend increase may reflect legitimate growth or an unnoticed cost driver worth reviewing.",
                    how="(1) Compare top services MTD vs last month. (2) Confirm with engineering whether any new services or capacity was intentionally added. (3) Set a billing alert if one does not already exist.",
                ))

    # Top services
    top_services = _get_top_services(first_of_month, today)
    if top_services:
        svc_lines = [f"  ${amt:.2f}  {svc}" for svc, amt in top_services]
        evidence.append("top_services_mtd:\n" + "\n".join(svc_lines))

        # Flag any single service consuming >60% of MTD spend
        for svc, amt in top_services:
            if mtd > 0 and (amt / mtd) > 0.60:
                if status == "PASS":
                    status = "WARN"
                pct = (amt / mtd) * 100
                findings.append(make_finding(
                    severity="MEDIUM",
                    message=f"Service '{svc}' accounts for {pct:.0f}% of MTD spend (${amt:.2f} of ${mtd:.2f}).",
                    recommendation=f"Review {svc} resource usage and confirm all resources are intentionally provisioned.",
                    cost_impact=f"Direct: ${amt:.2f} MTD. Reducing idle or oversized resources in this service would have the highest cost impact.",
                    why="High concentration of spend in one service is a risk signal — any growth or misconfiguration in that service disproportionately impacts the bill.",
                    how=f"(1) Open AWS Cost Explorer > filter by Service={svc}. (2) Drill down by resource or usage type. (3) Identify idle, oversized, or unexpectedly growing resources.",
                ))
                break  # Only flag the top offender

    if not findings:
        findings_msg = f"MTD spend ${mtd:.2f}"
        if prior is not None:
            pct_change = ((mtd - prior) / prior * 100) if prior > 0 else 0
            findings_msg += f" ({pct_change:+.1f}% vs same period last month ${prior:.2f})"
        evidence.insert(0, findings_msg)

    return make_check("billing-mtd-spend", "Billing: month-to-date spend", status, findings, evidence)


# ---------------------------------------------------------------------------
# Check 12: Billing — Cost Anomaly Detection findings
# ---------------------------------------------------------------------------

def check_billing_cost_anomalies(profile: str, region: str) -> dict[str, Any]:
    """Check for active Cost Anomaly Detection monitors and recent anomalies."""
    # Check if any monitors exist
    monitors_data = run_aws_json_safe(profile, region, ["ce", "get-anomaly-monitors"])
    evidence: list[str] = []

    if monitors_data is None:
        return make_check(
            "billing-cost-anomalies", "Billing: cost anomaly detection",
            "WARN",
            [make_finding(
                severity="MEDIUM",
                message="Could not query Cost Anomaly Detection — API error or permission missing.",
                recommendation=f"Ensure the AWS profile {profile} has ce:GetAnomalyMonitors permission and Cost Explorer is enabled.",
                cost_impact="None for the check.",
                why="Cost Anomaly Detection proactively alerts on unexpected spend spikes, reducing the time to detect billing problems.",
                how=f"Grant ce:GetAnomalyMonitors and ce:GetAnomalies to the IAM principal behind profile {profile}.",
            )],
            ["ce_anomaly_api_error=true"],
        )

    monitors = monitors_data.get("AnomalyMonitors", [])
    evidence.append(f"anomaly_monitors_count={len(monitors)}")

    findings: list[dict[str, Any]] = []
    status = "PASS"

    if not monitors:
        status = "WARN"
        findings.append(make_finding(
            severity="MEDIUM",
            message="AWS Cost Anomaly Detection has no monitors configured.",
            recommendation="Create at least one Cost Anomaly Detection monitor covering total account spend.",
            cost_impact="Low. Cost Anomaly Detection itself is free; SNS alert delivery is negligible ($0.00002/email).",
            why="Without anomaly monitors, unexpected spend spikes (runaway resources, misconfigured autoscaling) go undetected until the monthly bill arrives.",
            how="(1) Open Cost Management > Cost Anomaly Detection > Create monitor. (2) Select 'AWS services' scope for account-wide coverage. (3) Add an alert subscription with your email or SNS topic. (4) Set a minimum impact threshold (e.g., $20 or 20%).",
        ))
        return make_check("billing-cost-anomalies", "Billing: cost anomaly detection", status, findings, evidence)

    # Check for recent anomalies (last 14 days)
    end_date = utc_now().date()
    start_date = end_date - dt.timedelta(days=14)
    anomalies_data = run_aws_json_safe(profile, region, [
        "ce", "get-anomalies",
        "--date-interval", f"StartDate={start_date.strftime('%Y-%m-%d')},EndDate={end_date.strftime('%Y-%m-%d')}",
        "--max-results", "10",
    ])

    if anomalies_data is None:
        evidence.append("anomalies_api_error=true")
        return make_check("billing-cost-anomalies", "Billing: cost anomaly detection", status, findings, evidence)

    anomalies = anomalies_data.get("Anomalies", [])
    evidence.append(f"anomalies_last_14d={len(anomalies)}")

    open_anomalies = [a for a in anomalies if a.get("AnomalyEndDate") is None]
    resolved_anomalies = [a for a in anomalies if a.get("AnomalyEndDate") is not None]
    evidence.append(f"open_anomalies={len(open_anomalies)} resolved={len(resolved_anomalies)}")

    for anomaly in open_anomalies:
        impact = anomaly.get("Impact", {})
        total_impact = float(impact.get("TotalImpact", 0) or 0)
        max_impact = float(impact.get("MaxImpact", 0) or 0)
        start = anomaly.get("AnomalyStartDate", "unknown")
        root_causes = anomaly.get("RootCauses", [])
        root_desc = "; ".join(
            f"{rc.get('Service', '?')}/{rc.get('UsageType', '?')}"
            for rc in root_causes[:3]
        ) or "unknown"
        evidence.append(f"open_anomaly start={start} totalImpact=${total_impact:.2f} maxImpact=${max_impact:.2f} root={root_desc}")

        severity = "HIGH" if total_impact >= 100 else "MEDIUM"
        status = "FAIL" if severity == "HIGH" else ("WARN" if status == "PASS" else status)
        findings.append(make_finding(
            severity=severity,
            message=f"Open cost anomaly detected since {start}: total impact ${total_impact:.2f}, max daily impact ${max_impact:.2f}. Root causes: {root_desc}.",
            recommendation="Investigate the anomaly root causes and terminate or resize any unexpectedly running resources.",
            cost_impact=f"Direct: ${total_impact:.2f} estimated excess spend attributed to this anomaly.",
            why="An open anomaly means the unexpected spend is still ongoing and will continue to accrue until resolved.",
            how="(1) Open Cost Management > Cost Anomaly Detection > view the anomaly. (2) Click 'Explore usage' to drill into the root cause service. (3) Identify and terminate/resize the offending resources. (4) Mark the anomaly as resolved once addressed.",
        ))

    for anomaly in resolved_anomalies[:3]:
        impact = anomaly.get("Impact", {})
        total_impact = float(impact.get("TotalImpact", 0) or 0)
        start = anomaly.get("AnomalyStartDate", "unknown")
        end = anomaly.get("AnomalyEndDate", "unknown")
        evidence.append(f"resolved_anomaly start={start} end={end} totalImpact=${total_impact:.2f}")

    return make_check("billing-cost-anomalies", "Billing: cost anomaly detection", status, findings, evidence)


# ---------------------------------------------------------------------------
# Check 13: Billing — idle resource waste (unattached EBS + unassociated EIPs)
# ---------------------------------------------------------------------------

def check_billing_idle_resources(profile: str, region: str) -> dict[str, Any]:
    """Find unattached EBS volumes and unassociated Elastic IPs that are billing with no purpose."""
    findings: list[dict[str, Any]] = []
    evidence: list[str] = []
    status = "PASS"

    # Unattached EBS volumes (state=available means not attached to any instance)
    ebs_data = run_aws_json_safe(profile, region, [
        "ec2", "describe-volumes",
        "--filters", "Name=status,Values=available",
    ])

    if ebs_data is not None:
        volumes = ebs_data.get("Volumes", [])
        total_gb = sum(v.get("Size", 0) for v in volumes)
        evidence.append(f"unattached_ebs_volumes={len(volumes)} total_gb={total_gb}")

        for v in volumes:
            vid = v.get("VolumeId", "?")
            size = v.get("Size", 0)
            vtype = v.get("VolumeType", "?")
            create_time = str(v.get("CreateTime", ""))[:10]
            name_tag = next((t["Value"] for t in v.get("Tags", []) if t["Key"] == "Name"), "")
            evidence.append(f"unattached_ebs={vid} size={size}GB type={vtype} created={create_time} name={name_tag}")

        if volumes:
            # Estimate cost: gp2=$0.10/GB-mo, gp3=$0.08/GB-mo, io1=$0.125/GB-mo
            # Use $0.10/GB as a conservative average for a mixed fleet
            estimated_monthly = total_gb * 0.10
            status = "WARN"
            severity = "HIGH" if total_gb >= 500 else "MEDIUM"
            findings.append(make_finding(
                severity=severity,
                message=f"{len(volumes)} unattached EBS volume(s) totaling {total_gb} GB are billing with no workload attached (~${estimated_monthly:.0f}/month).",
                recommendation="Delete unneeded volumes or snapshot-then-delete if data may be needed later.",
                cost_impact=f"Direct: ~${estimated_monthly:.0f}/month at $0.10/GB-month for {total_gb} GB. Savings are immediate upon deletion.",
                why="Unattached EBS volumes continue to accrue storage charges at the same rate as attached volumes, with zero value delivered.",
                how="(1) List volumes: aws ec2 describe-volumes --filters Name=status,Values=available. (2) Confirm with the owner that the volume data is not needed. (3) If safe: aws ec2 delete-volume --volume-id <id>. (4) If uncertain: aws ec2 create-snapshot --volume-id <id> first, then delete.",
            ))
    else:
        evidence.append("ebs_api_error=true")

    # Unassociated Elastic IPs (no AssociationId = not linked to a running instance)
    eip_data = run_aws_json_safe(profile, region, ["ec2", "describe-addresses"])

    if eip_data is not None:
        addresses = eip_data.get("Addresses", [])
        unassociated = [a for a in addresses if not a.get("AssociationId")]
        evidence.append(f"total_eips={len(addresses)} unassociated={len(unassociated)}")

        for eip in unassociated:
            pub_ip = eip.get("PublicIp", "?")
            alloc_id = eip.get("AllocationId", "?")
            name_tag = next((t["Value"] for t in eip.get("Tags", []) if t["Key"] == "Name"), "")
            evidence.append(f"unassociated_eip={pub_ip} alloc={alloc_id} name={name_tag}")

        if unassociated:
            # $0.005/hr per unassociated EIP = ~$3.60/month each
            estimated_monthly = len(unassociated) * 3.60
            if status == "PASS":
                status = "WARN"
            findings.append(make_finding(
                severity="LOW",
                message=f"{len(unassociated)} Elastic IP(s) are unassociated with any running instance and accruing idle charges (~${estimated_monthly:.0f}/month).",
                recommendation="Release Elastic IPs that are no longer needed.",
                cost_impact=f"Direct: ~${estimated_monthly:.0f}/month ($0.005/hr × {len(unassociated)} EIPs). Savings are immediate upon release.",
                why="AWS charges $0.005/hr for every Elastic IP not associated with a running instance. These are pure waste if not reserved intentionally.",
                how="(1) Confirm no future use for the IP. (2) aws ec2 release-address --allocation-id <alloc_id>. Note: releasing an EIP permanently removes it — you cannot reclaim the same IP.",
            ))
    else:
        evidence.append("eip_api_error=true")

    return make_check("billing-idle-resources", "Billing: idle resource waste (EBS + EIPs)", status, findings, evidence)


# ---------------------------------------------------------------------------
# Check 14: Backup coverage and restore readiness
# ---------------------------------------------------------------------------

def check_backup_restore_readiness(profile: str, region: str) -> dict[str, Any]:
    plans_data = run_aws_json_safe(profile, region, ["backup", "list-backup-plans"])
    protected_data = run_aws_json_safe(profile, region, ["backup", "list-protected-resources"])
    backup_jobs_data = run_aws_json_safe(
        profile,
        region,
        ["backup", "list-backup-jobs", "--by-created-after", iso_z(utc_now() - dt.timedelta(days=7))],
    )
    restore_jobs_data = run_aws_json_safe(
        profile,
        region,
        ["backup", "list-restore-jobs", "--by-created-after", iso_z(utc_now() - dt.timedelta(days=90))],
    )
    rds_data = run_aws_json_safe(profile, region, ["rds", "describe-db-instances"])

    if plans_data is None or protected_data is None or backup_jobs_data is None or restore_jobs_data is None or rds_data is None:
        return make_check(
            "backup-restore-readiness", "Backup coverage and restore readiness",
            "ERROR", [], ["api_error=true"],
        )

    findings: list[dict[str, Any]] = []
    evidence: list[str] = []
    status = "PASS"

    plans = plans_data.get("BackupPlansList", [])
    protected = protected_data.get("Results", [])
    backup_jobs = backup_jobs_data.get("BackupJobs", [])
    restore_jobs = restore_jobs_data.get("RestoreJobs", [])

    evidence.append(f"backup_plans={len(plans)} protected_resources={len(protected)} backup_jobs_last_7d={len(backup_jobs)} restore_jobs_last_90d={len(restore_jobs)}")

    protected_arns = {item.get("ResourceArn", "") for item in protected}
    for db in rds_data.get("DBInstances", []):
        db_id = db.get("DBInstanceIdentifier", "unknown")
        db_arn = db.get("DBInstanceArn", "")
        if db_arn:
            evidence.append(f"rds_backup_coverage db={db_id} protected={db_arn in protected_arns}")
        if db_arn and db_arn not in protected_arns:
            status = "FAIL"
            findings.append(make_finding(
                severity="HIGH",
                message=f"RDS instance {db_id} is not covered by AWS Backup protected resources.",
                recommendation=f"Add {db_id} to an AWS Backup plan or document an equivalent tested backup strategy outside AWS Backup.",
                cost_impact="Low to medium. Backup storage costs money, but lack of recoverability is a far larger operational risk.",
                why="Having backups configured in the service is not enough; centralized backup coverage improves visibility, retention management, and restore operations.",
                how="Create or update an AWS Backup plan and assign the DB resource by ARN or tag policy. Confirm recovery points are created successfully.",
            ))

    failed_jobs = [job for job in backup_jobs if job.get("State") == "FAILED"]
    if failed_jobs:
        severity = "HIGH" if len(failed_jobs) >= 3 else "MEDIUM"
        status = "FAIL" if severity == "HIGH" else ("WARN" if status == "PASS" else status)
        evidence.extend(
            f"failed_backup_job resource={job.get('ResourceArn', '?')} created={job.get('CreationDate', '')}"
            for job in failed_jobs[:10]
        )
        findings.append(make_finding(
            severity=severity,
            message=f"{len(failed_jobs)} AWS Backup job(s) failed in the last 7 days.",
            recommendation="Review failed AWS Backup jobs and fix IAM, KMS, or resource-state issues blocking successful backups.",
            cost_impact="None for the check.",
            why="Failed backups invalidate recovery assumptions and are usually only discovered when a restore is urgently needed.",
            how="Open AWS Backup > Jobs, inspect each failed job, and correct the IAM, KMS, retention, or resource state issue before the next backup window.",
        ))

    completed_restore_jobs = [job for job in restore_jobs if job.get("Status") == "COMPLETED"]
    if not completed_restore_jobs:
        if status == "PASS":
            status = "WARN"
        findings.append(make_finding(
            severity="MEDIUM",
            message="No completed AWS Backup restore jobs were found in the last 90 days.",
            recommendation="Run and document periodic restore tests for critical backups.",
            cost_impact="Low. Restore testing incurs temporary resource and storage costs but prevents far more expensive recovery surprises.",
            why="Backup best practice is not just taking backups but verifying they can be restored within your recovery objectives.",
            how="Run a restore test from a recent recovery point for at least one critical database or workload and record the result in an operational runbook.",
        ))
    else:
        latest_restore = max((parse_aws_datetime(job.get("CreationDate")) for job in completed_restore_jobs), default=None)
        if latest_restore is not None:
            evidence.append(f"latest_completed_restore_test={latest_restore.strftime('%Y-%m-%d')}")

    return make_check("backup-restore-readiness", "Backup coverage and restore readiness", status, findings, evidence)


# ---------------------------------------------------------------------------
# Check 15: Alarm coverage for critical resources
# ---------------------------------------------------------------------------

def check_alarm_coverage(profile: str, region: str) -> dict[str, Any]:
    alarms_data = run_aws_json_safe(profile, region, ["cloudwatch", "describe-alarms", "--alarm-types", "MetricAlarm"])
    lbs_data = run_aws_json_safe(profile, region, ["elbv2", "describe-load-balancers"])
    rds_data = run_aws_json_safe(profile, region, ["rds", "describe-db-instances"])
    asgs_data = run_aws_json_safe(profile, region, ["autoscaling", "describe-auto-scaling-groups"])
    ec2_data = run_aws_json_safe(profile, region, ["ec2", "describe-instances"])
    if alarms_data is None or lbs_data is None or rds_data is None or asgs_data is None or ec2_data is None:
        return make_check("alarm-coverage", "Alarm coverage for critical resources", "ERROR", [], ["api_error=true"])

    alarms = [alarm for alarm in alarms_data.get("MetricAlarms", []) if not is_target_tracking_alarm_name(str(alarm.get("AlarmName", "")))]
    findings: list[dict[str, Any]] = []
    evidence: list[str] = []
    status = "PASS"

    def has_alarm(metric_names: set[str], dimension_name: str, dimension_value: str) -> bool:
        for alarm in alarms:
            if not metric_names.intersection(alarm_metric_names(alarm)):
                continue
            if dimension_value in alarm_dimension_values(alarm, dimension_name):
                return True
        return False

    for lb in lbs_data.get("LoadBalancers", []):
        if lb.get("Type") != "application":
            continue
        name = lb.get("LoadBalancerName", "unknown")
        dim = aws_resource_suffix(lb.get("LoadBalancerArn", ""), "loadbalancer/")
        missing: list[str] = []
        if not has_alarm({"HTTPCode_ELB_5XX_Count"}, "LoadBalancer", dim):
            missing.append("ELB 5XX")
        if not has_alarm({"HTTPCode_Target_5XX_Count"}, "LoadBalancer", dim):
            missing.append("Target 5XX")
        if not has_alarm({"TargetResponseTime"}, "LoadBalancer", dim):
            missing.append("TargetResponseTime")
        evidence.append(f"alarm_coverage alb={name} missing={','.join(missing) or 'none'}")
        if missing:
            if status == "PASS":
                status = "WARN"
            findings.append(make_finding(
                severity="MEDIUM",
                message=f"ALB {name} is missing operator-facing alarms for: {', '.join(missing)}.",
                recommendation=f"Create CloudWatch alarms for ALB {name} covering request errors and latency.",
                cost_impact="Low. CloudWatch alarm charges are minimal compared with the cost of late incident detection.",
                why="Alarm state alone is not enough; you need baseline coverage on customer-impacting metrics for each critical resource.",
                how="Add metric alarms on ALB 5XX and target response time with SNS or paging actions attached.",
            ))

    for db in rds_data.get("DBInstances", []):
        db_id = db.get("DBInstanceIdentifier", "unknown")
        missing: list[str] = []
        if not has_alarm({"CPUUtilization"}, "DBInstanceIdentifier", db_id):
            missing.append("CPUUtilization")
        if not has_alarm({"FreeStorageSpace"}, "DBInstanceIdentifier", db_id):
            missing.append("FreeStorageSpace")
        if not has_alarm({"FreeableMemory"}, "DBInstanceIdentifier", db_id):
            missing.append("FreeableMemory")
        if not has_alarm({"DatabaseConnections"}, "DBInstanceIdentifier", db_id):
            missing.append("DatabaseConnections")
        evidence.append(f"alarm_coverage rds={db_id} missing={','.join(missing) or 'none'}")
        if missing:
            if status == "PASS":
                status = "WARN"
            findings.append(make_finding(
                severity="MEDIUM",
                message=f"RDS instance {db_id} is missing CloudWatch alarms for: {', '.join(missing)}.",
                recommendation=f"Add baseline CloudWatch alarms for database capacity and latency precursors on {db_id}.",
                cost_impact="Low. Alarm costs are negligible.",
                why="RDS incidents are often first visible in memory, storage, or connection pressure before full outages occur.",
                how="Create alarms on CPU, storage, freeable memory, and connection count with notification actions.",
            ))

    for asg in asgs_data.get("AutoScalingGroups", []):
        name = asg.get("AutoScalingGroupName", "unknown")
        has_inservice = has_alarm({"GroupInServiceInstances"}, "AutoScalingGroupName", name)
        evidence.append(f"alarm_coverage asg={name} inServiceAlarm={has_inservice}")
        if not has_inservice:
            if status == "PASS":
                status = "WARN"
            findings.append(make_finding(
                severity="MEDIUM",
                message=f"ASG {name} is missing a CloudWatch alarm for GroupInServiceInstances.",
                recommendation=f"Add an operator-facing ASG capacity alarm for {name} instead of relying only on AWS-managed target-tracking alarms.",
                cost_impact="Low. Alarm costs are minimal.",
                why="AWS-managed target-tracking alarms are not a substitute for operator-facing fleet health alarms.",
                how="Create an alarm that pages when GroupInServiceInstances drops below expected capacity or diverges from desired capacity.",
            ))

    for reservation in ec2_data.get("Reservations", []):
        for instance in reservation.get("Instances", []):
            if instance.get("State", {}).get("Name") != "running":
                continue
            iid = instance.get("InstanceId", "?")
            has_status = has_alarm({"StatusCheckFailed", "StatusCheckFailed_Instance", "StatusCheckFailed_System"}, "InstanceId", iid)
            evidence.append(f"alarm_coverage ec2={iid} statusCheckAlarm={has_status}")
            if not has_status:
                if status == "PASS":
                    status = "WARN"
                findings.append(make_finding(
                    severity="MEDIUM",
                    message=f"EC2 instance {iid} is missing a CloudWatch status-check alarm.",
                    recommendation=f"Add a status-check alarm for EC2 instance {iid}.",
                    cost_impact="Low. Alarm costs are minimal.",
                    why="EC2 status-check alarms are the fastest route to detecting host or guest impairment before customers notice.",
                    how="Create an alarm on StatusCheckFailed or the instance/system variants and attach notification actions.",
                ))

    return make_check("alarm-coverage", "Alarm coverage for critical resources", status, findings, evidence[:30])


# ---------------------------------------------------------------------------
# Check 16: ACM certificate expiration
# ---------------------------------------------------------------------------

def check_certificate_expiry(profile: str, region: str) -> dict[str, Any]:
    certs_data = run_aws_json_safe(profile, region, ["acm", "list-certificates", "--certificate-statuses", "ISSUED"])
    if certs_data is None:
        return make_check("certificate-expiry", "ACM certificate expiration", "ERROR", [], ["api_error=true"])

    findings: list[dict[str, Any]] = []
    evidence: list[str] = []
    status = "PASS"
    now = utc_now()

    for summary in certs_data.get("CertificateSummaryList", []):
        cert_arn = summary.get("CertificateArn", "")
        detail = run_aws_json_safe(profile, region, ["acm", "describe-certificate", "--certificate-arn", cert_arn])
        if detail is None:
            evidence.append(f"certificate={cert_arn} detail_error=true")
            continue
        cert = detail.get("Certificate", {})
        in_use_by = cert.get("InUseBy", [])
        if not in_use_by:
            continue
        domain = cert.get("DomainName", cert_arn)
        not_after = parse_aws_datetime(cert.get("NotAfter"))
        if not_after is None:
            evidence.append(f"certificate={domain} expiry=unknown")
            continue
        days_left = (not_after - now).days
        evidence.append(f"certificate={domain} expiresInDays={days_left}")
        if days_left < 14:
            status = "FAIL"
            findings.append(make_finding(
                severity="HIGH",
                message=f"ACM certificate {domain} expires in {days_left} day(s).",
                recommendation=f"Renew or replace the ACM certificate for {domain} immediately.",
                cost_impact="None for the check. Expired certificates directly break TLS availability.",
                why="Certificate expiry causes immediate client trust failures and outage symptoms on public endpoints.",
                how="Review ACM renewal status, DNS validation, and every in-use attachment to ensure the renewed certificate is deployed before expiration.",
            ))
        elif days_left < 30:
            if status == "PASS":
                status = "WARN"
            findings.append(make_finding(
                severity="MEDIUM",
                message=f"ACM certificate {domain} expires in {days_left} day(s).",
                recommendation=f"Confirm ACM automatic renewal or rotation is healthy for {domain}.",
                cost_impact="None for the check.",
                why="Certificates near expiry are a predictable operational risk and should be closed before the final renewal window.",
                how="Verify DNS validation and listener attachments, then confirm the replacement certificate is issued and attached where needed.",
            ))

    if not evidence:
        evidence.append("no_in_use_issued_certificates=true")
    return make_check("certificate-expiry", "ACM certificate expiration", status, findings, evidence)


# ---------------------------------------------------------------------------
# Check 17: Slow query volume over last 24h
# ---------------------------------------------------------------------------

def check_slow_queries(profile: str, region: str) -> dict[str, Any]:
    log_groups_data = run_aws_json_safe(profile, region, ["logs", "describe-log-groups", "--log-group-name-prefix", "/aws/rds/instance/"])
    if log_groups_data is None:
        return make_check("slow-queries", "RDS slow queries over last 24h", "ERROR", [], ["api_error=true"])

    slow_groups = [
        lg.get("logGroupName", "")
        for lg in log_groups_data.get("logGroups", [])
        if str(lg.get("logGroupName", "")).endswith("/slowquery")
    ]
    if not slow_groups:
        return make_check("slow-queries", "RDS slow queries over last 24h", "PASS", [], ["no_slowquery_log_groups=true"])

    findings: list[dict[str, Any]] = []
    evidence: list[str] = []
    status = "PASS"
    end = utc_now()
    start = end - dt.timedelta(hours=24)
    query = "fields @message | filter @message like /# Query_time:/ | stats count() as slow_queries"

    for log_group in slow_groups:
        db_label = log_group.split("/")[4] if len(log_group.split("/")) > 4 else log_group
        try:
            query_id = start_logs_query(profile, region, log_group, int(start.timestamp()), int(end.timestamp()), query)
            rows = wait_for_logs_query(profile, region, query_id)
            count = 0
            if rows:
                count = int(float(rows[0].get("slow_queries", "0") or 0))
        except (AwsCommandError, ValueError):
            evidence.append(f"slowquery db={db_label} query_error=true")
            continue

        evidence.append(f"slowquery db={db_label} count24h={count}")
        if count >= 100:
            status = "FAIL"
            findings.append(make_finding(
                severity="HIGH",
                message=f"RDS slow query log for {db_label} recorded {count} slow queries in the last 24h.",
                recommendation=f"Investigate the top slow SQL patterns for {db_label} and remediate indexing, query shape, or workload pressure.",
                cost_impact="Potentially medium to high. Slow queries increase RDS compute and IO cost and often drive user-facing latency.",
                why="A high slow-query count is a direct signal of application inefficiency and rising database contention.",
                how="Use the slow query analysis tooling in this repo to identify the highest-frequency query shapes, then tune indexes and SQL plans.",
            ))
        elif count >= 25:
            if status == "PASS":
                status = "WARN"
            findings.append(make_finding(
                severity="MEDIUM",
                message=f"RDS slow query log for {db_label} recorded {count} slow queries in the last 24h.",
                recommendation=f"Review recent slow-query patterns for {db_label} before they become sustained performance issues.",
                cost_impact="None for the check.",
                why="Growing slow-query volume is usually an early signal of schema drift, missing indexes, or inefficient application access patterns.",
                how="Sample the top slow query shapes, correlate them with deploys or traffic changes, and tune the highest-cost patterns first.",
            ))

    return make_check("slow-queries", "RDS slow queries over last 24h", status, findings, evidence)


# ---------------------------------------------------------------------------
# Report writer
# ---------------------------------------------------------------------------

STATUS_ORDER = {"FAIL": 0, "WARN": 1, "ERROR": 2, "PASS": 3}
SEVERITY_ORDER = {"HIGH": 0, "MEDIUM": 1, "LOW": 2, "INFO": 3}
STATUS_EMOJI = {"FAIL": "❌", "WARN": "⚠️", "ERROR": "🔴", "PASS": "✅"}
SEVERITY_EMOJI = {"HIGH": "🔴", "MEDIUM": "🟡", "LOW": "🔵", "INFO": "⚪"}


def render_report(
    checks: list[dict[str, Any]],
    profile: str,
    region: str,
    run_time: dt.datetime,
) -> str:
    lines: list[str] = []
    lines.append(f"# AWS Health Review — {run_time.strftime('%Y-%m-%d')}\n")
    lines.append(f"**Run time (UTC):** {run_time.strftime('%Y-%m-%d %H:%M:%SZ')}  ")
    lines.append(f"**Profile:** `{profile}`  ")
    lines.append(f"**Region:** `{region}`\n")

    # Summary table
    lines.append("## Summary\n")
    lines.append("| # | Check | Status | Findings |")
    lines.append("|---|-------|--------|----------|")
    for i, check in enumerate(checks, 1):
        st = check["status"]
        emoji = STATUS_EMOJI.get(st, "")
        finding_count = len(check.get("findings", []))
        lines.append(f"| {i} | {check['title']} | {emoji} {st} | {finding_count} |")
    lines.append("")

    # Counts
    fail_count = sum(1 for c in checks if c["status"] == "FAIL")
    warn_count = sum(1 for c in checks if c["status"] == "WARN")
    pass_count = sum(1 for c in checks if c["status"] == "PASS")
    lines.append(f"**FAIL:** {fail_count}  **WARN:** {warn_count}  **PASS:** {pass_count}\n")

    # Detail sections
    lines.append("---\n")
    lines.append("## Check Results\n")
    for check in checks:
        st = check["status"]
        emoji = STATUS_EMOJI.get(st, "")
        lines.append(f"### {emoji} {check['title']}\n")
        lines.append(f"**Status:** {st}\n")

        findings = check.get("findings", [])
        if findings:
            lines.append("**Findings:**\n")
            for f in findings:
                sev = f.get("severity", "INFO")
                sev_emoji = SEVERITY_EMOJI.get(sev, "")
                lines.append(f"#### {sev_emoji} [{sev}] {f['message']}\n")

                if f.get("why"):
                    lines.append(f"**Why:** {f['why']}\n")
                if f.get("how"):
                    lines.append(f"**How to implement:** {f['how']}\n")

                lines.append(f"**Recommendation:** {f['recommendation']}\n")
                lines.append(f"**Cost impact:** {f['cost_impact']}\n")

        evidence = check.get("evidence", [])
        if evidence:
            lines.append("<details><summary>Evidence</summary>\n")
            lines.append("```")
            lines.append("\n".join(evidence[:30]))
            lines.append("```")
            lines.append("</details>\n")

        lines.append("")

    return "\n".join(lines)


# ---------------------------------------------------------------------------
# TODO.md integration
# ---------------------------------------------------------------------------

_TODO_TABLE_HEADER_RE = re.compile(r"^\| Priority \| Domain \| Task \| Jira \| Status \|")

SEVERITY_TO_PRIORITY = {"HIGH": "P0", "MEDIUM": "P1", "LOW": "P2"}
SEVERITY_TO_DOMAIN = {
    "cloudwatch-alarms": "CloudWatch",
    "aws-health-events": "AWS Health",
    "log-group-retention": "CloudWatch",
    "alb-target-health": "Network",
    "alb-access-logging": "Network",
    "alb-service-signals": "Network",
    "rds-health": "DB Performance",
    "rds-deep-metrics": "DB Performance",
    "asg-health": "ASG Deployment",
    "ec2-stopped": "Cost",
    "ec2-runtime-health": "EC2",
    "apprunner-health": "Agents Web App",
    "elasticache-health": "Session Performance",
    "alarms-no-actions": "CloudWatch",
    "backup-restore-readiness": "Backup",
    "alarm-coverage": "Observability",
    "certificate-expiry": "Network",
    "slow-queries": "DB Performance",
    "billing-mtd-spend": "Cost",
    "billing-cost-anomalies": "Cost",
    "billing-idle-resources": "Cost",
}

_TODO_TASK_SENTINEL = "<!-- aws-health-review:"


def build_todo_tasks(checks: list[dict[str, Any]], run_date: str) -> list[tuple[str, str, str]]:
    """Return list of (priority, domain, task_text) for HIGH and MEDIUM findings."""
    tasks: list[tuple[str, str, str]] = []
    for check in checks:
        check_id = check.get("id", "")
        domain = SEVERITY_TO_DOMAIN.get(check_id, "AWS Health")
        for finding in check.get("findings", []):
            sev = finding.get("severity", "LOW")
            if sev not in ("HIGH", "MEDIUM"):
                continue
            priority = SEVERITY_TO_PRIORITY[sev]
            task_text = (
                f"**AWS Health ({run_date}):** {finding['message']} "
                f"Recommendation: {finding['recommendation']}"
            )
            tasks.append((priority, domain, task_text))
    return tasks


def update_todo(todo_path: Path, checks: list[dict[str, Any]], run_date: str) -> int:
    """Append new health-review tasks to TODO.md. Returns count of tasks added."""
    tasks = build_todo_tasks(checks, run_date)
    if not tasks:
        return 0

    content = todo_path.read_text(encoding="utf-8")

    # Find the table header line
    lines = content.splitlines(keepends=True)
    header_line = None
    separator_line = None
    for i, line in enumerate(lines):
        if _TODO_TABLE_HEADER_RE.match(line.strip()):
            header_line = i
            if i + 1 < len(lines):
                separator_line = i + 1
            break

    if header_line is None or separator_line is None:
        return 0

    # Build new rows
    new_rows: list[str] = []
    for priority, domain, task_text in tasks:
        sentinel = f"{_TODO_TASK_SENTINEL}{run_date}--{stable_task_id(task_text)}-->"
        # Skip if already present (idempotent re-run)
        if sentinel in content:
            continue
        new_rows.append(
            f"| {priority} | {domain} | {task_text} {sentinel} | - | `OPEN` |\n"
        )

    if not new_rows:
        return 0

    # Insert after the separator line
    insert_at = separator_line + 1
    lines[insert_at:insert_at] = new_rows

    todo_path.write_text("".join(lines), encoding="utf-8")
    return len(new_rows)


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Weekly AWS health review (read-only).")
    parser.add_argument(
        "--profile",
        default=os.environ.get("PROFILE") or os.environ.get("AWS_PROFILE") or "wepro-readonly",
        help="AWS profile (default: PROFILE, AWS_PROFILE, or wepro-readonly)",
    )
    parser.add_argument("--region", default="us-east-1", help="AWS region (default: us-east-1)")
    parser.add_argument("--output", default="", help="Output report file (default: auto-generated in reports/)")
    parser.add_argument(
        "--no-todo-update",
        action="store_true",
        help="Skip updating TODO.md with findings",
    )
    parser.add_argument(
        "--todo-path",
        default="TODO.md",
        help="Path to TODO.md (default: TODO.md)",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    run_time = utc_now()
    run_date = run_time.strftime("%Y-%m-%d")
    run_stamp = run_time.strftime("%Y%m%d")

    profile = args.profile
    region = args.region

    print(f"[aws-health-review] Starting review at {run_time.strftime('%Y-%m-%dT%H:%M:%SZ')}", file=sys.stderr)
    print(f"[aws-health-review] Profile={profile} Region={region}", file=sys.stderr)

    checks = [
        ("CloudWatch alarms in ALARM state", check_cloudwatch_alarms),
        ("AWS Health account events", check_aws_health_events),
        ("CloudWatch log group retention", check_log_group_retention),
        ("ALB target group health", check_alb_target_health),
        ("ALB access logging", check_alb_access_logging),
        ("ALB 5XX and latency signals", check_alb_service_signals),
        ("RDS health and performance", check_rds_health),
        ("RDS memory, connections, and latency", check_rds_deep_metrics),
        ("ASG health", check_asg_health),
        ("EC2 stopped instances", check_ec2_stopped_instances),
        ("EC2 status checks and scheduled events", check_ec2_runtime_health),
        ("App Runner health", check_apprunner_health),
        ("ElastiCache health", check_elasticache_health),
        ("CloudWatch alarms without actions", check_alarms_no_actions),
        ("Backup coverage and restore readiness", check_backup_restore_readiness),
        ("Alarm coverage for critical resources", check_alarm_coverage),
        ("ACM certificate expiration", check_certificate_expiry),
        ("RDS slow queries over last 24h", check_slow_queries),
        ("Billing: month-to-date spend", check_billing_mtd_spend),
        ("Billing: cost anomaly detection", check_billing_cost_anomalies),
        ("Billing: idle resource waste (EBS + EIPs)", check_billing_idle_resources),
    ]

    results: list[dict[str, Any]] = []
    for label, fn in checks:
        print(f"[aws-health-review]   Running: {label} ...", file=sys.stderr)
        try:
            result = fn(profile, region)
        except Exception as exc:  # noqa: BLE001
            result = make_check(
                label.lower().replace(" ", "-"), label,
                "ERROR",
                [make_finding("INFO", f"Check raised an unexpected error: {exc}", "", "None")],
                [f"error={exc}"],
            )
        results.append(result)
        status = result.get("status", "?")
        finding_count = len(result.get("findings", []))
        print(f"[aws-health-review]   {label}: {status} ({finding_count} finding(s))", file=sys.stderr)

    # Render report
    report_text = render_report(results, profile, region, run_time)

    if args.output:
        out_path = Path(args.output)
    else:
        reports_dir = Path("reports")
        reports_dir.mkdir(exist_ok=True)
        out_path = reports_dir / f"aws-health-weekly-{run_stamp}.md"

    out_path.write_text(report_text, encoding="utf-8")
    print(f"[aws-health-review] Report written to: {out_path}", file=sys.stderr)

    # Update TODO.md
    if not args.no_todo_update:
        todo_path = Path(args.todo_path)
        if todo_path.exists():
            added = update_todo(todo_path, results, run_date)
            print(f"[aws-health-review] TODO.md updated: {added} new task(s) added.", file=sys.stderr)
        else:
            print(f"[aws-health-review] TODO.md not found at {todo_path} — skipping update.", file=sys.stderr)

    # Print summary to stdout
    fail_count = sum(1 for c in results if c["status"] == "FAIL")
    warn_count = sum(1 for c in results if c["status"] == "WARN")
    pass_count = sum(1 for c in results if c["status"] == "PASS")
    print(f"\nAWS Health Review — {run_date}")
    print(f"FAIL: {fail_count}  WARN: {warn_count}  PASS: {pass_count}")
    print(f"Report: {out_path}")

    return 1 if fail_count > 0 else 0


if __name__ == "__main__":
    sys.exit(main())

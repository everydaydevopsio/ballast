#!/usr/bin/env python3
# /// script
# requires-python = ">=3.10"
# ///
"""Generate a read-only current-state AWS health review report."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import subprocess
import sys
import time
from pathlib import Path
from typing import Any


class AwsCommandError(RuntimeError):
    """Raised when an AWS CLI command fails."""


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--profile",
        default=os.environ.get("PROFILE") or os.environ.get("AWS_PROFILE") or "wepro-readonly",
    )
    parser.add_argument("--region", default="us-east-1")
    parser.add_argument("--logs-hours", type=int, default=24)
    parser.add_argument("--output", help="Markdown output path")
    return parser.parse_args()


def utc_now() -> dt.datetime:
    return dt.datetime.now(dt.timezone.utc)


def iso_z(value: dt.datetime) -> str:
    return value.strftime("%Y-%m-%dT%H:%M:%SZ")


def run_aws(profile: str, region: str, args: list[str], expect_json: bool = True) -> Any:
    cmd = ["aws", "--profile", profile, "--region", region, *args]
    if expect_json:
        cmd.extend(["--output", "json"])
    result = subprocess.run(cmd, capture_output=True, text=True, check=False)
    if result.returncode != 0:
        raise AwsCommandError(result.stderr.strip() or "AWS command failed")
    payload = result.stdout.strip()
    if not expect_json:
        return payload
    if not payload:
        return {}
    return json.loads(payload)


def run_aws_safe(profile: str, region: str, args: list[str], expect_json: bool = True) -> Any | None:
    try:
        return run_aws(profile, region, args, expect_json=expect_json)
    except (AwsCommandError, json.JSONDecodeError):
        return None


def default_output_path(now: dt.datetime) -> Path:
    stamp = now.strftime("%Y%m%dT%H%M%SZ")
    return Path("reports") / f"aws-live-health-{stamp}.md"


def format_ts_ms(value: int | None) -> str:
    if value is None:
        return "n/a"
    return dt.datetime.fromtimestamp(value / 1000, tz=dt.timezone.utc).strftime("%Y-%m-%d %H:%M:%SZ")


def find_name_tag(tags: list[dict[str, Any]]) -> str:
    for tag in tags:
        if tag.get("Key") == "Name":
            return str(tag.get("Value", ""))
    return ""


def summarize_ec2(profile: str, region: str) -> dict[str, Any]:
    instances_payload = run_aws(profile, region, ["ec2", "describe-instances"])
    status_payload = run_aws(
        profile, region, ["ec2", "describe-instance-status", "--include-all-instances"]
    )

    instances: list[dict[str, Any]] = []
    for reservation in instances_payload.get("Reservations", []):
        instances.extend(reservation.get("Instances", []))

    status_map = {
        item["InstanceId"]: item for item in status_payload.get("InstanceStatuses", [])
    }

    running: list[dict[str, Any]] = []
    stopped: list[dict[str, Any]] = []
    impaired: list[str] = []
    inventory_lines: list[str] = []

    for instance in instances:
        instance_id = instance.get("InstanceId", "?")
        name = find_name_tag(instance.get("Tags", [])) or "-"
        state = instance.get("State", {}).get("Name", "unknown")
        instance_type = instance.get("InstanceType", "?")
        az = instance.get("Placement", {}).get("AvailabilityZone", "?")
        inventory_lines.append(
            f"- `{instance_id}` `{name}` state={state} type={instance_type} az={az}"
        )
        if state == "running":
            running.append(instance)
        elif state == "stopped":
            stopped.append(instance)

        status = status_map.get(instance_id)
        if status and state == "running":
            inst_ok = status.get("InstanceStatus", {}).get("Status") == "ok"
            sys_ok = status.get("SystemStatus", {}).get("Status") == "ok"
            ebs_ok = status.get("AttachedEbsStatus", {}).get("Status") in ("ok", None)
            if not (inst_ok and sys_ok and ebs_ok):
                impaired.append(
                    f"{instance_id} instance={status.get('InstanceStatus', {}).get('Status')} "
                    f"system={status.get('SystemStatus', {}).get('Status')} "
                    f"ebs={status.get('AttachedEbsStatus', {}).get('Status', 'n/a')}"
                )

    return {
        "total": len(instances),
        "running": len(running),
        "stopped": len(stopped),
        "impaired": impaired,
        "stopped_names": [
            f"{item.get('InstanceId')}({find_name_tag(item.get('Tags', [])) or item.get('InstanceType', '?')})"
            for item in stopped
        ],
        "inventory_lines": inventory_lines,
    }


def summarize_rds(profile: str, region: str) -> dict[str, Any]:
    payload = run_aws(profile, region, ["rds", "describe-db-instances"])
    dbs = payload.get("DBInstances", [])

    unavailable: list[str] = []
    public: list[str] = []
    single_az: list[str] = []
    lines: list[str] = []

    for db in dbs:
        db_id = db.get("DBInstanceIdentifier", "unknown")
        status = db.get("DBInstanceStatus", "unknown")
        engine = db.get("Engine", "")
        version = db.get("EngineVersion", "")
        multi_az = bool(db.get("MultiAZ"))
        public_access = bool(db.get("PubliclyAccessible"))
        backup = db.get("BackupRetentionPeriod", 0)
        lines.append(
            f"- `{db_id}` status={status} engine={engine} {version} "
            f"multiAZ={multi_az} public={public_access} backupRetention={backup}d"
        )
        if status != "available":
            unavailable.append(f"{db_id}({status})")
        if public_access:
            public.append(db_id)
        if not multi_az:
            single_az.append(db_id)

    return {
        "count": len(dbs),
        "unavailable": unavailable,
        "public": public,
        "single_az": single_az,
        "lines": lines,
    }


def summarize_alb(profile: str, region: str) -> dict[str, Any]:
    lbs_payload = run_aws(profile, region, ["elbv2", "describe-load-balancers"])
    tgs_payload = run_aws(profile, region, ["elbv2", "describe-target-groups"])

    lbs = lbs_payload.get("LoadBalancers", [])
    tgs = tgs_payload.get("TargetGroups", [])

    inactive: list[str] = []
    no_access_logs: list[str] = []
    unhealthy_targets: list[str] = []
    lb_lines: list[str] = []
    tg_lines: list[str] = []

    for lb in lbs:
        arn = lb.get("LoadBalancerArn", "")
        name = lb.get("LoadBalancerName", "unknown")
        state = lb.get("State", {}).get("Code", "unknown")
        lb_lines.append(f"- `{name}` state={state} scheme={lb.get('Scheme', '?')}")
        if state != "active":
            inactive.append(f"{name}({state})")

        attrs = run_aws_safe(
            profile,
            region,
            ["elbv2", "describe-load-balancer-attributes", "--load-balancer-arn", arn],
        )
        if attrs:
            attr_map = {item["Key"]: item["Value"] for item in attrs.get("Attributes", [])}
            if attr_map.get("access_logs.s3.enabled", "false").lower() != "true":
                no_access_logs.append(name)

    for tg in tgs:
        arn = tg.get("TargetGroupArn", "")
        name = tg.get("TargetGroupName", "unknown")
        health = run_aws_safe(
            profile, region, ["elbv2", "describe-target-health", "--target-group-arn", arn]
        )
        if not health:
            tg_lines.append(f"- `{name}` health=unknown")
            continue
        descriptions = health.get("TargetHealthDescriptions", [])
        healthy = 0
        total = len(descriptions)
        for item in descriptions:
            state = item.get("TargetHealth", {}).get("State")
            if state == "healthy":
                healthy += 1
            elif state not in ("healthy", "unused"):
                unhealthy_targets.append(
                    f"{name}/{item.get('Target', {}).get('Id', '?')}({state})"
                )
        tg_lines.append(f"- `{name}` healthy={healthy}/{total}")

    return {
        "lb_count": len(lbs),
        "inactive": inactive,
        "no_access_logs": no_access_logs,
        "unhealthy_targets": unhealthy_targets,
        "lb_lines": lb_lines,
        "tg_lines": tg_lines,
    }


def summarize_alarms(profile: str, region: str) -> dict[str, Any]:
    payload = run_aws(
        profile,
        region,
        [
            "cloudwatch",
            "describe-alarms",
            "--state-value",
            "ALARM",
            "--alarm-types",
            "MetricAlarm",
            "CompositeAlarm",
        ],
    )
    metric_alarms = payload.get("MetricAlarms", [])
    composite_alarms = payload.get("CompositeAlarms", [])

    names: list[str] = []
    low_traffic_target_tracking = 0
    lines: list[str] = []

    for alarm in metric_alarms:
        name = alarm.get("AlarmName", "unknown")
        metric = alarm.get("MetricName", "")
        namespace = alarm.get("Namespace", "")
        comparison = alarm.get("ComparisonOperator", "")
        threshold = alarm.get("Threshold", "")
        lines.append(
            f"- `{name}` metric={namespace}/{metric} comparison={comparison} threshold={threshold}"
        )
        names.append(name)
        if metric == "RequestCountPerTarget" and comparison == "LessThanThreshold":
            low_traffic_target_tracking += 1

    for alarm in composite_alarms:
        name = alarm.get("AlarmName", "unknown")
        lines.append(f"- `{name}` composite")
        names.append(name)

    return {
        "count": len(names),
        "names": names,
        "low_traffic_target_tracking": low_traffic_target_tracking,
        "lines": lines,
    }


def list_log_groups(profile: str, region: str) -> list[dict[str, Any]]:
    groups: list[dict[str, Any]] = []
    token: str | None = None
    while True:
        args = ["logs", "describe-log-groups"]
        if token:
            args.extend(["--next-token", token])
        payload = run_aws_safe(profile, region, args)
        if not payload:
            break
        groups.extend(payload.get("logGroups", []))
        token = payload.get("nextToken")
        if not token:
            break
    return groups


def latest_log_stream_ts(profile: str, region: str, group_name: str) -> int | None:
    payload = run_aws_safe(
        profile,
        region,
        [
            "logs",
            "describe-log-streams",
            "--log-group-name",
            group_name,
            "--order-by",
            "LastEventTime",
            "--descending",
            "--limit",
            "1",
        ],
    )
    if not payload:
        return None
    streams = payload.get("logStreams", [])
    if not streams:
        return None
    return streams[0].get("lastEventTimestamp")


def run_logs_insights_query(
    profile: str,
    region: str,
    log_groups: list[str],
    start: dt.datetime,
    end: dt.datetime,
    query: str,
) -> dict[str, Any]:
    if not log_groups:
        return {"status": "Complete", "results": [], "statistics": {}}

    chunk_size = 20
    combined_results: list[Any] = []
    combined_statistics: dict[str, float] = {}
    final_status = "Complete"

    for index in range(0, len(log_groups), chunk_size):
        chunk = log_groups[index : index + chunk_size]
        payload = run_aws(
            profile,
            region,
            [
                "logs",
                "start-query",
                "--start-time",
                str(int(start.timestamp())),
                "--end-time",
                str(int(end.timestamp())),
                "--query-string",
                query,
                "--log-group-names",
                *chunk,
            ],
        )
        query_id = payload["queryId"]

        chunk_result: dict[str, Any] = {"status": "Timeout", "results": [], "statistics": {}}
        for _ in range(30):
            result = run_aws(profile, region, ["logs", "get-query-results", "--query-id", query_id])
            status = result.get("status")
            if status in {"Complete", "Failed", "Cancelled", "Timeout", "Unknown"}:
                chunk_result = result
                break
            time.sleep(1)

        chunk_status = str(chunk_result.get("status", "Unknown"))
        if chunk_status != "Complete":
            final_status = chunk_status

        combined_results.extend(chunk_result.get("results", []))
        for key, value in chunk_result.get("statistics", {}).items():
            try:
                combined_statistics[key] = combined_statistics.get(key, 0.0) + float(value)
            except (TypeError, ValueError):
                continue

    return {
        "status": final_status,
        "results": combined_results[:20],
        "statistics": combined_statistics,
    }


def summarize_logs(profile: str, region: str, logs_hours: int) -> dict[str, Any]:
    now = utc_now()
    start = now - dt.timedelta(hours=logs_hours)
    start_ms = int(start.timestamp() * 1000)

    groups = list_log_groups(profile, region)
    no_retention = [g["logGroupName"] for g in groups if g.get("retentionInDays") is None]

    app_candidates = [
        g["logGroupName"]
        for g in groups
        if g["logGroupName"].startswith("/aws/apprunner/") and g["logGroupName"].endswith("/application")
    ]
    rds_error_groups = [
        g["logGroupName"]
        for g in groups
        if g["logGroupName"].startswith("/aws/rds/instance/") and g["logGroupName"].endswith("/error")
    ]
    rds_slow_groups = [
        g["logGroupName"]
        for g in groups
        if g["logGroupName"].startswith("/aws/rds/instance/") and g["logGroupName"].endswith("/slowquery")
    ]

    active_app_groups: list[str] = []
    for group_name in app_candidates:
        last_ts = latest_log_stream_ts(profile, region, group_name)
        if last_ts and last_ts >= start_ms:
            active_app_groups.append(group_name)

    app_query = (
        "fields @timestamp, @log, @message "
        "| filter @message like /ERROR|Error|Exception|Traceback|FATAL|panic/ "
        "| sort @timestamp desc | limit 20"
    )
    app_result = run_logs_insights_query(profile, region, active_app_groups, start, now, app_query)
    app_error_count = int(app_result.get("statistics", {}).get("recordsMatched", 0.0))

    rds_auth_lines: list[str] = []
    for group_name in rds_error_groups:
        payload = run_aws_safe(
            profile,
            region,
            [
                "logs",
                "filter-log-events",
                "--log-group-name",
                group_name,
                "--start-time",
                str(start_ms),
                "--limit",
                "20",
            ],
        )
        if not payload:
            continue
        for event in payload.get("events", []):
            message = event.get("message", "")
            if "Access denied" in message or "Aborted connection" in message:
                rds_auth_lines.append(
                    f"- `{group_name}` {format_ts_ms(event.get('timestamp'))} {message.strip()[:180]}"
                )

    slow_query_lines: list[str] = []
    for group_name in rds_slow_groups:
        result = run_logs_insights_query(
            profile,
            region,
            [group_name],
            start,
            now,
            "stats count() as slowQueryEvents by bin(1h)",
        )
        results = result.get("results", [])
        total = int(result.get("statistics", {}).get("recordsMatched", 0.0))
        top_hour = "n/a"
        top_count = 0
        for row in results:
            values = {item["field"]: item["value"] for item in row}
            count = int(float(values.get("slowQueryEvents", "0")))
            hour = values.get("bin(1h)", "n/a")
            if count > top_count:
                top_count = count
                top_hour = hour
        slow_query_lines.append(
            f"- `{group_name}` slow-query events last {logs_hours}h={total}, peak hour={top_hour} count={top_count}"
        )

    sample_lines: list[str] = []
    for row in app_result.get("results", [])[:10]:
        values = {item["field"]: item["value"] for item in row}
        sample_lines.append(
            f"- `{values.get('@log', 'unknown')}` {values.get('@timestamp', 'n/a')} {values.get('@message', '')[:180]}"
        )

    return {
        "logs_hours": logs_hours,
        "active_app_groups": active_app_groups,
        "app_error_count": app_error_count,
        "app_error_samples": sample_lines,
        "no_retention_count": len(no_retention),
        "no_retention_sample": [f"- `{name}`" for name in no_retention[:10]],
        "rds_auth_lines": rds_auth_lines[:10],
        "slow_query_lines": slow_query_lines,
    }


def overall_status(
    ec2: dict[str, Any],
    rds: dict[str, Any],
    alb: dict[str, Any],
    alarms: dict[str, Any],
    logs: dict[str, Any],
) -> str:
    if ec2["impaired"] or rds["unavailable"] or alb["inactive"] or alb["unhealthy_targets"]:
        return "DEGRADED"
    if alarms["count"] > alarms["low_traffic_target_tracking"]:
        return "ATTENTION"
    if logs["app_error_count"] > 0:
        return "ATTENTION"
    return "HEALTHY"


def build_report(
    now: dt.datetime,
    args: argparse.Namespace,
    ec2: dict[str, Any],
    rds: dict[str, Any],
    alb: dict[str, Any],
    alarms: dict[str, Any],
    logs: dict[str, Any],
) -> str:
    status = overall_status(ec2, rds, alb, alarms, logs)
    ec2_status_lines = (
        [f"- {line}" for line in ec2["impaired"]]
        if ec2["impaired"]
        else ["- All running instances are passing instance, system, and attached EBS checks."]
    )
    alarm_lines = alarms["lines"] or ["- No alarms currently in `ALARM` state."]
    app_error_lines = (
        logs["app_error_samples"]
        if logs["app_error_samples"]
        else ["- No error-like application log entries matched the query window."]
    )
    rds_auth_lines = (
        logs["rds_auth_lines"]
        if logs["rds_auth_lines"]
        else ["- No RDS access-denied or aborted-connection samples found in the review window."]
    )
    slow_query_lines = (
        logs["slow_query_lines"]
        if logs["slow_query_lines"]
        else ["- No active RDS slow-query log groups were detected."]
    )

    headline = [
        f"- Overall status: **{status}**",
        f"- EC2: `{ec2['running']}` running, `{ec2['stopped']}` stopped, `{len(ec2['impaired'])}` impaired status checks",
        f"- RDS: `{rds['count']}` instances, `{len(rds['unavailable'])}` unavailable",
        f"- ALB: `{alb['lb_count']}` load balancers, `{len(alb['unhealthy_targets'])}` unhealthy targets",
        f"- CloudWatch alarms in ALARM: `{alarms['count']}`",
        f"- Active app-log error matches last `{logs['logs_hours']}`h: `{logs['app_error_count']}`",
    ]

    current_risks: list[str] = []
    if rds["public"]:
        current_risks.append(
            f"- Publicly accessible RDS instances: {', '.join(f'`{item}`' for item in rds['public'])}"
        )
    if rds["single_az"]:
        current_risks.append(
            f"- Single-AZ RDS instances: {', '.join(f'`{item}`' for item in rds['single_az'])}"
        )
    if alb["no_access_logs"]:
        current_risks.append(
            f"- ALBs without access logging: {', '.join(f'`{item}`' for item in alb['no_access_logs'])}"
        )
    if logs["no_retention_count"]:
        current_risks.append(
            f"- Log groups without retention: `{logs['no_retention_count']}`"
        )
    if ec2["stopped_names"]:
        current_risks.append(
            f"- Stopped EC2 instances: {', '.join(f'`{item}`' for item in ec2['stopped_names'])}"
        )
    if logs["rds_auth_lines"]:
        current_risks.append("- RDS error logs show unauthenticated or denied connection attempts in the review window.")
    if logs["slow_query_lines"]:
        current_risks.append("- RDS slow-query logging is active; review counts below for tuning pressure.")
    if not current_risks:
        current_risks.append("- No immediate risk flags beyond routine operational noise.")

    report = [
        f"# AWS Live Health Review — {now.strftime('%Y-%m-%d %H:%M:%SZ')}",
        "",
        f"**Run time (UTC):** {now.strftime('%Y-%m-%d %H:%M:%SZ')}  ",
        f"**Profile:** `{args.profile}`  ",
        f"**Region:** `{args.region}`  ",
        f"**Log lookback:** last `{args.logs_hours}` hour(s)",
        "",
        "## Headline",
        "",
        *headline,
        "",
        "## Current Risks",
        "",
        *current_risks,
        "",
        "## EC2",
        "",
        *ec2["inventory_lines"],
        "",
        "### EC2 Status Check Findings",
        "",
        *ec2_status_lines,
        "",
        "## RDS",
        "",
        *rds["lines"],
        "",
        "## ALB",
        "",
        *alb["lb_lines"],
        "",
        "### Target Groups",
        "",
        *alb["tg_lines"],
        "",
        "## CloudWatch Alarms",
        "",
        *alarm_lines,
        "",
        "## CloudWatch Logs",
        "",
        f"- Active App Runner application log groups scanned: `{len(logs['active_app_groups'])}`",
        f"- Error-like application log matches last `{logs['logs_hours']}`h: `{logs['app_error_count']}`",
        "",
        "### Application Error Samples",
        "",
        *app_error_lines,
        "",
        "### RDS Auth/Error Signals",
        "",
        *rds_auth_lines,
        "",
        "### RDS Slow Query Signals",
        "",
        *slow_query_lines,
        "",
        "### Log Retention Drift",
        "",
        f"- Log groups without retention: `{logs['no_retention_count']}`",
        *logs["no_retention_sample"],
        "",
    ]
    return "\n".join(report)


def main() -> int:
    args = parse_args()
    now = utc_now()
    output_path = Path(args.output) if args.output else default_output_path(now)
    output_path.parent.mkdir(parents=True, exist_ok=True)

    ec2 = summarize_ec2(args.profile, args.region)
    rds = summarize_rds(args.profile, args.region)
    alb = summarize_alb(args.profile, args.region)
    alarms = summarize_alarms(args.profile, args.region)
    logs = summarize_logs(args.profile, args.region, args.logs_hours)

    report = build_report(now, args, ec2, rds, alb, alarms, logs)
    output_path.write_text(report, encoding="utf-8")

    print(f"[aws-live-health-review] Report written to: {output_path}")
    print(f"[aws-live-health-review] Overall status: {overall_status(ec2, rds, alb, alarms, logs)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# /// script
# requires-python = ">=3.10"
# ///
"""Weekly read-only AWS security baseline review."""

from __future__ import annotations

import argparse
import base64
import csv
import datetime as dt
import io
import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Any


class AwsCommandError(RuntimeError):
    """Raised when an AWS CLI command fails."""


def make_finding(
    severity: str,
    message: str,
    recommendation: str,
    cost_impact: str,
    cost_scope_services: list[str] | None = None,
    cost_change_summary: str | None = None,
) -> dict[str, Any]:
    """Build a finding with an explicit cost impact note and optional cost scope metadata."""
    finding: dict[str, Any] = {
        "severity": severity,
        "message": message,
        "recommendation": recommendation,
        "cost_impact": cost_impact,
    }
    if cost_scope_services:
        finding["cost_scope_services"] = sorted(set(cost_scope_services))
    if cost_change_summary:
        finding["cost_change_summary"] = cost_change_summary
    return finding


def run_aws_json(profile: str, region: str, args: list[str]) -> Any:
    cmd = [
        "aws",
        "--profile",
        profile,
        "--region",
        region,
        *args,
        "--output",
        "json",
    ]
    result = subprocess.run(cmd, capture_output=True, text=True, check=False)
    if result.returncode != 0:
        raise AwsCommandError(result.stderr.strip() or "AWS command failed")

    payload = result.stdout.strip()
    if not payload:
        return {}
    return json.loads(payload)


def utc_now() -> dt.datetime:
    return dt.datetime.now(dt.timezone.utc)


def extract_cidrs(permission: dict[str, Any]) -> list[str]:
    cidrs: list[str] = []
    for item in permission.get("IpRanges", []):
        cidr = item.get("CidrIp")
        if cidr:
            cidrs.append(str(cidr))
    for item in permission.get("Ipv6Ranges", []):
        cidr = item.get("CidrIpv6")
        if cidr:
            cidrs.append(str(cidr))
    return cidrs


def permission_covers_port(permission: dict[str, Any], port: int) -> bool:
    protocol = str(permission.get("IpProtocol", ""))
    if protocol == "-1":
        return True

    from_port = permission.get("FromPort")
    to_port = permission.get("ToPort")
    if from_port is None or to_port is None:
        return False

    try:
        start = int(from_port)
        end = int(to_port)
    except (TypeError, ValueError):
        return False

    return start <= port <= end


def _load_approved_admin_users() -> set[str]:
    """Load approved admin usernames from env, defaulting to empty."""
    raw = os.getenv("AWS_WEEKLY_SECURITY_REVIEW_APPROVED_ADMIN_USERS", "")
    return {value.strip() for value in raw.split(",") if value.strip()}


def _get_credential_report_rows(profile: str, region: str) -> list[dict[str, str]]:
    """Fetch the IAM credential report so MFA status can be evaluated in one call."""
    try:
        payload = run_aws_json(profile, region, ["iam", "get-credential-report"])
    except AwsCommandError as exc:
        if "CredentialReportNotPresent" not in str(exc):
            raise
        run_aws_json(profile, region, ["iam", "generate-credential-report"])
        payload = run_aws_json(profile, region, ["iam", "get-credential-report"])

    encoded = payload.get("Content")
    if not encoded:
        return []

    decoded = base64.b64decode(str(encoded)).decode("utf-8")
    return list(csv.DictReader(io.StringIO(decoded)))


def check_root_mfa(profile: str, region: str) -> dict[str, Any]:
    summary = run_aws_json(profile, region, ["iam", "get-account-summary"])
    enabled = summary.get("SummaryMap", {}).get("AccountMFAEnabled", 0) == 1
    users = run_aws_json(profile, region, ["iam", "list-users"]).get("Users", [])
    credential_report = {
        str(row.get("user", "")).strip(): row for row in _get_credential_report_rows(profile, region)
    }

    users_without_mfa: list[str] = []
    users_with_mfa: list[str] = []
    evidence = [f"rootAccountMFAEnabled={1 if enabled else 0}"]
    for user in users:
        user_name = str(user.get("UserName", "")).strip()
        if not user_name:
            continue
        report_row = credential_report.get(user_name, {})
        mfa_value = str(report_row.get("mfa_active", "")).strip().lower()
        has_mfa = mfa_value == "true"
        if has_mfa:
            users_with_mfa.append(user_name)
        else:
            users_without_mfa.append(user_name)
        evidence.append(
            f"user={user_name} mfaEnabled={str(has_mfa).lower()} source=credential-report"
        )

    findings: list[dict[str, Any]] = []
    status = "PASS"

    if not enabled:
        status = "FAIL"
        findings.append(
            make_finding(
                severity="HIGH",
                message="Root account MFA is not enabled.",
                recommendation="Enable MFA on the root account immediately.",
                cost_impact="Low. MFA itself has no direct AWS charge; may require MFA device/token costs managed outside AWS billing.",
            )
        )

    if users_without_mfa:
        if status == "PASS":
            status = "WARN"
        findings.append(
            make_finding(
                severity="MEDIUM",
                message=(
                    f"{len(users_without_mfa)} IAM user(s) do not have MFA enabled: "
                    f"{', '.join(sorted(users_without_mfa))}."
                ),
                recommendation=(
                    "Enable MFA for IAM users with console access and remove or rotate stale IAM users "
                    "that do not need interactive access."
                ),
                cost_impact="Low. MFA itself has no direct AWS charge; may require MFA device/token costs managed outside AWS billing.",
            )
        )

    evidence.append(f"iamUsersTotal={len(users_with_mfa) + len(users_without_mfa)}")
    evidence.append(f"iamUsersMFAEnabled={len(users_with_mfa)}")
    evidence.append(f"iamUsersMFADisabled={len(users_without_mfa)}")

    return {
        "id": "root-mfa",
        "title": "Root account MFA and IAM user MFA coverage",
        "status": status,
        "findings": findings,
        "evidence": evidence,
    }


def check_cloudtrail_baseline(profile: str, region: str) -> dict[str, Any]:
    trails = run_aws_json(
        profile,
        region,
        ["cloudtrail", "describe-trails", "--include-shadow-trails"],
    ).get("trailList", [])

    if not trails:
        return {
            "id": "cloudtrail-baseline",
            "title": "CloudTrail baseline",
            "status": "FAIL",
            "findings": [
                make_finding(
                    severity="HIGH",
                    message="No CloudTrail trails were found.",
                    recommendation="Create a multi-region CloudTrail and enable log delivery.",
                    cost_impact=(
                        "Low to medium recurring cost. First copy of management events is free; "
                        "charges can accrue for S3 storage/requests, data events, and optional CloudTrail Lake usage."
                    ),
                    cost_scope_services=["AWSCloudTrail", "AmazonS3"],
                    cost_change_summary="Add a new multi-region CloudTrail with centralized S3 log delivery.",
                )
            ],
            "evidence": ["trailCount=0"],
        }

    active_multi_region = []
    evidence = []
    for trail in trails:
        name = trail.get("Name")
        arn = trail.get("TrailARN")
        is_multi = bool(trail.get("IsMultiRegionTrail", False))

        if not (name or arn):
            continue

        status = run_aws_json(
            profile,
            region,
            ["cloudtrail", "get-trail-status", "--name", str(arn or name)],
        )
        is_logging = bool(status.get("IsLogging", False))
        evidence.append(f"{name}: multiRegion={is_multi}, logging={is_logging}")

        if is_multi and is_logging:
            active_multi_region.append(str(name))

    if active_multi_region:
        return {
            "id": "cloudtrail-baseline",
            "title": "CloudTrail baseline",
            "status": "PASS",
            "findings": [],
            "evidence": evidence,
        }

    return {
        "id": "cloudtrail-baseline",
        "title": "CloudTrail baseline",
        "status": "FAIL",
        "findings": [
            make_finding(
                severity="HIGH",
                message="No multi-region CloudTrail is currently logging.",
                recommendation="Enable logging on at least one multi-region trail.",
                cost_impact=(
                    "Low to medium recurring cost. Enabling logging typically adds S3 log storage/request costs "
                    "and may add event-based charges depending on enabled CloudTrail features."
                ),
                cost_scope_services=["AWSCloudTrail", "AmazonS3"],
                cost_change_summary="Enable logging on an existing multi-region CloudTrail and store logs in S3.",
            )
        ],
        "evidence": evidence,
    }


SENSITIVE_PORTS: dict[int, str] = {
    22: "SSH",
    3389: "RDP",
    3306: "MySQL",
    5432: "PostgreSQL",
    1433: "MSSQL",
}


def _parse_aws_timestamp(ts: str) -> dt.datetime:
    """Parse an AWS timestamp string (ISO 8601) to a UTC-aware datetime."""
    ts = ts.strip()
    if ts.endswith("Z"):
        ts = ts[:-1] + "+00:00"
    try:
        parsed = dt.datetime.fromisoformat(ts)
    except ValueError:
        parsed = dt.datetime.strptime(ts[:19], "%Y-%m-%dT%H:%M:%S").replace(tzinfo=dt.timezone.utc)
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=dt.timezone.utc)
    return parsed


def check_security_group_exposure(profile: str, region: str) -> dict[str, Any]:
    groups = run_aws_json(profile, region, ["ec2", "describe-security-groups"]).get(
        "SecurityGroups", []
    )

    risky: list[dict[str, Any]] = []
    seen: set[tuple[str, int]] = set()
    for group in groups:
        gid = group.get("GroupId", "unknown")
        gname = group.get("GroupName", "unknown")
        for permission in group.get("IpPermissions", []):
            cidrs = extract_cidrs(permission)
            is_public = any(c in {"0.0.0.0/0", "::/0"} for c in cidrs)
            if not is_public:
                continue

            for port, port_label in SENSITIVE_PORTS.items():
                if permission_covers_port(permission, port):
                    key = (str(gid), port)
                    if key not in seen:
                        seen.add(key)
                        risky.append(
                            {
                                "group_id": gid,
                                "group_name": gname,
                                "port": port,
                                "port_name": port_label,
                                "cidrs": cidrs,
                            }
                        )

    port_list = "/".join(str(p) for p in SENSITIVE_PORTS)
    if not risky:
        return {
            "id": "sg-public-admin-ports",
            "title": "Security group internet exposure (admin and database ports)",
            "status": "PASS",
            "findings": [],
            "evidence": [f"No 0.0.0.0/0 or ::/0 exposure found for ports {port_list}"],
        }

    findings = []
    evidence = []
    for item in risky:
        findings.append(
            make_finding(
                severity="HIGH",
                message=(
                    f"{item['port_name']} (port {item['port']}) exposed to internet in {item['group_id']} "
                    f"({item['group_name']})."
                ),
                recommendation="Restrict ingress to trusted CIDRs or private access paths.",
                cost_impact=(
                    "Low. Security group rule changes have no direct AWS charge, "
                    "but migrating to private access paths (for example, VPN or bastion) may introduce service costs."
                ),
            )
        )
        evidence.append(
            f"{item['group_id']} {item['group_name']} port={item['port']} cidrs={','.join(item['cidrs'])}"
        )

    return {
        "id": "sg-public-admin-ports",
        "title": "Security group internet exposure (admin and database ports)",
        "status": "FAIL",
        "findings": findings,
        "evidence": evidence,
    }


def check_iam_user_admin_policy(profile: str, region: str) -> dict[str, Any]:
    users = run_aws_json(profile, region, ["iam", "list-users"]).get("Users", [])
    approved_admin_users = _load_approved_admin_users()
    admin_access_name = "AdministratorAccess"

    matches: list[dict[str, Any]] = []
    for user in users:
        user_name = user.get("UserName")
        if not user_name:
            continue

        attached_user = run_aws_json(
            profile,
            region,
            ["iam", "list-attached-user-policies", "--user-name", str(user_name)],
        ).get("AttachedPolicies", [])

        direct_admin = False
        for policy in attached_user:
            arn = str(policy.get("PolicyArn", ""))
            name = str(policy.get("PolicyName", ""))
            if name == admin_access_name or arn.endswith(f":policy/{admin_access_name}"):
                direct_admin = True
                break

        groups = run_aws_json(
            profile,
            region,
            ["iam", "list-groups-for-user", "--user-name", str(user_name)],
        ).get("Groups", [])

        inherited_groups: list[str] = []
        for group in groups:
            group_name = str(group.get("GroupName", ""))
            if not group_name:
                continue
            attached_group = run_aws_json(
                profile,
                region,
                ["iam", "list-attached-group-policies", "--group-name", group_name],
            ).get("AttachedPolicies", [])
            has_group_admin = any(
                str(policy.get("PolicyName", "")) == admin_access_name
                or str(policy.get("PolicyArn", "")).endswith(f":policy/{admin_access_name}")
                for policy in attached_group
            )
            if has_group_admin:
                inherited_groups.append(group_name)

        if direct_admin or inherited_groups:
            matches.append(
                {
                    "user": str(user_name),
                    "direct": direct_admin,
                    "groups": sorted(set(inherited_groups)),
                }
            )

    if not matches:
        return {
            "id": "iam-user-admin-policy",
            "title": "IAM users with AdministratorAccess (direct or indirect)",
            "status": "PASS",
            "findings": [],
            "evidence": ["No IAM users have AdministratorAccess (directly or through groups)"],
        }

    unauthorized = [m for m in matches if m["user"] not in approved_admin_users]

    evidence = []
    for match in sorted(matches, key=lambda item: item["user"]):
        paths: list[str] = []
        if match["direct"]:
            paths.append("direct")
        if match["groups"]:
            paths.append(f"group:{','.join(match['groups'])}")
        evidence.append(f"user={match['user']} paths={';'.join(paths)}")

    if not unauthorized:
        return {
            "id": "iam-user-admin-policy",
            "title": "IAM users with AdministratorAccess (direct or indirect)",
            "status": "PASS",
            "findings": [],
            "evidence": evidence
            + [
                "All users with AdministratorAccess are in the configured approved allowlist."
            ],
        }

    findings = [
        make_finding(
            severity="MEDIUM",
            message=(
                f"IAM user {m['user']} has AdministratorAccess "
                f"({'directly' if m['direct'] else 'indirectly via group'}), "
                "but is not in the approved allowlist."
            ),
            recommendation=(
                "Remove AdministratorAccess for non-allowlisted users. "
                "Use least-privilege roles/policies and break-glass processes if needed."
            ),
            cost_impact="Low. IAM policy/group attachment changes generally do not add direct AWS charges.",
        )
        for m in sorted(unauthorized, key=lambda item: item["user"])
    ]
    return {
        "id": "iam-user-admin-policy",
        "title": "IAM users with AdministratorAccess (direct or indirect)",
        "status": "WARN",
        "findings": findings,
        "evidence": evidence,
    }


def check_s3_public_access_block(profile: str, region: str) -> dict[str, Any]:
    caller = run_aws_json(profile, region, ["sts", "get-caller-identity"])
    account_id = caller.get("Account")
    if not account_id:
        return {
            "id": "s3-public-access-block-account",
            "title": "S3 account-level Public Access Block",
            "status": "ERROR",
            "findings": [
                make_finding(
                    severity="MEDIUM",
                    message="Could not resolve AWS account ID from STS.",
                    recommendation="Verify credentials and retry.",
                    cost_impact="None expected for this troubleshooting action.",
                )
            ],
            "evidence": ["sts:get-caller-identity returned no account"],
        }

    try:
        block = run_aws_json(
            profile,
            region,
            ["s3control", "get-public-access-block", "--account-id", str(account_id)],
        ).get("PublicAccessBlockConfiguration", {})
        missing_configuration = False
    except AwsCommandError as exc:
        if "NoSuchPublicAccessBlockConfiguration" in str(exc):
            # Account-level block was never configured — treat as all flags disabled.
            block = {}
            missing_configuration = True
        else:
            raise

    required_flags = [
        "BlockPublicAcls",
        "IgnorePublicAcls",
        "BlockPublicPolicy",
        "RestrictPublicBuckets",
    ]
    disabled = [flag for flag in required_flags if not block.get(flag, False)]

    if not disabled:
        return {
            "id": "s3-public-access-block-account",
            "title": "S3 account-level Public Access Block",
            "status": "PASS",
            "findings": [],
            "evidence": ["All four Public Access Block flags are enabled"],
        }

    status = "FAIL" if missing_configuration else "WARN"
    severity = "HIGH" if missing_configuration else "MEDIUM"
    message = (
        "S3 account-level Public Access Block is not configured."
        if missing_configuration
        else "S3 account-level Public Access Block is not fully enabled."
    )
    return {
        "id": "s3-public-access-block-account",
        "title": "S3 account-level Public Access Block",
        "status": status,
        "findings": [
            make_finding(
                severity=severity,
                message=message,
                recommendation="Enable all four account-level Public Access Block settings.",
                cost_impact=(
                    "Low direct AWS cost. Enabling account-level block settings is not billed, "
                    "but may require application or data-sharing workflow adjustments."
                ),
            )
        ],
        "evidence": [f"Disabled flags: {', '.join(disabled)}"],
    }


def check_rds_public_access(profile: str, region: str) -> dict[str, Any]:
    instances = run_aws_json(profile, region, ["rds", "describe-db-instances"]).get(
        "DBInstances", []
    )

    public: list[dict[str, Any]] = []
    evidence: list[str] = []
    for inst in instances:
        identifier = str(inst.get("DBInstanceIdentifier", "unknown"))
        is_public = bool(inst.get("PubliclyAccessible", False))
        engine = str(inst.get("Engine", "unknown"))
        evidence.append(
            f"{identifier}: engine={engine} publiclyAccessible={str(is_public).lower()}"
        )
        if is_public:
            public.append({"identifier": identifier, "engine": engine})

    if not public:
        return {
            "id": "rds-public-access",
            "title": "RDS publicly accessible instances",
            "status": "PASS",
            "findings": [],
            "evidence": evidence or ["No RDS instances found"],
        }

    findings = [
        make_finding(
            severity="HIGH",
            message=(
                f"RDS instance {p['identifier']} ({p['engine']}) is publicly accessible."
            ),
            recommendation=(
                "Set PubliclyAccessible=false and restrict the RDS security group ingress "
                "to app-tier security groups only. Ensure no connection strings rely on the public endpoint."
            ),
            cost_impact="Low. Disabling public access has no direct AWS cost.",
        )
        for p in public
    ]
    return {
        "id": "rds-public-access",
        "title": "RDS publicly accessible instances",
        "status": "FAIL",
        "findings": findings,
        "evidence": evidence,
    }


def check_iam_access_key_age(profile: str, region: str) -> dict[str, Any]:
    now = utc_now()
    users = run_aws_json(profile, region, ["iam", "list-users"]).get("Users", [])

    findings: list[dict[str, Any]] = []
    evidence: list[str] = []

    for user in users:
        user_name = str(user.get("UserName", "")).strip()
        if not user_name:
            continue
        keys = run_aws_json(
            profile, region, ["iam", "list-access-keys", "--user-name", user_name]
        ).get("AccessKeyMetadata", [])

        for key in keys:
            key_id = str(key.get("AccessKeyId", "unknown"))
            status = str(key.get("Status", "unknown"))
            created_str = str(key.get("CreateDate", ""))
            try:
                created = _parse_aws_timestamp(created_str)
                age_days = (now - created).days
            except Exception:
                age_days = -1

            evidence.append(
                f"user={user_name} keyId={key_id[:8]}*** status={status} ageDays={age_days}"
            )

            if status != "Active":
                continue

            if age_days >= 180:
                findings.append(
                    make_finding(
                        severity="HIGH",
                        message=(
                            f"IAM user {user_name} has an active access key ({key_id[:8]}***) "
                            f"that is {age_days} days old (>180 days)."
                        ),
                        recommendation=(
                            "Rotate access keys older than 90 days. For service accounts, "
                            "migrate to IAM roles where possible to eliminate long-lived static keys."
                        ),
                        cost_impact="Low. Key rotation has no direct AWS cost.",
                    )
                )
            elif age_days >= 90:
                findings.append(
                    make_finding(
                        severity="MEDIUM",
                        message=(
                            f"IAM user {user_name} has an active access key ({key_id[:8]}***) "
                            f"that is {age_days} days old (>90 days)."
                        ),
                        recommendation=(
                            "Rotate access keys older than 90 days. For service accounts, "
                            "migrate to IAM roles where possible to eliminate long-lived static keys."
                        ),
                        cost_impact="Low. Key rotation has no direct AWS cost.",
                    )
                )

    if not findings:
        return {
            "id": "iam-access-key-age",
            "title": "IAM access key age",
            "status": "PASS",
            "findings": [],
            "evidence": evidence or ["No IAM users or access keys found"],
        }

    has_high = any(f["severity"] == "HIGH" for f in findings)
    return {
        "id": "iam-access-key-age",
        "title": "IAM access key age",
        "status": "FAIL" if has_high else "WARN",
        "findings": findings,
        "evidence": evidence,
    }


def check_iam_password_policy(profile: str, region: str) -> dict[str, Any]:
    try:
        policy = run_aws_json(
            profile, region, ["iam", "get-account-password-policy"]
        ).get("PasswordPolicy", {})
    except AwsCommandError as exc:
        if "NoSuchEntity" in str(exc):
            return {
                "id": "iam-password-policy",
                "title": "IAM account password policy",
                "status": "FAIL",
                "findings": [
                    make_finding(
                        severity="HIGH",
                        message="No IAM account password policy is configured.",
                        recommendation=(
                            "Create an account password policy: minimum length >= 14, "
                            "require uppercase, lowercase, numbers, and symbols, "
                            "and set max password age <= 90 days."
                        ),
                        cost_impact="Low. Password policy configuration has no direct AWS cost.",
                    )
                ],
                "evidence": ["No password policy found (NoSuchEntity)"],
            }
        raise

    findings: list[dict[str, Any]] = []
    evidence: list[str] = []

    min_length = int(policy.get("MinimumPasswordLength", 0))
    require_upper = bool(policy.get("RequireUppercaseCharacters", False))
    require_lower = bool(policy.get("RequireLowercaseCharacters", False))
    require_numbers = bool(policy.get("RequireNumbers", False))
    require_symbols = bool(policy.get("RequireSymbols", False))
    max_age = policy.get("MaxPasswordAge")
    reuse_prevention = policy.get("PasswordReusePrevention")

    evidence.append(
        f"minLength={min_length} requireUpper={require_upper} requireLower={require_lower} "
        f"requireNumbers={require_numbers} requireSymbols={require_symbols}"
    )
    evidence.append(f"maxPasswordAge={max_age} passwordReusePrevention={reuse_prevention}")

    if min_length < 14:
        findings.append(
            make_finding(
                severity="MEDIUM",
                message=f"Password minimum length is {min_length} (recommended: >= 14).",
                recommendation="Set MinimumPasswordLength to at least 14 in the account password policy.",
                cost_impact="Low. Password policy changes have no direct AWS cost.",
            )
        )

    if not require_symbols:
        findings.append(
            make_finding(
                severity="MEDIUM",
                message="Password policy does not require symbols.",
                recommendation="Enable RequireSymbols in the account password policy.",
                cost_impact="Low. Password policy changes have no direct AWS cost.",
            )
        )

    max_age_val = int(max_age) if max_age is not None else None
    if max_age_val is None or max_age_val > 90:
        max_age_str = "not set" if max_age_val is None else f"{max_age_val} days"
        findings.append(
            make_finding(
                severity="MEDIUM",
                message=f"Password max age is {max_age_str} (recommended: <= 90 days).",
                recommendation="Set MaxPasswordAge to 90 days or fewer in the account password policy.",
                cost_impact="Low. Password policy changes have no direct AWS cost.",
            )
        )

    if not findings:
        return {
            "id": "iam-password-policy",
            "title": "IAM account password policy",
            "status": "PASS",
            "findings": [],
            "evidence": evidence,
        }

    return {
        "id": "iam-password-policy",
        "title": "IAM account password policy",
        "status": "WARN",
        "findings": findings,
        "evidence": evidence,
    }


def check_s3_bucket_public_access(profile: str, region: str) -> dict[str, Any]:
    buckets = run_aws_json(profile, region, ["s3api", "list-buckets"]).get("Buckets", [])

    findings: list[dict[str, Any]] = []
    evidence: list[str] = []

    required_flags = ["BlockPublicAcls", "IgnorePublicAcls", "BlockPublicPolicy", "RestrictPublicBuckets"]

    for bucket in buckets:
        name = str(bucket.get("Name", "unknown"))
        try:
            block = run_aws_json(
                profile, region,
                ["s3api", "get-public-access-block", "--bucket", name],
            ).get("PublicAccessBlockConfiguration", {})
        except AwsCommandError as exc:
            if "NoSuchPublicAccessBlockConfiguration" in str(exc):
                block = {}
            else:
                evidence.append(f"bucket={name} checkError={str(exc)[:80]}")
                continue

        disabled = [f for f in required_flags if not block.get(f, False)]
        if not disabled:
            evidence.append(f"bucket={name} publicAccessBlock=all-enabled")
        else:
            evidence.append(
                f"bucket={name} publicAccessBlock=DISABLED[{','.join(disabled)}]"
            )
            findings.append(
                make_finding(
                    severity="HIGH",
                    message=(
                        f"S3 bucket '{name}' does not have all public access block settings enabled "
                        f"(disabled: {', '.join(disabled)})."
                    ),
                    recommendation=(
                        "Enable all four Public Access Block settings on the bucket. "
                        "Verify no application requires public bucket access before making this change."
                    ),
                    cost_impact="Low. Public access block settings have no direct AWS cost.",
                )
            )

    if not findings:
        return {
            "id": "s3-bucket-public-access",
            "title": "S3 bucket-level public access block",
            "status": "PASS",
            "findings": [],
            "evidence": evidence or ["No S3 buckets found"],
        }

    return {
        "id": "s3-bucket-public-access",
        "title": "S3 bucket-level public access block",
        "status": "FAIL",
        "findings": findings,
        "evidence": evidence,
    }


def check_guardduty_enabled(profile: str, region: str) -> dict[str, Any]:
    detector_ids = run_aws_json(
        profile, region, ["guardduty", "list-detectors"]
    ).get("DetectorIds", [])

    if not detector_ids:
        return {
            "id": "guardduty-enabled",
            "title": "GuardDuty threat detection",
            "status": "FAIL",
            "findings": [
                make_finding(
                    severity="HIGH",
                    message="GuardDuty is not enabled in this region.",
                    recommendation=(
                        "Enable GuardDuty for continuous threat detection covering CloudTrail, "
                        "VPC Flow Logs, and DNS logs. Enable in all active regions."
                    ),
                    cost_impact=(
                        "Low to medium recurring cost. GuardDuty charges per volume of analyzed events "
                        "(CloudTrail management events, VPC Flow Logs, DNS query logs). "
                        "Typical cost for a small account: $1–$30/month."
                    ),
                )
            ],
            "evidence": ["detectorCount=0"],
        }

    evidence: list[str] = []
    findings: list[dict[str, Any]] = []

    for detector_id in detector_ids:
        detector = run_aws_json(
            profile, region, ["guardduty", "get-detector", "--detector-id", detector_id]
        )
        status = str(detector.get("Status", "UNKNOWN")).upper()
        evidence.append(f"detectorId={detector_id} status={status}")

        if status != "ENABLED":
            findings.append(
                make_finding(
                    severity="HIGH",
                    message=(
                        f"GuardDuty detector {detector_id} exists but is not enabled (status={status})."
                    ),
                    recommendation="Enable the GuardDuty detector to resume continuous threat detection.",
                    cost_impact=(
                        "Low to medium recurring cost. GuardDuty charges per volume of analyzed events."
                    ),
                )
            )

    if not findings:
        return {
            "id": "guardduty-enabled",
            "title": "GuardDuty threat detection",
            "status": "PASS",
            "findings": [],
            "evidence": evidence,
        }

    return {
        "id": "guardduty-enabled",
        "title": "GuardDuty threat detection",
        "status": "FAIL",
        "findings": findings,
        "evidence": evidence,
    }


def run_checks(profile: str, region: str) -> list[dict[str, Any]]:
    checks = [
        check_root_mfa,
        check_cloudtrail_baseline,
        check_security_group_exposure,
        check_iam_user_admin_policy,
        check_s3_public_access_block,
        check_rds_public_access,
        check_iam_access_key_age,
        check_iam_password_policy,
        check_s3_bucket_public_access,
        check_guardduty_enabled,
    ]

    results = []
    for check in checks:
        try:
            results.append(check(profile, region))
        except AwsCommandError as exc:
            results.append(
                {
                    "id": check.__name__,
                    "title": check.__name__.replace("_", " "),
                    "status": "ERROR",
                    "findings": [
                        make_finding(
                            severity="MEDIUM",
                            message="AWS CLI command failed while running this check.",
                            recommendation=f"Confirm permissions for AWS profile {profile} and retry.",
                            cost_impact="None expected for retrying this read-only check.",
                        )
                    ],
                    "evidence": [str(exc)],
                }
            )

    return results


def parse_pricing_items(price_list: list[Any]) -> list[dict[str, Any]]:
    items: list[dict[str, Any]] = []
    for item in price_list:
        if isinstance(item, dict):
            items.append(item)
            continue
        if isinstance(item, str):
            try:
                parsed = json.loads(item)
            except json.JSONDecodeError:
                continue
            if isinstance(parsed, dict):
                items.append(parsed)
    return items


def first_price_dimension(item: dict[str, Any]) -> dict[str, Any] | None:
    ondemand = item.get("terms", {}).get("OnDemand", {})
    for offer in ondemand.values():
        dimensions = list(offer.get("priceDimensions", {}).values())
        if not dimensions:
            continue
        for dimension in dimensions:
            if str(dimension.get("beginRange", "")) == "0":
                return dimension
        return dimensions[0]
    return None


def targeted_service_price_samples(profile: str, service_code: str) -> list[dict[str, str]]:
    samples: list[dict[str, str]] = []

    if service_code == "AWSCloudTrail":
        cloudtrail_usagetypes = [
            ("USE1-FreeEventsRecorded", "CloudTrail management events (first copy)"),
            ("USE1-PaidEventsRecorded", "CloudTrail management events (additional copies)"),
            ("USE1-DataEventsRecorded", "CloudTrail data events"),
            ("USE1-NetworkEventsRecorded", "CloudTrail network events"),
            ("USE1-InsightsEvents", "CloudTrail Insights analyzed events"),
            (
                "USE1-Ingestion-Bytes-1yearstore-Live-CloudTrail-Logs",
                "CloudTrail Lake ingestion (live CloudTrail logs)",
            ),
            ("USE1-PaidStorage-ByteHrs", "CloudTrail Lake storage"),
        ]
        for usagetype, label in cloudtrail_usagetypes:
            payload = run_aws_json(
                profile,
                "us-east-1",
                [
                    "pricing",
                    "get-products",
                    "--service-code",
                    service_code,
                    "--filters",
                    f"Type=TERM_MATCH,Field=usagetype,Value={usagetype}",
                    "--max-results",
                    "1",
                ],
            )
            pricing_items = parse_pricing_items(payload.get("PriceList", []))
            if not pricing_items:
                continue
            dimension = first_price_dimension(pricing_items[0])
            if not dimension:
                continue
            samples.append(
                {
                    "usagetype": label,
                    "unit": str(dimension.get("unit", "n/a")),
                    "usd": str(dimension.get("pricePerUnit", {}).get("USD", "n/a")),
                }
            )
        return samples

    if service_code == "AmazonS3":
        s3_queries = [
            (
                [
                    "pricing",
                    "get-products",
                    "--service-code",
                    "AmazonS3",
                    "--filters",
                    "Type=TERM_MATCH,Field=location,Value=US East (N. Virginia)",
                    "Type=TERM_MATCH,Field=volumeType,Value=Standard",
                    "Type=TERM_MATCH,Field=storageClass,Value=General Purpose",
                    "--max-results",
                    "1",
                ],
                "S3 Standard storage",
            ),
            (
                [
                    "pricing",
                    "get-products",
                    "--service-code",
                    "AmazonS3",
                    "--filters",
                    "Type=TERM_MATCH,Field=location,Value=US East (N. Virginia)",
                    "Type=TERM_MATCH,Field=usagetype,Value=Requests-Tier1",
                    "--max-results",
                    "1",
                ],
                "S3 PUT/COPY/POST/LIST requests",
            ),
        ]
        for args, label in s3_queries:
            payload = run_aws_json(profile, "us-east-1", args)
            pricing_items = parse_pricing_items(payload.get("PriceList", []))
            if not pricing_items:
                continue
            dimension = first_price_dimension(pricing_items[0])
            if not dimension:
                continue
            samples.append(
                {
                    "usagetype": label,
                    "unit": str(dimension.get("unit", "n/a")),
                    "usd": str(dimension.get("pricePerUnit", {}).get("USD", "n/a")),
                }
            )
        return samples

    return samples


def extract_price_samples(
    pricing_items: list[dict[str, Any]],
    service_code: str,
    context_text: str,
    limit: int = 4,
) -> list[dict[str, str]]:
    samples: list[dict[str, str]] = []
    seen: set[tuple[str, str, str]] = set()

    service_keywords: dict[str, list[str]] = {
        "AWSCloudTrail": [
            "eventsrecorded",
            "dataevents",
            "insights",
            "networkevents",
            "ingestion",
            "paidevents",
            "paidstorage",
        ],
        "AmazonS3": [
            "request",
            "requests-tier1",
            "timedstorage",
            "storage",
            "standard",
            "put",
            "gb-mo",
        ],
    }
    base_keywords = service_keywords.get(service_code, [])
    contextual = [
        token.lower()
        for token in context_text.replace("/", " ").replace(",", " ").split()
        if len(token) >= 4
    ]
    keywords = sorted(set(base_keywords + contextual))

    for item in pricing_items:
        product = item.get("product", {})
        attributes = product.get("attributes", {})
        usagetype = str(attributes.get("usagetype", "unknown"))

        ondemand = item.get("terms", {}).get("OnDemand", {})
        for offer in ondemand.values():
            for dim in offer.get("priceDimensions", {}).values():
                usd = str(dim.get("pricePerUnit", {}).get("USD", ""))
                unit = str(dim.get("unit", ""))
                if not usd or not unit:
                    continue
                key = (usagetype, unit, usd)
                if key in seen:
                    continue
                seen.add(key)
                samples.append({"usagetype": usagetype, "unit": unit, "usd": usd})

    def score(sample: dict[str, str]) -> tuple[int, int]:
        usagetype = sample["usagetype"].lower()
        unit = sample["unit"].lower()
        keyword_hits = sum(1 for keyword in keywords if keyword in usagetype or keyword in unit)
        usefulness = 0
        if "event" in unit or "request" in unit:
            usefulness += 1
        if "gb" in unit or "gb-mo" in unit:
            usefulness += 1
        return (keyword_hits, usefulness)

    ranked = sorted(samples, key=score, reverse=True)
    return ranked[:limit]


def scenario_cost_hint(usd: str, unit: str) -> str:
    try:
        rate = float(usd)
    except ValueError:
        return "n/a"

    unit_upper = unit.upper()
    if "EVENT" in unit_upper or "REQUEST" in unit_upper:
        low_qty, exp_qty, high_qty = 100_000, 1_000_000, 10_000_000
    elif "GB" in unit_upper:
        low_qty, exp_qty, high_qty = 10, 100, 1_000
    elif "HRS" in unit_upper or "HOUR" in unit_upper:
        low_qty, exp_qty, high_qty = 100, 500, 730
    else:
        low_qty, exp_qty, high_qty = 100, 1_000, 10_000

    low = rate * low_qty
    exp = rate * exp_qty
    high = rate * high_qty
    return "".join(
        [
            f"low~${low:.4f} ({low_qty:,} {unit}), ",
            f"expected~${exp:.4f} ({exp_qty:,} {unit}), ",
            f"high~${high:.4f} ({high_qty:,} {unit})",
        ]
    )


def pricing_products_for_service(profile: str, service_code: str) -> list[dict[str, Any]]:
    with_location = run_aws_json(
        profile,
        "us-east-1",
        [
            "pricing",
            "get-products",
            "--service-code",
            service_code,
            "--filters",
            "Type=TERM_MATCH,Field=location,Value=US East (N. Virginia)",
            "--max-results",
            "100",
        ],
    ).get("PriceList", [])
    items = parse_pricing_items(with_location)
    if items:
        return items

    fallback = run_aws_json(
        profile,
        "us-east-1",
        ["pricing", "get-products", "--service-code", service_code, "--max-results", "100"],
    ).get("PriceList", [])
    return parse_pricing_items(fallback)


def enrich_results_with_cost_reviews(profile: str, results: list[dict[str, Any]]) -> None:
    for result in results:
        for finding in result.get("findings", []):
            services = finding.get("cost_scope_services", [])
            if not isinstance(services, list) or not services:
                continue

            change_summary = str(finding.get("cost_change_summary", "")).strip() or "Not specified"
            reviews: list[dict[str, Any]] = []
            for service_code in services:
                service = str(service_code).strip()
                if not service:
                    continue
                try:
                    samples = targeted_service_price_samples(profile, service)
                    if not samples:
                        pricing_items = pricing_products_for_service(profile, service)
                        context_text = (
                            f"{finding.get('message', '')} "
                            f"{finding.get('recommendation', '')} "
                            f"{change_summary}"
                        )
                        samples = extract_price_samples(
                            pricing_items=pricing_items,
                            service_code=service,
                            context_text=context_text,
                            limit=4,
                        )
                    if not samples:
                        reviews.append(
                            {
                                "service": service,
                                "error": "No pricing samples returned from AWS Pricing API.",
                            }
                        )
                        continue
                    reviews.append(
                        {
                            "service": service,
                            "samples": samples,
                        }
                    )
                except AwsCommandError as exc:
                    reviews.append({"service": service, "error": str(exc)})

            finding["cost_review"] = {
                "change_summary": change_summary,
                "services": reviews,
            }


def severity_totals(results: list[dict[str, Any]]) -> dict[str, int]:
    totals = {"HIGH": 0, "MEDIUM": 0, "LOW": 0, "INFO": 0}
    for result in results:
        for finding in result.get("findings", []):
            sev = str(finding.get("severity", "INFO")).upper()
            if sev not in totals:
                totals["INFO"] += 1
            else:
                totals[sev] += 1
    return totals


def default_output_path(now: dt.datetime) -> Path:
    reports_dir = Path("reports")
    reports_dir.mkdir(parents=True, exist_ok=True)
    stamp = now.strftime("%Y%m%d")
    return reports_dir / f"aws-security-weekly-{stamp}.md"


def render_report(
    now: dt.datetime,
    account_id: str,
    profile: str,
    region: str,
    results: list[dict[str, Any]],
) -> str:
    totals = severity_totals(results)

    lines: list[str] = []
    lines.append("# AWS Weekly Security Baseline Report")
    lines.append("")
    lines.append(f"Generated (UTC): {now.strftime('%Y-%m-%d %H:%M:%SZ')}")
    lines.append(f"AWS Account: {account_id}")
    lines.append(f"AWS Profile: {profile}")
    lines.append(f"AWS Region Scope: {region}")
    lines.append("")
    lines.append("## Summary")
    lines.append("")
    lines.append(f"- High findings: {totals['HIGH']}")
    lines.append(f"- Medium findings: {totals['MEDIUM']}")
    lines.append(f"- Low findings: {totals['LOW']}")
    lines.append(f"- Info findings: {totals['INFO']}")
    lines.append("")
    lines.append("## Check Results")
    lines.append("")

    for result in results:
        lines.append(f"### {result['title']}")
        lines.append("")
        status = result["status"]
        status_emoji = {"PASS": "✅", "WARN": "⚠️", "FAIL": "❌", "ERROR": "🔴"}.get(status, "❓")
        lines.append(f"- Status: {status_emoji} {status}")

        findings = result.get("findings", [])
        if findings:
            lines.append("- Findings:")
            for finding in findings:
                lines.append(
                    f"  - [{finding['severity']}] {finding['message']} "
                    f"Recommendation: {finding['recommendation']} "
                    f"Cost impact: {finding.get('cost_impact', 'Not assessed.')}"
                )
                cost_review = finding.get("cost_review")
                if isinstance(cost_review, dict):
                    lines.append("    Cost review:")
                    lines.append(
                        f"      - Planned change: {cost_review.get('change_summary', 'Not specified')}"
                    )
                    for service_review in cost_review.get("services", []):
                        service = service_review.get("service", "unknown")
                        error = service_review.get("error")
                        if error:
                            lines.append(
                                f"      - Service {service}: pricing lookup failed ({error})"
                            )
                            continue
                        lines.append(
                            f"      - Service {service}: pricing samples from AWS Pricing API (us-east-1)"
                        )
                        for sample in service_review.get("samples", []):
                            usagetype = sample.get("usagetype", "unknown")
                            usd = sample.get("usd", "n/a")
                            unit = sample.get("unit", "n/a")
                            lines.append(
                                f"        - {usagetype}: {usd} USD/{unit}; "
                                f"scenario hint: {scenario_cost_hint(usd, unit)}"
                            )
        else:
            lines.append("- Findings: None")

        evidence = result.get("evidence", [])
        if evidence:
            lines.append("- Evidence:")
            for entry in evidence:
                lines.append(f"  - {entry}")
        lines.append("")

    lines.append("## Notes")
    lines.append("")
    lines.append("- This baseline is read-only and intended for weekly trend tracking.")
    lines.append("- Expand with additional checks over time as your environment matures.")
    lines.append("")
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description="Generate a weekly AWS security baseline report.")
    parser.add_argument(
        "--profile",
        default=os.environ.get("PROFILE") or os.environ.get("AWS_PROFILE") or "wepro-readonly",
        help="AWS profile to use (default: PROFILE, AWS_PROFILE, or wepro-readonly).",
    )
    parser.add_argument("--region", default="us-east-1", help="AWS region to scope regional checks.")
    parser.add_argument("--output", help="Output Markdown path. Defaults to reports/aws-security-weekly-YYYYMMDD.md")
    args = parser.parse_args()

    now = utc_now()

    try:
        caller = run_aws_json(args.profile, args.region, ["sts", "get-caller-identity"])
        account_id = str(caller.get("Account", "unknown"))
    except AwsCommandError as exc:
        sys.stderr.write(f"Failed to resolve caller identity: {exc}\n")
        return 2

    results = run_checks(args.profile, args.region)
    enrich_results_with_cost_reviews(args.profile, results)

    output_path = Path(args.output) if args.output else default_output_path(now)
    output_path.parent.mkdir(parents=True, exist_ok=True)

    report = render_report(now, account_id, args.profile, args.region, results)
    output_path.write_text(report + "\n", encoding="utf-8")

    print(output_path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

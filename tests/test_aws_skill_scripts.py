import base64
import importlib.util
import os
import unittest
from pathlib import Path
from unittest import mock


REPO_ROOT = Path(__file__).resolve().parents[1]


def load_module(name: str, relative_path: str):
    path = REPO_ROOT / relative_path
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    assert spec is not None and spec.loader is not None
    spec.loader.exec_module(module)
    return module


aws_health_review = load_module(
    "aws_health_review",
    "skills/common/aws-health-review/scripts/aws_health_review.py",
)
aws_live_health_review = load_module(
    "aws_live_health_review",
    "skills/common/aws-live-health-review/scripts/aws_live_health_review.py",
)
aws_weekly_security_review = load_module(
    "aws_weekly_security_review",
    "skills/common/aws-weekly-security-review/scripts/aws_weekly_security_review.py",
)


class AwsHealthReviewTests(unittest.TestCase):
    def test_billing_mtd_spend_clamps_prior_month_end(self) -> None:
        requested_periods: list[str] = []

        def fake_run_aws_json_safe(profile: str, region: str, args: list[str]):
            requested_periods.append(args[3])
            return {"ResultsByTime": [{"Total": {"UnblendedCost": {"Amount": "100"}}}]}

        with mock.patch.object(
            aws_health_review,
            "utc_now",
            return_value=aws_health_review.dt.datetime(2026, 3, 31, tzinfo=aws_health_review.dt.timezone.utc),
        ), mock.patch.object(
            aws_health_review,
            "run_aws_json_safe",
            side_effect=fake_run_aws_json_safe,
        ):
            result = aws_health_review.check_billing_mtd_spend("profile", "us-east-1")

        self.assertEqual(result["status"], "PASS")
        self.assertIn("Start=2026-03-01,End=2026-03-31", requested_periods)
        self.assertIn("Start=2026-02-01,End=2026-02-28", requested_periods)


class AwsLiveHealthReviewTests(unittest.TestCase):
    def test_logs_insights_queries_are_chunked_to_twenty_groups(self) -> None:
        start_calls: list[list[str]] = []
        next_query_id = 0

        def fake_run_aws(profile: str, region: str, args: list[str], expect_json: bool = True):
            nonlocal next_query_id
            if args[:2] == ["logs", "start-query"]:
                log_groups = args[args.index("--log-group-names") + 1 :]
                start_calls.append(log_groups)
                next_query_id += 1
                return {"queryId": f"q{next_query_id}"}
            if args[:2] == ["logs", "get-query-results"]:
                query_id = args[-1]
                query_number = int(query_id[1:])
                return {
                    "status": "Complete",
                    "results": [[{"field": "@message", "value": f"batch-{query_number}"}]],
                    "statistics": {"recordsMatched": float(query_number)},
                }
            raise AssertionError(args)

        groups = [f"/aws/apprunner/service-{index}/application" for index in range(21)]
        with mock.patch.object(aws_live_health_review, "run_aws", side_effect=fake_run_aws):
            result = aws_live_health_review.run_logs_insights_query(
                "profile",
                "us-east-1",
                groups,
                aws_live_health_review.dt.datetime(2026, 4, 1, tzinfo=aws_live_health_review.dt.timezone.utc),
                aws_live_health_review.dt.datetime(2026, 4, 2, tzinfo=aws_live_health_review.dt.timezone.utc),
                "fields @message",
            )

        self.assertEqual(len(start_calls), 2)
        self.assertEqual(len(start_calls[0]), 20)
        self.assertEqual(len(start_calls[1]), 1)
        self.assertEqual(result["statistics"]["recordsMatched"], 3.0)


class AwsWeeklySecurityReviewTests(unittest.TestCase):
    def test_scenario_cost_hint_returns_human_string(self) -> None:
        result = aws_weekly_security_review.scenario_cost_hint("0.01", "Requests")
        self.assertIsInstance(result, str)
        self.assertIn("expected~$", result)
        self.assertNotIn("('", result)

    def test_load_approved_admin_users_from_env(self) -> None:
        with mock.patch.dict(
            os.environ,
            {"AWS_WEEKLY_SECURITY_REVIEW_APPROVED_ADMIN_USERS": "alice,bob@example.com"},
            clear=False,
        ):
            self.assertEqual(
                aws_weekly_security_review._load_approved_admin_users(),
                {"alice", "bob@example.com"},
            )

    def test_check_root_mfa_uses_credential_report(self) -> None:
        report = (
            "user,mfa_active\n"
            "alice,true\n"
            "bob,false\n"
        )
        encoded_report = base64.b64encode(report.encode("utf-8")).decode("utf-8")

        def fake_run_aws_json(profile: str, region: str, args: list[str]):
            if args == ["iam", "get-account-summary"]:
                return {"SummaryMap": {"AccountMFAEnabled": 1}}
            if args == ["iam", "list-users"]:
                return {"Users": [{"UserName": "alice"}, {"UserName": "bob"}]}
            if args == ["iam", "get-credential-report"]:
                return {"Content": encoded_report}
            if args and args[:2] == ["iam", "list-mfa-devices"]:
                raise AssertionError("per-user MFA device lookup should not be called")
            raise AssertionError(args)

        with mock.patch.object(
            aws_weekly_security_review,
            "run_aws_json",
            side_effect=fake_run_aws_json,
        ):
            result = aws_weekly_security_review.check_root_mfa("profile", "us-east-1")

        self.assertEqual(result["status"], "WARN")
        self.assertIn("bob", result["findings"][0]["message"])

    def test_s3_public_access_block_missing_configuration_is_fail(self) -> None:
        def fake_run_aws_json(profile: str, region: str, args: list[str]):
            if args == ["sts", "get-caller-identity"]:
                return {"Account": "123456789012"}
            if args[:2] == ["s3control", "get-public-access-block"]:
                raise aws_weekly_security_review.AwsCommandError("NoSuchPublicAccessBlockConfiguration")
            raise AssertionError(args)

        with mock.patch.object(
            aws_weekly_security_review,
            "run_aws_json",
            side_effect=fake_run_aws_json,
        ):
            result = aws_weekly_security_review.check_s3_public_access_block("profile", "us-east-1")

        self.assertEqual(result["status"], "FAIL")
        self.assertEqual(result["findings"][0]["severity"], "HIGH")


if __name__ == "__main__":
    unittest.main()

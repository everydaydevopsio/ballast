from __future__ import annotations

import io
import json
import os
import tempfile
import unittest
from pathlib import Path
import sys
from contextlib import redirect_stdout
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from ballast import cli


class PatchInstallTests(unittest.TestCase):
    @staticmethod
    def make_git_boundary(directory: Path) -> None:
        git_dir = directory / ".git"
        git_dir.mkdir(parents=True, exist_ok=True)
        (git_dir / "HEAD").write_text("ref: refs/heads/main\n", encoding="utf-8")

    def test_build_doctor_report_recommends_upgrades(self) -> None:
        output = cli.build_doctor_report(
            "ballast-python",
            "5.0.2",
            Path("/tmp/project/.rulesrc.json"),
            {
                "targets": ["cursor"],
                "agents": ["linting", "testing"],
                "skills": ["owasp-security-scan"],
                "ballastVersion": "5.0.1",
                "languages": ["typescript", "ansible"],
                "paths": {
                    "typescript": ["apps/web"],
                    "ansible": ["infra/ansible"],
                },
                "tools": {
                    "typescript": ["pnpm", "corepack"],
                    "ansible": ["ansible-lint", "molecule"],
                },
                "discovery": {"excludePaths": ["examples", "tmp"]},
                "taskSystem": "jira",
                "deploymentModel": "serverless",
                "publishingProfiles": ["cli", "web"],
            },
            [
                {
                    "name": "ballast-typescript",
                    "version": "5.0.2",
                    "path": "/tmp/ballast-typescript",
                },
                {
                    "name": "ballast-python",
                    "version": "5.0.1",
                    "path": "/tmp/ballast-python",
                },
                {"name": "ballast-go", "version": None, "path": None},
            ],
        )

        self.assertIn(
            "Run ballast doctor --fix to install or upgrade local Ballast CLIs.",
            output,
        )
        self.assertIn(
            "Refresh .rulesrc.json to Ballast 5.0.2: ballast install --refresh-config",
            output,
        )
        self.assertIn("- targets: cursor", output)
        self.assertIn("- skills: owasp-security-scan", output)
        self.assertIn("- languages: typescript, ansible", output)
        self.assertIn("- paths: typescript=apps/web; ansible=infra/ansible", output)
        self.assertIn(
            "- tools: typescript=pnpm,corepack; ansible=ansible-lint,molecule",
            output,
        )
        self.assertIn("- discovery.excludePaths: examples,tmp", output)
        self.assertIn("- taskSystem: jira", output)
        self.assertIn("- deploymentModel: serverless", output)
        self.assertIn("- publishingProfiles: cli, web", output)

    def test_build_content_writes_rule_marker_and_detects_drift(self) -> None:
        content = cli.build_content("linting", "codex", "python")

        self.assertRegex(
            content,
            r'^<!-- ballast:rule id="python/linting" version="[^"]+" checksum="[a-f0-9]{64}" -->\n',
        )
        self.assertEqual(
            cli.parse_rule_marker(content),
            {
                "ruleId": "python/linting",
                "version": cli.ballast_version(),
                "checksum": cli.parse_rule_marker(content)["checksum"],
            },
        )
        self.assertTrue(cli.verify_rule_checksum(content))
        self.assertFalse(cli.verify_rule_checksum(content + "\nManual edit\n"))

    def test_common_agents_match_packaged_content_dirs(self) -> None:
        common_root = Path(cli.__file__).resolve().parent / "agents" / "common"
        expected = sorted(
            entry.name
            for entry in common_root.iterdir()
            if entry.is_dir()
            and ((entry / "content.md").exists() or any(entry.glob("content-*.md")))
        )

        self.assertEqual(sorted(cli.COMMON_AGENTS), expected)

    def test_install_supports_plan_lifecycle_agent(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root,
                "codex",
                ["plan-lifecycle"],
                [],
                "python",
                False,
                False,
                False,
            )

            self.assertEqual(result.errors, [])
            self.assertIn("plan-lifecycle", result.installed)
            self.assertTrue((root / ".codex" / "rules" / "plan-lifecycle.md").exists())

    def test_rule_marker_requires_generated_header_position(self) -> None:
        marker = (
            '<!-- ballast:rule id="python/linting" version="dev" '
            'checksum="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" -->\n'
        )
        body_example = "# Notes\n\n```md\n" + marker + "```\n"

        self.assertIsNone(cli.parse_rule_marker(body_example))
        self.assertEqual(cli.strip_rule_marker(body_example), body_example)

        with_frontmatter = "---\ntitle: Rule\n---\n" + marker + "# Rule\n"
        parsed = cli.parse_rule_marker(with_frontmatter)
        self.assertIsNotNone(parsed)
        self.assertEqual(parsed["ruleId"], "python/linting")
        self.assertEqual(
            cli.strip_rule_marker(with_frontmatter), "---\ntitle: Rule\n---\n# Rule\n"
        )

    def test_parser_top_level_help_flag_exits_zero(self) -> None:
        with self.assertRaises(SystemExit) as exc:
            cli.parser().parse_args(["--help"])

        self.assertEqual(exc.exception.code, 0)

    def test_parser_top_level_version_flag_exits_zero(self) -> None:
        with self.assertRaises(SystemExit) as exc:
            cli.parser().parse_args(["--version"])

        self.assertEqual(exc.exception.code, 0)

    def test_parser_doctor_command(self) -> None:
        args = cli.parser().parse_args(["doctor"])
        self.assertEqual(args.command, "doctor")

    def test_parser_accepts_repeated_and_comma_separated_targets(self) -> None:
        args = cli.parser().parse_args(
            [
                "install",
                "--target",
                "cursor,claude",
                "--target",
                "codex",
                "--agent",
                "linting",
            ]
        )

        self.assertEqual(args.target, ["cursor,claude", "codex"])

    def test_parser_accepts_deployment_model(self) -> None:
        args = cli.parser().parse_args(
            [
                "install",
                "--target",
                "codex",
                "--agent",
                "publishing",
                "--deployment-model",
                "kubernetes",
            ]
        )

        self.assertEqual(args.deployment_model, "kubernetes")

    def test_parser_accepts_task_system(self) -> None:
        args = cli.parser().parse_args(
            [
                "install",
                "--target",
                "codex",
                "--agent",
                "tasks",
                "--task-system",
                "linear",
            ]
        )

        self.assertEqual(args.task_system, "linear")

    def test_parser_rejects_invalid_task_system(self) -> None:
        with io.StringIO() as buf, mock.patch("sys.stderr", buf):
            with self.assertRaises(SystemExit) as exc:
                cli.parser().parse_args(
                    ["install", "--agent", "tasks", "--task-system", "asana"]
                )
            output = buf.getvalue()

        self.assertEqual(exc.exception.code, 2)
        self.assertIn("invalid choice: 'asana'", output)
        self.assertIn("github", output)
        self.assertIn("jira", output)
        self.assertIn("linear", output)
        self.assertIn("none", output)

    def test_build_content_for_gemini_prefers_non_codex_header(self) -> None:
        content = cli.build_content("linting", "gemini", "python")

        self.assertIn("# Python Linting Rules", content)
        self.assertNotIn("Codex (CLI and app)", content)

    def test_manifest_targets_omit_tools_policy_in_rules(self) -> None:
        for target in ["codex", "claude", "gemini"]:
            content = cli.build_content(
                "testing",
                target,
                "python",
                tools={
                    "python": ["uv", "pyenv"],
                    "typescript": ["pnpm", "corepack"],
                },
            )

            self.assertNotIn("Repository Tool Policy", content)

    def test_cursor_and_opencode_rules_keep_tools_policy(self) -> None:
        for target in ["cursor", "opencode"]:
            content = cli.build_content(
                "testing",
                target,
                "python",
                tools={
                    "python": ["uv", "pyenv"],
                    "typescript": ["pnpm", "corepack"],
                },
            )

            self.assertIn("## Repository Tool Policy", content)
            self.assertIn("python=uv,pyenv", content)
            self.assertIn("typescript=pnpm,corepack", content)
            self.assertIn("uv run <command>", content)
            self.assertIn("pnpm exec", content)

    def test_resolves_content_fragment_includes(self) -> None:
        for language in ["go", "python", "typescript"]:
            content = cli.read_content("testing", language)

            self.assertIn(
                "1. Start from acceptance criteria in `PRD.md`, the linked issue, or the current task.",
                content,
            )
            self.assertNotIn("{{include:", content)

    def test_missing_fragment_include_raises(self) -> None:
        with self.assertRaisesRegex(FileNotFoundError, "does-not-exist.md"):
            cli.resolve_content_includes(
                "{{include:common/fragments/does-not-exist.md}}"
            )

    def test_include_path_escape_rejected(self) -> None:
        with self.assertRaisesRegex(ValueError, "Invalid include path"):
            cli.resolve_content_includes("{{include:../secrets.md}}")

    def test_recursive_fragment_include_raises(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fragments = root / "common" / "fragments"
            fragments.mkdir(parents=True)
            (fragments / "loop.md").write_text(
                "{{include:common/fragments/loop.md}}\n", encoding="utf-8"
            )

            with self.assertRaisesRegex(ValueError, "Recursive include"):
                cli.resolve_content_includes(
                    "{{include:common/fragments/loop.md}}", root
                )

    def test_publishing_suffixes_exclude_opt_in_variants_by_default(self) -> None:
        suffixes = cli.list_rule_suffixes("publishing", "python")

        self.assertNotIn("apt", suffixes)
        self.assertNotIn("brew", suffixes)
        self.assertEqual(len(suffixes), 6)

    def test_publishing_suffixes_honor_explicit_profiles(self) -> None:
        suffixes = cli.list_rule_suffixes(
            "publishing", "python", ["cli", "apt", "brew"]
        )

        self.assertEqual(suffixes, ["cli", "apt", "brew"])

    def test_publishing_api_omits_kubernetes_sections_for_none(self) -> None:
        content = cli.build_content(
            "publishing", "codex", "python", "api", deployment_model="none"
        )

        self.assertNotIn("Kubernetes Helm Chart: Probes Configuration", content)
        self.assertNotIn("Minimal Go Implementation", content)
        self.assertNotIn("BALLAST_IF_DEPLOYMENT", content)

    def test_publishing_api_keeps_kubernetes_sections_for_kubernetes(self) -> None:
        content = cli.build_content(
            "publishing", "codex", "python", "api", deployment_model="kubernetes"
        )

        self.assertIn("Kubernetes Helm Chart: Probes Configuration", content)
        self.assertIn("Minimal Go Implementation", content)
        self.assertNotIn("BALLAST_IF_DEPLOYMENT", content)

    def test_local_dev_has_no_mcp_suffix(self) -> None:
        suffixes = cli.list_rule_suffixes("local-dev", "python")

        self.assertEqual(suffixes, ["badges", "env", "license"])

    def test_manifests_include_tools_policy_once(self) -> None:
        tools = {"python": ["uv", "pyenv"]}
        for builder in [
            cli.build_codex_agents_md,
            cli.build_claude_md,
            cli.build_gemini_md,
        ]:
            content = builder(["testing"], [], "python", tools=tools)

            self.assertIn("### Repository Tool Policy", content)
            self.assertIn("python=uv,pyenv", content)
            self.assertIn("uv run <command>", content)
            self.assertGreater(
                content.index("### Repository Tool Policy"),
                content.index("## Installed agent rules"),
            )
            self.assertEqual(content.count("Repository Tool Policy"), 1)

    def test_patch_handles_crlf_support_files(self) -> None:
        existing = "\r\n".join(
            [
                "# CLAUDE.md",
                "",
                "## Repository Facts",
                "",
                "- Canonical GitHub repo: `<OWNER/REPO>`",
                "",
                "## Installed agent rules",
                "",
                "Created by Ballast. Do not edit this section.",
                "",
                "- `.claude/rules/common/docs.md` — old entry",
                "",
            ]
        )
        canonical = "\n".join(
            [
                "# CLAUDE.md",
                "",
                "## Repository Facts",
                "",
                "- Canonical GitHub repo: `acme/widgets`",
                "",
                "## Installed agent rules",
                "",
                "Created by Ballast. Do not edit this section.",
                "",
                "- `.claude/rules/common/docs.md` — new entry",
                "",
            ]
        )

        merged = cli.patch_codex_agents_md(existing, canonical)

        self.assertIn("- Canonical GitHub repo: `acme/widgets`", merged)
        self.assertIn("new entry", merged)
        self.assertNotIn("old entry", merged)
        self.assertIn("# CLAUDE.md", merged)

    def test_patch_fills_placeholder_repository_facts(self) -> None:
        existing = "\n".join(
            [
                "# CLAUDE.md",
                "",
                "## Repository Facts",
                "",
                "- Canonical GitHub repo: `<OWNER/REPO>`",
                "- Primary package manager: `bun`",
                "- Coverage threshold: `<value>`",
                "",
                "## Installed agent rules",
                "",
                "Created by Ballast. Do not edit this section.",
                "",
            ]
        )
        canonical = "\n".join(
            [
                "# CLAUDE.md",
                "",
                "## Repository Facts",
                "",
                "- Canonical GitHub repo: `acme/widgets`",
                "- Primary package manager: `pnpm`",
                "- Coverage threshold: `<value>`",
                "",
                "## Installed agent rules",
                "",
                "Created by Ballast. Do not edit this section.",
                "",
            ]
        )

        merged = cli.patch_codex_agents_md(existing, canonical)

        self.assertIn("- Canonical GitHub repo: `acme/widgets`", merged)
        self.assertIn("- Primary package manager: `bun`", merged)
        self.assertIn("- Coverage threshold: `<value>`", merged)

    def test_patch_drops_stale_tool_policy_when_canonical_omits_it(self) -> None:
        existing = (
            "# Rules\n\nIntro.\n\n## Repository Tool Policy\n\n"
            "- Check `.rulesrc.json` `tools` before adding, installing, or running language tooling.\n\n"
            "## Keep\n\nBody.\n"
        )
        canonical = "# Rules\n\nIntro.\n\n## Keep\n\nBody.\n"

        merged = cli.patch_rule_content(existing, canonical, "claude")

        self.assertNotIn("Repository Tool Policy", merged)
        self.assertIn("## Keep", merged)

    def test_patch_preserves_user_section_reusing_policy_heading(self) -> None:
        existing = (
            "# Rules\n\n## Repository Tool Policy\n\n"
            "Our team's own notes about tooling, not Ballast-generated.\n"
        )
        canonical = "# Rules\n\nIntro.\n"

        merged = cli.patch_rule_content(existing, canonical, "claude")

        self.assertIn("Our team's own notes about tooling", merged)

    def test_patch_keeps_tool_policy_when_canonical_includes_it(self) -> None:
        existing = "# Rules\n\n## Repository Tool Policy\n\n- Existing bullets.\n"
        canonical = "# Rules\n\n## Repository Tool Policy\n\n- Canonical bullets.\n"

        merged = cli.patch_rule_content(existing, canonical, "claude")

        self.assertIn("## Repository Tool Policy", merged)

    def test_manifests_omit_tools_policy_without_tools(self) -> None:
        content = cli.build_codex_agents_md(["testing"], [], "python")

        self.assertNotIn("Repository Tool Policy", content)

    def test_build_content_renders_publishing_deployment_model_token(self) -> None:
        content = cli.build_content(
            "publishing", "codex", "python", "apps", deployment_model="kubernetes"
        )

        self.assertIn("Kubernetes deployment model", content)
        self.assertIn(
            "Deployment guidance is active (`deploymentModel: kubernetes`).",
            content,
        )
        self.assertIn("charts/<app>/", content)
        self.assertNotIn("{{BALLAST_DEPLOYMENT_MODEL_GUIDANCE}}", content)

    def test_build_content_renders_inactive_deployment_model(self) -> None:
        content = cli.build_content(
            "publishing", "codex", "python", "web", deployment_model="none"
        )

        self.assertIn("Deployment guidance is reference-only", content)
        self.assertIn("Deployment is inactive", content)
        self.assertIn("do not create deploy-on-main workflows", content)

    def test_build_content_renders_docker_deployment_model(self) -> None:
        content = cli.build_content(
            "publishing", "codex", "python", "web", deployment_model="docker"
        )

        self.assertIn("deploymentModel: docker", content)
        self.assertIn("GHCR and Docker Hub", content)
        self.assertIn("image vulnerability scans", content)

    def test_build_content_marks_optional_publishing_variants(self) -> None:
        for suffix in ["apt", "brew"]:
            content = cli.build_content("publishing", "codex", "python", suffix)
            self.assertIn(
                "This optional publishing variant is inactive by default.",
                content,
            )
            self.assertIn(
                "Treat this rule as reference-only unless it is explicitly configured",
                content,
            )

    def test_build_content_renders_none_task_system(self) -> None:
        content = cli.render_task_system_guidance("none")

        self.assertIn("taskSystem: none", content)
        self.assertIn(
            "External issue tracking is disabled (`taskSystem: none`).",
            content,
        )
        self.assertIn("No task-system MCP server is required", content)
        self.assertNotIn("{{BALLAST_TASK_SYSTEM_GUIDANCE}}", content)
        self.assertNotIn("{{taskSystem}}", content)
        self.assertNotIn("All durable work items must be created there", content)

    def test_build_content_renders_active_task_system(self) -> None:
        content = cli.render_task_system_guidance("github")

        self.assertIn(
            "External issue tracking is active (`taskSystem: github`).",
            content,
        )

    def test_apply_task_system_guidance_defaults_missing_task_system(self) -> None:
        content = cli.apply_task_system_guidance(
            "\n".join(
                [
                    "{{BALLAST_TASK_SYSTEM_GUIDANCE}}",
                    'When asked, "configure MCP for {{taskSystem}}"',
                ]
            ),
            "tasks",
            None,
        )

        self.assertIn("**GitHub** as the system of record", content)
        self.assertIn('"configure MCP for GitHub"', content)
        self.assertNotIn("**github**", content)
        self.assertNotIn("{{BALLAST_TASK_SYSTEM_GUIDANCE}}", content)
        self.assertNotIn("{{taskSystem}}", content)

    def test_build_claude_md_lists_split_task_rules(self) -> None:
        content = cli.build_claude_md(["tasks"], [], "python")

        self.assertIn("`.claude/rules/tasks-task-system.md`", content)
        self.assertIn("`.claude/rules/tasks-todo.md`", content)
        self.assertNotIn("`.claude/rules/tasks.md`", content)

        old = os.environ.get("BALLAST_RULE_SUBDIR")
        os.environ["BALLAST_RULE_SUBDIR"] = "common"
        try:
            content = cli.build_claude_md(["tasks"], [], "python")
        finally:
            if old is None:
                os.environ.pop("BALLAST_RULE_SUBDIR", None)
            else:
                os.environ["BALLAST_RULE_SUBDIR"] = old

        self.assertIn("`.claude/rules/common/tasks-task-system.md`", content)
        self.assertIn("`.claude/rules/common/tasks-todo.md`", content)
        self.assertNotIn("`.claude/rules/common/tasks.md`", content)

    def test_destination_rejects_invalid_rule_subdir(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            old = os.environ.get("BALLAST_RULE_SUBDIR")
            os.environ["BALLAST_RULE_SUBDIR"] = "../escape"
            try:
                with self.assertRaisesRegex(ValueError, "Invalid BALLAST_RULE_SUBDIR"):
                    cli.destination(root, "codex", "python-linting")
            finally:
                if old is None:
                    os.environ.pop("BALLAST_RULE_SUBDIR", None)
                else:
                    os.environ["BALLAST_RULE_SUBDIR"] = old

    def test_load_config_supports_legacy_single_target(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".rulesrc.json").write_text(
                '{"target":"cursor","agents":["linting"]}',
                encoding="utf-8",
            )

            config = cli.load_config(root, "python")

            self.assertEqual(config["targets"], ["cursor"])
            self.assertEqual(config["agents"], ["linting"])

    def test_load_config_normalizes_discovery_exclude_paths(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".rulesrc.json").write_text(
                json.dumps(
                    {
                        "targets": ["codex"],
                        "agents": ["linting"],
                        "discovery": {
                            "excludePaths": ["examples", "tmp", "examples", ""]
                        },
                    }
                ),
                encoding="utf-8",
            )

            config = cli.load_config(root, "python")

            self.assertEqual(config["discovery"], {"excludePaths": ["examples", "tmp"]})

    def test_normalize_tools_filters_invalid_entries(self) -> None:
        self.assertEqual(cli.normalize_tools(None), {})
        self.assertEqual(
            cli.normalize_tools(
                {
                    " TypeScript ": ["PNPM", "corepack", "pnpm", "", 42],
                    "": ["ignored"],
                    "python": "uv",
                    123: ["ignored"],
                    "go": ["go"],
                }
            ),
            {
                "typescript": ["pnpm", "corepack"],
                "go": ["go"],
            },
        )

    def test_save_config_defaults_and_preserves_tools(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".rulesrc.json").write_text(
                json.dumps(
                    {
                        "targets": ["codex"],
                        "agents": ["linting"],
                        "languages": ["python"],
                        "paths": {"python": ["."]},
                        "tools": {"python": ["poetry", "pyenv"]},
                    }
                ),
                encoding="utf-8",
            )

            cli.save_config(root, "typescript", "codex", ["linting"])
            config = cli.load_config(root, "typescript")

            self.assertEqual(config["tools"]["python"], ["poetry", "pyenv"])
            self.assertEqual(config["tools"]["typescript"], ["pnpm", "corepack"])

    def test_install_writes_configured_tools_into_codex_and_claude_rules(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".rulesrc.json").write_text(
                json.dumps(
                    {
                        "targets": ["codex", "claude"],
                        "agents": ["testing"],
                        "languages": ["python"],
                        "paths": {"python": ["."]},
                        "tools": {"Python": ["UV", "pyenv"]},
                    }
                ),
                encoding="utf-8",
            )

            for target in ["codex", "claude"]:
                result = cli.install(
                    root,
                    target,
                    ["testing"],
                    [],
                    "python",
                    True,
                    False,
                    False,
                )
                self.assertEqual(result.errors, [])

            for rule_path in [
                root / ".codex" / "rules" / "python-testing.md",
                root / ".claude" / "rules" / "python-testing.md",
            ]:
                content = rule_path.read_text(encoding="utf-8")
                self.assertNotIn("Repository Tool Policy", content)

            for manifest_path in [root / "AGENTS.md", root / "CLAUDE.md"]:
                content = manifest_path.read_text(encoding="utf-8")
                self.assertIn("### Repository Tool Policy", content)
                self.assertIn("python=uv,pyenv", content)
                self.assertIn("uv run <command>", content)
                self.assertEqual(content.count("Repository Tool Policy"), 1)

    def test_load_config_normalizes_publishing_profiles(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".rulesrc.json").write_text(
                json.dumps(
                    {
                        "targets": ["codex"],
                        "agents": ["publishing"],
                        "publishingProfiles": [
                            "APP",
                            " library ",
                            "sdk",
                            "cli",
                            "cli",
                            "",
                            "unknown",
                        ],
                    }
                ),
                encoding="utf-8",
            )

            config = cli.load_config(root, "python")

            self.assertEqual(
                config["publishingProfiles"], ["apps", "libraries", "sdks", "cli"]
            )

    def test_resolve_project_root_supports_ansible_markers(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "ansible.cfg").write_text("[defaults]\n", encoding="utf-8")
            self.make_git_boundary(root)
            nested = root / "roles" / "novnc"
            nested.mkdir(parents=True)

            resolved = cli.resolve_project_root(nested)

            self.assertEqual(resolved, root)

    def test_resolve_project_root_supports_ansible_requirements_yaml(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "requirements.yaml").write_text("---\n", encoding="utf-8")
            self.make_git_boundary(root)
            nested = root / "roles" / "novnc"
            nested.mkdir(parents=True)

            resolved = cli.resolve_project_root(nested)

            self.assertEqual(resolved, root)

    def test_resolve_project_root_supports_terraform_markers(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".terraform-version").write_text("1.8.5\n", encoding="utf-8")
            (root / "versions.tf").write_text("terraform {}\n", encoding="utf-8")
            self.make_git_boundary(root)
            nested = root / "modules" / "network"
            nested.mkdir(parents=True)

            resolved = cli.resolve_project_root(nested)

            self.assertEqual(resolved, root)

    def test_resolve_project_root_supports_docker_markers(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "Dockerfile.prod").write_text("FROM alpine\n", encoding="utf-8")
            self.make_git_boundary(root)
            nested = root / "docker" / "scripts"
            nested.mkdir(parents=True)

            resolved = cli.resolve_project_root(nested)

            self.assertEqual(resolved, root)

    def test_resolve_project_root_ignores_docker_marker_directories(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "Dockerfile.prod").mkdir()
            nested = root / "docker" / "scripts"
            nested.mkdir(parents=True)

            resolved = cli.resolve_project_root(nested)

            self.assertEqual(resolved, nested.resolve())

    def test_resolve_project_root_returns_unmarked_cwd_under_non_git_parent(
        self,
    ) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".rulesrc.json").write_text("{}", encoding="utf-8")
            child = root / "new-product"
            child.mkdir()

            resolved = cli.resolve_project_root(child)

            self.assertEqual(resolved, child)

    def test_resolve_project_root_does_not_cross_git_boundary(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            # parent has a project marker
            (root / "playbook.yml").write_text("---\n", encoding="utf-8")
            child = root / "child-project"
            child.mkdir()
            # child is its own git repo with no project markers
            self.make_git_boundary(child)

            resolved = cli.resolve_project_root(child)

            self.assertEqual(resolved, child)

    def test_resolve_project_root_returns_child_repo_root_for_nested_cwd(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "playbook.yml").write_text("---\n", encoding="utf-8")
            child = root / "child-project"
            nested = child / "subdir"
            nested.mkdir(parents=True)
            self.make_git_boundary(child)

            resolved = cli.resolve_project_root(nested)

            self.assertEqual(resolved, child)

    def test_normalize_target_tokens_ignores_non_string_items(self) -> None:
        self.assertEqual(
            cli.normalize_target_tokens(["cursor,claude", 7, None, "codex"]),
            ["cursor", "claude", "codex"],
        )

    def test_run_install_writes_shared_rulesrc_for_explicit_flags(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.make_git_boundary(root)
            old_cwd = Path.cwd()
            os.chdir(root)
            try:
                args = cli.parser().parse_args(
                    ["install", "--target", "codex", "--all", "--yes"]
                )
                exit_code = cli.run_install(args)
            finally:
                os.chdir(old_cwd)

            self.assertEqual(exit_code, 0)
            self.assertTrue((root / ".rulesrc.json").exists())
            content = (root / ".rulesrc.json").read_text(encoding="utf-8")
            self.assertIn('"ballastVersion":', content)
            self.assertIn('"languages": [', content)
            self.assertIn('"python"', content)
            self.assertIn('"paths": {', content)
            self.assertIn('"plan-lifecycle"', content)
            self.assertTrue((root / ".codex" / "rules" / "plan-lifecycle.md").exists())

    def test_run_install_writes_deployment_model_for_publishing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.make_git_boundary(root)
            old_cwd = Path.cwd()
            os.chdir(root)
            try:
                args = cli.parser().parse_args(
                    [
                        "install",
                        "--target",
                        "codex",
                        "--agent",
                        "publishing",
                        "--deployment-model",
                        "hosted",
                        "--yes",
                    ]
                )
                exit_code = cli.run_install(args)
            finally:
                os.chdir(old_cwd)

            self.assertEqual(exit_code, 0)
            config = (root / ".rulesrc.json").read_text(encoding="utf-8")
            self.assertIn('"deploymentModel": "hosted"', config)
            content = (root / ".codex" / "rules" / "publishing-apps.md").read_text(
                encoding="utf-8"
            )
            self.assertIn("Hosted platform deployment model", content)

    def test_run_install_writes_task_system_for_tasks(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.make_git_boundary(root)
            old_cwd = Path.cwd()
            os.chdir(root)
            try:
                args = cli.parser().parse_args(
                    [
                        "install",
                        "--target",
                        "codex",
                        "--agent",
                        "tasks",
                        "--task-system",
                        "none",
                        "--yes",
                    ]
                )
                exit_code = cli.run_install(args)
            finally:
                os.chdir(old_cwd)

            self.assertEqual(exit_code, 0)
            config = (root / ".rulesrc.json").read_text(encoding="utf-8")
            self.assertIn('"taskSystem": "none"', config)
            task_rule = root / ".codex" / "rules" / "tasks-task-system.md"
            self.assertTrue(task_rule.exists())
            content = task_rule.read_text(encoding="utf-8")
            self.assertIn("taskSystem: none", content)
            self.assertIn("No task-system MCP server is required", content)
            self.assertNotIn("{{taskSystem}}", content)

    def test_run_install_defaults_tasks_to_github(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.make_git_boundary(root)
            old_cwd = Path.cwd()
            os.chdir(root)
            try:
                args = cli.parser().parse_args(
                    [
                        "install",
                        "--target",
                        "codex",
                        "--agent",
                        "tasks",
                        "--yes",
                    ]
                )
                exit_code = cli.run_install(args)
            finally:
                os.chdir(old_cwd)

            self.assertEqual(exit_code, 0)
            config = (root / ".rulesrc.json").read_text(encoding="utf-8")
            self.assertIn('"taskSystem": "github"', config)
            content = (root / ".codex" / "rules" / "tasks-task-system.md").read_text(
                encoding="utf-8"
            )
            self.assertIn("**GitHub** as the system of record", content)
            self.assertNotIn("{{taskSystem}}", content)

    def test_run_install_changing_task_system_refreshes_existing_task_rules(
        self,
    ) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.make_git_boundary(root)
            old_cwd = Path.cwd()
            os.chdir(root)
            try:
                first_args = cli.parser().parse_args(
                    [
                        "install",
                        "--target",
                        "codex",
                        "--agent",
                        "tasks",
                        "--task-system",
                        "jira",
                        "--yes",
                    ]
                )
                second_args = cli.parser().parse_args(
                    [
                        "install",
                        "--target",
                        "codex",
                        "--agent",
                        "tasks",
                        "--task-system",
                        "linear",
                        "--yes",
                    ]
                )
                self.assertEqual(cli.run_install(first_args), 0)
                self.assertEqual(cli.run_install(second_args), 0)
            finally:
                os.chdir(old_cwd)

            config = (root / ".rulesrc.json").read_text(encoding="utf-8")
            self.assertIn('"taskSystem": "linear"', config)
            content = (root / ".codex" / "rules" / "tasks-task-system.md").read_text(
                encoding="utf-8"
            )
            self.assertIn("taskSystem: linear", content)
            self.assertIn("**Linear** as the system of record", content)
            self.assertNotIn("taskSystem: jira", content)
            self.assertNotIn("**jira** as the system of record", content)

    def test_run_install_writes_multi_target_shared_rulesrc(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.make_git_boundary(root)
            old_cwd = Path.cwd()
            os.chdir(root)
            try:
                args = cli.parser().parse_args(
                    [
                        "install",
                        "--target",
                        "cursor",
                        "--target",
                        "claude",
                        "--agent",
                        "linting",
                        "--yes",
                    ]
                )
                exit_code = cli.run_install(args)
            finally:
                os.chdir(old_cwd)

            self.assertEqual(exit_code, 0)
            self.assertTrue(
                (root / ".cursor" / "rules" / "python-linting.mdc").exists()
            )
            self.assertTrue((root / ".claude" / "rules" / "python-linting.md").exists())
            content = (root / ".rulesrc.json").read_text(encoding="utf-8")
            self.assertIn('"targets": [', content)
            self.assertIn('"cursor"', content)
            self.assertIn('"claude"', content)

    def test_manual_installs_accumulate_languages_in_shared_rulesrc(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            cli.save_config(
                root, "python", ["claude"], ["linting"], ["owasp-security-scan"]
            )
            cli.save_config(
                root, "go", ["claude"], ["linting"], ["owasp-security-scan"]
            )

            content = (root / ".rulesrc.json").read_text(encoding="utf-8")
            self.assertIn('"ballastVersion":', content)
            self.assertIn('"languages": [', content)
            self.assertIn('"python"', content)
            self.assertIn('"go"', content)
            self.assertIn('"python": [', content)
            self.assertIn('"go": [', content)
            self.assertIn('"skills": [', content)
            self.assertIn('"targets": [', content)

    def test_save_config_preserves_discovery_exclude_paths(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".rulesrc.json").write_text(
                json.dumps(
                    {
                        "targets": ["codex"],
                        "agents": ["linting"],
                        "discovery": {"excludePaths": ["examples"]},
                    }
                ),
                encoding="utf-8",
            )

            cli.save_config(root, "python", ["codex"], ["linting"], [])

            config = json.loads((root / ".rulesrc.json").read_text(encoding="utf-8"))
            self.assertEqual(config["discovery"], {"excludePaths": ["examples"]})

    def test_install_supports_ansible_language_profile(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root,
                "codex",
                ["linting"],
                [],
                "ansible",
                False,
                False,
                False,
            )

            self.assertIn("linting", result.installed)
            rule = root / ".codex" / "rules" / "ansible-linting.md"
            self.assertTrue(rule.exists())
            self.assertIn(
                "Ansible linting specialist", rule.read_text(encoding="utf-8")
            )

    def test_install_supports_terraform_language_profile(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root,
                "codex",
                ["linting"],
                [],
                "terraform",
                False,
                False,
                False,
            )

            self.assertIn("linting", result.installed)
            rule = root / ".codex" / "rules" / "terraform-linting.md"
            self.assertTrue(rule.exists())
            content = rule.read_text(encoding="utf-8")
            self.assertIn("Terraform linting specialist", content)
            self.assertIn(".terraform-version", content)
            self.assertIn("tfenv install", content)
            self.assertIn("trivy config", content)
            self.assertIn("tfsec", content)
            self.assertIn("plugin blocks", content)
            self.assertIn("tflint --init", content)
            self.assertIn("OpenTofu", content)
            self.assertIn("tofu fmt", content)

    def test_install_supports_terraform_testing_best_practices(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root,
                "codex",
                ["testing"],
                [],
                "terraform",
                False,
                False,
                False,
            )

            self.assertIn("testing", result.installed)
            rule = root / ".codex" / "rules" / "terraform-testing.md"
            self.assertTrue(rule.exists())
            content = rule.read_text(encoding="utf-8")
            self.assertIn("terraform test", content)
            self.assertIn("Terraform 1.6", content)
            self.assertIn("Terratest", content)
            self.assertIn("concurrency:", content)
            self.assertIn("PR validation", content)
            self.assertIn("OpenTofu", content)
            self.assertIn("tofu test", content)

    def test_install_supports_dart_flutter_language_profile(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root,
                "codex",
                ["linting", "logging", "testing"],
                [],
                "dart",
                False,
                False,
                False,
            )

            self.assertIn("linting", result.installed)
            linting = root / ".codex" / "rules" / "dart-linting.md"
            logging = root / ".codex" / "rules" / "dart-logging.md"
            testing = root / ".codex" / "rules" / "dart-testing.md"
            git_hooks = root / ".codex" / "rules" / "git-hooks.md"
            self.assertTrue(linting.exists())
            self.assertTrue(logging.exists())
            self.assertTrue(testing.exists())
            self.assertTrue(git_hooks.exists())
            self.assertIn("flutter_lints", linting.read_text(encoding="utf-8"))
            self.assertIn("flutter analyze", linting.read_text(encoding="utf-8"))
            self.assertIn("dart:developer", logging.read_text(encoding="utf-8"))
            self.assertIn("package:logging", logging.read_text(encoding="utf-8"))
            self.assertIn("flutter_test", testing.read_text(encoding="utf-8"))
            self.assertIn("integration_test", testing.read_text(encoding="utf-8"))
            git_hooks_content = git_hooks.read_text(encoding="utf-8")
            self.assertIn("dart format --set-exit-if-changed", git_hooks_content)
            self.assertIn("flutter analyze", git_hooks_content)
            self.assertIn("flutter test", git_hooks_content)

    def test_install_supports_docker_language_profile(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root,
                "codex",
                ["linting", "logging", "testing"],
                [],
                "docker",
                False,
                False,
                False,
            )

            self.assertIn("linting", result.installed)
            linting = root / ".codex" / "rules" / "docker-linting.md"
            logging = root / ".codex" / "rules" / "docker-logging.md"
            testing = root / ".codex" / "rules" / "docker-testing.md"
            git_hooks = root / ".codex" / "rules" / "git-hooks.md"
            self.assertTrue(linting.exists())
            self.assertTrue(logging.exists())
            self.assertTrue(testing.exists())
            self.assertTrue(git_hooks.exists())
            self.assertIn("hadolint", linting.read_text(encoding="utf-8"))
            self.assertIn("container logs", logging.read_text(encoding="utf-8"))
            self.assertIn("docker build", testing.read_text(encoding="utf-8"))
            git_hooks_content = git_hooks.read_text(encoding="utf-8")
            self.assertIn("docker compose config", git_hooks_content)
            self.assertIn("image vulnerability scans", git_hooks_content)

    def test_install_creates_skill_files(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root,
                "claude",
                ["linting"],
                ["owasp-security-scan"],
                "python",
                False,
                False,
                False,
            )

            self.assertIn("owasp-security-scan", result.installed_skills)
            skill = root / ".claude" / "skills" / "owasp-security-scan.skill"
            self.assertTrue(skill.exists())
            self.assertTrue(skill.read_bytes().startswith(b"PK\x03\x04"))

    def test_install_skips_existing_managed_skill_without_force(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            skill = root / ".opencode" / "skills" / "owasp-security-scan.md"
            skill.parent.mkdir(parents=True)
            skill.write_text("stale skill content", encoding="utf-8")

            result = cli.install(
                root,
                "opencode",
                [],
                ["owasp-security-scan"],
                "python",
                False,
                False,
                False,
            )

            self.assertEqual(result.installed_skills, [])
            self.assertEqual(skill.read_text(encoding="utf-8"), "stale skill content")

    def test_install_refresh_overwrites_existing_managed_skill(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            skill = root / ".opencode" / "skills" / "owasp-security-scan.md"
            skill.parent.mkdir(parents=True)
            skill.write_text("stale skill content", encoding="utf-8")

            with mock.patch.dict(os.environ, {"BALLAST_REFRESH_SKILLS": "1"}):
                result = cli.install(
                    root,
                    "opencode",
                    [],
                    ["owasp-security-scan"],
                    "python",
                    False,
                    False,
                    False,
                )

            self.assertEqual(result.installed_skills, ["owasp-security-scan"])
            self.assertIn(
                "## Scan Architecture",
                skill.read_text(encoding="utf-8"),
            )
            self.assertIn(
                "Created by [Ballast]",
                skill.read_text(encoding="utf-8"),
            )

    def test_install_adds_ballast_to_gitignore(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            cli.install(
                root,
                "cursor",
                ["linting"],
                [],
                "python",
                False,
                False,
                False,
            )

            self.assertIn(
                ".ballast/", (root / ".gitignore").read_text(encoding="utf-8")
            )

    def test_install_records_gitignore_error_and_continues(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".gitignore").mkdir()

            result = cli.install(
                root,
                "cursor",
                ["linting"],
                [],
                "python",
                False,
                False,
                False,
            )

            self.assertTrue(any(agent == "gitignore" for agent, _ in result.errors))
            self.assertTrue(
                (root / ".cursor" / "rules" / "python-linting.mdc").exists()
            )

    def test_install_supports_publishing_agent(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root,
                "cursor",
                ["publishing"],
                [],
                "python",
                False,
                False,
                False,
            )

            self.assertIn("publishing", result.installed)
            self.assertTrue(
                (root / ".cursor" / "rules" / "publishing-libraries.mdc").exists()
            )
            self.assertTrue(
                (root / ".cursor" / "rules" / "publishing-sdks.mdc").exists()
            )
            self.assertTrue(
                (root / ".cursor" / "rules" / "publishing-apps.mdc").exists()
            )

    def test_install_supports_docs_agent(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root,
                "cursor",
                ["docs"],
                [],
                "python",
                False,
                False,
                False,
            )

            self.assertIn("docs", result.installed)
            rule = root / ".cursor" / "rules" / "docs.mdc"
            self.assertTrue(rule.exists())
            self.assertIn("Documentation Agent", rule.read_text(encoding="utf-8"))
            self.assertIn("publish-docs", rule.read_text(encoding="utf-8"))
            self.assertTrue(rule.read_text(encoding="utf-8").startswith("---\n"))

    def test_install_supports_docs_agent_for_opencode_with_frontmatter(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root,
                "opencode",
                ["docs"],
                [],
                "python",
                False,
                False,
                False,
            )

            self.assertIn("docs", result.installed)
            rule = root / ".opencode" / "docs.md"
            self.assertTrue(rule.exists())
            content = rule.read_text(encoding="utf-8")
            self.assertTrue(content.startswith("---\n"))
            self.assertIn("mode: subagent", content)
            self.assertIn("Documentation Agent", content)

    def test_parse_skill_tokens_supports_all(self) -> None:
        self.assertEqual(
            cli.parse_skill_tokens(None, True, "python"),
            [
                "owasp-security-scan",
                "aws-health-review",
                "aws-live-health-review",
                "aws-weekly-security-review",
                "github-health-check",
                "github-pr-copilot-cycle",
                "ballast-audit",
                "ballast-project-maintenance",
                "docker-registry-publish",
                "speckit-bootstrap",
                "speckit-reverse-engineer",
                "speckit-delivery",
            ],
        )
        self.assertEqual(
            cli.parse_skill_tokens(
                "owasp-security-scan,aws-health-review,github-health-check",
                False,
                "python",
            ),
            ["owasp-security-scan", "aws-health-review", "github-health-check"],
        )

    def test_build_cursor_skill_format_uses_folded_description_text(self) -> None:
        content = cli.build_cursor_skill_format("owasp-security-scan", "python")

        self.assertIn("alwaysApply: false", content)
        self.assertIn(
            'description: "Run OWASP-aligned security scans across Go, TypeScript, and Python codebases.',
            content,
        )
        self.assertIn("Created by [Ballast]", content)
        self.assertNotIn("description: >", content)

    def test_build_cursor_skill_format_supports_ballast_audit(self) -> None:
        content = cli.build_cursor_skill_format("ballast-audit", "python")

        self.assertIn("alwaysApply: false", content)
        self.assertIn(
            'description: "audit AI rule and skill files for context density, duplication, and bloat"',
            content,
        )
        self.assertIn("# Ballast Audit Skill", content)

    def test_build_support_file_includes_skills(self) -> None:
        content = cli.build_codex_agents_md(
            ["linting"], ["owasp-security-scan"], "python"
        )
        self.assertIn("## Repository Facts", content)
        self.assertIn("Canonical GitHub repo: `<OWNER/REPO>`", content)
        self.assertIn("## Installed skills", content)
        self.assertIn("`.codex/skills/owasp-security-scan/SKILL.md`", content)

    def test_build_gemini_md_includes_repository_facts_and_rules(self) -> None:
        content = cli.build_gemini_md(["linting"], ["owasp-security-scan"], "python")

        self.assertIn("# GEMINI.md", content)
        self.assertIn("## Repository Facts", content)
        self.assertIn("## Memory Tiering", content)
        self.assertIn("## Installed agent rules", content)
        self.assertIn("`.gemini/rules/python-linting.md`", content)
        self.assertIn("## Installed skills", content)
        self.assertIn("`.gemini/rules/owasp-security-scan.md`", content)

    def test_skill_destination_returns_expected_paths(self) -> None:
        root = Path("/tmp/project")
        self.assertEqual(
            cli.skill_destination(root, "cursor", "owasp-security-scan"),
            root / ".cursor" / "rules" / "owasp-security-scan.mdc",
        )
        self.assertEqual(
            cli.skill_destination(root, "claude", "owasp-security-scan"),
            root / ".claude" / "skills" / "owasp-security-scan.skill",
        )
        self.assertEqual(
            cli.skill_destination(root, "opencode", "owasp-security-scan"),
            root / ".opencode" / "skills" / "owasp-security-scan.md",
        )
        self.assertEqual(
            cli.skill_destination(root, "codex", "owasp-security-scan"),
            root / ".codex" / "skills" / "owasp-security-scan" / "SKILL.md",
        )
        with self.assertRaisesRegex(ValueError, "Unknown target: bogus"):
            cli.skill_destination(root, "bogus", "owasp-security-scan")

    def test_resolve_target_and_agents_uses_saved_skill_only_config(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            cli.save_config(root, "python", ["claude"], [], ["owasp-security-scan"])

            args = cli.parser().parse_args(["install", "--yes"])
            resolved = cli.resolve_target_and_agents(args, root, "python")

            self.assertEqual(
                resolved, (["claude"], [], ["owasp-security-scan"], "", "")
            )

    def test_resolve_target_and_agents_supports_multi_target_config(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            cli.save_config(
                root,
                "python",
                ["cursor", "claude"],
                ["linting"],
                ["owasp-security-scan"],
            )

            args = cli.parser().parse_args(["install", "--yes"])
            resolved = cli.resolve_target_and_agents(args, root, "python")

            self.assertEqual(
                resolved,
                (
                    ["cursor", "claude"],
                    ["linting", "git-hooks"],
                    ["owasp-security-scan"],
                    "",
                    "",
                ),
            )

    def test_resolve_target_and_agents_prompts_for_task_system(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            args = cli.parser().parse_args(["install"])

            with (
                mock.patch.object(cli, "is_ci_mode", return_value=False),
                mock.patch.object(cli, "prompt_targets", return_value=["codex"]),
                mock.patch.object(cli, "prompt_agents", return_value=["tasks"]),
                mock.patch.object(cli, "prompt_skills", return_value=[]),
                mock.patch.object(cli, "prompt_task_system", return_value="jira"),
            ):
                resolved = cli.resolve_target_and_agents(args, root, "python")

            self.assertEqual(resolved, (["codex"], ["tasks"], [], "", "jira"))

    def test_resolve_target_and_agents_prompts_for_multiple_targets(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            args = cli.parser().parse_args(["install"])

            with (
                mock.patch.object(cli, "is_ci_mode", return_value=False),
                mock.patch.object(
                    cli, "prompt_targets", return_value=["cursor", "claude"]
                ),
                mock.patch.object(cli, "prompt_agents", return_value=["linting"]),
                mock.patch.object(
                    cli, "prompt_skills", return_value=["owasp-security-scan"]
                ),
            ):
                resolved = cli.resolve_target_and_agents(args, root, "python")

            self.assertEqual(
                resolved,
                (
                    ["cursor", "claude"],
                    ["linting"],
                    ["owasp-security-scan"],
                    "",
                    "",
                ),
            )

    def test_install_skill_only_updates_codex_support_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root,
                "codex",
                [],
                ["owasp-security-scan"],
                "python",
                False,
                False,
                False,
            )

            self.assertEqual(result.installed, [])
            self.assertEqual(result.installed_skills, ["owasp-security-scan"])
            self.assertTrue(
                (
                    root / ".codex" / "skills" / "owasp-security-scan" / "SKILL.md"
                ).exists()
            )
            self.assertTrue(
                (
                    root
                    / ".codex"
                    / "skills"
                    / "owasp-security-scan"
                    / "references"
                    / "owasp-mapping.md"
                ).exists()
            )
            agents_md = (root / "AGENTS.md").read_text(encoding="utf-8")
            self.assertIn("## Installed skills", agents_md)
            self.assertIn("`.codex/skills/owasp-security-scan/SKILL.md`", agents_md)

    def test_refresh_removes_legacy_codex_rule_format_skill(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            legacy = root / ".codex" / "rules" / "owasp-security-scan.md"
            legacy.parent.mkdir(parents=True, exist_ok=True)
            legacy.write_text(
                "<!-- Created by Ballast. Do not edit this section. -->\n\nlegacy",
                encoding="utf-8",
            )

            with mock.patch.dict(os.environ, {"BALLAST_REFRESH_SKILLS": "1"}):
                result = cli.install(
                    root,
                    "codex",
                    [],
                    ["owasp-security-scan"],
                    "python",
                    False,
                    False,
                    False,
                )

            self.assertEqual(result.installed_skills, ["owasp-security-scan"])
            self.assertFalse(legacy.exists())
            self.assertTrue(
                (
                    root / ".codex" / "skills" / "owasp-security-scan" / "SKILL.md"
                ).exists()
            )

    def test_install_skips_existing_skill_when_force_and_patch_are_false(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            skill_path = root / ".opencode" / "skills" / "owasp-security-scan.md"
            skill_path.parent.mkdir(parents=True, exist_ok=True)
            skill_path.write_text("stale skill content", encoding="utf-8")

            result = cli.install(
                root,
                "opencode",
                [],
                ["owasp-security-scan"],
                "python",
                False,
                False,
                False,
            )

            self.assertEqual(result.installed_skills, [])
            self.assertEqual(
                skill_path.read_text(encoding="utf-8"), "stale skill content"
            )

    def test_install_patch_creates_missing_skill(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root,
                "opencode",
                [],
                ["owasp-security-scan"],
                "python",
                False,
                True,
                False,
            )

            self.assertEqual(result.installed_skills, ["owasp-security-scan"])
            skill_path = root / ".opencode" / "skills" / "owasp-security-scan.md"
            self.assertIn(
                "## Scan Architecture",
                skill_path.read_text(encoding="utf-8"),
            )

    def test_install_patch_merges_existing_skill(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            skill_path = root / ".cursor" / "rules" / "owasp-security-scan.mdc"
            skill_path.parent.mkdir(parents=True, exist_ok=True)
            skill_path.write_text(
                """---
description: Team customized skill
alwaysApply: true
---

Team intro.

## Usage

Keep team-specific usage notes.
""",
                encoding="utf-8",
            )

            result = cli.install(
                root,
                "cursor",
                [],
                ["owasp-security-scan"],
                "python",
                False,
                True,
                False,
            )

            self.assertEqual(result.installed_skills, ["owasp-security-scan"])
            content = skill_path.read_text(encoding="utf-8")
            self.assertIn("description: Team customized skill", content)
            self.assertIn("alwaysApply: true", content)
            self.assertIn("Keep team-specific usage notes.", content)
            self.assertIn("## Scan Architecture", content)

    def test_install_patch_merges_claude_skill_archive(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            skill_path = root / ".claude" / "skills" / "owasp-security-scan.skill"
            skill_path.parent.mkdir(parents=True, exist_ok=True)
            existing_skill_content = (
                "# owasp-security-scan\n\n"
                "Team intro preserved by patch.\n\n"
                "## Team Custom Section\n\n"
                "Keep this team-specific section.\n"
            )
            skill_path.write_bytes(
                cli.build_claude_skill(
                    "owasp-security-scan", "python", existing_skill_content
                )
            )

            result = cli.install(
                root,
                "claude",
                [],
                ["owasp-security-scan"],
                "python",
                False,
                True,
                False,
            )

            self.assertEqual(result.installed_skills, ["owasp-security-scan"])
            skill_md = cli.read_claude_skill_content(skill_path)
            self.assertIn("Team intro preserved by patch.", skill_md)
            self.assertIn("Team Custom Section", skill_md)
            self.assertIn("## Scan Architecture", skill_md)

    def test_install_patch_overwrites_unreadable_claude_skill_archive(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            skill_path = root / ".claude" / "skills" / "owasp-security-scan.skill"
            skill_path.parent.mkdir(parents=True, exist_ok=True)
            skill_path.write_bytes(b"not-a-zip-archive")

            result = cli.install(
                root,
                "claude",
                [],
                ["owasp-security-scan"],
                "python",
                False,
                True,
                False,
            )

            self.assertEqual(result.errors, [])
            self.assertEqual(result.installed_skills, ["owasp-security-scan"])
            skill_md = cli.read_claude_skill_content(skill_path)
            self.assertIn("## Scan Architecture", skill_md)
            self.assertNotIn("not-a-zip-archive", skill_md)

    def test_install_force_overwrites_existing_claude_skill_archive(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            skill_path = root / ".claude" / "skills" / "owasp-security-scan.skill"
            skill_path.parent.mkdir(parents=True, exist_ok=True)
            existing_skill_content = (
                "# owasp-security-scan\n\n"
                "Team intro that should be discarded on force.\n\n"
                "## Team Custom Section\n\n"
                "This section should be gone after force.\n"
            )
            skill_path.write_bytes(
                cli.build_claude_skill(
                    "owasp-security-scan", "python", existing_skill_content
                )
            )

            result = cli.install(
                root,
                "claude",
                [],
                ["owasp-security-scan"],
                "python",
                True,
                False,
                False,
            )

            self.assertEqual(result.installed_skills, ["owasp-security-scan"])
            skill_md = cli.read_claude_skill_content(skill_path)
            self.assertNotIn("Team intro that should be discarded on force.", skill_md)
            self.assertNotIn("Team Custom Section", skill_md)
            self.assertIn("## Scan Architecture", skill_md)

    def test_install_force_overwrites_existing_skill(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            skill_path = root / ".opencode" / "skills" / "owasp-security-scan.md"
            skill_path.parent.mkdir(parents=True, exist_ok=True)
            skill_path.write_text("stale skill content", encoding="utf-8")

            result = cli.install(
                root,
                "opencode",
                [],
                ["owasp-security-scan"],
                "python",
                True,
                False,
                False,
            )

            self.assertEqual(result.installed_skills, ["owasp-security-scan"])
            content = skill_path.read_text(encoding="utf-8")
            self.assertIn("## Scan Architecture", content)
            self.assertNotEqual(content, "stale skill content")

    def test_install_creates_language_prefixed_rule_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root, "codex", ["linting"], [], "python", False, False, False
            )

            self.assertIn("linting", result.installed)
            rule = root / ".codex" / "rules" / "python-linting.md"
            self.assertTrue(rule.exists())
            self.assertIn("Python linting specialist", rule.read_text(encoding="utf-8"))

    def test_install_moves_python_hook_guidance_to_dedicated_rule(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root, "codex", ["linting"], [], "python", False, False, False
            )

            self.assertIn("linting", result.installed)
            rule = root / ".codex" / "rules" / "python-linting.md"
            content = rule.read_text(encoding="utf-8")
            self.assertNotIn("{{BALLAST_HOOK_GUIDANCE}}", content)
            self.assertNotIn("pre-commit install", content)
            self.assertNotIn("pre-commit install --hook-type pre-push", content)

            git_hooks = root / ".codex" / "rules" / "git-hooks.md"
            self.assertTrue(git_hooks.exists())
            git_hooks_content = git_hooks.read_text(encoding="utf-8")
            self.assertIn("Git hook specialist", git_hooks_content)
            self.assertIn("pre-commit install", git_hooks_content)
            self.assertIn("pre-commit install --hook-type pre-push", git_hooks_content)
            self.assertIn("gitleaks", git_hooks_content)
            self.assertNotIn("scripts/check-no-secrets.sh", git_hooks_content)

    def test_install_writes_ansible_git_hooks_guidance(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root, "codex", ["linting"], [], "ansible", False, False, False
            )

            self.assertIn("git-hooks", result.installed)
            git_hooks = root / ".codex" / "rules" / "git-hooks.md"
            content = git_hooks.read_text(encoding="utf-8")
            self.assertIn("pre-commit install --hook-type pre-push", content)
            self.assertIn("ansible-playbook --syntax-check", content)
            self.assertIn("pre-push stage", content)
            self.assertIn("pre-commit autoupdate", content)
            self.assertIn("gitleaks", content)
            self.assertIn("ansible-lint --profile=safety", content)
            self.assertNotIn("Use Husky for TypeScript-only repositories.", content)
            self.assertNotIn("Husky", content)
            self.assertNotIn("lint-staged", content)
            self.assertNotIn("npx lint-staged", content)
            self.assertNotIn("scripts/check-no-secrets.sh", content)

    def test_render_terraform_git_hooks_guidance_uses_initialized_commands(
        self,
    ) -> None:
        content = cli.render_git_hooks_guidance("terraform", "pre-commit")
        self.assertIn("terraform fmt -check -recursive", content)
        self.assertIn("terraform init -backend=false", content)
        self.assertIn("terraform validate", content)
        self.assertIn("tflint --init", content)
        self.assertIn("tflint --recursive", content)
        self.assertIn("trivy config .", content)
        self.assertIn("tfsec", content)
        self.assertIn("gitleaks", content)
        self.assertIn("cloud/runtime posture scanning", content)
        self.assertNotIn("scripts/check-no-secrets.sh", content)

    def test_render_go_git_hooks_guidance_declares_gitleaks(self) -> None:
        content = cli.render_git_hooks_guidance("go", "pre-commit")

        self.assertIn("sub-pre-commit", content)
        self.assertIn("pre-commit install --hook-type pre-push", content)
        self.assertIn("Go unit tests", content)
        self.assertIn("gitleaks", content)
        self.assertIn("govulncheck", content)
        self.assertIn("go test -race", content)
        self.assertNotIn("scripts/check-no-secrets.sh", content)

    def test_render_dart_git_hooks_guidance_declares_gitleaks(self) -> None:
        content = cli.render_git_hooks_guidance("dart", "pre-commit")

        self.assertIn("dart format --set-exit-if-changed .", content)
        self.assertIn("flutter analyze", content)
        self.assertIn("flutter test integration_test", content)
        self.assertIn("gitleaks", content)
        self.assertNotIn("scripts/check-no-secrets.sh", content)

    def test_patch_preserves_existing_sections(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            rule = root / ".cursor" / "rules" / "python-linting.mdc"
            rule.parent.mkdir(parents=True, exist_ok=True)
            rule.write_text(
                """---
description: Team customized linting rules
alwaysApply: true
---

Team intro.

## Your Responsibilities

Keep team-specific wording.

## Team Overrides

Keep this note.
""",
                encoding="utf-8",
            )

            result = cli.install(
                root, "cursor", ["linting"], [], "python", False, True, False
            )

            self.assertIn("linting", result.installed)
            content = rule.read_text(encoding="utf-8")
            self.assertIn("description: Team customized linting rules", content)
            self.assertIn("Keep team-specific wording.", content)
            self.assertIn("## Team Overrides", content)
            self.assertIn("## Baseline Tooling", content)
            self.assertIn("globs:", content)

    def test_patch_updates_codex_agents_md_section_only(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            rule = root / ".codex" / "rules" / "python-linting.md"
            rule.parent.mkdir(parents=True, exist_ok=True)
            rule.write_text(
                """# Python Linting Rules

Team intro.

## Your Responsibilities

Keep my custom rule text.
""",
                encoding="utf-8",
            )
            (root / "AGENTS.md").write_text(
                """# AGENTS.md

## Team Notes

Keep this section.

## Installed agent rules

Read and follow these rule files in `.codex/rules/` when they apply:

- `.codex/rules/old.md` - Old rule
""",
                encoding="utf-8",
            )

            cli.install(root, "codex", ["linting"], [], "python", False, True, False)

            agents_md = (root / "AGENTS.md").read_text(encoding="utf-8")
            self.assertIn("## Team Notes", agents_md)
            self.assertRegex(
                agents_md,
                r"Created by \[Ballast\]\(https://github\.com/everydaydevopsio/ballast\) "
                r"v[0-9A-Za-z._-]+\. Do not edit this section\.",
            )
            self.assertIn("`.codex/rules/python-linting.md`", agents_md)
            self.assertNotIn("`.codex/rules/old.md`", agents_md)

    def test_skill_only_patch_keeps_codex_rule_references_from_rulesrc(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            cli.save_config(
                root,
                "python",
                "codex",
                ["linting"],
                ["owasp-security-scan"],
            )
            (root / "AGENTS.md").write_text(
                cli.build_codex_agents_md(
                    ["linting"], ["owasp-security-scan"], "python"
                ),
                encoding="utf-8",
            )

            result = cli.install(
                root,
                "codex",
                [],
                ["owasp-security-scan", "github-health-check"],
                "python",
                False,
                True,
                False,
            )

            self.assertEqual(result.errors, [])
            agents_md = (root / "AGENTS.md").read_text(encoding="utf-8")
            self.assertIn("`.codex/rules/python-linting.md`", agents_md)
            self.assertIn("`.codex/skills/owasp-security-scan/SKILL.md`", agents_md)
            self.assertNotIn("`.codex/skills/github-health-check/SKILL.md`", agents_md)

    def test_patch_updates_claude_md_section_only(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            rule = root / ".claude" / "rules" / "python-linting.md"
            rule.parent.mkdir(parents=True, exist_ok=True)
            rule.write_text(
                """# Python Linting Rules

Team intro.

## Your Responsibilities

Keep my custom rule text.
""",
                encoding="utf-8",
            )
            (root / "CLAUDE.md").write_text(
                """# CLAUDE.md

## Team Notes

Keep this section.

## Installed agent rules

Read and follow these rule files in `.claude/rules/` when they apply:

- `.claude/rules/old.md` - Old rule
""",
                encoding="utf-8",
            )

            cli.install(
                root, "claude", ["linting"], [], "python", False, False, False, True
            )

            claude_md = (root / "CLAUDE.md").read_text(encoding="utf-8")
            self.assertIn("## Team Notes", claude_md)
            self.assertRegex(
                claude_md,
                r"Created by \[Ballast\]\(https://github\.com/everydaydevopsio/ballast\) "
                r"v[0-9A-Za-z._-]+\. Do not edit this section\.",
            )
            self.assertIn("`.claude/rules/python-linting.md`", claude_md)
            self.assertNotIn("`.claude/rules/old.md`", claude_md)

    def test_install_creates_gemini_support_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root, "gemini", ["linting"], [], "python", False, False, False
            )

            self.assertIn("linting", result.installed)
            gemini_md = (root / "GEMINI.md").read_text(encoding="utf-8")
            self.assertIn("## Repository Facts", gemini_md)
            self.assertIn("## Memory Tiering", gemini_md)
            self.assertIn("`.gemini/rules/python-linting.md`", gemini_md)

            self.assertFalse((root / "AGENTS.md").exists())

    def test_install_patches_existing_gemini_md_by_default(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "GEMINI.md").write_text(
                """# GEMINI.md

## Team Notes

Keep this section.

## Installed agent rules

Created by [Ballast](https://github.com/everydaydevopsio/ballast) v9.9.9-test. Do not edit this section.

Read and follow these rule files in `.gemini/rules/` when they apply:

- `.gemini/rules/old.md` - Old rule
""",
                encoding="utf-8",
            )

            result = cli.install(
                root, "gemini", ["linting"], [], "python", False, False, False
            )

            self.assertIn(str(root / "GEMINI.md"), result.installed_support_files)
            self.assertNotIn(str(root / "GEMINI.md"), result.skipped_support_files)
            gemini_md = (root / "GEMINI.md").read_text(encoding="utf-8")
            self.assertIn("## Team Notes", gemini_md)
            self.assertIn("`.gemini/rules/python-linting.md`", gemini_md)
            self.assertNotIn("`.gemini/rules/old.md`", gemini_md)
            self.assertFalse((root / "AGENTS.md").exists())

    def test_patch_updates_gemini_md_section_only(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            rule = root / ".gemini" / "rules" / "python-linting.md"
            rule.parent.mkdir(parents=True, exist_ok=True)
            rule.write_text(
                """# Python Linting Rules

Team intro.

## Your Responsibilities

Keep my custom rule text.
""",
                encoding="utf-8",
            )
            (root / "GEMINI.md").write_text(
                """# GEMINI.md

## Team Notes

Keep this section.

## Installed agent rules

Read and follow these rule files in `.gemini/rules/` when they apply:

- `.gemini/rules/old.md` - Old rule
""",
                encoding="utf-8",
            )

            cli.install(root, "gemini", ["linting"], [], "python", False, True, False)

            gemini_md = (root / "GEMINI.md").read_text(encoding="utf-8")
            self.assertIn("## Team Notes", gemini_md)
            self.assertIn("Keep this section.", gemini_md)
            self.assertIn("`.gemini/rules/python-linting.md`", gemini_md)
            self.assertNotIn("`.gemini/rules/old.md`", gemini_md)

    def test_run_install_invalid_target_message_lists_gemini(self) -> None:
        args = cli.parser().parse_args(["install", "--yes"])

        with (
            mock.patch.object(
                cli,
                "resolve_target_and_agents",
                return_value=(["bogus"], ["linting"], []),
            ),
            io.StringIO() as buf,
            redirect_stdout(buf),
        ):
            exit_code = cli.run_install(args)
            output = buf.getvalue()

        self.assertEqual(exit_code, 1)
        self.assertIn(
            "Invalid --target. Use: cursor, claude, opencode, codex, gemini",
            output,
        )

    def test_run_install_force_support_file_declined_skips_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "pyproject.toml").write_text(
                "[project]\nname='demo'\n", encoding="utf-8"
            )
            (root / "AGENTS.md").write_text(
                "# AGENTS.md\n\nTeam customizations.\n",
                encoding="utf-8",
            )
            args = cli.parser().parse_args(
                [
                    "install",
                    "--target",
                    "codex",
                    "--skill",
                    "owasp-security-scan",
                    "--force",
                ]
            )

            with (
                mock.patch.object(cli, "resolve_project_root", return_value=root),
                mock.patch.object(cli, "is_ci_mode", return_value=False),
                mock.patch.object(cli, "is_stdin_interactive", return_value=True),
                mock.patch.object(cli, "prompt_yes_no", return_value=False),
                io.StringIO() as buf,
                redirect_stdout(buf),
            ):
                exit_code = cli.run_install(args)
                output = buf.getvalue()

            self.assertEqual(exit_code, 0)
            self.assertIn("Skipped support file: AGENTS.md", output)
            self.assertIn(
                "Team customizations.",
                (root / "AGENTS.md").read_text(encoding="utf-8"),
            )

    def test_run_install_force_support_file_accepted_overwrites_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "pyproject.toml").write_text(
                "[project]\nname='demo'\n", encoding="utf-8"
            )
            (root / "AGENTS.md").write_text(
                "# AGENTS.md\n\nTeam customizations.\n",
                encoding="utf-8",
            )
            args = cli.parser().parse_args(
                [
                    "install",
                    "--target",
                    "codex",
                    "--skill",
                    "owasp-security-scan",
                    "--force",
                ]
            )

            with (
                mock.patch.object(cli, "resolve_project_root", return_value=root),
                mock.patch.object(cli, "is_ci_mode", return_value=False),
                mock.patch.object(cli, "is_stdin_interactive", return_value=True),
                mock.patch.object(cli, "prompt_yes_no", return_value=True),
            ):
                exit_code = cli.run_install(args)

            self.assertEqual(exit_code, 0)
            content = (root / "AGENTS.md").read_text(encoding="utf-8")
            self.assertIn("## Installed skills", content)
            self.assertNotIn("Team customizations.", content)

    def test_run_install_force_support_file_non_interactive_aborts(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "pyproject.toml").write_text(
                "[project]\nname='demo'\n", encoding="utf-8"
            )
            (root / "AGENTS.md").write_text(
                "# AGENTS.md\n\nTeam customizations.\n",
                encoding="utf-8",
            )
            args = cli.parser().parse_args(
                [
                    "install",
                    "--target",
                    "codex",
                    "--skill",
                    "owasp-security-scan",
                    "--force",
                    "--yes",
                ]
            )

            with (
                mock.patch.object(cli, "resolve_project_root", return_value=root),
                io.StringIO() as buf,
                redirect_stdout(buf),
            ):
                exit_code = cli.run_install(args)
                output = buf.getvalue()

            self.assertEqual(exit_code, 1)
            self.assertIn(
                "Cannot overwrite existing support file AGENTS.md in non-interactive mode",
                output,
            )
            self.assertIn(
                "Team customizations.",
                (root / "AGENTS.md").read_text(encoding="utf-8"),
            )

    def test_run_install_force_support_file_non_tty_aborts(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "pyproject.toml").write_text(
                "[project]\nname='demo'\n", encoding="utf-8"
            )
            (root / "AGENTS.md").write_text(
                "# AGENTS.md\n\nTeam customizations.\n",
                encoding="utf-8",
            )
            args = cli.parser().parse_args(
                [
                    "install",
                    "--target",
                    "codex",
                    "--skill",
                    "owasp-security-scan",
                    "--force",
                ]
            )

            with (
                mock.patch.object(cli, "resolve_project_root", return_value=root),
                mock.patch.object(cli, "is_ci_mode", return_value=False),
                mock.patch.object(cli, "is_stdin_interactive", return_value=False),
                io.StringIO() as buf,
                redirect_stdout(buf),
            ):
                exit_code = cli.run_install(args)
                output = buf.getvalue()

            self.assertEqual(exit_code, 1)
            self.assertIn(
                "Cannot overwrite existing support file AGENTS.md in non-interactive mode",
                output,
            )
            self.assertIn(
                "Team customizations.",
                (root / "AGENTS.md").read_text(encoding="utf-8"),
            )

    def test_is_stdin_interactive_detects_non_tty_stream(self) -> None:
        with mock.patch.object(cli.sys, "stdin", io.StringIO("")):
            self.assertFalse(cli.is_stdin_interactive())

    def test_patch_flag_updates_claude_md_section(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "CLAUDE.md").write_text(
                """# CLAUDE.md

## Installed agent rules

- `.claude/rules/old.md` - Old rule
""",
                encoding="utf-8",
            )

            result = cli.install(
                root, "claude", ["linting"], [], "python", False, True, False
            )

            self.assertIn(str(root / "CLAUDE.md"), result.installed_support_files)
            claude_md = (root / "CLAUDE.md").read_text(encoding="utf-8")
            self.assertIn("`.claude/rules/python-linting.md`", claude_md)
            self.assertNotIn("`.claude/rules/old.md`", claude_md)

    def test_default_patch_preserves_unmanaged_codex_support_section(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "AGENTS.md").write_text(
                """# AGENTS.md

## Installed agent rules

- `.codex/rules/old.md` - Team managed rule
""",
                encoding="utf-8",
            )

            cli.install(root, "codex", ["linting"], [], "python", False, False, False)

            agents_md = (root / "AGENTS.md").read_text(encoding="utf-8")
            self.assertIn("`.codex/rules/old.md`", agents_md)
            self.assertIn("`.codex/rules/python-linting.md`", agents_md)

    def test_patch_merges_frontmatter_keys(self) -> None:
        existing = """---
description: Team customized linting rules
alwaysApply: true
tools:
  read: false
---

## Existing

User content.
"""
        canonical = """---
description: Canonical description
alwaysApply: false
globs:
  - '*.py'
tools:
  read: true
  write: true
---

## Existing

Canonical content.
"""

        merged = cli.patch_rule_content(existing, canonical, "cursor")

        self.assertIn("description: Team customized linting rules", merged)
        self.assertIn("alwaysApply: true", merged)
        self.assertIn("globs:", merged)
        self.assertIn("  read: false", merged)
        self.assertIn("  write: true", merged)

    def test_patch_codex_agents_md_ignores_heading_inside_code_fence(self) -> None:
        existing = """# AGENTS.md

```md
## Installed agent rules
```

## Installed agent rules

- `.codex/rules/old.md` - Old rule
"""
        canonical = """# AGENTS.md

## Installed agent rules

Created by [Ballast](https://github.com/everydaydevopsio/ballast) v9.9.9-test. Do not edit this section.

- `.codex/rules/python-linting.md` - New rule
"""

        merged = cli.patch_codex_agents_md(existing, canonical)

        self.assertIn("```md\n## Installed agent rules\n```", merged)
        self.assertIn("`.codex/rules/python-linting.md`", merged)
        self.assertNotIn("`.codex/rules/old.md`", merged)

    def test_patch_codex_agents_md_preserves_unmanaged_sections_in_managed_only_mode(
        self,
    ) -> None:
        existing = """# AGENTS.md

## Installed agent rules

- `.codex/rules/old.md` - Team managed rule
"""
        canonical = """# AGENTS.md

## Installed agent rules

Created by [Ballast](https://github.com/everydaydevopsio/ballast) v9.9.9-test. Do not edit this section.

- `.codex/rules/python-linting.md` - New rule
"""

        merged = cli.patch_codex_agents_md(
            existing, canonical, replace_unmanaged_sections=False
        )

        self.assertIn("`.codex/rules/old.md`", merged)
        self.assertIn("`.codex/rules/python-linting.md`", merged)

    def test_patch_codex_agents_md_replaces_legacy_managed_notice_in_managed_only_mode(
        self,
    ) -> None:
        existing = """# AGENTS.md

## Installed agent rules

Created by Ballast. Do not edit this section.

- `.codex/rules/old.md` - Old rule
"""
        canonical = """# AGENTS.md

## Installed agent rules

Created by [Ballast](https://github.com/everydaydevopsio/ballast) v9.9.9-test. Do not edit this section.

- `.codex/rules/python-linting.md` - New rule
"""

        merged = cli.patch_codex_agents_md(
            existing, canonical, replace_unmanaged_sections=False
        )

        self.assertNotIn("`.codex/rules/old.md`", merged)
        self.assertIn("`.codex/rules/python-linting.md`", merged)


if __name__ == "__main__":
    unittest.main()

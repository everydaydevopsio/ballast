from __future__ import annotations

import io
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

    def test_build_content_for_gemini_prefers_non_codex_header(self) -> None:
        content = cli.build_content("linting", "gemini", "python")

        self.assertIn("# Python Linting Rules", content)
        self.assertNotIn("Codex (CLI and app)", content)

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

    def test_resolve_project_root_supports_ansible_markers(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "ansible.cfg").write_text("[defaults]\n", encoding="utf-8")
            nested = root / "roles" / "novnc"
            nested.mkdir(parents=True)

            resolved = cli.resolve_project_root(nested)

            self.assertEqual(resolved, root)

    def test_resolve_project_root_supports_ansible_requirements_yaml(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "requirements.yaml").write_text("---\n", encoding="utf-8")
            nested = root / "roles" / "novnc"
            nested.mkdir(parents=True)

            resolved = cli.resolve_project_root(nested)

            self.assertEqual(resolved, root)

    def test_resolve_project_root_supports_terraform_markers(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".terraform-version").write_text("1.8.5\n", encoding="utf-8")
            (root / "versions.tf").write_text("terraform {}\n", encoding="utf-8")
            nested = root / "modules" / "network"
            nested.mkdir(parents=True)

            resolved = cli.resolve_project_root(nested)

            self.assertEqual(resolved, root)

    def test_normalize_target_tokens_ignores_non_string_items(self) -> None:
        self.assertEqual(
            cli.normalize_target_tokens(["cursor,claude", 7, None, "codex"]),
            ["cursor", "claude", "codex"],
        )

    def test_run_install_writes_shared_rulesrc_for_explicit_flags(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
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

    def test_run_install_writes_multi_target_shared_rulesrc(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
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
        self.assertNotIn("description: >", content)

    def test_build_support_file_includes_skills(self) -> None:
        content = cli.build_codex_agents_md(
            ["linting"], ["owasp-security-scan"], "python"
        )
        self.assertIn("## Repository Facts", content)
        self.assertIn("Canonical GitHub repo: `<OWNER/REPO>`", content)
        self.assertIn("## Installed skills", content)
        self.assertIn("`.codex/rules/owasp-security-scan.md`", content)

    def test_build_gemini_md_includes_agents_import_and_rules(self) -> None:
        content = cli.build_gemini_md(["linting"], ["owasp-security-scan"], "python")

        self.assertIn("# GEMINI.md", content)
        self.assertIn("@./AGENTS.md", content)
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
            root / ".codex" / "rules" / "owasp-security-scan.md",
        )

    def test_resolve_target_and_agents_uses_saved_skill_only_config(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            cli.save_config(root, "python", ["claude"], [], ["owasp-security-scan"])

            args = cli.parser().parse_args(["install", "--yes"])
            resolved = cli.resolve_target_and_agents(args, root, "python")

            self.assertEqual(resolved, (["claude"], [], ["owasp-security-scan"]))

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
                ),
            )

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
                (root / ".codex" / "rules" / "owasp-security-scan.md").exists()
            )
            agents_md = (root / "AGENTS.md").read_text(encoding="utf-8")
            self.assertIn("## Installed skills", agents_md)
            self.assertIn("`.codex/rules/owasp-security-scan.md`", agents_md)

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
            self.assertIn("`.codex/rules/owasp-security-scan.md`", agents_md)
            self.assertNotIn("`.codex/rules/github-health-check.md`", agents_md)

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

    def test_install_creates_gemini_and_agents_support_files(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(
                root, "gemini", ["linting"], [], "python", False, False, False
            )

            self.assertIn("linting", result.installed)
            gemini_md = (root / "GEMINI.md").read_text(encoding="utf-8")
            self.assertIn("@./AGENTS.md", gemini_md)
            self.assertIn("`.gemini/rules/python-linting.md`", gemini_md)

            agents_md = (root / "AGENTS.md").read_text(encoding="utf-8")
            self.assertIn("## Repository Facts", agents_md)
            self.assertIn("`.codex/rules/python-linting.md`", agents_md)

    def test_install_skips_existing_gemini_md_without_patch(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "GEMINI.md").write_text(
                """# GEMINI.md

## Team Notes

Keep this section.
""",
                encoding="utf-8",
            )

            result = cli.install(
                root, "gemini", ["linting"], [], "python", False, False, False
            )

            self.assertIn(str(root / "GEMINI.md"), result.skipped_support_files)
            gemini_md = (root / "GEMINI.md").read_text(encoding="utf-8")
            self.assertIn("## Team Notes", gemini_md)
            self.assertTrue((root / "AGENTS.md").exists())

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


if __name__ == "__main__":
    unittest.main()

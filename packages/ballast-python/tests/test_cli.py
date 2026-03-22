from __future__ import annotations

import os
import tempfile
import unittest
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from ballast import cli


class PatchInstallTests(unittest.TestCase):
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

    def test_run_install_writes_shared_rulesrc_for_explicit_flags(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            old_cwd = Path.cwd()
            os.chdir(root)
            try:
                args = cli.parser().parse_args(["install", "--target", "codex", "--all", "--yes"])
                exit_code = cli.run_install(args)
            finally:
                os.chdir(old_cwd)

            self.assertEqual(exit_code, 0)
            self.assertTrue((root / ".rulesrc.json").exists())
            content = (root / ".rulesrc.json").read_text(encoding="utf-8")
            self.assertIn('"languages": [', content)
            self.assertIn('"python"', content)
            self.assertIn('"paths": {', content)

    def test_manual_installs_accumulate_languages_in_shared_rulesrc(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            cli.save_config(root, "python", "claude", ["linting"])
            cli.save_config(root, "go", "claude", ["linting"])

            content = (root / ".rulesrc.json").read_text(encoding="utf-8")
            self.assertIn('"languages": [', content)
            self.assertIn('"python"', content)
            self.assertIn('"go"', content)
            self.assertIn('"python": [', content)
            self.assertIn('"go": [', content)

    def test_install_creates_language_prefixed_rule_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(root, "codex", ["linting"], "python", False, False, False)

            self.assertIn("linting", result.installed)
            rule = root / ".codex" / "rules" / "python-linting.md"
            self.assertTrue(rule.exists())
            self.assertIn("Python linting specialist", rule.read_text(encoding="utf-8"))

    def test_install_replaces_hook_guidance_token_for_python_rules(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            result = cli.install(root, "codex", ["linting"], "python", False, False, False)

            self.assertIn("linting", result.installed)
            rule = root / ".codex" / "rules" / "python-linting.md"
            content = rule.read_text(encoding="utf-8")
            self.assertNotIn("{{BALLAST_HOOK_GUIDANCE}}", content)
            self.assertIn("pre-commit install", content)

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

            result = cli.install(root, "cursor", ["linting"], "python", False, True, False)

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

            cli.install(root, "codex", ["linting"], "python", False, True, False)

            agents_md = (root / "AGENTS.md").read_text(encoding="utf-8")
            self.assertIn("## Team Notes", agents_md)
            self.assertRegex(
                agents_md,
                r"Created by Ballast v[0-9A-Za-z._-]+\. Do not edit this section\.",
            )
            self.assertIn("`.codex/rules/python-linting.md`", agents_md)
            self.assertNotIn("`.codex/rules/old.md`", agents_md)

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

            cli.install(root, "claude", ["linting"], "python", False, False, False, True)

            claude_md = (root / "CLAUDE.md").read_text(encoding="utf-8")
            self.assertIn("## Team Notes", claude_md)
            self.assertRegex(
                claude_md,
                r"Created by Ballast v[0-9A-Za-z._-]+\. Do not edit this section\.",
            )
            self.assertIn("`.claude/rules/python-linting.md`", claude_md)
            self.assertNotIn("`.claude/rules/old.md`", claude_md)

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

            result = cli.install(root, "claude", ["linting"], "python", False, True, False)

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

Created by Ballast v9.9.9-test. Do not edit this section.

- `.codex/rules/python-linting.md` - New rule
"""

        merged = cli.patch_codex_agents_md(existing, canonical)

        self.assertIn("```md\n## Installed agent rules\n```", merged)
        self.assertIn("`.codex/rules/python-linting.md`", merged)
        self.assertNotIn("`.codex/rules/old.md`", merged)


if __name__ == "__main__":
    unittest.main()

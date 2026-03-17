from __future__ import annotations

import tempfile
import unittest
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from ballast import cli


class PatchInstallTests(unittest.TestCase):
    def test_patch_preserves_existing_sections(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            rule = root / ".cursor" / "rules" / "linting.mdc"
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

    def test_patch_updates_codex_agents_md_section_only(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            rule = root / ".codex" / "rules" / "linting.md"
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
            self.assertIn("`.codex/rules/linting.md`", agents_md)
            self.assertNotIn("`.codex/rules/old.md`", agents_md)


if __name__ == "__main__":
    unittest.main()

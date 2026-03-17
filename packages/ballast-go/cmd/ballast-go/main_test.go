package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPatchRuleContentPreservesExistingSections(t *testing.T) {
	existing := `---
description: Team customized linting rules
alwaysApply: true
---

Team intro.

## Your Responsibilities

Keep team-specific wording.

## Team Overrides

Keep this note.
`
	canonical := `---
description: Canonical description
alwaysApply: false
---

Canonical intro.

## Your Responsibilities

Canonical responsibilities.

## When Completed

Canonical completion checklist.
`

	merged := patchRuleContent(existing, canonical, "cursor")
	if !strings.Contains(merged, "description: Team customized linting rules") {
		t.Fatalf("expected user frontmatter to be preserved: %s", merged)
	}
	if !strings.Contains(merged, "Keep team-specific wording.") {
		t.Fatalf("expected user section to be preserved: %s", merged)
	}
	if !strings.Contains(merged, "## Team Overrides") {
		t.Fatalf("expected user-added section to remain: %s", merged)
	}
	if !strings.Contains(merged, "## When Completed") {
		t.Fatalf("expected canonical section to be appended: %s", merged)
	}
}

func TestInstallPatchUpdatesCodexAgentsMDSectionOnly(t *testing.T) {
	tmpDir := t.TempDir()
	rulePath := filepath.Join(tmpDir, ".codex", "rules", "linting.md")
	if err := os.MkdirAll(filepath.Dir(rulePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rulePath, []byte(`# Go Linting Rules

Team intro.

## Your Responsibilities

Keep my custom rule text.
`), 0o644); err != nil {
		t.Fatal(err)
	}
	agentsMD := filepath.Join(tmpDir, "AGENTS.md")
	if err := os.WriteFile(agentsMD, []byte("# AGENTS.md\n\n## Team Notes\n\nKeep this section.\n\n## Installed agent rules\n\nRead and follow these rule files in `.codex/rules/` when they apply:\n\n- `.codex/rules/old.md` - Old rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		target:      "codex",
		agents:      []string{"linting"},
		language:    "go",
		force:       false,
		patch:       true,
		saveConfig:  false,
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}

	content, err := os.ReadFile(agentsMD)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "## Team Notes") {
		t.Fatalf("expected user notes to remain: %s", text)
	}
	if !regexp.MustCompile(`Created by Ballast v[0-9A-Za-z._-]+\. Do not edit this section\.`).MatchString(text) {
		t.Fatalf("expected ballast notice to be present: %s", text)
	}
	if !strings.Contains(text, "`.codex/rules/linting.md`") {
		t.Fatalf("expected linting rule to be installed: %s", text)
	}
	if strings.Contains(text, "`.codex/rules/old.md`") {
		t.Fatalf("expected old installed-rules entry to be replaced: %s", text)
	}
}

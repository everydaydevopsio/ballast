package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = original
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}

	var out bytes.Buffer
	if _, err := out.ReadFrom(reader); err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}

	return out.String()
}

func TestRunTopLevelHelpFlag(t *testing.T) {
	output := captureStdout(t, func() {
		exitCode := run([]string{"--help"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if !strings.Contains(output, "Usage: ballast-go install [options]") {
		t.Fatalf("expected help output, got %q", output)
	}
}

func TestRunTopLevelVersionFlag(t *testing.T) {
	output := captureStdout(t, func() {
		exitCode := run([]string{"--version"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if !strings.Contains(strings.TrimSpace(output), resolveVersion()) {
		t.Fatalf("expected version output %q, got %q", resolveVersion(), output)
	}
}

func TestRunInstallHelpFlag(t *testing.T) {
	output := captureStdout(t, func() {
		exitCode := run([]string{"install", "--help"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if !strings.Contains(output, "Usage: ballast-go install [options]") {
		t.Fatalf("expected help output, got %q", output)
	}
}

func TestRunDoctorCommand(t *testing.T) {
	output := captureStdout(t, func() {
		exitCode := run([]string{"doctor"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if !strings.Contains(output, "Ballast doctor") {
		t.Fatalf("expected doctor output, got %q", output)
	}
}

func TestBuildDoctorReportRecommendsUpgrades(t *testing.T) {
	output := buildDoctorReport(
		"ballast-go",
		"5.0.2",
		"/tmp/project/.rulesrc.json",
		&rulesConfig{
			Target:         "cursor",
			Agents:         []string{"linting", "testing"},
			BallastVersion: "5.0.1",
		},
		[]installedCLIStatus{
			{Name: "ballast-typescript", Version: "5.0.2", Path: "/tmp/ballast-typescript"},
			{Name: "ballast-python", Version: "5.0.1", Path: "/tmp/ballast-python"},
			{Name: "ballast-go"},
		},
	)

	if !strings.Contains(output, "Run ballast doctor --fix to install or upgrade local Ballast CLIs.") {
		t.Fatalf("expected cli fix recommendation, got %q", output)
	}
	if !strings.Contains(output, "Refresh .rulesrc.json to Ballast 5.0.2: ballast install --refresh-config") {
		t.Fatalf("expected config refresh recommendation, got %q", output)
	}
}

func TestBuildDoctorReportRecommendsFixForUnknownVersion(t *testing.T) {
	output := buildDoctorReport(
		"ballast-go",
		"5.0.2",
		"",
		nil,
		[]installedCLIStatus{
			{Name: "ballast-typescript", Version: "", Path: "/tmp/ballast-typescript"},
		},
	)

	if !strings.Contains(output, "Run ballast doctor --fix to install or upgrade local Ballast CLIs.") {
		t.Fatalf("expected cli fix recommendation for unknown version, got %q", output)
	}
}

func TestInstallAddsBallastToGitignore(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		target:      "cursor",
		agents:      []string{"linting"},
		language:    "go",
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), ".ballast/") {
		t.Fatalf("expected .ballast/ in .gitignore, got %q", string(content))
	}
}

func TestInstallRecordsGitignoreErrorAndContinues(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmpDir, ".gitignore"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		target:      "cursor",
		agents:      []string{"linting"},
		language:    "go",
	})
	if len(result.errors) == 0 || result.errors[0].agent != "gitignore" {
		t.Fatalf("expected gitignore error, got %+v", result.errors)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".cursor", "rules", "go-linting.mdc")); err != nil {
		t.Fatalf("expected install to continue, got %v", err)
	}
}

func TestInstallSupportsPublishingAgent(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		target:      "cursor",
		agents:      []string{"publishing"},
		language:    "go",
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}
	if !slices.Equal(result.installed, []string{"publishing"}) {
		t.Fatalf("expected publishing install, got %+v", result.installed)
	}
	for _, file := range []string{
		"publishing-libraries.mdc",
		"publishing-sdks.mdc",
		"publishing-apps.mdc",
	} {
		if _, err := os.Stat(filepath.Join(tmpDir, ".cursor", "rules", file)); err != nil {
			t.Fatalf("expected %s to exist, got %v", file, err)
		}
	}
}

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

func TestPatchRuleContentMergesFrontmatterAndHandlesCRLF(t *testing.T) {
	existing := "---\r\ndescription: Team customized linting rules\r\nalwaysApply: true\r\ntools:\r\n  read: false\r\n---\r\n\r\n## Your Responsibilities\r\n\r\nKeep team-specific wording.\r\n"
	canonical := "---\ndescription: Canonical description\nglobs:\n  - '*.go'\ntools:\n  read: true\n  write: true\n---\n\n## Your Responsibilities\n\nCanonical responsibilities.\n\n## Commands\n\nCanonical commands.\n"

	merged := patchRuleContent(existing, canonical, "cursor")
	if !strings.Contains(merged, "description: Team customized linting rules") {
		t.Fatalf("expected user frontmatter to win: %s", merged)
	}
	if !strings.Contains(merged, "globs:") {
		t.Fatalf("expected canonical frontmatter keys to be retained: %s", merged)
	}
	if !strings.Contains(merged, "  read: false") || !strings.Contains(merged, "  write: true") {
		t.Fatalf("expected nested frontmatter blocks to merge: %s", merged)
	}
	if !strings.Contains(merged, "Keep team-specific wording.") {
		t.Fatalf("expected user section text to be preserved: %s", merged)
	}
	if !strings.Contains(merged, "## Commands") {
		t.Fatalf("expected canonical section to be appended: %s", merged)
	}
}

func TestInstallCreatesLanguagePrefixedRuleFile(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		target:      "codex",
		agents:      []string{"linting"},
		language:    "go",
		force:       false,
		saveConfig:  false,
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}
	if len(result.installed) != 1 || result.installed[0] != "linting" {
		t.Fatalf("expected linting to be installed, got %+v", result.installed)
	}

	rulePath := filepath.Join(tmpDir, ".codex", "rules", "go-linting.md")
	content, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatalf("read go-linting.md: %v", err)
	}
	if !strings.Contains(string(content), "Go linting specialist") {
		t.Fatalf("expected go-specific linting content, got %s", string(content))
	}
	if strings.Contains(string(content), "{{BALLAST_HOOK_GUIDANCE}}") {
		t.Fatalf("expected hook guidance token to be replaced, got %s", string(content))
	}
	if !strings.Contains(string(content), "pre-commit install") {
		t.Fatalf("expected concrete pre-commit guidance, got %s", string(content))
	}
	if !strings.Contains(string(content), "pre-commit install --hook-type pre-push") {
		t.Fatalf("expected concrete pre-push guidance, got %s", string(content))
	}
}

func TestBuildCursorSkillFormatIncludesOnDemandFrontmatter(t *testing.T) {
	content, err := buildCursorSkillFormat("owasp-security-scan", "go")
	if err != nil {
		t.Fatalf("buildCursorSkillFormat: %v", err)
	}
	if !strings.Contains(content, "alwaysApply: false") {
		t.Fatalf("expected alwaysApply false frontmatter: %s", content)
	}
	if !strings.Contains(content, "description: \"Run OWASP-aligned security scans across Go, TypeScript, and Python codebases.") {
		t.Fatalf("expected skill description in frontmatter: %s", content)
	}
	if strings.Contains(content, "description: >") {
		t.Fatalf("expected folded YAML description to be resolved: %s", content)
	}
	if strings.Contains(content, "description: Run OWASP-aligned security scans across Go, TypeScript, and Python codebases.") {
		t.Fatalf("expected description to remain quoted: %s", content)
	}
	if strings.Contains(content, "\n---\n---\n") {
		t.Fatalf("expected normalized markdown body without duplicate frontmatter: %s", content)
	}
}

func TestBuildClaudeSkillIncludesSkillAndReferences(t *testing.T) {
	content, err := buildClaudeSkill("owasp-security-scan", "go")
	if err != nil {
		t.Fatalf("buildClaudeSkill: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("open skill archive: %v", err)
	}

	entries := map[string]bool{}
	for _, file := range reader.File {
		entries[file.Name] = true
	}

	expected := []string{
		"SKILL.md",
		"references/owasp-mapping.md",
		"references/remediation-guide.md",
		"references/ci-workflow.md",
		"references/tool-config.md",
	}
	for _, name := range expected {
		if !entries[name] {
			t.Fatalf("expected archive entry %q, got %+v", name, entries)
		}
	}
}

func TestSkillDestinationReturnsExpectedPaths(t *testing.T) {
	tmpDir := t.TempDir()

	cases := []struct {
		target string
		dir    string
		file   string
	}{
		{target: "cursor", dir: filepath.Join(tmpDir, ".cursor", "rules"), file: filepath.Join(tmpDir, ".cursor", "rules", "owasp-security-scan.mdc")},
		{target: "claude", dir: filepath.Join(tmpDir, ".claude", "skills"), file: filepath.Join(tmpDir, ".claude", "skills", "owasp-security-scan.skill")},
		{target: "opencode", dir: filepath.Join(tmpDir, ".opencode", "skills"), file: filepath.Join(tmpDir, ".opencode", "skills", "owasp-security-scan.md")},
		{target: "codex", dir: filepath.Join(tmpDir, ".codex", "rules"), file: filepath.Join(tmpDir, ".codex", "rules", "owasp-security-scan.md")},
	}

	for _, tc := range cases {
		dir, file, err := skillDestination(tmpDir, tc.target, "owasp-security-scan")
		if err != nil {
			t.Fatalf("skillDestination(%s): %v", tc.target, err)
		}
		if dir != tc.dir || file != tc.file {
			t.Fatalf("unexpected destination for %s: got (%s, %s), want (%s, %s)", tc.target, dir, file, tc.dir, tc.file)
		}
	}
}

func TestInstallCreatesClaudeSkillAndPersistsConfig(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		target:      "claude",
		skills:      []string{"owasp-security-scan"},
		language:    "go",
		force:       false,
		saveConfig:  true,
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}
	if len(result.installedSkills) != 1 || result.installedSkills[0] != "owasp-security-scan" {
		t.Fatalf("expected skill install result, got %+v", result.installedSkills)
	}

	skillPath := filepath.Join(tmpDir, ".claude", "skills", "owasp-security-scan.skill")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected skill file at %s: %v", skillPath, err)
	}

	configContent, err := os.ReadFile(filepath.Join(tmpDir, ".rulesrc.json"))
	if err != nil {
		t.Fatalf("read .rulesrc.json: %v", err)
	}
	text := string(configContent)
	if !strings.Contains(text, `"skills": [`+"\n"+`    "owasp-security-scan"`) {
		t.Fatalf("expected skills array in shared config: %s", text)
	}

	claudeContent, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(claudeContent), "## Installed skills") {
		t.Fatalf("expected installed skills section in CLAUDE.md: %s", string(claudeContent))
	}
	if !strings.Contains(string(claudeContent), "`.claude/skills/owasp-security-scan.skill`") {
		t.Fatalf("expected skill entry in CLAUDE.md: %s", string(claudeContent))
	}
}

func TestResolveTargetAndAgentsReturnsSavedSkillOnlyConfig(t *testing.T) {
	tmpDir := t.TempDir()

	if err := saveConfig(tmpDir, "go", rulesConfig{
		Target: "codex",
		Skills: []string{"owasp-security-scan"},
	}); err != nil {
		t.Fatalf("save skill-only config: %v", err)
	}

	resolved, err := resolveTargetAndAgents(resolveOptions{
		projectRoot: tmpDir,
		language:    "go",
	})
	if err != nil {
		t.Fatalf("resolveTargetAndAgents: %v", err)
	}
	if resolved == nil {
		t.Fatal("expected resolved config, got nil")
	}
	if resolved.Target != "codex" || len(resolved.Agents) != 0 || !slices.Equal(resolved.Skills, []string{"owasp-security-scan"}) {
		t.Fatalf("unexpected resolved config: %#v", resolved)
	}
}

func TestInstallCreatesCodexSupportFileForSkillOnlyInstall(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		target:      "codex",
		skills:      []string{"owasp-security-scan"},
		language:    "go",
		force:       false,
		saveConfig:  false,
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}
	if !slices.Equal(result.installedSkills, []string{"owasp-security-scan"}) {
		t.Fatalf("expected installed skill, got %+v", result.installedSkills)
	}
	agentsMD, err := os.ReadFile(filepath.Join(tmpDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	text := string(agentsMD)
	if !strings.Contains(text, "## Installed skills") {
		t.Fatalf("expected installed skills section in AGENTS.md: %s", text)
	}
	if !strings.Contains(text, "`.codex/rules/owasp-security-scan.md`") {
		t.Fatalf("expected codex skill entry in AGENTS.md: %s", text)
	}
}

func TestInstallSkipsExistingSkillWithoutForce(t *testing.T) {
	tmpDir := t.TempDir()
	skillPath := filepath.Join(tmpDir, ".opencode", "skills", "owasp-security-scan.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("existing skill"), 0o644); err != nil {
		t.Fatalf("seed skill file: %v", err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		target:      "opencode",
		skills:      []string{"owasp-security-scan"},
		language:    "go",
		force:       false,
		saveConfig:  false,
	})
	if !slices.Equal(result.skippedSkills, []string{"owasp-security-scan"}) {
		t.Fatalf("expected skipped skill, got %+v", result.skippedSkills)
	}
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read existing skill: %v", err)
	}
	if string(content) != "existing skill" {
		t.Fatalf("expected existing skill to remain untouched, got %q", string(content))
	}
}

func TestValidatedRuleSubdirRejectsInvalidValues(t *testing.T) {
	t.Setenv("BALLAST_RULE_SUBDIR", "../escape")
	_, err := validatedRuleSubdir()
	if err == nil {
		t.Fatal("expected validatedRuleSubdir to reject invalid BALLAST_RULE_SUBDIR")
	}
}

func TestRunInstallWritesSharedRulesrcForExplicitFlags(t *testing.T) {
	tmpDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/test\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	exitCode := runInstall([]string{"install", "--target", "codex", "--all", "--yes"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".rulesrc.json")); err != nil {
		t.Fatalf("expected .rulesrc.json to be created: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(tmpDir, ".rulesrc.json"))
	if err != nil {
		t.Fatalf("read .rulesrc.json: %v", err)
	}
	if !strings.Contains(string(content), `"ballastVersion": "`+resolveVersion()+`"`) {
		t.Fatalf("expected ballastVersion in shared config: %s", string(content))
	}
	if !strings.Contains(string(content), `"languages": [`+"\n"+`    "go"`) {
		t.Fatalf("expected go language in shared config: %s", string(content))
	}
	if !strings.Contains(string(content), `"paths": {`) {
		t.Fatalf("expected paths in shared config: %s", string(content))
	}
}

func TestSaveConfigAccumulatesLanguagesInSharedRulesrc(t *testing.T) {
	tmpDir := t.TempDir()

	if err := saveConfig(tmpDir, "typescript", rulesConfig{
		Target:    "claude",
		Agents:    []string{"linting"},
		Languages: []string{"typescript"},
	}); err != nil {
		t.Fatalf("save typescript config: %v", err)
	}
	if err := saveConfig(tmpDir, "go", rulesConfig{
		Target:    "claude",
		Agents:    []string{"linting"},
		Languages: []string{"go"},
	}); err != nil {
		t.Fatalf("save go config: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, ".rulesrc.json"))
	if err != nil {
		t.Fatalf("read .rulesrc.json: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, `"ballastVersion": "`+resolveVersion()+`"`) {
		t.Fatalf("expected ballastVersion in shared config: %s", text)
	}
	if !strings.Contains(text, `"typescript"`) || !strings.Contains(text, `"go"`) {
		t.Fatalf("expected accumulated languages in shared config: %s", text)
	}
	if !strings.Contains(text, `"typescript": [`) || !strings.Contains(text, `"go": [`) {
		t.Fatalf("expected accumulated language paths in shared config: %s", text)
	}
}

func TestPatchFlagUpdatesClaudeMDSection(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/test\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# CLAUDE.md\n\n## Installed agent rules\n\n- `.claude/rules/old.md` - Old rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		target:      "claude",
		agents:      []string{"linting"},
		language:    "go",
		force:       false,
		patch:       true,
		saveConfig:  false,
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "`.claude/rules/go-linting.md`") {
		t.Fatalf("expected go linting entry in CLAUDE.md: %s", text)
	}
	if strings.Contains(text, "`.claude/rules/old.md`") {
		t.Fatalf("expected old installed-rules entry to be replaced: %s", text)
	}
}

func TestInstallPatchUpdatesCodexAgentsMDSectionOnly(t *testing.T) {
	tmpDir := t.TempDir()
	rulePath := filepath.Join(tmpDir, ".codex", "rules", "go-linting.md")
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
	if !regexp.MustCompile(`Created by \[Ballast\]\(https://github\.com/everydaydevopsio/ballast\) v[0-9A-Za-z._-]+\. Do not edit this section\.`).MatchString(text) {
		t.Fatalf("expected ballast notice to be present: %s", text)
	}
	if !strings.Contains(text, "`.codex/rules/go-linting.md`") {
		t.Fatalf("expected linting rule to be installed: %s", text)
	}
	if strings.Contains(text, "`.codex/rules/old.md`") {
		t.Fatalf("expected old installed-rules entry to be replaced: %s", text)
	}
}

func TestInstallPatchUpdatesClaudeMDSectionOnly(t *testing.T) {
	tmpDir := t.TempDir()
	rulePath := filepath.Join(tmpDir, ".claude", "rules", "go-linting.md")
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
	claudeMD := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte("# CLAUDE.md\n\n## Team Notes\n\nKeep this section.\n\n## Installed agent rules\n\nRead and follow these rule files in `.claude/rules/` when they apply:\n\n- `.claude/rules/old.md` - Old rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		target:      "claude",
		agents:      []string{"linting"},
		language:    "go",
		force:       false,
		patch:       false,
		patchClaude: true,
		saveConfig:  false,
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}

	content, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "## Team Notes") {
		t.Fatalf("expected user notes to remain: %s", text)
	}
	if !regexp.MustCompile(`Created by \[Ballast\]\(https://github\.com/everydaydevopsio/ballast\) v[0-9A-Za-z._-]+\. Do not edit this section\.`).MatchString(text) {
		t.Fatalf("expected ballast notice to be present: %s", text)
	}
	if !strings.Contains(text, "`.claude/rules/go-linting.md`") {
		t.Fatalf("expected linting rule to be installed: %s", text)
	}
	if strings.Contains(text, "`.claude/rules/old.md`") {
		t.Fatalf("expected old installed-rules entry to be replaced: %s", text)
	}
}

func TestPatchCodexAgentsMDIgnoresHeadingInsideCodeFence(t *testing.T) {
	existing := "# AGENTS.md\n\n```md\n## Installed agent rules\n```\n\n## Installed agent rules\n\nOld rules\n"
	canonical := "# AGENTS.md\n\n## Installed agent rules\n\nCreated by [Ballast](https://github.com/everydaydevopsio/ballast) v9.9.9-test. Do not edit this section.\n\nNew rules\n"

	merged := patchCodexAgentsMD(existing, canonical)
	if !strings.Contains(merged, "Created by [Ballast](https://github.com/everydaydevopsio/ballast) v9.9.9-test. Do not edit this section.") {
		t.Fatalf("expected canonical installed rules section to be inserted: %s", merged)
	}
	if !strings.Contains(merged, "```md\n## Installed agent rules\n```") {
		t.Fatalf("expected fenced code block to be preserved without matching: %s", merged)
	}
}

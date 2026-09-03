package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
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
	if !strings.Contains(output, "aws-health-review") {
		t.Fatalf("expected new skills in help output, got %q", output)
	}
	if !strings.Contains(output, "github-health-check") {
		t.Fatalf("expected github-health-check in help output, got %q", output)
	}
	if !strings.Contains(output, "tasks") {
		t.Fatalf("expected tasks agent in help output, got %q", output)
	}
}

func TestListAgentsIncludesAllRegistryAgents(t *testing.T) {
	got := listAgents("go")
	want := []string{
		"local-dev",
		"docs",
		"cicd",
		"observability",
		"publishing",
		"git-hooks",
		"tasks",
		"plan-lifecycle",
		"spec-kit",
		"testing-process",
		"linting",
		"logging",
		"testing",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestCommonAgentsMatchEmbeddedContentDirs(t *testing.T) {
	entries, err := embeddedAgentsFS.ReadDir("agents/common")
	if err != nil {
		t.Fatalf("read embedded common agents: %v", err)
	}

	expected := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		contentEntries, err := embeddedAgentsFS.ReadDir("agents/common/" + entry.Name())
		if err != nil {
			t.Fatalf("read embedded common agent %q: %v", entry.Name(), err)
		}
		for _, contentEntry := range contentEntries {
			name := contentEntry.Name()
			if name == "content.md" || (strings.HasPrefix(name, "content-") && strings.HasSuffix(name, ".md")) {
				expected = append(expected, entry.Name())
				break
			}
		}
	}

	got := append([]string{}, commonAgents...)
	sort.Strings(expected)
	sort.Strings(got)
	if !slices.Equal(got, expected) {
		t.Fatalf("commonAgents must match embedded agents/common content dirs: expected %v, got %v", expected, got)
	}
}

func TestListSkillsIncludesAllRegistrySkills(t *testing.T) {
	got := listSkills("go")
	want := []string{
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
	}
	if !slices.Equal(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
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
			Targets:        []string{"cursor"},
			Agents:         []string{"linting", "testing"},
			Skills:         []string{"owasp-security-scan"},
			BallastVersion: "5.0.1",
			Languages:      []string{"typescript", "ansible"},
			Paths: map[string][]string{
				"typescript": {"apps/web"},
				"ansible":    {"infra/ansible"},
			},
			Tools: map[string][]string{
				"typescript": {"pnpm", "corepack"},
				"ansible":    {"ansible-lint", "molecule"},
			},
			Discovery: &discoveryConfig{
				ExcludePaths: []string{"examples", "tmp"},
			},
			TaskSystem:         "jira",
			DeploymentModel:    "serverless",
			PublishingProfiles: []string{"cli", "web"},
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
	if !strings.Contains(output, "- skills: owasp-security-scan") {
		t.Fatalf("expected skills in doctor output, got %q", output)
	}
	if !strings.Contains(output, "- languages: typescript, ansible") {
		t.Fatalf("expected languages in doctor output, got %q", output)
	}
	if !strings.Contains(output, "- paths: typescript=apps/web; ansible=infra/ansible") {
		t.Fatalf("expected paths in doctor output, got %q", output)
	}
	if !strings.Contains(output, "- tools: typescript=pnpm,corepack; ansible=ansible-lint,molecule") {
		t.Fatalf("expected tools in doctor output, got %q", output)
	}
	if !strings.Contains(output, "- discovery.excludePaths: examples,tmp") {
		t.Fatalf("expected discovery exclude paths in doctor output, got %q", output)
	}
	if !strings.Contains(output, "- taskSystem: jira") {
		t.Fatalf("expected task system in doctor output, got %q", output)
	}
	if !strings.Contains(output, "- deploymentModel: serverless") {
		t.Fatalf("expected deployment model in doctor output, got %q", output)
	}
	if !strings.Contains(output, "- publishingProfiles: cli, web") {
		t.Fatalf("expected publishing profiles in doctor output, got %q", output)
	}
}

func TestBuildContentWritesRuleMarkerAndDetectsDrift(t *testing.T) {
	content, err := buildContent("linting", "codex", "go", "", "pre-commit", "", "")
	if err != nil {
		t.Fatalf("build content: %v", err)
	}
	marker, ok := parseRuleMarker(content)
	if !ok {
		t.Fatalf("expected rule marker in %q", content[:120])
	}
	if marker.ruleID != "go/linting" {
		t.Fatalf("expected rule id go/linting, got %q", marker.ruleID)
	}
	if marker.version != resolveVersion() {
		t.Fatalf("expected marker version %q, got %q", resolveVersion(), marker.version)
	}
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(marker.checksum) {
		t.Fatalf("expected sha256 checksum, got %q", marker.checksum)
	}
	if !verifyRuleChecksum(content) {
		t.Fatalf("expected generated content checksum to verify")
	}
	if verifyRuleChecksum(content + "\nManual edit\n") {
		t.Fatalf("expected checksum verification to fail after body edit")
	}
}

func TestParseRuleMarkerRequiresGeneratedPrefix(t *testing.T) {
	content := "<!--\nballast:rule id=\"go/linting\" version=\"dev\" checksum=\"0123456789abcdef\" -->\n# Rule\n"
	if marker, ok := parseRuleMarker(content); ok {
		t.Fatalf("expected non-generated marker prefix to be ignored, got %#v", marker)
	}
	if stripped := stripRuleMarker(content); stripped != content {
		t.Fatalf("expected non-generated marker prefix to remain, got %q", stripped)
	}
}

func TestParseRuleMarkerRequiresGeneratedHeaderPosition(t *testing.T) {
	marker := "<!-- ballast:rule id=\"go/linting\" version=\"dev\" checksum=\"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\" -->\n"
	bodyExample := "# Notes\n\n```md\n" + marker + "```\n"
	if parsed, ok := parseRuleMarker(bodyExample); ok {
		t.Fatalf("expected copied marker in body to be ignored, got %#v", parsed)
	}
	if stripped := stripRuleMarker(bodyExample); stripped != bodyExample {
		t.Fatalf("expected copied marker in body to remain, got %q", stripped)
	}

	withFrontmatter := "---\ntitle: Rule\n---\n" + marker + "# Rule\n"
	parsed, ok := parseRuleMarker(withFrontmatter)
	if !ok {
		t.Fatal("expected marker after frontmatter to parse")
	}
	if parsed.ruleID != "go/linting" {
		t.Fatalf("expected rule id go/linting, got %q", parsed.ruleID)
	}
	if stripped := stripRuleMarker(withFrontmatter); stripped != "---\ntitle: Rule\n---\n# Rule\n" {
		t.Fatalf("expected marker after frontmatter to be stripped, got %q", stripped)
	}
}

func makeGitBoundary(t *testing.T, dir string) {
	t.Helper()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
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

func TestBuildDoctorReportOmitsEmptyTargetsAndAgents(t *testing.T) {
	output := buildDoctorReport(
		"ballast-go",
		"5.0.2",
		"/tmp/project/.rulesrc.json",
		&rulesConfig{
			Skills:         []string{"owasp-security-scan"},
			BallastVersion: "5.0.2",
		},
		[]installedCLIStatus{
			{Name: "ballast-go", Version: "5.0.2", Path: "/tmp/ballast-go"},
		},
	)

	if strings.Contains(output, "- targets: ") {
		t.Fatalf("expected empty targets line to be omitted, got %q", output)
	}
	if strings.Contains(output, "- agents: ") {
		t.Fatalf("expected empty agents line to be omitted, got %q", output)
	}
	if !strings.Contains(output, "- skills: owasp-security-scan") {
		t.Fatalf("expected skills in doctor output, got %q", output)
	}
}

func TestInstallAddsBallastToGitignore(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"cursor"},
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

func TestFindProjectRootSupportsAnsibleMarkers(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "ansible.cfg"), []byte("[defaults]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	makeGitBoundary(t, tmpDir)
	nested := filepath.Join(tmpDir, "roles", "novnc")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := findProjectRoot(nested)
	if err != nil {
		t.Fatalf("findProjectRoot returned error: %v", err)
	}
	if root != tmpDir {
		t.Fatalf("expected ansible project root %q, got %q", tmpDir, root)
	}
}

func TestFindProjectRootSupportsAnsibleRequirementsYaml(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "requirements.yaml"), []byte("---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	makeGitBoundary(t, tmpDir)
	nested := filepath.Join(tmpDir, "roles", "novnc")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := findProjectRoot(nested)
	if err != nil {
		t.Fatalf("findProjectRoot returned error: %v", err)
	}
	if root != tmpDir {
		t.Fatalf("expected ansible project root %q, got %q", tmpDir, root)
	}
}

func TestFindProjectRootSupportsTerraformMarkers(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, ".terraform-version"), []byte("1.8.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "versions.tf"), []byte("terraform {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	makeGitBoundary(t, tmpDir)
	nested := filepath.Join(tmpDir, "modules", "network")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := findProjectRoot(nested)
	if err != nil {
		t.Fatalf("findProjectRoot returned error: %v", err)
	}
	if root != tmpDir {
		t.Fatalf("expected terraform project root %q, got %q", tmpDir, root)
	}
}

func TestFindProjectRootSupportsDartFlutterMarkers(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "pubspec.yaml"), []byte("name: mobile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "analysis_options.yaml"), []byte("include: package:flutter_lints/flutter.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".metadata"), []byte("project_type: app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	makeGitBoundary(t, tmpDir)
	nested := filepath.Join(tmpDir, "lib", "src")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := findProjectRoot(nested)
	if err != nil {
		t.Fatalf("findProjectRoot returned error: %v", err)
	}
	if root != tmpDir {
		t.Fatalf("expected dart flutter project root %q, got %q", tmpDir, root)
	}
}

func TestFindProjectRootSupportsDockerMarkers(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "Dockerfile.prod"), []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	makeGitBoundary(t, tmpDir)
	nested := filepath.Join(tmpDir, "docker", "scripts")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := findProjectRoot(nested)
	if err != nil {
		t.Fatalf("findProjectRoot returned error: %v", err)
	}
	if root != tmpDir {
		t.Fatalf("expected docker project root %q, got %q", tmpDir, root)
	}
}

func TestFindProjectRootDoesNotCrossGitBoundary(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "playbook.yml"), []byte("---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(tmpDir, "child-project")
	makeGitBoundary(t, child)

	root, err := findProjectRoot(child)
	if err != nil {
		t.Fatalf("findProjectRoot returned error: %v", err)
	}
	if root != child {
		t.Fatalf("expected %q (cwd), got %q", child, root)
	}
}

func TestFindProjectRootReturnsUnmarkedCwdUnderNonGitParent(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, ".rulesrc.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(tmpDir, "new-product")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := findProjectRoot(child)
	if err != nil {
		t.Fatalf("findProjectRoot returned error: %v", err)
	}
	if root != child {
		t.Fatalf("expected %q (unmarked cwd), got %q", child, root)
	}
}

func TestFindProjectRootReturnsChildRepoRootForNestedCwd(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "playbook.yml"), []byte("---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(tmpDir, "child-project")
	nested := filepath.Join(child, "subdir")
	makeGitBoundary(t, child)
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := findProjectRoot(nested)
	if err != nil {
		t.Fatalf("findProjectRoot returned error: %v", err)
	}
	if root != child {
		t.Fatalf("expected %q (child repo root), got %q", child, root)
	}
}

func TestInstallRecordsGitignoreErrorAndContinues(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmpDir, ".gitignore"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"cursor"},
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
		targets:     []string{"cursor"},
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

func TestInstallSupportsDocsAgent(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"cursor"},
		agents:      []string{"docs"},
		language:    "go",
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}
	if !slices.Equal(result.installed, []string{"docs"}) {
		t.Fatalf("expected docs install, got %+v", result.installed)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, ".cursor", "rules", "docs.mdc"))
	if err != nil {
		t.Fatalf("expected docs.mdc to exist: %v", err)
	}
	if !strings.Contains(string(content), "Documentation Agent") {
		t.Fatalf("expected docs content, got %q", string(content))
	}
	if !strings.Contains(string(content), "publish-docs") {
		t.Fatalf("expected publish-docs guidance, got %q", string(content))
	}
	if !strings.HasPrefix(string(content), "---\n") {
		t.Fatalf("expected cursor frontmatter delimiters, got %q", string(content))
	}
}

func TestInstallSupportsDocsAgentForOpenCodeWithFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"opencode"},
		agents:      []string{"docs"},
		language:    "go",
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}
	if !slices.Equal(result.installed, []string{"docs"}) {
		t.Fatalf("expected docs install, got %+v", result.installed)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, ".opencode", "docs.md"))
	if err != nil {
		t.Fatalf("expected docs.md to exist: %v", err)
	}
	if !strings.HasPrefix(string(content), "---\n") {
		t.Fatalf("expected opencode frontmatter delimiters, got %q", string(content))
	}
	if !strings.Contains(string(content), "mode: subagent") {
		t.Fatalf("expected opencode mode in frontmatter, got %q", string(content))
	}
	if !strings.Contains(string(content), "Documentation Agent") {
		t.Fatalf("expected docs content, got %q", string(content))
	}
}

func TestInstallSupportsTerraformLanguageProfile(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"cursor"},
		agents:      []string{"linting"},
		language:    "terraform",
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}
	if !slices.Contains(result.installed, "linting") || !slices.Contains(result.installed, "git-hooks") {
		t.Fatalf("expected terraform linting and git-hooks install, got %+v", result.installed)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, ".cursor", "rules", "terraform-linting.mdc"))
	if err != nil {
		t.Fatalf("expected terraform-linting.mdc to exist: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "Terraform linting specialist") {
		t.Fatalf("expected terraform linting content, got %q", text)
	}
	if !strings.Contains(text, ".terraform-version") ||
		!strings.Contains(text, "tfenv install") ||
		!strings.Contains(text, "trivy config") ||
		!strings.Contains(text, "tfsec") ||
		!strings.Contains(text, "plugin blocks") ||
		!strings.Contains(text, "tflint --init") ||
		!strings.Contains(text, "OpenTofu") ||
		!strings.Contains(text, "tofu fmt") {
		t.Fatalf("expected terraform hook and tooling guidance, got %q", text)
	}
}

func TestInstallSupportsTerraformTestingBestPractices(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"codex"},
		agents:      []string{"testing"},
		language:    "terraform",
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}
	if !slices.Contains(result.installed, "testing") {
		t.Fatalf("expected terraform testing install, got %+v", result.installed)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, ".codex", "rules", "terraform-testing.md"))
	if err != nil {
		t.Fatalf("expected terraform-testing.md to exist: %v", err)
	}
	text := string(content)
	for _, want := range []string{
		"terraform test",
		"Terraform 1.6",
		"Terratest",
		"concurrency:",
		"PR validation",
		"OpenTofu",
		"tofu test",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected terraform testing content to mention %q, got %q", want, text)
		}
	}
}

func TestInstallSupportsDartFlutterLanguageProfile(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"codex"},
		agents:      []string{"linting", "logging", "testing"},
		language:    "dart",
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}
	for _, want := range []string{"linting", "logging", "testing", "git-hooks"} {
		if !slices.Contains(result.installed, want) {
			t.Fatalf("expected dart %s install, got %+v", want, result.installed)
		}
	}

	checks := map[string][]string{
		"dart-linting.md": {
			"Dart and Flutter linting specialist",
			"flutter_lints",
			"dart format --set-exit-if-changed",
			"flutter analyze",
		},
		"dart-logging.md": {
			"Dart and Flutter logging specialist",
			"dart:developer",
			"Crashlytics",
		},
		"dart-testing.md": {
			"Dart and Flutter testing specialist",
			"flutter test",
			"integration_test",
		},
		"git-hooks.md": {
			"dart format --set-exit-if-changed",
			"flutter analyze",
			"flutter test",
			".dart_tool/",
		},
	}
	for file, wants := range checks {
		content, err := os.ReadFile(filepath.Join(tmpDir, ".codex", "rules", file))
		if err != nil {
			t.Fatalf("expected %s to exist: %v", file, err)
		}
		text := string(content)
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Fatalf("expected %s content to mention %q, got %q", file, want, text)
			}
		}
	}
}

func TestRenderGitHooksContentSupportsTerraform(t *testing.T) {
	got, err := readContent("git-hooks", "terraform", "", "standalone", "github", "none")
	if err != nil {
		t.Fatalf("read terraform git-hooks content: %v", err)
	}
	if !strings.Contains(got, ".terraform-version") {
		t.Fatalf("expected terraform git-hooks content to mention .terraform-version, got %q", got)
	}
	if !strings.Contains(got, "tfenv install") {
		t.Fatalf("expected terraform git-hooks content to mention tfenv install, got %q", got)
	}
	if !strings.Contains(got, "terraform fmt -check -recursive") ||
		!strings.Contains(got, "terraform init -backend=false") ||
		!strings.Contains(got, "terraform validate") ||
		!strings.Contains(got, "tflint --init") ||
		!strings.Contains(got, "tflint --recursive") ||
		!strings.Contains(got, "trivy config .") ||
		!strings.Contains(got, "tfsec") {
		t.Fatalf("expected terraform git-hooks content to mention terraform checks, got %q", got)
	}
	if !strings.Contains(got, "gitleaks") || strings.Contains(got, "scripts/check-no-secrets.sh") {
		t.Fatalf("expected terraform git-hooks content to use gitleaks guidance, got %q", got)
	}
}

func TestRenderGitHooksContentSupportsDartFlutter(t *testing.T) {
	got, err := readContent("git-hooks", "dart", "", "standalone", "github", "none")
	if err != nil {
		t.Fatalf("read dart git-hooks content: %v", err)
	}
	for _, want := range []string{
		"dart format --set-exit-if-changed .",
		"flutter analyze",
		"flutter test",
		"flutter test integration_test",
		".dart_tool/",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected dart git-hooks content to mention %q, got %q", want, got)
		}
	}
	if !strings.Contains(got, "gitleaks") || strings.Contains(got, "scripts/check-no-secrets.sh") {
		t.Fatalf("expected dart git-hooks content to use gitleaks guidance, got %q", got)
	}
}

func TestRenderGitHooksContentSupportsDocker(t *testing.T) {
	got, err := readContent("git-hooks", "docker", "", "standalone", "github", "none")
	if err != nil {
		t.Fatalf("read docker git-hooks content: %v", err)
	}
	for _, want := range []string{
		"Dockerfile and container configuration",
		"hadolint",
		"docker compose config",
		"pre-commit install --hook-type pre-push",
		"image vulnerability scans",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected docker git-hooks content to mention %q, got %q", want, got)
		}
	}
	if !strings.Contains(got, "gitleaks") || strings.Contains(got, "scripts/check-no-secrets.sh") {
		t.Fatalf("expected docker git-hooks content to use gitleaks guidance, got %q", got)
	}
}

func TestRenderGitHooksContentSupportsGoGitleaksGuidance(t *testing.T) {
	got, err := readContent("git-hooks", "go", "", "standalone", "github", "none")
	if err != nil {
		t.Fatalf("read go git-hooks content: %v", err)
	}
	for _, want := range []string{
		"sub-pre-commit",
		"pre-commit install --hook-type pre-push",
		"Go unit tests",
		"gitleaks",
		"govulncheck",
		"go test -race",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected go git-hooks content to mention %q, got %q", want, got)
		}
	}
	if strings.Contains(got, "scripts/check-no-secrets.sh") {
		t.Fatalf("expected go git-hooks content to avoid local no-secrets script, got %q", got)
	}
}

func TestRenderGitHooksContentSupportsAnsibleGitleaksGuidance(t *testing.T) {
	got, err := readContent("git-hooks", "ansible", "", "standalone", "github", "none")
	if err != nil {
		t.Fatalf("read ansible git-hooks content: %v", err)
	}
	for _, want := range []string{
		"ansible-lint",
		"yamllint",
		"ansible-playbook --syntax-check",
		"pre-push stage",
		"pre-commit autoupdate",
		"gitleaks",
		"ansible-lint --profile=safety",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected ansible git-hooks content to mention %q, got %q", want, got)
		}
	}
	for _, unwanted := range []string{
		"Use Husky for TypeScript-only repositories.",
		"Husky",
		"lint-staged",
		"npx lint-staged",
		"scripts/check-no-secrets.sh",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("expected ansible git-hooks content not to mention %q, got %q", unwanted, got)
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
		targets:     []string{"codex"},
		agents:      []string{"linting"},
		language:    "go",
		force:       false,
		saveConfig:  false,
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}
	if !slices.Contains(result.installed, "linting") {
		t.Fatalf("expected linting to be installed, got %+v", result.installed)
	}
	if !slices.Contains(result.installed, "git-hooks") {
		t.Fatalf("expected git-hooks to be installed with linting, got %+v", result.installed)
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
	if strings.Contains(string(content), "pre-commit install") {
		t.Fatalf("expected hook guidance to move out of linting, got %s", string(content))
	}

	gitHooksPath := filepath.Join(tmpDir, ".codex", "rules", "git-hooks.md")
	gitHooksContent, err := os.ReadFile(gitHooksPath)
	if err != nil {
		t.Fatalf("read git-hooks.md: %v", err)
	}
	if !strings.Contains(string(gitHooksContent), "pre-commit install --hook-type pre-push") {
		t.Fatalf("expected dedicated git-hooks guidance, got %s", string(gitHooksContent))
	}
	if !strings.Contains(string(gitHooksContent), "gitleaks") {
		t.Fatalf("expected pre-commit secret detection guidance, got %s", string(gitHooksContent))
	}
	if strings.Contains(string(gitHooksContent), "scripts/check-no-secrets.sh") {
		t.Fatalf("expected no legacy no-secrets script guidance, got %s", string(gitHooksContent))
	}
}

func TestInstallWritesRulesForMultipleTargets(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"cursor", "claude"},
		agents:      []string{"linting"},
		language:    "go",
		force:       false,
		saveConfig:  true,
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}
	if !slices.Contains(result.installed, "linting") {
		t.Fatalf("expected linting to be installed, got %+v", result.installed)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".cursor", "rules", "go-linting.mdc")); err != nil {
		t.Fatalf("expected cursor rule file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".claude", "rules", "go-linting.md")); err != nil {
		t.Fatalf("expected claude rule file: %v", err)
	}
	config, err := os.ReadFile(filepath.Join(tmpDir, ".rulesrc.json"))
	if err != nil {
		t.Fatalf("read .rulesrc.json: %v", err)
	}
	text := string(config)
	if !strings.Contains(text, `"targets": [`+"\n"+`    "cursor",`) || !strings.Contains(text, `"claude"`) {
		t.Fatalf("expected multi-target config, got %s", text)
	}
}

func TestInstallWritesConfiguredToolsIntoCodexAndClaudeRules(t *testing.T) {
	tmpDir := t.TempDir()
	config := `{
  "targets": ["codex", "claude"],
  "agents": ["testing"],
  "languages": ["python"],
  "paths": {"python": ["."]},
  "tools": {"Python": ["UV", "pyenv"]}
}`
	if err := os.WriteFile(filepath.Join(tmpDir, ".rulesrc.json"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"codex", "claude"},
		agents:      []string{"testing"},
		language:    "python",
		force:       true,
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}

	for _, rulePath := range []string{
		filepath.Join(tmpDir, ".codex", "rules", "python-testing.md"),
		filepath.Join(tmpDir, ".claude", "rules", "python-testing.md"),
	} {
		content, err := os.ReadFile(rulePath)
		if err != nil {
			t.Fatalf("read %s: %v", rulePath, err)
		}
		if strings.Contains(string(content), "Repository Tool Policy") {
			t.Fatalf("expected no per-rule tool policy in %s, got %s", rulePath, string(content))
		}
	}

	for _, manifestPath := range []string{
		filepath.Join(tmpDir, "AGENTS.md"),
		filepath.Join(tmpDir, "CLAUDE.md"),
	} {
		content, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("read %s: %v", manifestPath, err)
		}
		text := string(content)
		if !strings.Contains(text, "### Repository Tool Policy") {
			t.Fatalf("expected tool policy in %s, got %s", manifestPath, text)
		}
		if !strings.Contains(text, "python=uv,pyenv") {
			t.Fatalf("expected configured tools in %s, got %s", manifestPath, text)
		}
		if !strings.Contains(text, "uv run <command>") {
			t.Fatalf("expected uv guidance in %s, got %s", manifestPath, text)
		}
		if strings.Count(text, "Repository Tool Policy") != 1 {
			t.Fatalf("expected exactly one tool policy in %s, got %s", manifestPath, text)
		}
	}
}

func TestInstallRendersDefaultToolsWhenSavingConfig(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"codex"},
		agents:      []string{"testing"},
		language:    "python",
		force:       true,
		saveConfig:  true,
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}

	rulePath := filepath.Join(tmpDir, ".codex", "rules", "python-testing.md")
	content, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatalf("read %s: %v", rulePath, err)
	}
	if strings.Contains(string(content), "Repository Tool Policy") {
		t.Fatalf("expected no per-rule tool policy in %s, got %s", rulePath, string(content))
	}

	manifestPath := filepath.Join(tmpDir, "AGENTS.md")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read %s: %v", manifestPath, err)
	}
	text := string(manifest)
	if !strings.Contains(text, "### Repository Tool Policy") {
		t.Fatalf("expected tool policy in %s, got %s", manifestPath, text)
	}
	if !strings.Contains(text, "python=uv,pyenv") {
		t.Fatalf("expected default tools in %s, got %s", manifestPath, text)
	}
	if !strings.Contains(text, "uv run <command>") {
		t.Fatalf("expected uv guidance in %s, got %s", manifestPath, text)
	}
}

func TestResolvesContentFragmentIncludes(t *testing.T) {
	content, err := readContent("testing-process", "go", "", "pre-commit", "github", "none")
	if err != nil {
		t.Fatalf("readContent(testing-process): %v", err)
	}
	if !strings.Contains(content, "1. Start from acceptance criteria in `PRD.md`, the linked issue, or the current task.") {
		t.Fatalf("expected TDD fragment expanded, got %q", content)
	}
	if strings.Contains(content, "{{include:") {
		t.Fatalf("expected include tokens resolved, got %q", content)
	}
}

func TestLanguageTestingRulesPointAtTestingProcess(t *testing.T) {
	for _, language := range []string{"go", "python", "typescript"} {
		content, err := readContent("testing", language, "", "pre-commit", "github", "none")
		if err != nil {
			t.Fatalf("readContent(%s): %v", language, err)
		}
		if !strings.Contains(content, "testing-process") {
			t.Fatalf("expected pointer to testing-process for %s, got %q", language, content)
		}
		if strings.Contains(content, "## TDD Process Discipline") {
			t.Fatalf("expected TDD section removed for %s, got %q", language, content)
		}
	}
}

func TestMissingFragmentIncludeFails(t *testing.T) {
	_, err := resolveContentIncludes("{{include:common/fragments/does-not-exist.md}}", nil)
	if err == nil || !strings.Contains(err.Error(), "does-not-exist.md") {
		t.Fatalf("expected missing fragment error, got %v", err)
	}
}

func TestIncludePathEscapeRejected(t *testing.T) {
	for _, bad := range []string{
		"../secrets.md",
		"/etc/passwd.md",
		"C:/windows/system.md",
		`C:\windows.md`,
		`\\server\share.md`,
		`common\fragments\tdd-process.md`,
	} {
		_, err := resolveContentIncludes("{{include:"+bad+"}}", nil)
		if err == nil || !strings.Contains(err.Error(), "invalid include path") {
			t.Fatalf("expected invalid path error for %q, got %v", bad, err)
		}
	}
}

func TestRecursiveFragmentIncludeFails(t *testing.T) {
	root := t.TempDir()
	fragments := filepath.Join(root, "agents", "common", "fragments")
	if err := os.MkdirAll(fragments, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fragments, "loop.md"), []byte("{{include:common/fragments/loop.md}}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("BALLAST_REPO_ROOT", root)

	_, err := resolveContentIncludes("{{include:common/fragments/loop.md}}", nil)
	if err == nil || !strings.Contains(err.Error(), "recursive include") {
		t.Fatalf("expected recursion error, got %v", err)
	}
}

func TestTaskSystemRuleRendersOnlyConfiguredSystemAndTarget(t *testing.T) {
	claude, err := buildContent("tasks", "claude", "go", "task-system", "pre-commit", "github", "none")
	if err != nil {
		t.Fatalf("buildContent: %v", err)
	}
	if !strings.Contains(claude, "GITHUB_PERSONAL_ACCESS_TOKEN") {
		t.Fatalf("expected github MCP config, got %q", claude)
	}
	if strings.Contains(claude, "JIRA_API_TOKEN") || strings.Contains(claude, "LINEAR_API_KEY") {
		t.Fatalf("expected other task systems omitted, got %q", claude)
	}
	if !strings.Contains(claude, "**Claude Code:**") || strings.Contains(claude, "**Codex:**") {
		t.Fatalf("expected only claude platform steps, got %q", claude)
	}
	if strings.Contains(claude, "BALLAST_IF") {
		t.Fatalf("expected conditional markers stripped, got %q", claude)
	}

	codex, err := buildContent("tasks", "codex", "go", "task-system", "pre-commit", "jira", "none")
	if err != nil {
		t.Fatalf("buildContent: %v", err)
	}
	if !strings.Contains(codex, "JIRA_API_TOKEN") || strings.Contains(codex, "GITHUB_PERSONAL_ACCESS_TOKEN") {
		t.Fatalf("expected only jira MCP config, got %q", codex)
	}
	if !strings.Contains(codex, "**Codex:**") || strings.Contains(codex, "**Claude Code:**") {
		t.Fatalf("expected only codex platform steps, got %q", codex)
	}
}

func TestPublishingSuffixesExcludeOptInVariantsByDefault(t *testing.T) {
	suffixes, err := listRuleSuffixes("publishing", "go")
	if err != nil {
		t.Fatalf("listRuleSuffixes: %v", err)
	}
	suffixes = filterPublishingSuffixes("publishing", suffixes, nil)

	if contains(suffixes, "apt") || contains(suffixes, "brew") {
		t.Fatalf("expected opt-in variants excluded by default, got %v", suffixes)
	}
	if len(suffixes) != 6 {
		t.Fatalf("expected 6 default publishing suffixes, got %v", suffixes)
	}
}

func TestPublishingSuffixesHonorExplicitProfiles(t *testing.T) {
	suffixes, err := listRuleSuffixes("publishing", "go")
	if err != nil {
		t.Fatalf("listRuleSuffixes: %v", err)
	}
	selected := filterPublishingSuffixes("publishing", suffixes, []string{"cli", "apt", "brew"})

	if len(selected) != 3 || !contains(selected, "apt") || !contains(selected, "brew") {
		t.Fatalf("expected explicit opt-in profiles honored, got %v", selected)
	}
}

func TestPublishingAPIOmitsKubernetesSectionsForNone(t *testing.T) {
	content, err := buildContent("publishing", "codex", "go", "api", "pre-commit", "github", "none")
	if err != nil {
		t.Fatalf("buildContent: %v", err)
	}
	if strings.Contains(content, "Kubernetes Helm Chart: Probes Configuration") {
		t.Fatalf("expected kubernetes section stripped for deploymentModel none, got %q", content)
	}
	if strings.Contains(content, "Minimal Go Implementation") {
		t.Fatalf("expected health endpoint code stripped for deploymentModel none, got %q", content)
	}
	if strings.Contains(content, "BALLAST_IF_DEPLOYMENT") {
		t.Fatalf("expected conditional markers removed, got %q", content)
	}
}

func TestPublishingAPIKeepsKubernetesSectionsForKubernetes(t *testing.T) {
	content, err := buildContent("publishing", "codex", "go", "api", "pre-commit", "github", "kubernetes")
	if err != nil {
		t.Fatalf("buildContent: %v", err)
	}
	if !strings.Contains(content, "Kubernetes Helm Chart: Probes Configuration") {
		t.Fatalf("expected kubernetes section kept, got %q", content)
	}
	if !strings.Contains(content, "Minimal Go Implementation") {
		t.Fatalf("expected health endpoint code kept, got %q", content)
	}
	if strings.Contains(content, "BALLAST_IF_DEPLOYMENT") {
		t.Fatalf("expected conditional markers removed, got %q", content)
	}
}

func TestLocalDevHasNoMcpSuffix(t *testing.T) {
	suffixes, err := listRuleSuffixes("local-dev", "go")
	if err != nil {
		t.Fatalf("listRuleSuffixes: %v", err)
	}
	if contains(suffixes, "mcp") {
		t.Fatalf("expected mcp rule removed from local-dev, got %v", suffixes)
	}
}

func TestPatchFillsPlaceholderRepositoryFacts(t *testing.T) {
	existing := strings.Join([]string{
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
	}, "\n")
	canonical := strings.Join([]string{
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
	}, "\n")

	merged := patchCodexAgentsMD(existing, canonical)

	if !strings.Contains(merged, "- Canonical GitHub repo: `acme/widgets`") {
		t.Fatalf("expected placeholder fact filled, got %q", merged)
	}
	if !strings.Contains(merged, "- Primary package manager: `bun`") {
		t.Fatalf("expected user-edited fact preserved, got %q", merged)
	}
	if !strings.Contains(merged, "- Coverage threshold: `<value>`") {
		t.Fatalf("expected unresolved placeholder kept, got %q", merged)
	}
}

func TestPatchReplacesPristineRulesWholesale(t *testing.T) {
	canonicalOld, err := buildContent("testing", "claude", "go", "", "pre-commit", "github", "none")
	if err != nil {
		t.Fatalf("buildContent: %v", err)
	}
	canonicalNew := strings.ReplaceAll(canonicalOld, "## Framework Markers", "## Renamed Markers")

	merged := patchRuleContent(canonicalOld, canonicalNew, "claude")

	if merged != normalizeLineEndings(canonicalNew) {
		t.Fatalf("expected pristine rule replaced wholesale, got %q", merged)
	}
}

func TestPatchMergesWhenRuleWasUserEdited(t *testing.T) {
	canonical, err := buildContent("testing", "claude", "go", "", "pre-commit", "github", "none")
	if err != nil {
		t.Fatalf("buildContent: %v", err)
	}
	edited := canonical + "\n## Team Notes\n\nKeep this.\n"

	merged := patchRuleContent(edited, canonical, "claude")

	if !strings.Contains(merged, "## Team Notes") || !strings.Contains(merged, "Keep this.") {
		t.Fatalf("expected user-edited section preserved, got %q", merged)
	}
}

func TestPatchDropsStaleToolPolicyWhenCanonicalOmitsIt(t *testing.T) {
	existing := "# Rules\n\nIntro.\n\n## Repository Tool Policy\n\n- Check `.rulesrc.json` `tools` before adding, installing, or running language tooling.\n\n## Keep\n\nBody.\n"
	canonical := "# Rules\n\nIntro.\n\n## Keep\n\nBody.\n"

	merged := patchRuleContent(existing, canonical, "claude")

	if strings.Contains(merged, "Repository Tool Policy") {
		t.Fatalf("expected stale tool policy to be dropped, got %q", merged)
	}
	if !strings.Contains(merged, "## Keep") {
		t.Fatalf("expected other sections preserved, got %q", merged)
	}
}

func TestPatchPreservesUserSectionReusingPolicyHeading(t *testing.T) {
	existing := "# Rules\n\n## Repository Tool Policy\n\nOur team's own notes about tooling, not Ballast-generated.\n"
	canonical := "# Rules\n\nIntro.\n"

	merged := patchRuleContent(existing, canonical, "claude")

	if !strings.Contains(merged, "Our team's own notes about tooling") {
		t.Fatalf("expected user-authored section preserved, got %q", merged)
	}
}

func TestPatchKeepsToolPolicyWhenCanonicalIncludesIt(t *testing.T) {
	existing := "# Rules\n\n## Repository Tool Policy\n\n- Existing bullets.\n"
	canonical := "# Rules\n\n## Repository Tool Policy\n\n- Canonical bullets.\n"

	merged := patchRuleContent(existing, canonical, "claude")

	if !strings.Contains(merged, "## Repository Tool Policy") {
		t.Fatalf("expected tool policy preserved, got %q", merged)
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
	if !strings.Contains(content, "Created by [Ballast]") {
		t.Fatalf("expected managed skill marker: %s", content)
	}
	if strings.Contains(content, "description: Run OWASP-aligned security scans across Go, TypeScript, and Python codebases.") {
		t.Fatalf("expected description to remain quoted: %s", content)
	}
	if strings.Contains(content, "\n---\n---\n") {
		t.Fatalf("expected normalized markdown body without duplicate frontmatter: %s", content)
	}
}

func TestBuildCursorSkillFormatIncludesBallastAuditFrontmatter(t *testing.T) {
	content, err := buildCursorSkillFormat("ballast-audit", "go")
	if err != nil {
		t.Fatalf("buildCursorSkillFormat(ballast-audit): %v", err)
	}
	if !strings.Contains(content, "alwaysApply: false") {
		t.Fatalf("expected alwaysApply false frontmatter: %s", content)
	}
	if !strings.Contains(content, "description: \"audit AI rule and skill files for context density, duplication, and bloat\"") {
		t.Fatalf("expected ballast-audit description in frontmatter: %s", content)
	}
	if !strings.Contains(content, "# Ballast Audit Skill") {
		t.Fatalf("expected ballast-audit body: %s", content)
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
		{target: "codex", dir: filepath.Join(tmpDir, ".codex", "skills", "owasp-security-scan"), file: filepath.Join(tmpDir, ".codex", "skills", "owasp-security-scan", "SKILL.md")},
		{target: "gemini", dir: filepath.Join(tmpDir, ".gemini", "rules"), file: filepath.Join(tmpDir, ".gemini", "rules", "owasp-security-scan.md")},
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

func TestDestinationReturnsGeminiRulePath(t *testing.T) {
	tmpDir := t.TempDir()

	dir, file, err := destination(tmpDir, "gemini", "go-linting")
	if err != nil {
		t.Fatalf("destination(gemini): %v", err)
	}
	if dir != geminiRuleDir(tmpDir) {
		t.Fatalf("unexpected gemini dir: got %q want %q", dir, geminiRuleDir(tmpDir))
	}
	if file != filepath.Join(tmpDir, ".gemini", "rules", "go-linting.md") {
		t.Fatalf("unexpected gemini file path: %q", file)
	}
	if geminiMDPath(tmpDir) != filepath.Join(tmpDir, "GEMINI.md") {
		t.Fatalf("unexpected gemini support file path: %q", geminiMDPath(tmpDir))
	}
}

func TestBuildGeminiMDIncludesRepositoryFactsAndSkills(t *testing.T) {
	content, err := buildGeminiMD([]string{"linting"}, []string{"owasp-security-scan"}, "go", nil, nil)
	if err != nil {
		t.Fatalf("buildGeminiMD: %v", err)
	}

	if !strings.Contains(content, "# GEMINI.md") {
		t.Fatalf("expected GEMINI.md heading, got %q", content)
	}
	if !strings.Contains(content, "## Repository Facts") {
		t.Fatalf("expected repository facts section, got %q", content)
	}
	if !strings.Contains(content, "## Memory Tiering") {
		t.Fatalf("expected memory tiering section, got %q", content)
	}
	if !strings.Contains(content, "`.gemini/rules/go-linting.md`") {
		t.Fatalf("expected gemini linting rule, got %q", content)
	}
	if !strings.Contains(content, "`.gemini/rules/owasp-security-scan.md`") {
		t.Fatalf("expected gemini skill entry, got %q", content)
	}
}

func TestBuildContentGeminiIncludesMandates(t *testing.T) {
	content, err := buildContent("linting", "gemini", "go", "", "standalone", "github", "none")
	if err != nil {
		t.Fatalf("buildContent(gemini): %v", err)
	}

	if !strings.Contains(content, "## Gemini Mandates") {
		t.Fatalf("expected Gemini mandates in content, got %q", content)
	}
	if !strings.Contains(content, "### Narrative Flow") {
		t.Fatalf("expected narrative flow section, got %q", content)
	}
	if !strings.Contains(content, "Go linting specialist") {
		t.Fatalf("expected go linting body, got %q", content)
	}
}

func TestBuildContentRendersPublishingDeploymentModelToken(t *testing.T) {
	content, err := buildContent("publishing", "codex", "go", "apps", "standalone", "github", "kubernetes")
	if err != nil {
		t.Fatalf("buildContent(publishing): %v", err)
	}

	if strings.Contains(content, deploymentModelGuidanceToken) {
		t.Fatalf("expected deployment model token to be replaced, got %q", content)
	}
	if !strings.Contains(content, "Kubernetes deployment model") {
		t.Fatalf("expected kubernetes guidance, got %q", content)
	}
	if !strings.Contains(content, "Deployment guidance is active (`deploymentModel: kubernetes`).") {
		t.Fatalf("expected active deployment activation guidance, got %q", content)
	}
	if !strings.Contains(content, "charts/<app>/") {
		t.Fatalf("expected app chart guidance, got %q", content)
	}
}

func TestBuildContentRendersInactiveDeploymentModel(t *testing.T) {
	content, err := buildContent("publishing", "codex", "go", "web", "standalone", "github", "none")
	if err != nil {
		t.Fatalf("buildContent(publishing): %v", err)
	}

	if strings.Contains(content, deploymentModelGuidanceToken) {
		t.Fatalf("expected deployment model token to be replaced, got %q", content)
	}
	if !strings.Contains(content, "Deployment is inactive") {
		t.Fatalf("expected inactive deployment guidance, got %q", content)
	}
	if !strings.Contains(content, "Deployment guidance is reference-only") {
		t.Fatalf("expected reference-only deployment guidance, got %q", content)
	}
	if !strings.Contains(content, "do not create deploy-on-main workflows") {
		t.Fatalf("expected deploy-on-main guardrail, got %q", content)
	}
}

func TestBuildContentRendersDockerDeploymentModel(t *testing.T) {
	content, err := buildContent("publishing", "codex", "go", "web", "standalone", "github", "docker")
	if err != nil {
		t.Fatalf("buildContent(publishing): %v", err)
	}

	if strings.Contains(content, deploymentModelGuidanceToken) {
		t.Fatalf("expected deployment model token to be replaced, got %q", content)
	}
	if !strings.Contains(content, "Deployment guidance is active (`deploymentModel: docker`).") {
		t.Fatalf("expected docker deployment activation guidance, got %q", content)
	}
	if !strings.Contains(content, "GHCR and Docker Hub") {
		t.Fatalf("expected registry guidance, got %q", content)
	}
}

func TestBuildContentMarksOptionalPublishingVariants(t *testing.T) {
	for _, suffix := range []string{"apt", "brew"} {
		content, err := buildContent("publishing", "codex", "go", suffix, "standalone", "github", "none")
		if err != nil {
			t.Fatalf("buildContent(publishing %s): %v", suffix, err)
		}
		if !strings.Contains(content, "This optional publishing variant is inactive by default.") {
			t.Fatalf("expected optional activation guidance in %s, got %q", suffix, content)
		}
		if !strings.Contains(content, "Treat this rule as reference-only unless it is explicitly configured") {
			t.Fatalf("expected reference-only optional guidance in %s, got %q", suffix, content)
		}
	}
}

func TestBuildContentRendersActiveTaskSystem(t *testing.T) {
	content, err := buildContent("tasks", "codex", "go", "task-system", "standalone", "github", "none")
	if err != nil {
		t.Fatalf("buildContent(tasks): %v", err)
	}

	if !strings.Contains(content, "External issue tracking is active (`taskSystem: github`).") {
		t.Fatalf("expected active task-system guidance, got %q", content)
	}
	if !strings.Contains(content, "**GitHub** as the system of record") {
		t.Fatalf("expected GitHub display name in task-system guidance, got %q", content)
	}
	if strings.Contains(content, "**github**") {
		t.Fatalf("expected GitHub brand capitalization, got %q", content)
	}
}

func TestBuildContentRendersNoneTaskSystem(t *testing.T) {
	content, err := buildContent("tasks", "codex", "go", "task-system", "standalone", "none", "none")
	if err != nil {
		t.Fatalf("buildContent(tasks): %v", err)
	}

	if strings.Contains(content, taskSystemGuidanceToken) || strings.Contains(content, taskSystemToken) {
		t.Fatalf("expected task system tokens to be replaced, got %q", content)
	}
	if !strings.Contains(content, "taskSystem: none") {
		t.Fatalf("expected none task system guidance, got %q", content)
	}
	if !strings.Contains(content, "External issue tracking is disabled (`taskSystem: none`).") {
		t.Fatalf("expected disabled task-system guidance, got %q", content)
	}
	if strings.Contains(content, "All durable work items must be created there") {
		t.Fatalf("expected no mandatory tracker guidance, got %q", content)
	}
}

func TestInstallCreatesGeminiSupportFileOnly(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"gemini"},
		agents:      []string{"linting"},
		language:    "go",
		force:       false,
		saveConfig:  false,
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}

	geminiPath := filepath.Join(tmpDir, "GEMINI.md")
	content, err := os.ReadFile(geminiPath)
	if err != nil {
		t.Fatalf("read GEMINI.md: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "## Repository Facts") {
		t.Fatalf("expected repository facts section, got %q", text)
	}
	if !strings.Contains(text, "## Memory Tiering") {
		t.Fatalf("expected memory tiering section, got %q", text)
	}
	if !strings.Contains(text, "`.gemini/rules/go-linting.md`") {
		t.Fatalf("expected gemini linting rule entry, got %q", text)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("expected AGENTS.md to remain absent, stat err=%v", err)
	}
}

func TestInstallPatchUpdatesGeminiMDSectionOnly(t *testing.T) {
	tmpDir := t.TempDir()
	rulePath := filepath.Join(tmpDir, ".gemini", "rules", "go-linting.md")
	if err := os.MkdirAll(filepath.Dir(rulePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rulePath, []byte("# placeholder\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	geminiMD := filepath.Join(tmpDir, "GEMINI.md")
	if err := os.WriteFile(geminiMD, []byte("# GEMINI.md\n\n## Team Notes\n\nKeep this section.\n\n## Installed agent rules\n\nRead and follow these rule files in `.gemini/rules/` when they apply:\n\n- `.gemini/rules/old.md` - Old rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"gemini"},
		agents:      []string{"linting"},
		language:    "go",
		force:       false,
		patchGemini: true,
		saveConfig:  false,
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}

	content, err := os.ReadFile(geminiMD)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "## Team Notes") {
		t.Fatalf("expected team notes to remain, got %q", text)
	}
	if !strings.Contains(text, "`.gemini/rules/go-linting.md`") {
		t.Fatalf("expected gemini linting rule entry, got %q", text)
	}
	if strings.Contains(text, "`.gemini/rules/old.md`") {
		t.Fatalf("expected old installed-rules entry to be replaced, got %q", text)
	}
}

func TestRunInstallWritesGeminiSupportFilesAndConfig(t *testing.T) {
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

	exitCode := runInstall([]string{"install", "--target", "gemini", "--agent", "linting", "--yes"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, ".gemini", "rules", "go-linting.md")); err != nil {
		t.Fatalf("expected gemini rule file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "GEMINI.md")); err != nil {
		t.Fatalf("expected GEMINI.md to be created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("expected AGENTS.md to stay absent, stat err=%v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, ".rulesrc.json"))
	if err != nil {
		t.Fatalf("read .rulesrc.json: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, `"targets": [`+"\n"+`    "gemini"`) {
		t.Fatalf("expected gemini target in config, got %q", text)
	}
}

func TestRunInstallPromptsToPatchExistingGeminiMD(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")
	t.Setenv("TF_BUILD", "")
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
	geminiPath := filepath.Join(tmpDir, "GEMINI.md")
	if err := os.WriteFile(geminiPath, []byte("# GEMINI.md\n\n## Team Notes\n\nKeep this section.\n\n## Installed agent rules\n\nCreated by [Ballast](https://github.com/everydaydevopsio/ballast) v9.9.9-test. Do not edit this section.\n\n- `.gemini/rules/old.md` - Old rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	originalStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = originalStdin
		_ = reader.Close()
	})
	if _, err := writer.WriteString("y\n"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		exitCode := runInstall([]string{"install", "--target", "gemini", "--agent", "linting"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	content, err := os.ReadFile(geminiPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "## Team Notes") {
		t.Fatalf("expected team notes to remain after patch, got %q", text)
	}
	if !strings.Contains(text, "`.gemini/rules/go-linting.md`") {
		t.Fatalf("expected linting rule to be installed, got %q", text)
	}
	if strings.Contains(text, "`.gemini/rules/old.md`") {
		t.Fatalf("expected old rule entry to be replaced, got %q", text)
	}
	if !strings.Contains(output, "GEMINI.md -> ") {
		t.Fatalf("expected GEMINI.md support file output, got %q", output)
	}
}

func TestInstallCreatesClaudeSkillAndPersistsConfig(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"claude"},
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
	if !strings.Contains(string(claudeContent), "## Repository Facts") {
		t.Fatalf("expected repository facts section in CLAUDE.md: %s", string(claudeContent))
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
		Targets: []string{"codex"},
		Skills:  []string{"owasp-security-scan"},
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
	if !slices.Equal(resolved.Targets, []string{"codex"}) || len(resolved.Agents) != 0 || !slices.Equal(resolved.Skills, []string{"owasp-security-scan"}) {
		t.Fatalf("unexpected resolved config: %#v", resolved)
	}
}

func TestResolveTargetAndAgentsReturnsSavedMultiTargetConfig(t *testing.T) {
	tmpDir := t.TempDir()

	if err := saveConfig(tmpDir, "go", rulesConfig{
		Targets: []string{"cursor", "claude"},
		Agents:  []string{"linting"},
	}); err != nil {
		t.Fatalf("save multi-target config: %v", err)
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
	if !slices.Equal(resolved.Targets, []string{"cursor", "claude"}) {
		t.Fatalf("expected multi-target config, got %#v", resolved.Targets)
	}
}

func TestResolveTargetAndAgentsRejectsInvalidTargetFlags(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := resolveTargetAndAgents(resolveOptions{
		projectRoot: tmpDir,
		targets:     []string{"cursor", "bogus"},
		agents:      []string{"linting"},
		language:    "go",
	})
	if err == nil {
		t.Fatal("expected invalid target error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --target: bogus. Use: cursor, claude, opencode, codex") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallCreatesCodexSupportFileForSkillOnlyInstall(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"codex"},
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
	skillPath := filepath.Join(tmpDir, ".codex", "skills", "owasp-security-scan", "SKILL.md")
	skillContent, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("expected native codex skill file: %v", err)
	}
	if !strings.HasPrefix(string(skillContent), "---\nname: owasp-security-scan") {
		t.Fatalf("expected preserved skill frontmatter, got %s", string(skillContent))
	}
	if !strings.Contains(string(skillContent), "Created by [Ballast]") {
		t.Fatalf("expected managed marker in native codex skill, got %s", string(skillContent))
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".codex", "skills", "owasp-security-scan", "references", "owasp-mapping.md")); err != nil {
		t.Fatalf("expected copied skill reference: %v", err)
	}
	agentsMD, err := os.ReadFile(filepath.Join(tmpDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	text := string(agentsMD)
	if !strings.Contains(text, "## Repository Facts") {
		t.Fatalf("expected repository facts section in AGENTS.md: %s", text)
	}
	if !strings.Contains(text, "Canonical GitHub repo: `<OWNER/REPO>`") {
		t.Fatalf("expected repository facts scaffold in AGENTS.md: %s", text)
	}
	if !strings.Contains(text, "## Installed skills") {
		t.Fatalf("expected installed skills section in AGENTS.md: %s", text)
	}
	if !strings.Contains(text, "`.codex/skills/owasp-security-scan/SKILL.md`") {
		t.Fatalf("expected codex skill entry in AGENTS.md: %s", text)
	}
}

func TestInstallSkipsExistingOpencodeSkillWithoutForceOrPatch(t *testing.T) {
	tmpDir := t.TempDir()
	skillPath := filepath.Join(tmpDir, ".opencode", "skills", "owasp-security-scan.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("stale skill content"), 0o644); err != nil {
		t.Fatalf("seed skill file: %v", err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"opencode"},
		skills:      []string{"owasp-security-scan"},
		language:    "go",
		force:       false,
		saveConfig:  false,
	})
	if len(result.installedSkills) != 0 {
		t.Fatalf("expected existing skill to be skipped, got %+v", result.installedSkills)
	}
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read existing skill: %v", err)
	}
	text := string(content)
	if text != "stale skill content" {
		t.Fatalf("expected stale skill content to remain unchanged, got %q", text)
	}
}

func TestInstallPatchCreatesMissingSkill(t *testing.T) {
	tmpDir := t.TempDir()

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"opencode"},
		skills:      []string{"owasp-security-scan"},
		language:    "go",
		force:       false,
		patch:       true,
		saveConfig:  false,
	})
	if !slices.Equal(result.installedSkills, []string{"owasp-security-scan"}) {
		t.Fatalf("expected patched install to create skill, got %+v", result.installedSkills)
	}
	content, err := os.ReadFile(filepath.Join(tmpDir, ".opencode", "skills", "owasp-security-scan.md"))
	if err != nil {
		t.Fatalf("read created skill: %v", err)
	}
	if !strings.Contains(string(content), "## Scan Architecture") {
		t.Fatalf("expected canonical skill content, got %q", string(content))
	}
	if !strings.Contains(string(content), "Created by [Ballast]") {
		t.Fatalf("expected managed skill marker, got %q", string(content))
	}
}

func TestInstallPatchMergesExistingSkill(t *testing.T) {
	tmpDir := t.TempDir()
	skillPath := filepath.Join(tmpDir, ".cursor", "rules", "owasp-security-scan.mdc")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte(`---
description: Team customized skill
alwaysApply: true
---

Team intro.

## Usage

Keep team-specific usage notes.
`), 0o644); err != nil {
		t.Fatalf("seed skill file: %v", err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"cursor"},
		skills:      []string{"owasp-security-scan"},
		language:    "go",
		force:       false,
		patch:       true,
		saveConfig:  false,
	})
	if !slices.Equal(result.installedSkills, []string{"owasp-security-scan"}) {
		t.Fatalf("expected patched skill install, got %+v", result.installedSkills)
	}
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read merged skill: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "description: Team customized skill") {
		t.Fatalf("expected custom frontmatter to remain: %s", text)
	}
	if !strings.Contains(text, "alwaysApply: true") {
		t.Fatalf("expected custom alwaysApply to remain: %s", text)
	}
	if !strings.Contains(text, "Keep team-specific usage notes.") {
		t.Fatalf("expected custom section to remain: %s", text)
	}
	if !strings.Contains(text, "## Scan Architecture") {
		t.Fatalf("expected canonical skill content to be merged: %s", text)
	}
}

func TestInstallPatchMergesClaudeSkillArchive(t *testing.T) {
	tmpDir := t.TempDir()
	skillPath := filepath.Join(tmpDir, ".claude", "skills", "owasp-security-scan.skill")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	existingSkillContent := "# owasp-security-scan\n\nTeam intro preserved by patch.\n\n## Team Custom Section\n\nKeep this team-specific section.\n"
	initialArchive, err := buildClaudeSkill("owasp-security-scan", "go", existingSkillContent)
	if err != nil {
		t.Fatalf("build initial skill archive: %v", err)
	}
	if err := os.WriteFile(skillPath, initialArchive, 0o644); err != nil {
		t.Fatalf("seed skill archive: %v", err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"claude"},
		skills:      []string{"owasp-security-scan"},
		language:    "go",
		force:       false,
		patch:       true,
		saveConfig:  false,
	})
	if !slices.Equal(result.installedSkills, []string{"owasp-security-scan"}) {
		t.Fatalf("expected patched skill install, got %+v", result.installedSkills)
	}
	skillMd, err := readClaudeSkillContent(skillPath)
	if err != nil {
		t.Fatalf("read merged skill archive: %v", err)
	}
	if !strings.Contains(skillMd, "Team intro preserved by patch.") {
		t.Fatalf("expected user intro to remain: %s", skillMd)
	}
	if !strings.Contains(skillMd, "Team Custom Section") {
		t.Fatalf("expected custom section to remain: %s", skillMd)
	}
	if !strings.Contains(skillMd, "## Scan Architecture") {
		t.Fatalf("expected canonical skill content to be merged: %s", skillMd)
	}
}

func TestInstallPatchOverwritesUnreadableClaudeSkillArchive(t *testing.T) {
	tmpDir := t.TempDir()
	skillPath := filepath.Join(tmpDir, ".claude", "skills", "owasp-security-scan.skill")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("not-a-zip-archive"), 0o644); err != nil {
		t.Fatalf("seed unreadable skill archive: %v", err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"claude"},
		skills:      []string{"owasp-security-scan"},
		language:    "go",
		force:       false,
		patch:       true,
		saveConfig:  false,
	})
	if !slices.Equal(result.installedSkills, []string{"owasp-security-scan"}) {
		t.Fatalf("expected patched skill install, got %+v", result.installedSkills)
	}
	skillMd, err := readClaudeSkillContent(skillPath)
	if err != nil {
		t.Fatalf("read overwritten skill archive: %v", err)
	}
	if strings.Contains(skillMd, "not-a-zip-archive") {
		t.Fatalf("expected unreadable archive to be replaced with canonical content: %s", skillMd)
	}
	if !strings.Contains(skillMd, "## Scan Architecture") {
		t.Fatalf("expected canonical skill content after overwrite fallback: %s", skillMd)
	}
}

func TestInstallRefreshOverwritesExistingCodexSkill(t *testing.T) {
	tmpDir := t.TempDir()
	legacySkillPath := filepath.Join(tmpDir, ".codex", "rules", "owasp-security-scan.md")
	if err := os.MkdirAll(filepath.Dir(legacySkillPath), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(legacySkillPath, []byte("<!-- Created by [Ballast](https://github.com/everydaydevopsio/ballast). Do not edit this section. -->\n\nstale skill content\n"), 0o644); err != nil {
		t.Fatalf("seed stale skill: %v", err)
	}

	t.Setenv("BALLAST_REFRESH_SKILLS", "1")

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"codex"},
		skills:      []string{"owasp-security-scan"},
		language:    "go",
		force:       false,
		patch:       false,
		saveConfig:  false,
	})
	if !slices.Equal(result.installedSkills, []string{"owasp-security-scan"}) {
		t.Fatalf("expected refreshed skill install, got %+v", result.installedSkills)
	}
	if _, err := os.Stat(legacySkillPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy codex skill to be removed, stat err=%v", err)
	}
	skillPath := filepath.Join(tmpDir, ".codex", "skills", "owasp-security-scan", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read refreshed skill: %v", err)
	}
	text := string(content)
	if strings.Contains(text, "stale skill content") {
		t.Fatalf("expected stale skill content to be replaced: %s", text)
	}
	if !strings.Contains(text, "# OWASP Security Scan Skill") {
		t.Fatalf("expected canonical skill content after refresh: %s", text)
	}
}

func TestInstallForceOverwritesExistingClaudeSkillArchive(t *testing.T) {
	tmpDir := t.TempDir()
	skillPath := filepath.Join(tmpDir, ".claude", "skills", "owasp-security-scan.skill")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	existingSkillContent := "# owasp-security-scan\n\nTeam-only intro that should be removed on force.\n\n## Team Custom Section\n\nThis section should not survive a force overwrite.\n"
	initialArchive, err := buildClaudeSkill("owasp-security-scan", "go", existingSkillContent)
	if err != nil {
		t.Fatalf("build initial skill archive: %v", err)
	}
	if err := os.WriteFile(skillPath, initialArchive, 0o644); err != nil {
		t.Fatalf("seed skill archive: %v", err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"claude"},
		skills:      []string{"owasp-security-scan"},
		language:    "go",
		force:       true,
		patch:       false,
		saveConfig:  false,
	})
	if !slices.Equal(result.installedSkills, []string{"owasp-security-scan"}) {
		t.Fatalf("expected force install, got %+v", result.installedSkills)
	}
	skillMd, err := readClaudeSkillContent(skillPath)
	if err != nil {
		t.Fatalf("read overwritten skill archive: %v", err)
	}
	if strings.Contains(skillMd, "Team-only intro") {
		t.Fatalf("expected team intro to be removed by force overwrite: %s", skillMd)
	}
	if strings.Contains(skillMd, "Team Custom Section") {
		t.Fatalf("expected custom section to be removed by force overwrite: %s", skillMd)
	}
	if !strings.Contains(skillMd, "## Scan Architecture") {
		t.Fatalf("expected canonical skill content after force overwrite: %s", skillMd)
	}
}

func TestInstallForceOverwritesExistingSkill(t *testing.T) {
	tmpDir := t.TempDir()
	skillPath := filepath.Join(tmpDir, ".opencode", "skills", "owasp-security-scan.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("stale skill content"), 0o644); err != nil {
		t.Fatalf("seed skill file: %v", err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"opencode"},
		skills:      []string{"owasp-security-scan"},
		language:    "go",
		force:       true,
		saveConfig:  false,
	})
	if !slices.Equal(result.installedSkills, []string{"owasp-security-scan"}) {
		t.Fatalf("expected force install to overwrite skill, got %+v", result.installedSkills)
	}
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read overwritten skill: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "## Scan Architecture") {
		t.Fatalf("expected canonical skill content, got %q", text)
	}
	if text == "stale skill content" {
		t.Fatalf("expected stale skill content to be overwritten")
	}
}

func TestValidatedRuleSubdirRejectsInvalidValues(t *testing.T) {
	t.Setenv("BALLAST_RULE_SUBDIR", "../escape")
	_, err := validatedRuleSubdir()
	if err == nil {
		t.Fatal("expected validatedRuleSubdir to reject invalid BALLAST_RULE_SUBDIR")
	}
}

func TestRunInstallWritesSharedRulesrcForMultipleTargets(t *testing.T) {
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

	exitCode := runInstall([]string{"install", "--target", "cursor,claude", "--all", "--yes"})
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
	if !strings.Contains(string(content), `"targets": [`+"\n"+`    "cursor",`) || !strings.Contains(string(content), `"claude"`) {
		t.Fatalf("expected target list in shared config: %s", string(content))
	}
	if !strings.Contains(string(content), `"languages": [`+"\n"+`    "go"`) {
		t.Fatalf("expected go language in shared config: %s", string(content))
	}
	if !strings.Contains(string(content), `"paths": {`) {
		t.Fatalf("expected paths in shared config: %s", string(content))
	}
}

func TestRunInstallWritesDeploymentModelForPublishing(t *testing.T) {
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

	exitCode := runInstall([]string{"install", "--target", "codex", "--agent", "publishing", "--deployment-model", "hosted", "--yes"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	content, err := os.ReadFile(filepath.Join(tmpDir, ".rulesrc.json"))
	if err != nil {
		t.Fatalf("read .rulesrc.json: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, `"deploymentModel": "hosted"`) {
		t.Fatalf("expected deploymentModel in config: %s", text)
	}

	ruleContent, err := os.ReadFile(filepath.Join(tmpDir, ".codex", "rules", "publishing-apps.md"))
	if err != nil {
		t.Fatalf("read publishing-apps.md: %v", err)
	}
	if !strings.Contains(string(ruleContent), "Hosted platform deployment model") {
		t.Fatalf("expected hosted guidance in publishing rule: %s", string(ruleContent))
	}
}

func TestRunInstallPromptsForTaskSystemAndDeploymentModelOnFirstRun(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")
	t.Setenv("TF_BUILD", "")
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

	originalStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = originalStdin
		_ = reader.Close()
	})
	if _, err := writer.WriteString("linear\nserverless\n"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	exitCode := runInstall([]string{"install", "--target", "codex", "--language", "go", "--all"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	content, err := os.ReadFile(filepath.Join(tmpDir, ".rulesrc.json"))
	if err != nil {
		t.Fatalf("read .rulesrc.json: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, `"taskSystem": "linear"`) {
		t.Fatalf("expected prompted taskSystem in config: %s", text)
	}
	if !strings.Contains(text, `"deploymentModel": "serverless"`) {
		t.Fatalf("expected prompted deploymentModel in config: %s", text)
	}
	taskRule, err := os.ReadFile(filepath.Join(tmpDir, ".codex", "rules", "tasks-task-system.md"))
	if err != nil {
		t.Fatalf("read tasks-task-system.md: %v", err)
	}
	if !strings.Contains(string(taskRule), "linear") || strings.Contains(string(taskRule), "{{taskSystem}}") {
		t.Fatalf("expected task rule to render prompted task system: %s", string(taskRule))
	}
}

func TestRunInstallDefaultsRequiredOptionsInNonInteractiveMode(t *testing.T) {
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

	exitCode := runInstall([]string{"install", "--target", "codex", "--language", "go", "--all", "--yes"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	content, err := os.ReadFile(filepath.Join(tmpDir, ".rulesrc.json"))
	if err != nil {
		t.Fatalf("read .rulesrc.json: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, `"taskSystem": "github"`) {
		t.Fatalf("expected default taskSystem in config: %s", text)
	}
	if !strings.Contains(text, `"deploymentModel": "none"`) {
		t.Fatalf("expected default deploymentModel in config: %s", text)
	}
}

func TestRunInstallRejectsInvalidDeploymentModel(t *testing.T) {
	output := captureStdout(t, func() {
		exitCode := runInstall([]string{"install", "--target", "codex", "--agent", "publishing", "--deployment-model", "bogus", "--yes"})
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	})
	if !strings.Contains(output, "Invalid --deployment-model") {
		t.Fatalf("expected invalid deployment model message, got %q", output)
	}
}

func TestRunInstallRejectsInvalidTaskSystem(t *testing.T) {
	output := captureStdout(t, func() {
		exitCode := runInstall([]string{"install", "--target", "codex", "--agent", "tasks", "--task-system", "notion", "--yes"})
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	})
	if !strings.Contains(output, "Invalid --task-system") {
		t.Fatalf("expected invalid task system message, got %q", output)
	}
}

func TestRunInstallForceSupportFileDeclinedSkipsFile(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")
	t.Setenv("TF_BUILD", "")
	originalInteractive := isStdinInteractiveFunc
	isStdinInteractiveFunc = func() bool { return true }
	t.Cleanup(func() {
		isStdinInteractiveFunc = originalInteractive
	})
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
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# AGENTS.md\n\nTeam customizations.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	originalStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = originalStdin
		_ = reader.Close()
	})
	if _, err := writer.WriteString("n\n"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		exitCode := runInstall([]string{"install", "--target", "codex", "--skill", "owasp-security-scan", "--force"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	content, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Team customizations.") {
		t.Fatalf("expected AGENTS.md to remain unchanged: %s", string(content))
	}
	if !strings.Contains(output, "Skipped support file: AGENTS.md") {
		t.Fatalf("expected skip notice in output, got %q", output)
	}
}

func TestRunInstallForceSupportFileAcceptedOverwritesFile(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")
	t.Setenv("TF_BUILD", "")
	originalInteractive := isStdinInteractiveFunc
	isStdinInteractiveFunc = func() bool { return true }
	t.Cleanup(func() {
		isStdinInteractiveFunc = originalInteractive
	})
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
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# AGENTS.md\n\nTeam customizations.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	originalStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = originalStdin
		_ = reader.Close()
	})
	if _, err := writer.WriteString("y\n"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	exitCode := runInstall([]string{"install", "--target", "codex", "--skill", "owasp-security-scan", "--force"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	content, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "## Installed skills") {
		t.Fatalf("expected canonical AGENTS.md content, got %s", text)
	}
	if strings.Contains(text, "Team customizations.") {
		t.Fatalf("expected AGENTS.md to be overwritten, got %s", text)
	}
}

func TestRunInstallForceSupportFileNonInteractiveAborts(t *testing.T) {
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
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# AGENTS.md\n\nTeam customizations.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		exitCode := runInstall([]string{"install", "--target", "codex", "--skill", "owasp-security-scan", "--force", "--yes"})
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	})

	content, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Team customizations.") {
		t.Fatalf("expected AGENTS.md to remain unchanged: %s", string(content))
	}
	if !strings.Contains(output, "Cannot overwrite existing support file AGENTS.md in non-interactive mode") {
		t.Fatalf("expected non-interactive error, got %q", output)
	}
}

func TestRunInstallForceSupportFileNonTTYAborts(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")
	t.Setenv("TF_BUILD", "")
	originalInteractive := isStdinInteractiveFunc
	isStdinInteractiveFunc = func() bool { return false }
	t.Cleanup(func() {
		isStdinInteractiveFunc = originalInteractive
	})
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
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# AGENTS.md\n\nTeam customizations.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		exitCode := runInstall([]string{"install", "--target", "codex", "--skill", "owasp-security-scan", "--force"})
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	})

	content, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Team customizations.") {
		t.Fatalf("expected AGENTS.md to remain unchanged: %s", string(content))
	}
	if !strings.Contains(output, "Cannot overwrite existing support file AGENTS.md in non-interactive mode") {
		t.Fatalf("expected non-interactive error, got %q", output)
	}
}

func TestIsStdinInteractive(t *testing.T) {
	originalStdin := os.Stdin
	t.Cleanup(func() {
		os.Stdin = originalStdin
	})

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = reader
	if isStdinInteractive() {
		t.Fatal("expected pipe stdin to be non-interactive")
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	os.Stdin = devNull
	if isStdinInteractive() {
		t.Fatal("expected non-TTY character device stdin to be non-interactive")
	}
}

func TestSaveConfigAccumulatesLanguagesInSharedRulesrc(t *testing.T) {
	tmpDir := t.TempDir()

	if err := saveConfig(tmpDir, "typescript", rulesConfig{
		Targets:   []string{"claude"},
		Agents:    []string{"linting"},
		Languages: []string{"typescript"},
	}); err != nil {
		t.Fatalf("save typescript config: %v", err)
	}
	if err := saveConfig(tmpDir, "go", rulesConfig{
		Targets:   []string{"claude"},
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

func TestSaveConfigNormalizesTargetsAndOmitsLegacyField(t *testing.T) {
	tmpDir := t.TempDir()

	if err := saveConfig(tmpDir, "go", rulesConfig{
		Targets: []string{"cursor", "claude", "cursor"},
		Agents:  []string{"linting"},
	}); err != nil {
		t.Fatalf("save multi-target config: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, ".rulesrc.json"))
	if err != nil {
		t.Fatalf("read .rulesrc.json: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, `"targets": [`+"\n"+`    "cursor",`) || !strings.Contains(text, `"claude"`) {
		t.Fatalf("expected normalized targets array in shared config: %s", text)
	}
	if strings.Contains(text, `"target":`) {
		t.Fatalf("expected legacy target field to be omitted: %s", text)
	}
}

func TestNormalizeTargetsDetailedReturnsInvalidTokens(t *testing.T) {
	normalized, invalid := normalizeTargetsDetailed([]string{"cursor,claude", "bogus", "codex"})
	if !slices.Equal(normalized, []string{"cursor", "claude", "codex"}) {
		t.Fatalf("unexpected normalized targets: %#v", normalized)
	}
	if !slices.Equal(invalid, []string{"bogus"}) {
		t.Fatalf("unexpected invalid targets: %#v", invalid)
	}
}

func TestLoadConfigSupportsLegacyTargetField(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, ".rulesrc.json"), []byte(`{"target":"cursor","agents":["linting"],"taskSystem":"jira","deploymentModel":"SERVERLESS","publishingProfiles":["APP"," library ","sdk","cli","cli","","unknown"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := loadConfig(tmpDir, "go")
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if !slices.Equal(cfg.Targets, []string{"cursor"}) {
		t.Fatalf("expected legacy target to normalize into targets, got %#v", cfg.Targets)
	}
	if cfg.TaskSystem != "jira" {
		t.Fatalf("expected taskSystem to be loaded from config, got %#v", cfg)
	}
	if cfg.DeploymentModel != "serverless" {
		t.Fatalf("expected deploymentModel to be normalized from config, got %#v", cfg)
	}
	if !slices.Equal(cfg.PublishingProfiles, []string{"apps", "libraries", "sdks", "cli"}) {
		t.Fatalf("expected publishingProfiles to be normalized from config, got %#v", cfg.PublishingProfiles)
	}
}

func TestLoadConfigNormalizesDiscoveryExcludePaths(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, ".rulesrc.json"), []byte(`{"targets":["codex"],"agents":["linting"],"discovery":{"excludePaths":["examples","tmp","examples",""]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := loadConfig(tmpDir, "go")
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.Discovery == nil {
		t.Fatal("expected discovery config")
	}
	if !slices.Equal(cfg.Discovery.ExcludePaths, []string{"examples", "tmp"}) {
		t.Fatalf("expected normalized exclude paths, got %#v", cfg.Discovery.ExcludePaths)
	}
}

func TestSaveConfigDefaultsAndPreservesLanguageTools(t *testing.T) {
	tmpDir := t.TempDir()
	if err := saveConfig(tmpDir, "python", rulesConfig{
		Targets:   []string{"codex"},
		Agents:    []string{"local-dev"},
		Languages: []string{"python"},
		Tools: map[string][]string{
			"python": {"poetry", "pyenv", "poetry"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := saveConfig(tmpDir, "typescript", rulesConfig{
		Targets:   []string{"codex"},
		Agents:    []string{"local-dev"},
		Languages: []string{"typescript"},
	}); err != nil {
		t.Fatal(err)
	}

	cfg := loadConfig(tmpDir, "go")
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if !slices.Equal(cfg.Tools["python"], []string{"poetry", "pyenv"}) {
		t.Fatalf("expected python tool override to be preserved, got %#v", cfg.Tools)
	}
	if !slices.Equal(cfg.Tools["typescript"], []string{"pnpm", "corepack"}) {
		t.Fatalf("expected typescript default tools, got %#v", cfg.Tools)
	}
}

func TestSaveConfigPreservesDiscoveryExcludePaths(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, ".rulesrc.json"), []byte(`{"targets":["codex"],"agents":["linting"],"discovery":{"excludePaths":["examples"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := saveConfig(tmpDir, "go", rulesConfig{
		Targets:   []string{"codex"},
		Agents:    []string{"linting"},
		Languages: []string{"go"},
	}); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, ".rulesrc.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Discovery discoveryConfig `json:"discovery"`
	}
	if err := json.Unmarshal(content, &saved); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(saved.Discovery.ExcludePaths, []string{"examples"}) {
		t.Fatalf("expected discovery exclude paths to be preserved, got %#v", saved.Discovery.ExcludePaths)
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
		targets:     []string{"claude"},
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

func TestInstallUpdatesCodexAgentsMDSectionByDefault(t *testing.T) {
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
	if err := os.WriteFile(agentsMD, []byte("# AGENTS.md\n\n## Team Notes\n\nKeep this section.\n\n## Installed agent rules\n\nCreated by [Ballast](https://github.com/everydaydevopsio/ballast) v9.9.9-test. Do not edit this section.\n\nRead and follow these rule files in `.codex/rules/` when they apply:\n\n- `.codex/rules/old.md` - Old rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"codex"},
		agents:      []string{"linting"},
		language:    "go",
		force:       false,
		patch:       false,
		saveConfig:  false,
	})
	if len(result.errors) > 0 {
		t.Fatalf("unexpected install errors: %+v", result.errors)
	}
	if contains(result.skippedSupportFiles, agentsMD) {
		t.Fatalf("expected AGENTS.md to be patched by default, got skipped support files: %#v", result.skippedSupportFiles)
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

func TestInstallDefaultPatchPreservesUnmanagedCodexSupportSection(t *testing.T) {
	tmpDir := t.TempDir()
	agentsMD := filepath.Join(tmpDir, "AGENTS.md")
	if err := os.WriteFile(agentsMD, []byte("# AGENTS.md\n\n## Installed agent rules\n\n- `.codex/rules/old.md` - Team managed rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"codex"},
		agents:      []string{"linting"},
		language:    "go",
		force:       false,
		patch:       false,
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
	if !strings.Contains(text, "`.codex/rules/old.md`") {
		t.Fatalf("expected unmanaged section to be preserved: %s", text)
	}
	if !strings.Contains(text, "`.codex/rules/go-linting.md`") {
		t.Fatalf("expected linting rule to be installed: %s", text)
	}
}

func TestInstallSkipsSupportFileWriteWhenExistingFileCannotBeRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("write-only chmod semantics differ on Windows")
	}

	tests := []struct {
		name        string
		target      string
		path        string
		expectedErr string
	}{
		{name: "codex", target: "codex", path: "AGENTS.md", expectedErr: "codex"},
		{name: "claude", target: "claude", path: "CLAUDE.md", expectedErr: "claude"},
		{name: "gemini", target: "gemini", path: "GEMINI.md", expectedErr: "gemini"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			supportPath := filepath.Join(tmpDir, test.path)
			original := "# " + test.path + "\n\n## Installed agent rules\n\nDo not replace this.\n"
			if err := os.WriteFile(supportPath, []byte(original), 0o200); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = os.Chmod(supportPath, 0o600)
			})

			result := install(installOptions{
				projectRoot: tmpDir,
				targets:     []string{test.target},
				agents:      []string{"linting"},
				language:    "go",
				force:       false,
				patch:       false,
				saveConfig:  false,
			})

			if len(result.errors) == 0 {
				t.Fatalf("expected read error for %s", test.path)
			}
			if !strings.Contains(result.errors[0].agent, test.expectedErr) {
				t.Fatalf("expected %s read error, got %+v", test.expectedErr, result.errors)
			}
			if contains(result.installedSupport, supportPath) {
				t.Fatalf("expected unreadable support file not to be reported as installed")
			}
			if err := os.Chmod(supportPath, 0o600); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(supportPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != original {
				t.Fatalf("expected original support file to be preserved, got %q", string(content))
			}
		})
	}
}

func TestSkillOnlyPatchKeepsCodexRuleReferencesFromRulesrc(t *testing.T) {
	tmpDir := t.TempDir()
	if err := saveConfig(tmpDir, "go", rulesConfig{
		Targets: []string{"codex"},
		Agents:  []string{"linting"},
		Skills:  []string{"owasp-security-scan"},
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	agentsMD := filepath.Join(tmpDir, "AGENTS.md")
	initial, err := buildCodexAgentsMD([]string{"linting"}, []string{"owasp-security-scan"}, "go", nil, nil)
	if err != nil {
		t.Fatalf("build AGENTS.md: %v", err)
	}
	if err := os.WriteFile(agentsMD, []byte(initial), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}

	result := install(installOptions{
		projectRoot: tmpDir,
		targets:     []string{"codex"},
		agents:      []string{},
		skills:      []string{"owasp-security-scan", "github-health-check"},
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
	if !strings.Contains(text, "`.codex/rules/go-linting.md`") {
		t.Fatalf("expected config-backed agent entry to remain: %s", text)
	}
	if !strings.Contains(text, "`.codex/skills/owasp-security-scan/SKILL.md`") {
		t.Fatalf("expected saved skill entry to remain: %s", text)
	}
	if strings.Contains(text, "`.codex/skills/github-health-check/SKILL.md`") {
		t.Fatalf("expected unsaved skill entry to stay absent: %s", text)
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
		targets:     []string{"claude"},
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

func TestPatchCodexAgentsMDPreservesUnmanagedSectionsInManagedOnlyMode(t *testing.T) {
	existing := "# AGENTS.md\n\n## Installed agent rules\n\n- `.codex/rules/old.md` - Team managed rule\n"
	canonical := "# AGENTS.md\n\n## Installed agent rules\n\nCreated by [Ballast](https://github.com/everydaydevopsio/ballast) v9.9.9-test. Do not edit this section.\n\n- `.codex/rules/go-linting.md` - New rule\n"

	merged := patchCodexAgentsMDWithOptions(existing, canonical, false)
	if !strings.Contains(merged, "`.codex/rules/old.md`") {
		t.Fatalf("expected unmanaged section to be preserved: %s", merged)
	}
	if !strings.Contains(merged, "`.codex/rules/go-linting.md`") {
		t.Fatalf("expected managed section to be appended: %s", merged)
	}
}

func TestPatchCodexAgentsMDReplacesLegacyManagedNoticeInManagedOnlyMode(t *testing.T) {
	existing := "# AGENTS.md\n\n## Installed agent rules\n\nCreated by Ballast. Do not edit this section.\n\n- `.codex/rules/old.md` - Old rule\n"
	canonical := "# AGENTS.md\n\n## Installed agent rules\n\nCreated by [Ballast](https://github.com/everydaydevopsio/ballast) v9.9.9-test. Do not edit this section.\n\n- `.codex/rules/go-linting.md` - New rule\n"

	merged := patchCodexAgentsMDWithOptions(existing, canonical, false)
	if strings.Contains(merged, "`.codex/rules/old.md`") {
		t.Fatalf("expected legacy managed section to be replaced: %s", merged)
	}
	if !strings.Contains(merged, "`.codex/rules/go-linting.md`") {
		t.Fatalf("expected managed section to contain canonical rule: %s", merged)
	}
}

func TestPatchCodexAgentsMDUsesSingleFinalNewline(t *testing.T) {
	existing := "# CLAUDE.md\n\n## Installed skills\n\nCreated by Ballast. Do not edit this section.\n\n- old\n"
	canonical := "# CLAUDE.md\n\n## Installed skills\n\nCreated by Ballast. Do not edit this section.\n\n- new\n"

	merged := patchCodexAgentsMDWithOptions(existing, canonical, false)
	if !strings.HasSuffix(merged, "\n") {
		t.Fatalf("expected merged content to end with a newline, got %q", merged)
	}
	if strings.HasSuffix(merged, "\n\n") {
		t.Fatalf("expected merged content not to end with a blank line, got %q", merged)
	}
	if !strings.Contains(merged, "- new\n") {
		t.Fatalf("expected canonical skill section, got %q", merged)
	}
}

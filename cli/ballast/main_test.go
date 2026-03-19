package main

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestRunVersionSkipsLanguageDetection(t *testing.T) {
	t.Run("plain version flag", func(t *testing.T) {
		output := captureStdout(t, func() {
			exitCode := run([]string{"--version"})
			if exitCode != 0 {
				t.Fatalf("expected exit code 0, got %d", exitCode)
			}
		})

		if got := strings.TrimSpace(output); got != version {
			t.Fatalf("expected version output %q, got %q", version, got)
		}
	})

	t.Run("version with explicit language", func(t *testing.T) {
		output := captureStdout(t, func() {
			exitCode := run([]string{"--language", "go", "--version"})
			if exitCode != 0 {
				t.Fatalf("expected exit code 0, got %d", exitCode)
			}
		})

		if got := strings.TrimSpace(output); got != version {
			t.Fatalf("expected version output %q, got %q", version, got)
		}
	})
}

func TestRunWithoutArgsPrintsUsage(t *testing.T) {
	output := captureStdout(t, func() {
		exitCode := run(nil)
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if !strings.Contains(output, "Usage:\n  ballast [flags] <command> [command flags]") {
		t.Fatalf("expected usage output, got %q", output)
	}
	if !strings.Contains(output, "Commands:") {
		t.Fatalf("expected commands section, got %q", output)
	}
	if !strings.Contains(output, "Flags:") {
		t.Fatalf("expected flags section, got %q", output)
	}
	if strings.Contains(output, "Could not detect repository language") {
		t.Fatalf("expected no language detection error, got %q", output)
	}
}

func TestRunHelpAndVersionCommands(t *testing.T) {
	t.Run("help command", func(t *testing.T) {
		output := captureStdout(t, func() {
			exitCode := run([]string{"help"})
			if exitCode != 0 {
				t.Fatalf("expected exit code 0, got %d", exitCode)
			}
		})

		if !strings.Contains(output, "Commands:") {
			t.Fatalf("expected help output, got %q", output)
		}
	})

	t.Run("version command", func(t *testing.T) {
		output := captureStdout(t, func() {
			exitCode := run([]string{"version"})
			if exitCode != 0 {
				t.Fatalf("expected exit code 0, got %d", exitCode)
			}
		})

		if got := strings.TrimSpace(output); got != version {
			t.Fatalf("expected version output %q, got %q", version, got)
		}
	})
}

func TestDetectRepoProfilesFindsMultiLanguageMonorepo(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "frontend", "tsconfig.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "services", "api", "pyproject.toml"), "[project]\nname='api'\n")
	mustWriteFile(t, filepath.Join(root, "tools", "worker", "go.mod"), "module example.com/worker\n\ngo 1.24\n")

	profiles, err := detectRepoProfiles(root)
	if err != nil {
		t.Fatalf("detectRepoProfiles returned error: %v", err)
	}

	if len(profiles) != 3 {
		t.Fatalf("expected 3 profiles, got %d: %#v", len(profiles), profiles)
	}

	want := []repoProfile{
		{Language: langTypeScript, Paths: []string{filepath.Join(root, "apps", "frontend")}},
		{Language: langPython, Paths: []string{filepath.Join(root, "services", "api")}},
		{Language: langGo, Paths: []string{filepath.Join(root, "tools", "worker")}},
	}
	if !reflect.DeepEqual(profiles, want) {
		t.Fatalf("expected profiles %#v, got %#v", want, profiles)
	}
}

func TestResolveMonorepoPlanUsesConfigAndSplitsCommonFromLanguageAgents(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "target": "cursor",
  "agents": ["local-dev", "linting", "testing"],
  "languages": ["typescript", "python", "go"],
  "paths": {
    "typescript": ["apps/frontend"],
    "python": ["services/api"],
    "go": ["tools/worker"]
  }
}`)

	plan, err := resolveMonorepoPlan(root, []string{"install"})
	if err != nil {
		t.Fatalf("resolveMonorepoPlan returned error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected monorepo plan, got nil")
	}

	if len(plan.Invocations) != 4 {
		t.Fatalf("expected 4 backend invocations, got %d: %#v", len(plan.Invocations), plan.Invocations)
	}

	if plan.Invocations[0].Language != langTypeScript || plan.Invocations[0].Dir != root {
		t.Fatalf("expected common install at repo root via TypeScript backend, got %#v", plan.Invocations[0])
	}
	if got := strings.Join(plan.Invocations[0].Args, " "); !strings.Contains(got, "--agent local-dev") {
		t.Fatalf("expected common agent selection, got %q", got)
	}

	wantDirs := []string{
		root,
		root,
		root,
	}
	for i, wantDir := range wantDirs {
		if plan.Invocations[i+1].Dir != wantDir {
			t.Fatalf("expected plan[%d] dir %q, got %#v", i+1, wantDir, plan.Invocations[i+1])
		}
		if got := strings.Join(plan.Invocations[i+1].Args, " "); strings.Contains(got, "local-dev") {
			t.Fatalf("expected language-only agent selection for %q, got %q", wantDir, got)
		}
		if got := strings.Join(plan.Invocations[i+1].Args, " "); !strings.Contains(got, "--agent linting,testing") {
			t.Fatalf("expected language agent selection for %q, got %q", wantDir, got)
		}
		if plan.Invocations[i+1].Env["BALLAST_RULE_SUBDIR"] == "" {
			t.Fatalf("expected monorepo rule subdir env for invocation %#v", plan.Invocations[i+1])
		}
	}
}

func TestRunMonorepoInstallExecutesEachBackendAtRepoRoot(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "apps", "frontend", "tsconfig.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "services", "api", "pyproject.toml"), "[project]\nname='api'\n")
	mustWriteFile(t, filepath.Join(root, "tools", "worker", "go.mod"), "module example.com/worker\n\ngo 1.24\n")

	originalEnsure := ensureInstalledFunc
	originalExec := execToolFunc
	t.Cleanup(func() {
		ensureInstalledFunc = originalEnsure
		execToolFunc = originalExec
	})

	ensureInstalledFunc = func(tool toolConfig) error { return nil }
	var invocations []backendInvocation
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		invocations = append(invocations, backendInvocation{
			Binary: binary,
			Args:   append([]string(nil), args...),
			Dir:    dir,
			Env:    env,
		})
		return 0, nil
	}

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"install", "--target", "cursor", "--all", "--yes"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(invocations) != 4 {
		t.Fatalf("expected 4 invocations, got %d: %#v", len(invocations), invocations)
	}

	if invocations[0].Dir != root {
		t.Fatalf("expected root common install, got %#v", invocations[0])
	}
	if got := strings.Join(invocations[0].Args, " "); !strings.Contains(got, "--agent local-dev,cicd,observability") {
		t.Fatalf("expected common agents in first invocation, got %q", got)
	}

	for _, invocation := range invocations[1:] {
		if invocation.Dir != root {
			t.Fatalf("unexpected invocation dir: %#v", invocation)
		}
		got := strings.Join(invocation.Args, " ")
		if !strings.Contains(got, "--agent linting,logging,testing") {
			t.Fatalf("expected language agents for scoped install, got %q", got)
		}
		if invocation.Env["BALLAST_DISABLE_SUPPORT_FILES"] != "1" {
			t.Fatalf("expected support files to be disabled for monorepo backend invocation: %#v", invocation)
		}
		if invocation.Env["BALLAST_RULE_SUBDIR"] == "" {
			t.Fatalf("expected language subdir env on monorepo invocation: %#v", invocation)
		}
	}

	config, err := os.ReadFile(filepath.Join(root, ".rulesrc.json"))
	if err != nil {
		t.Fatalf("read saved monorepo config: %v", err)
	}
	configText := string(config)
	if strings.Contains(configText, root) {
		t.Fatalf("expected saved monorepo config to use relative paths, got %q", configText)
	}
	if !strings.Contains(configText, `"apps/frontend"`) {
		t.Fatalf("expected saved monorepo config to include relative TypeScript path, got %q", configText)
	}
}

func TestResolveMonorepoPlanRejectsEscapingPathsFromConfig(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "target": "cursor",
  "agents": ["linting"],
  "languages": ["python", "go"],
  "paths": {
    "python": ["../escape"],
    "go": ["tools/worker"]
  }
}`)

	plan, err := resolveMonorepoPlan(root, []string{"install"})
	if err == nil {
		t.Fatalf("expected path validation error, got plan %#v", plan)
	}
	if !strings.Contains(err.Error(), "escapes repository root") {
		t.Fatalf("expected escape error, got %v", err)
	}
}

func TestResolveMonorepoPlanRejectsUnsupportedAgents(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "target": "cursor",
  "agents": ["not-an-agent"],
  "languages": ["typescript", "python"],
  "paths": {
    "typescript": ["apps/frontend"],
    "python": ["services/api"]
  }
}`)

	plan, err := resolveMonorepoPlan(root, []string{"install"})
	if err == nil {
		t.Fatalf("expected unsupported agent error, got plan %#v", plan)
	}
	if !strings.Contains(err.Error(), "unsupported agent selection") {
		t.Fatalf("expected unsupported agent error, got %v", err)
	}
}

func TestRunMonorepoInstallDoesNotPersistConfigOnFailure(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "apps", "frontend", "tsconfig.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "services", "api", "pyproject.toml"), "[project]\nname='api'\n")

	originalEnsure := ensureInstalledFunc
	originalExec := execToolFunc
	t.Cleanup(func() {
		ensureInstalledFunc = originalEnsure
		execToolFunc = originalExec
	})

	ensureInstalledFunc = func(tool toolConfig) error { return nil }
	callCount := 0
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		callCount++
		if callCount == 2 {
			return 1, nil
		}
		return 0, nil
	}

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"install", "--target", "cursor", "--all", "--yes"})
		if exitCode == 0 {
			t.Fatal("expected failing monorepo install")
		}
	})

	if _, err := os.Stat(filepath.Join(root, ".rulesrc.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no persisted monorepo config after failure, got err=%v", err)
	}
}

func TestDetectRepoProfilesPropagatesWalkErrors(t *testing.T) {
	originalWalk := walkDirFunc
	t.Cleanup(func() {
		walkDirFunc = originalWalk
	})

	walkDirFunc = func(root string, fn fs.WalkDirFunc) error {
		return errors.New("walk failed")
	}

	profiles, err := detectRepoProfiles(t.TempDir())
	if err == nil {
		t.Fatalf("expected walk error, got profiles %#v", profiles)
	}
	if !strings.Contains(err.Error(), "scan repo for language profiles") {
		t.Fatalf("expected wrapped walk error, got %v", err)
	}
}

func TestUpdateMonorepoSupportFilesCreatesClaudeMdAtRoot(t *testing.T) {
	root := t.TempDir()
	plan := &monorepoPlan{
		Target:   "claude",
		Common:   []string{"local-dev"},
		Language: []string{"linting"},
		Config: monorepoConfig{
			Languages: []string{"typescript", "python", "go"},
		},
	}

	if err := updateMonorepoSupportFiles(root, plan, []string{"install", "--target", "claude", "--all", "--yes"}); err != nil {
		t.Fatalf("updateMonorepoSupportFiles returned error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "`.claude/rules/common/local-dev-env.md`") {
		t.Fatalf("expected common rules entry in CLAUDE.md, got %q", text)
	}
	if !strings.Contains(text, "`.claude/rules/typescript/typescript-linting.md`") {
		t.Fatalf("expected language rules entry in CLAUDE.md, got %q", text)
	}
	if !strings.Contains(text, "`.claude/rules/python/python-linting.md`") {
		t.Fatalf("expected python rules entry in CLAUDE.md, got %q", text)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	fn()
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}

	os.Stdout = writer
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	var buf bytes.Buffer
	var copyErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, copyErr = io.Copy(&buf, reader)
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	wg.Wait()
	if copyErr != nil {
		t.Fatalf("read stdout: %v", copyErr)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}

	return buf.String()
}

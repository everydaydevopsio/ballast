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
		if !strings.Contains(output, "doctor") {
			t.Fatalf("expected doctor in help output, got %q", output)
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

func TestRunDoctorReportsAllBackends(t *testing.T) {
	originalCollect := collectDoctorBackendsFunc
	t.Cleanup(func() {
		collectDoctorBackendsFunc = originalCollect
	})

	collectDoctorBackendsFunc = func(root string) []doctorBackendStatus {
		return []doctorBackendStatus{
			{Name: "ballast-typescript", Version: "5.0.2", Location: "/tmp/ts", Found: true},
			{Name: "ballast-python", Version: "5.0.2", Location: "/tmp/py", Found: true},
			{Name: "ballast-go", Version: "5.0.2", Location: "/tmp/go", Found: true},
		}
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{"ballastVersion":"5.0.2","target":"claude","agents":["local-dev"]}`)

	output := captureStdout(t, func() {
		withWorkingDir(t, root, func() {
			exitCode := run([]string{"doctor"})
			if exitCode != 0 {
				t.Fatalf("expected exit code 0, got %d", exitCode)
			}
		})
	})

	if !strings.Contains(output, "Ballast doctor") {
		t.Fatalf("expected wrapper doctor output, got %q", output)
	}
	for _, name := range []string{"ballast-typescript", "ballast-python", "ballast-go"} {
		if !strings.Contains(output, name) {
			t.Fatalf("expected %s in doctor output, got %q", name, output)
		}
	}
	if !strings.Contains(output, "ballastVersion: 5.0.2") {
		t.Fatalf("expected config version in doctor output, got %q", output)
	}
}

func TestRunDoctorFixPrintsMode(t *testing.T) {
	originalRun := runCommandFunc
	originalVersion := version
	t.Cleanup(func() {
		runCommandFunc = originalRun
		version = originalVersion
	})
	version = "5.0.2"
	runCommandFunc = func(name string, args []string) error { return nil }

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	output := captureStdout(t, func() {
		withWorkingDir(t, root, func() {
			exitCode := run([]string{"doctor", "--fix"})
			if exitCode != 0 {
				t.Fatalf("expected exit code 0, got %d", exitCode)
			}
		})
	})
	if !strings.Contains(output, "Mode: fix") {
		t.Fatalf("expected fix mode in doctor output, got %q", output)
	}
}

func TestRunDoctorFixInstallsBackendsAndRefreshesConfig(t *testing.T) {
	originalRun := runCommandFunc
	originalEnsure := ensureInstalledFunc
	originalExec := execToolFunc
	originalVersion := version
	t.Cleanup(func() {
		runCommandFunc = originalRun
		ensureInstalledFunc = originalEnsure
		execToolFunc = originalExec
		version = originalVersion
	})
	version = "5.0.2"

	var commands [][]string
	runCommandFunc = func(name string, args []string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	}
	ensureInstalledFunc = func(tool toolConfig) error { return nil }
	var invocation backendInvocation
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		invocation = backendInvocation{Binary: binary, Args: append([]string(nil), args...)}
		return 0, nil
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), "[project]\nname='api'\n")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{"target":"claude","agents":["local-dev"]}`)

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"doctor", "--fix"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(commands) != 3 {
		t.Fatalf("expected install commands for all backends, got %#v", commands)
	}
	if got := strings.Join(invocation.Args, " "); !strings.Contains(got, "install --yes") {
		t.Fatalf("expected refresh-config install invocation, got %q", got)
	}
}

func TestRunInstallRefreshConfigUsesSavedConfig(t *testing.T) {
	originalEnsure := ensureInstalledFunc
	originalExec := execToolFunc
	t.Cleanup(func() {
		ensureInstalledFunc = originalEnsure
		execToolFunc = originalExec
	})

	ensureInstalledFunc = func(tool toolConfig) error { return nil }
	var invocation backendInvocation
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		invocation = backendInvocation{Binary: binary, Args: append([]string(nil), args...)}
		return 0, nil
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), "[project]\nname='api'\n")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{"target":"claude","agents":["local-dev"]}`)

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"install", "--refresh-config"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if got := strings.Join(invocation.Args, " "); got != "install --yes" {
		t.Fatalf("expected refresh-config to forward install --yes, got %q", got)
	}
}

func TestRunInstallCLICommand(t *testing.T) {
	originalRun := runCommandFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
	})

	var commands [][]string
	runCommandFunc = func(name string, args []string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), "[project]\nname='api'\n")
	withWorkingDir(t, root, func() {
		exitCode := run([]string{"install-cli", "--language", "python", "--version", "5.0.2"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(commands) != 1 {
		t.Fatalf("expected 1 install command, got %#v", commands)
	}
	if got := strings.Join(commands[0], " "); got != `env UV_TOOL_DIR=`+filepath.Join(root, ".ballast", "tools", "python")+` UV_TOOL_BIN_DIR=`+filepath.Join(root, ".ballast", "bin")+` uv tool install --reinstall --from https://github.com/everydaydevopsio/ballast/releases/download/v5.0.2/ballast_python-5.0.2-py3-none-any.whl ballast-python` {
		t.Fatalf("unexpected install-cli command: %q", got)
	}
}

func TestRunInstallCLICommandInstallsAllLanguagesByDefault(t *testing.T) {
	originalRun := runCommandFunc
	originalVersion := version
	t.Cleanup(func() {
		runCommandFunc = originalRun
		version = originalVersion
	})
	version = "5.0.2"

	var commands [][]string
	runCommandFunc = func(name string, args []string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	withWorkingDir(t, root, func() {
		exitCode := run([]string{"install-cli"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(commands) != 3 {
		t.Fatalf("expected 3 install commands, got %#v", commands)
	}
	if got := strings.Join(commands[1], " "); got != `env UV_TOOL_DIR=`+filepath.Join(root, ".ballast", "tools", "python")+` UV_TOOL_BIN_DIR=`+filepath.Join(root, ".ballast", "bin")+` uv tool install --reinstall --from https://github.com/everydaydevopsio/ballast/releases/download/v5.0.2/ballast_python-5.0.2-py3-none-any.whl ballast-python` {
		t.Fatalf("unexpected default python install command: %q", got)
	}
}

func TestRunInstallCLICreatesLocalBallastDirectories(t *testing.T) {
	originalRun := runCommandFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
	})

	runCommandFunc = func(name string, args []string) error { return nil }

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	withWorkingDir(t, root, func() {
		exitCode := run([]string{"install-cli", "--language", "go", "--version", "5.0.2"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if _, err := os.Stat(filepath.Join(root, ".ballast", "bin")); err != nil {
		t.Fatalf("expected .ballast/bin to exist, got %v", err)
	}
}

func TestPythonInstallCommandErrorsForDevVersionOutsideSourceTree(t *testing.T) {
	originalVersion := version
	originalExecutable := osExecutableFunc
	t.Cleanup(func() {
		version = originalVersion
		osExecutableFunc = originalExecutable
	})

	version = "dev"
	osExecutableFunc = func() (string, error) {
		return filepath.Join(t.TempDir(), "outside-ballast", "ballast"), nil
	}

	root := t.TempDir()
	command, err := toolsByLanguage[langPython].installCommand("", root)
	if err == nil {
		t.Fatalf("expected dev version outside source tree to error, got command %#v", command)
	}
	if !strings.Contains(err.Error(), "requires a release version or a ballast source checkout") {
		t.Fatalf("unexpected install error: %v", err)
	}
}

func TestRunInstallCLIUsesLocalSourcesForDevWrapper(t *testing.T) {
	originalRun := runCommandFunc
	originalVersion := version
	originalExecutable := osExecutableFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
		version = originalVersion
		osExecutableFunc = originalExecutable
	})

	version = "dev"
	sourceRoot := t.TempDir()
	projectRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-typescript", "package.json"), `{"name":"@everydaydevopsio/ballast"}`)
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-python", "pyproject.toml"), "[project]\nname='ballast-python'\n")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-go", "go.mod"), "module example.com/ballast-go\n\ngo 1.24\n")
	osExecutableFunc = func() (string, error) {
		return filepath.Join(sourceRoot, "cli", "ballast", "ballast"), nil
	}

	var commands [][]string
	runCommandFunc = func(name string, args []string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	}

	mustWriteFile(t, filepath.Join(projectRoot, "package.json"), "{}")
	withWorkingDir(t, projectRoot, func() {
		exitCode := run([]string{"install-cli"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(commands) != 3 {
		t.Fatalf("expected 3 install commands, got %#v", commands)
	}
	if got := strings.Join(commands[0], " "); got != "npm install --prefix "+filepath.Join(projectRoot, ".ballast", "tools", "typescript")+" "+filepath.Join(sourceRoot, "packages", "ballast-typescript") {
		t.Fatalf("unexpected local typescript install command: %q", got)
	}
	if got := strings.Join(commands[1], " "); got != "env UV_TOOL_DIR="+filepath.Join(projectRoot, ".ballast", "tools", "python")+" UV_TOOL_BIN_DIR="+filepath.Join(projectRoot, ".ballast", "bin")+" uv tool install --reinstall "+filepath.Join(sourceRoot, "packages", "ballast-python") {
		t.Fatalf("unexpected local python install command: %q", got)
	}
	if got := strings.Join(commands[2], " "); got != "go build -C "+filepath.Join(sourceRoot, "packages", "ballast-go")+" -o "+filepath.Join(projectRoot, ".ballast", "bin", "ballast-go")+" ./cmd/ballast-go" {
		t.Fatalf("unexpected local go install command: %q", got)
	}
}

func TestRunInstallCLIUsesLocalSourcesInsideSourceCheckoutForReleaseWrapper(t *testing.T) {
	originalRun := runCommandFunc
	originalVersion := version
	originalExecutable := osExecutableFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
		version = originalVersion
		osExecutableFunc = originalExecutable
	})

	version = "5.0.4"
	sourceRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(sourceRoot, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-typescript", "package.json"), `{"name":"@everydaydevopsio/ballast"}`)
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-python", "pyproject.toml"), "[project]\nname='ballast-python'\n")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-go", "go.mod"), "module example.com/ballast-go\n\ngo 1.24\n")
	osExecutableFunc = func() (string, error) {
		return filepath.Join(t.TempDir(), "bin", "ballast"), nil
	}

	var commands [][]string
	runCommandFunc = func(name string, args []string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	}

	withWorkingDir(t, sourceRoot, func() {
		exitCode := run([]string{"install-cli"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(commands) != 3 {
		t.Fatalf("expected 3 install commands, got %#v", commands)
	}
	if got := strings.Join(commands[0], " "); got != "npm install --prefix "+filepath.Join(sourceRoot, ".ballast", "tools", "typescript")+" "+filepath.Join(sourceRoot, "packages", "ballast-typescript") {
		t.Fatalf("unexpected local typescript install command: %q", got)
	}
	if got := strings.Join(commands[1], " "); got != "env UV_TOOL_DIR="+filepath.Join(sourceRoot, ".ballast", "tools", "python")+" UV_TOOL_BIN_DIR="+filepath.Join(sourceRoot, ".ballast", "bin")+" uv tool install --reinstall "+filepath.Join(sourceRoot, "packages", "ballast-python") {
		t.Fatalf("unexpected local python install command: %q", got)
	}
	if got := strings.Join(commands[2], " "); got != "go build -C "+filepath.Join(sourceRoot, "packages", "ballast-go")+" -o "+filepath.Join(sourceRoot, ".ballast", "bin", "ballast-go")+" ./cmd/ballast-go" {
		t.Fatalf("unexpected local go install command: %q", got)
	}
}

func TestRunInstallCLIUsesLocalSourcesFromNestedPackageDirForReleaseWrapper(t *testing.T) {
	originalRun := runCommandFunc
	originalVersion := version
	originalExecutable := osExecutableFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
		version = originalVersion
		osExecutableFunc = originalExecutable
	})

	version = "5.0.4"
	sourceRoot := t.TempDir()
	nestedRoot := filepath.Join(sourceRoot, "packages", "ballast-python")
	mustWriteFile(t, filepath.Join(sourceRoot, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-typescript", "package.json"), `{"name":"@everydaydevopsio/ballast"}`)
	mustWriteFile(t, filepath.Join(nestedRoot, "pyproject.toml"), "[project]\nname='ballast-python'\n")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-go", "go.mod"), "module example.com/ballast-go\n\ngo 1.24\n")
	osExecutableFunc = func() (string, error) {
		return filepath.Join(t.TempDir(), "bin", "ballast"), nil
	}

	var commands [][]string
	runCommandFunc = func(name string, args []string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	}

	withWorkingDir(t, nestedRoot, func() {
		exitCode := run([]string{"install-cli"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(commands) != 3 {
		t.Fatalf("expected 3 install commands, got %#v", commands)
	}
	if got := strings.Join(commands[0], " "); got != "npm install --prefix "+filepath.Join(nestedRoot, ".ballast", "tools", "typescript")+" "+filepath.Join(sourceRoot, "packages", "ballast-typescript") {
		t.Fatalf("unexpected local typescript install command: %q", got)
	}
	if got := strings.Join(commands[1], " "); got != "env UV_TOOL_DIR="+filepath.Join(nestedRoot, ".ballast", "tools", "python")+" UV_TOOL_BIN_DIR="+filepath.Join(nestedRoot, ".ballast", "bin")+" uv tool install --reinstall "+filepath.Join(sourceRoot, "packages", "ballast-python") {
		t.Fatalf("unexpected local python install command: %q", got)
	}
	if got := strings.Join(commands[2], " "); got != "go build -C "+filepath.Join(sourceRoot, "packages", "ballast-go")+" -o "+filepath.Join(nestedRoot, ".ballast", "bin", "ballast-go")+" ./cmd/ballast-go" {
		t.Fatalf("unexpected local go install command: %q", got)
	}
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

func TestResolveMonorepoPlanInvokesOncePerLanguage(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "frontend-a", "tsconfig.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "apps", "frontend-b", "tsconfig.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "services", "api", "pyproject.toml"), "[project]\nname='api'\n")

	plan, err := resolveMonorepoPlan(root, []string{"install", "--target", "cursor", "--all"})
	if err != nil {
		t.Fatalf("resolveMonorepoPlan returned error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected monorepo plan, got nil")
	}
	if len(plan.Invocations) != 3 {
		t.Fatalf("expected 3 invocations (common + typescript + python), got %d: %#v", len(plan.Invocations), plan.Invocations)
	}
}

func TestResolveMonorepoPlanIgnoresStaleRulesrcProfiles(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "apps", "frontend", "tsconfig.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "services", "api", "pyproject.toml"), "[project]\nname='api'\n")
	mustWriteFile(t, filepath.Join(root, "tools", "worker", "go.mod"), "module example.com/worker\n\ngo 1.24\n")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "target": "cursor",
  "agents": ["local-dev", "cicd", "observability", "linting", "logging", "testing"],
  "languages": ["typescript"],
  "paths": {
    "typescript": ["."]
  }
}`)

	plan, err := resolveMonorepoPlan(root, []string{"install", "--target", "cursor", "--all"})
	if err != nil {
		t.Fatalf("resolveMonorepoPlan returned error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected monorepo plan, got nil")
	}
	if len(plan.Invocations) != 4 {
		t.Fatalf("expected 4 invocations (common + typescript + python + go), got %d: %#v", len(plan.Invocations), plan.Invocations)
	}

	langs := []language{
		plan.Invocations[1].Language,
		plan.Invocations[2].Language,
		plan.Invocations[3].Language,
	}
	if !reflect.DeepEqual(langs, []language{langTypeScript, langPython, langGo}) {
		t.Fatalf("expected language invocations for detected repo profiles, got %#v", langs)
	}
	if !strings.Contains(strings.Join(plan.Config.Languages, ","), "python") || !strings.Contains(strings.Join(plan.Config.Languages, ","), "go") {
		t.Fatalf("expected saved config to include detected languages, got %#v", plan.Config)
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

func TestRunMonorepoInstallPrefersRepoLocalBackends(t *testing.T) {
	sourceRoot := t.TempDir()
	projectRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-typescript", "dist", "cli.js"), "console.log('ts')")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-typescript", "package.json"), "{\"name\":\"ballast-typescript\"}")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-python", "pyproject.toml"), "[project]\nname='ballast-python'\n")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-python", "ballast", "__main__.py"), "print('py')")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-go", "go.mod"), "module example.com/ballast-go\n")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-go", "ballast-go"), "#!/bin/sh\n")
	mustWriteFile(t, filepath.Join(projectRoot, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(projectRoot, "apps", "frontend", "tsconfig.json"), "{}")
	mustWriteFile(t, filepath.Join(projectRoot, "services", "api", "pyproject.toml"), "[project]\nname='api'\n")
	mustWriteFile(t, filepath.Join(projectRoot, "tools", "worker", "go.mod"), "module example.com/worker\n\ngo 1.24\n")

	originalEnsure := ensureInstalledFunc
	originalExec := execToolFunc
	originalExecutable := osExecutableFunc
	t.Cleanup(func() {
		ensureInstalledFunc = originalEnsure
		execToolFunc = originalExec
		osExecutableFunc = originalExecutable
	})

	osExecutableFunc = func() (string, error) {
		return filepath.Join(sourceRoot, "cli", "ballast", "ballast"), nil
	}
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

	withWorkingDir(t, filepath.Join(projectRoot, "apps", "frontend"), func() {
		exitCode := run([]string{"install", "--target", "claude", "--all", "--yes"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(invocations) != 4 {
		t.Fatalf("expected 4 invocations, got %d: %#v", len(invocations), invocations)
	}

	if invocations[0].Binary != "node" {
		t.Fatalf("expected TypeScript backend to use node local entrypoint, got %#v", invocations[0])
	}
	if len(invocations[0].Args) < 2 || invocations[0].Args[0] != filepath.Join(sourceRoot, "packages", "ballast-typescript", "dist", "cli.js") {
		t.Fatalf("expected local TypeScript CLI path, got %#v", invocations[0])
	}
	if invocations[0].Env["BALLAST_REPO_ROOT"] != sourceRoot {
		t.Fatalf("expected BALLAST_REPO_ROOT on local TypeScript invocation, got %#v", invocations[0])
	}

	if invocations[1].Binary != "node" {
		t.Fatalf("expected language TypeScript backend to use node local entrypoint, got %#v", invocations[1])
	}
	if invocations[2].Binary != "python3" {
		t.Fatalf("expected Python backend to use python3 local module entrypoint, got %#v", invocations[2])
	}
	if got := strings.Join(invocations[2].Args, " "); got != "-m ballast install --target claude --yes --agent linting,logging,testing" {
		t.Fatalf("unexpected Python invocation args: %q", got)
	}
	if invocations[2].Env["BALLAST_REPO_ROOT"] != sourceRoot {
		t.Fatalf("expected BALLAST_REPO_ROOT on local Python invocation, got %#v", invocations[2])
	}
	if invocations[2].Env["PYTHONPATH"] != filepath.Join(sourceRoot, "packages", "ballast-python") {
		t.Fatalf("expected PYTHONPATH for local Python invocation, got %#v", invocations[2])
	}

	if invocations[3].Binary != filepath.Join(sourceRoot, "packages", "ballast-go", "ballast-go") {
		t.Fatalf("expected Go backend to use local binary, got %#v", invocations[3])
	}
	if invocations[3].Env["BALLAST_REPO_ROOT"] != sourceRoot {
		t.Fatalf("expected BALLAST_REPO_ROOT on local Go invocation, got %#v", invocations[3])
	}
}

func TestRunMonorepoInstallPrefersSiblingBackendsNextToWrapper(t *testing.T) {
	sourceRoot := t.TempDir()
	projectRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-typescript", "dist", "cli.js"), "console.log('ts')")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-typescript", "package.json"), "{\"name\":\"ballast-typescript\"}")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-python", "pyproject.toml"), "[project]\nname='ballast-python'\n")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-python", "ballast", "__main__.py"), "print('py')")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-go", "go.mod"), "module example.com/ballast-go\n")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-go", "ballast-go"), "#!/bin/sh\n")
	mustWriteFile(t, filepath.Join(sourceRoot, ".ci", "bin", "ballast-typescript"), "#!/bin/sh\n")
	mustWriteFile(t, filepath.Join(sourceRoot, ".ci", "bin", "ballast-python"), "#!/bin/sh\n")
	mustWriteFile(t, filepath.Join(sourceRoot, ".ci", "bin", "ballast-go"), "#!/bin/sh\n")
	mustWriteFile(t, filepath.Join(projectRoot, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(projectRoot, "apps", "frontend", "tsconfig.json"), "{}")
	mustWriteFile(t, filepath.Join(projectRoot, "services", "api", "pyproject.toml"), "[project]\nname='api'\n")
	mustWriteFile(t, filepath.Join(projectRoot, "tools", "worker", "go.mod"), "module example.com/worker\n\ngo 1.24\n")

	originalEnsure := ensureInstalledFunc
	originalExec := execToolFunc
	originalExecutable := osExecutableFunc
	t.Cleanup(func() {
		ensureInstalledFunc = originalEnsure
		execToolFunc = originalExec
		osExecutableFunc = originalExecutable
	})

	osExecutableFunc = func() (string, error) {
		return filepath.Join(sourceRoot, ".ci", "bin", "ballast"), nil
	}
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

	withWorkingDir(t, filepath.Join(projectRoot, "apps", "frontend"), func() {
		exitCode := run([]string{"install", "--target", "cursor", "--all", "--yes"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(invocations) != 4 {
		t.Fatalf("expected 4 invocations, got %d: %#v", len(invocations), invocations)
	}
	if invocations[0].Binary != filepath.Join(sourceRoot, ".ci", "bin", "ballast-typescript") {
		t.Fatalf("expected sibling TypeScript backend binary, got %#v", invocations[0])
	}
	if invocations[2].Binary != filepath.Join(sourceRoot, ".ci", "bin", "ballast-python") {
		t.Fatalf("expected sibling Python backend binary, got %#v", invocations[2])
	}
	if invocations[3].Binary != filepath.Join(sourceRoot, ".ci", "bin", "ballast-go") {
		t.Fatalf("expected sibling Go backend binary, got %#v", invocations[3])
	}
}

func TestRunSingleLanguageInstallPrefersLocalBackendWithoutNilEnvPanic(t *testing.T) {
	sourceRoot := t.TempDir()
	projectRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-typescript", "dist", "cli.js"), "console.log('ts')")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-typescript", "package.json"), "{\"name\":\"ballast-typescript\"}")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-python", "pyproject.toml"), "[project]\nname='ballast-python'\n")
	mustWriteFile(t, filepath.Join(sourceRoot, "packages", "ballast-go", "go.mod"), "module example.com/ballast-go\n")
	mustWriteFile(t, filepath.Join(projectRoot, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(projectRoot, "tsconfig.json"), "{}")

	originalEnsure := ensureInstalledFunc
	originalExec := execToolFunc
	originalExecutable := osExecutableFunc
	t.Cleanup(func() {
		ensureInstalledFunc = originalEnsure
		execToolFunc = originalExec
		osExecutableFunc = originalExecutable
	})

	osExecutableFunc = func() (string, error) {
		return filepath.Join(sourceRoot, "cli", "ballast", "ballast"), nil
	}

	ensureCalled := false
	ensureInstalledFunc = func(tool toolConfig) error {
		ensureCalled = true
		return nil
	}

	var invocation backendInvocation
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		invocation = backendInvocation{
			Binary: binary,
			Args:   append([]string(nil), args...),
			Dir:    dir,
			Env:    env,
		}
		return 0, nil
	}

	withWorkingDir(t, projectRoot, func() {
		exitCode := run([]string{"install", "--target", "codex", "--all", "--patch"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if ensureCalled {
		t.Fatal("expected local backend resolution to skip ensureInstalled")
	}
	if invocation.Binary != "node" {
		t.Fatalf("expected local TypeScript backend to run via node, got %#v", invocation)
	}
	if invocation.Env["BALLAST_REPO_ROOT"] != sourceRoot {
		t.Fatalf("expected BALLAST_REPO_ROOT in local invocation env, got %#v", invocation)
	}
}

func TestRunSingleLanguageInstallReinstallsBackendWhenVersionMismatches(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "tsconfig.json"), "{}")

	originalEnsure := ensureInstalledFunc
	originalExec := execToolFunc
	originalRun := runCommandFunc
	originalVersion := version
	t.Cleanup(func() {
		ensureInstalledFunc = originalEnsure
		execToolFunc = originalExec
		runCommandFunc = originalRun
		version = originalVersion
	})

	version = "5.0.0"

	var installCommands [][]string
	runCommandFunc = func(name string, args []string) error {
		installCommands = append(installCommands, append([]string{name}, args...))
		localBinary := filepath.Join(root, ".ballast", "tools", "typescript", "node_modules", ".bin", "ballast-typescript")
		mustWriteFile(t, localBinary, "#!/usr/bin/env bash\necho 5.0.0\n")
		if err := os.Chmod(localBinary, 0o755); err != nil {
			t.Fatalf("chmod %s: %v", localBinary, err)
		}
		return nil
	}

	var invocation backendInvocation
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		invocation = backendInvocation{
			Binary: binary,
			Args:   append([]string(nil), args...),
			Dir:    dir,
			Env:    env,
		}
		return 0, nil
	}

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"install", "--target", "cursor", "--all", "--yes"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(installCommands) != 1 {
		t.Fatalf("expected exactly one reinstall command, got %d", len(installCommands))
	}
	if got := strings.Join(installCommands[0], " "); got != "npm install --prefix "+filepath.Join(root, ".ballast", "tools", "typescript")+" @everydaydevopsio/ballast@5.0.0" {
		t.Fatalf("unexpected reinstall command: %q", got)
	}
	if invocation.Binary != filepath.Join(root, ".ballast", "tools", "typescript", "node_modules", ".bin", "ballast-typescript") {
		t.Fatalf("expected installed TypeScript backend to be executed, got %#v", invocation)
	}
}

func TestEnsureInstalledUsesPinnedVersionWhenBackendVersionDiffers(t *testing.T) {
	originalRun := runCommandFunc
	originalVersion := version
	t.Cleanup(func() {
		runCommandFunc = originalRun
		version = originalVersion
	})

	version = "5.0.0"

	var commands [][]string
	runCommandFunc = func(name string, args []string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/test\n\ngo 1.24\n")
	localBinary := filepath.Join(root, ".ballast", "bin", "ballast-go")
	mustWriteFile(t, localBinary, "#!/usr/bin/env bash\necho 4.1.7\n")
	if err := os.Chmod(localBinary, 0o755); err != nil {
		t.Fatalf("chmod %s: %v", localBinary, err)
	}
	withWorkingDir(t, root, func() {
		err := ensureInstalled(toolConfig{
			binary: "ballast-go",
			installCommand: func(version string, projectRoot string) ([]string, error) {
				return []string{"go", "install", "example.com/ballast-go@" + version, projectRoot}, nil
			},
		})
		if err != nil {
			t.Fatalf("ensureInstalled returned error: %v", err)
		}
	})

	if len(commands) != 1 {
		t.Fatalf("expected one reinstall command, got %d", len(commands))
	}
	if got := strings.Join(commands[0], " "); got != "go install example.com/ballast-go@5.0.0 "+root {
		t.Fatalf("unexpected install command: %q", got)
	}
}

func TestEnsureInstalledSkipsVersionCheckForDevWrapperBuilds(t *testing.T) {
	originalRun := runCommandFunc
	originalVersion := version
	t.Cleanup(func() {
		runCommandFunc = originalRun
		version = originalVersion
	})

	version = "dev"

	runCommandFunc = func(name string, args []string) error {
		t.Fatalf("expected dev builds to skip backend reinstalls, got %s %s", name, strings.Join(args, " "))
		return nil
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/test\n\ngo 1.24\n")
	localBinary := filepath.Join(root, ".ballast", "bin", "ballast-go")
	mustWriteFile(t, localBinary, "#!/usr/bin/env bash\necho 4.1.7\n")
	if err := os.Chmod(localBinary, 0o755); err != nil {
		t.Fatalf("chmod %s: %v", localBinary, err)
	}
	withWorkingDir(t, root, func() {
		err := ensureInstalled(toolConfig{
			binary: "ballast-go",
			installCommand: func(version string, projectRoot string) ([]string, error) {
				return []string{"go", "install", "example.com/ballast-go@" + version, projectRoot}, nil
			},
		})
		if err != nil {
			t.Fatalf("ensureInstalled returned error: %v", err)
		}
	})
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

func TestPatchInstalledRulesSectionIgnoresHeadingInsideCodeFence(t *testing.T) {
	existing := "# CLAUDE.md\n\n```md\n## Installed agent rules\n```\n\n## Installed agent rules\n\n- `.claude/rules/old.md` — Old rule\n"
	canonical := "# CLAUDE.md\n\n## Installed agent rules\n\nCreated by Ballast. Do not edit this section.\n\n- `.claude/rules/typescript-linting.md` — Rules for typescript/linting\n"

	merged := patchInstalledRulesSection(existing, canonical)
	if !strings.Contains(merged, "```md\n## Installed agent rules\n```") {
		t.Fatalf("expected fenced code block to remain untouched, got %q", merged)
	}
	if !strings.Contains(merged, "`.claude/rules/typescript-linting.md`") {
		t.Fatalf("expected installed rules section to be updated, got %q", merged)
	}
	if strings.Contains(merged, "`.claude/rules/old.md`") {
		t.Fatalf("expected old installed-rules entry to be replaced, got %q", merged)
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

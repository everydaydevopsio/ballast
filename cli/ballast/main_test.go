package main

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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
		if !strings.Contains(output, "upgrade") {
			t.Fatalf("expected upgrade in help output, got %q", output)
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
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "ballastVersion":"5.0.2",
  "target":"claude",
  "agents":["local-dev"],
  "languages":["typescript","ansible"],
  "paths":{
    "typescript":["apps/web"],
    "ansible":["infra/ansible"]
  }
}`)

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
	if !strings.Contains(output, "languages: typescript, ansible") {
		t.Fatalf("expected config languages in doctor output, got %q", output)
	}
	if !strings.Contains(output, "paths: typescript=apps/web; ansible=infra/ansible") {
		t.Fatalf("expected config paths in doctor output, got %q", output)
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

func TestRunDoctorPatchRequiresFix(t *testing.T) {
	output := captureStdout(t, func() {
		exitCode := run([]string{"doctor", "--patch"})
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	})

	if !strings.Contains(output, "--patch requires --fix") {
		t.Fatalf("expected doctor --patch guidance, got %q", output)
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

func TestRunDoctorFixUsesConfigVersionForBackendInstallsInsideSourceCheckout(t *testing.T) {
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
	version = "5.0.6"

	var commands [][]string
	runCommandFunc = func(name string, args []string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	}
	ensureInstalledFunc = func(tool toolConfig) error { return nil }
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		return 0, nil
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-typescript", "package.json"), `{"name":"@everydaydevopsio/ballast"}`)
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-python", "pyproject.toml"), "[project]\nname='ballast-python'\n")
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-go", "go.mod"), "module example.com/ballast-go\n\ngo 1.24\n")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "ballastVersion":"5.0.5",
  "target":"claude",
  "agents":["local-dev"],
  "languages":["typescript"],
  "paths":{"typescript":["."]}
}`)

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"doctor", "--fix"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(commands) != 3 {
		t.Fatalf("expected install commands for all backends, got %#v", commands)
	}
	if got := strings.Join(commands[0], " "); got != "npm install --prefix "+filepath.Join(root, ".ballast", "tools", "typescript")+" @everydaydevopsio/ballast@5.0.5" {
		t.Fatalf("expected doctor fix to pin typescript backend to config version, got %q", got)
	}
	if got := strings.Join(commands[1], " "); got != "env UV_TOOL_DIR="+filepath.Join(root, ".ballast", "tools", "python")+" UV_TOOL_BIN_DIR="+filepath.Join(root, ".ballast", "bin")+" uv tool install --reinstall --from https://github.com/everydaydevopsio/ballast/releases/download/v5.0.5/ballast_python-5.0.5-py3-none-any.whl ballast-python" {
		t.Fatalf("expected doctor fix to pin python backend to config version, got %q", got)
	}
	got := strings.Join(commands[2], " ")
	if !strings.Contains(got, "https://github.com/everydaydevopsio/ballast/releases/download/v5.0.5/ballast-go_5.0.5_") {
		t.Fatalf("expected doctor fix to pin go backend to config version, got %q", got)
	}
	if strings.Contains(got, filepath.Join(root, "packages", "ballast-go")) {
		t.Fatalf("expected doctor fix to avoid local source builds when config pins a release, got %q", got)
	}
	config, err := loadDoctorConfig(root)
	if err != nil {
		t.Fatalf("loadDoctorConfig returned error: %v", err)
	}
	if config == nil || config.BallastVersion != "5.0.5" {
		t.Fatalf("expected doctor fix to preserve pinned ballastVersion, got %#v", config)
	}
}

func TestEnsureLocalToolDirsAddsBallastToGitignore(t *testing.T) {
	root := t.TempDir()

	if err := ensureLocalToolDirs(root); err != nil {
		t.Fatalf("ensureLocalToolDirs failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), ".ballast/") {
		t.Fatalf("expected .ballast/ in .gitignore, got %q", string(content))
	}
}

func TestEnsureLocalToolDirsContinuesWhenGitignoreCannotBeUpdated(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".gitignore"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := ensureLocalToolDirs(root); err != nil {
		t.Fatalf("expected directory creation to continue, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".ballast", "bin")); err != nil {
		t.Fatalf("expected .ballast/bin to exist, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".ballast", "tools")); err != nil {
		t.Fatalf("expected .ballast/tools to exist, got %v", err)
	}
}

func TestRunUpgradeUpdatesConfigVersionAndInstallsMatchingBackends(t *testing.T) {
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
	version = "5.0.6"

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
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-typescript", "package.json"), `{"name":"@everydaydevopsio/ballast"}`)
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-python", "pyproject.toml"), "[project]\nname='ballast-python'\n")
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-go", "go.mod"), "module example.com/ballast-go\n\ngo 1.24\n")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "ballastVersion":"5.0.5",
  "target":"claude",
  "agents":["local-dev"],
  "languages":["typescript"],
  "paths":{"typescript":["."]}
}`)

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"upgrade"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	config, err := loadDoctorConfig(root)
	if err != nil {
		t.Fatalf("loadDoctorConfig returned error: %v", err)
	}
	if config == nil || config.BallastVersion != "5.0.6" {
		t.Fatalf("expected upgrade to rewrite ballastVersion to 5.0.6, got %#v", config)
	}
	if len(commands) != 3 {
		t.Fatalf("expected upgrade to install all backends, got %#v", commands)
	}
	if got := strings.Join(commands[0], " "); got != "npm install --prefix "+filepath.Join(root, ".ballast", "tools", "typescript")+" @everydaydevopsio/ballast@5.0.6" {
		t.Fatalf("expected upgrade to install latest typescript backend, got %q", got)
	}
	if got := strings.Join(invocation.Args, " "); !strings.Contains(got, "install --yes") {
		t.Fatalf("expected upgrade to refresh config via install --yes, got %q", got)
	}
}

func TestRunUpgradePatchForwardsPatchToRefreshInstall(t *testing.T) {
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
	version = "5.0.6"

	runCommandFunc = func(name string, args []string) error { return nil }
	ensureInstalledFunc = func(tool toolConfig) error { return nil }
	var invocation backendInvocation
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		invocation = backendInvocation{Binary: binary, Args: append([]string(nil), args...)}
		return 0, nil
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-typescript", "package.json"), `{"name":"@everydaydevopsio/ballast"}`)
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-python", "pyproject.toml"), "[project]\nname='ballast-python'\n")
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-go", "go.mod"), "module example.com/ballast-go\n\ngo 1.24\n")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "ballastVersion":"5.0.5",
  "target":"claude",
  "agents":["local-dev"],
  "languages":["typescript"],
  "paths":{"typescript":["."]}
}`)

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"upgrade", "--patch"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if got := strings.Join(invocation.Args, " "); !strings.Contains(got, "install --patch --yes") {
		t.Fatalf("expected upgrade --patch to refresh config via install --patch --yes, got %q", got)
	}
}

func TestRunUpgradeForceForwardsForceToRefreshInstall(t *testing.T) {
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
	version = "5.0.6"

	runCommandFunc = func(name string, args []string) error { return nil }
	ensureInstalledFunc = func(tool toolConfig) error { return nil }
	var invocation backendInvocation
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		invocation = backendInvocation{Binary: binary, Args: append([]string(nil), args...)}
		return 0, nil
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-typescript", "package.json"), `{"name":"@everydaydevopsio/ballast"}`)
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-python", "pyproject.toml"), "[project]\nname='ballast-python'\n")
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-go", "go.mod"), "module example.com/ballast-go\n\ngo 1.24\n")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "ballastVersion":"5.0.5",
  "target":"claude",
  "agents":["local-dev"],
  "languages":["typescript"],
  "paths":{"typescript":["."]}
}`)

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"upgrade", "--force"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if got := strings.Join(invocation.Args, " "); !strings.Contains(got, "install --force --yes") {
		t.Fatalf("expected upgrade --force to refresh config via install --force --yes, got %q", got)
	}
}

func TestRunDoctorFixForwardsSelectedLanguageToRefreshInstall(t *testing.T) {
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
	version = "5.0.6"

	var commands [][]string
	runCommandFunc = func(name string, args []string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	}
	ensureInstalledFunc = func(tool toolConfig) error { return nil }
	refreshCount := 0
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		refreshCount++
		return 0, nil
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-typescript", "package.json"), `{"name":"@everydaydevopsio/ballast"}`)
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-python", "pyproject.toml"), "[project]\nname='ballast-python'\n")
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-go", "go.mod"), "module example.com/ballast-go\n\ngo 1.24\n")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "ballastVersion":"5.0.5",
  "target":"claude",
  "agents":["linting"],
  "languages":["typescript","python","go"],
  "paths":{
    "typescript":["packages/ballast-typescript"],
    "python":["packages/ballast-python"],
    "go":["packages/ballast-go"]
  }
}`)

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"--language", "go", "doctor", "--fix"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(commands) != 1 {
		t.Fatalf("expected one backend install command for go, got %#v", commands)
	}
	if refreshCount != 1 {
		t.Fatalf("expected refresh install to run once for the selected language, got %d", refreshCount)
	}
}

func TestRunDoctorFixSkipsRefreshWhenConfigIsMissing(t *testing.T) {
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
	version = "5.0.6"

	runCommandFunc = func(name string, args []string) error { return nil }
	ensureInstalledFunc = func(tool toolConfig) error { return nil }
	refreshCount := 0
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		refreshCount++
		return 0, nil
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")

	if exitCode := runDoctorFix(root, langGo, false, false); exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if refreshCount != 0 {
		t.Fatalf("expected no refresh install when .rulesrc.json is missing, got %d", refreshCount)
	}
}

func TestRunDoctorFixReportsRewriteFailure(t *testing.T) {
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
	version = "5.0.6"

	runCommandFunc = func(name string, args []string) error { return nil }
	ensureInstalledFunc = func(tool toolConfig) error { return nil }
	refreshCount := 0
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		refreshCount++
		if refreshCount == 1 {
			if err := os.Chmod(filepath.Join(dir, ".rulesrc.json"), 0o444); err != nil {
				t.Fatalf("failed to make .rulesrc.json read-only: %v", err)
			}
		}
		return 0, nil
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-typescript", "package.json"), `{"name":"@everydaydevopsio/ballast"}`)
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-python", "pyproject.toml"), "[project]\nname='ballast-python'\n")
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-go", "go.mod"), "module example.com/ballast-go\n\ngo 1.24\n")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "ballastVersion":"5.0.5",
  "target":"claude",
  "agents":["linting"],
  "languages":["typescript","python","go"],
  "paths":{
    "typescript":["packages/ballast-typescript"],
    "python":["packages/ballast-python"],
    "go":["packages/ballast-go"]
  }
}`)

	output := captureStdout(t, func() {
		withWorkingDir(t, root, func() {
			exitCode := run([]string{"--language", "go", "upgrade"})
			if exitCode != 1 {
				t.Fatalf("expected exit code 1, got %d", exitCode)
			}
		})
	})

	if refreshCount != 1 {
		t.Fatalf("expected a single refresh install before rewrite failure, got %d", refreshCount)
	}
	if !strings.Contains(output, "rulesrc.json") {
		t.Fatalf("expected rewrite failure to be reported, got %q", output)
	}
}

func TestRunUpgradeRejectsUnknownOption(t *testing.T) {
	output := captureStdout(t, func() {
		exitCode := run([]string{"upgrade", "--bogus"})
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	})

	if !strings.Contains(output, "unknown upgrade option: --bogus") {
		t.Fatalf("expected upgrade option error, got %q", output)
	}
}

func TestRunUpgradeRequiresExistingConfig(t *testing.T) {
	output := captureStdout(t, func() {
		withWorkingDir(t, t.TempDir(), func() {
			exitCode := run([]string{"upgrade"})
			if exitCode != 1 {
				t.Fatalf("expected exit code 1, got %d", exitCode)
			}
		})
	})

	if !strings.Contains(output, "upgrade requires an existing .rulesrc.json") {
		t.Fatalf("expected missing config error, got %q", output)
	}
}

func TestDetectBrewInstallReturnsFalseWhenBrewNotFound(t *testing.T) {
	originalLookPath := execLookPathFunc
	t.Cleanup(func() { execLookPathFunc = originalLookPath })

	execLookPathFunc = func(file string) (string, error) {
		return "", errors.New("not found")
	}

	if detectBrewInstall() {
		t.Fatal("expected detectBrewInstall to return false when brew is not on PATH")
	}
}

func TestDetectBrewInstallReturnsFalseWhenExeOutsidePrefix(t *testing.T) {
	originalLookPath := execLookPathFunc
	originalOutput := runCommandOutputFunc
	originalExe := osExecutableFunc
	t.Cleanup(func() {
		execLookPathFunc = originalLookPath
		runCommandOutputFunc = originalOutput
		osExecutableFunc = originalExe
	})

	execLookPathFunc = func(file string) (string, error) { return "/usr/local/bin/brew", nil }
	runCommandOutputFunc = func(name string, args []string) (string, error) {
		return "/opt/homebrew", nil
	}
	osExecutableFunc = func() (string, error) { return "/usr/local/bin/ballast", nil }

	if detectBrewInstall() {
		t.Fatal("expected detectBrewInstall to return false when exe is outside brew prefix")
	}
}

func TestDetectBrewInstallReturnsTrueWhenExeUnderPrefix(t *testing.T) {
	originalLookPath := execLookPathFunc
	originalOutput := runCommandOutputFunc
	originalExe := osExecutableFunc
	t.Cleanup(func() {
		execLookPathFunc = originalLookPath
		runCommandOutputFunc = originalOutput
		osExecutableFunc = originalExe
	})

	execLookPathFunc = func(file string) (string, error) { return "/opt/homebrew/bin/brew", nil }
	runCommandOutputFunc = func(name string, args []string) (string, error) {
		return "/opt/homebrew", nil
	}
	osExecutableFunc = func() (string, error) { return "/opt/homebrew/bin/ballast", nil }

	if !detectBrewInstall() {
		t.Fatal("expected detectBrewInstall to return true when exe is under brew prefix")
	}
}

func TestRunUpdateRunsBrewWhenBrewInstalled(t *testing.T) {
	originalRun := runCommandFunc
	originalLookPath := execLookPathFunc
	originalOutput := runCommandOutputFunc
	originalExe := osExecutableFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
		execLookPathFunc = originalLookPath
		runCommandOutputFunc = originalOutput
		osExecutableFunc = originalExe
	})

	execLookPathFunc = func(file string) (string, error) { return "/opt/homebrew/bin/brew", nil }
	runCommandOutputFunc = func(name string, args []string) (string, error) {
		return "/opt/homebrew", nil
	}
	osExecutableFunc = func() (string, error) { return "/opt/homebrew/bin/ballast", nil }

	var commands [][]string
	runCommandFunc = func(name string, args []string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	}

	exitCode := run([]string{"update"})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	if len(commands) < 2 {
		t.Fatalf("expected at least 2 commands (brew update + brew upgrade), got %v", commands)
	}
	if got := strings.Join(commands[0], " "); got != "brew update" {
		t.Fatalf("expected first command to be 'brew update', got %q", got)
	}
	if got := commands[1][0]; got != "brew" {
		t.Fatalf("expected second command to start with 'brew', got %q", got)
	}
	if got := commands[1][1]; got != "upgrade" {
		t.Fatalf("expected second command to be 'brew upgrade ...', got %v", commands[1])
	}
}

func TestRunUpdateFailsWhenNotBrewInstalled(t *testing.T) {
	originalLookPath := execLookPathFunc
	t.Cleanup(func() { execLookPathFunc = originalLookPath })

	execLookPathFunc = func(file string) (string, error) {
		return "", errors.New("not found")
	}

	output := captureStdout(t, func() {
		exitCode := run([]string{"update"})
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	})

	if !strings.Contains(output, "only supported for Homebrew installations") {
		t.Fatalf("expected non-brew message, got %q", output)
	}
}

func TestRunUpdateRejectsUnknownOption(t *testing.T) {
	originalRun := runCommandFunc
	t.Cleanup(func() { runCommandFunc = originalRun })

	var commands [][]string
	runCommandFunc = func(name string, args []string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	}

	output := captureStdout(t, func() {
		exitCode := run([]string{"update", "--bogus"})
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	})

	if !strings.Contains(output, "unknown update option: --bogus") {
		t.Fatalf("expected update option error, got %q", output)
	}
	if len(commands) != 0 {
		t.Fatalf("expected no commands to run for invalid update args, got %v", commands)
	}
}

func TestRunUpdateFailsWhenBrewUpdateFails(t *testing.T) {
	originalRun := runCommandFunc
	originalLookPath := execLookPathFunc
	originalOutput := runCommandOutputFunc
	originalExe := osExecutableFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
		execLookPathFunc = originalLookPath
		runCommandOutputFunc = originalOutput
		osExecutableFunc = originalExe
	})

	execLookPathFunc = func(file string) (string, error) { return "/opt/homebrew/bin/brew", nil }
	runCommandOutputFunc = func(name string, args []string) (string, error) {
		return "/opt/homebrew", nil
	}
	osExecutableFunc = func() (string, error) { return "/opt/homebrew/bin/ballast", nil }
	runCommandFunc = func(name string, args []string) error {
		if name == "brew" && len(args) > 0 && args[0] == "update" {
			return errors.New("brew update error")
		}
		return nil
	}

	output := captureStdout(t, func() {
		exitCode := run([]string{"update"})
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	})

	if !strings.Contains(output, "brew update failed") {
		t.Fatalf("expected brew update failure message, got %q", output)
	}
}

func TestRunUpdateFailsWhenBrewUpgradeFails(t *testing.T) {
	originalRun := runCommandFunc
	originalLookPath := execLookPathFunc
	originalOutput := runCommandOutputFunc
	originalExe := osExecutableFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
		execLookPathFunc = originalLookPath
		runCommandOutputFunc = originalOutput
		osExecutableFunc = originalExe
	})

	execLookPathFunc = func(file string) (string, error) { return "/opt/homebrew/bin/brew", nil }
	runCommandOutputFunc = func(name string, args []string) (string, error) {
		return "/opt/homebrew", nil
	}
	osExecutableFunc = func() (string, error) { return "/opt/homebrew/bin/ballast", nil }
	runCommandFunc = func(name string, args []string) error {
		if name == "brew" && len(args) > 0 && args[0] == "upgrade" {
			return errors.New("brew upgrade error")
		}
		return nil
	}

	output := captureStdout(t, func() {
		exitCode := run([]string{"update"})
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	})

	if !strings.Contains(output, "brew upgrade failed") {
		t.Fatalf("expected brew upgrade failure message, got %q", output)
	}
}

func TestRunUpdateSuccessMessage(t *testing.T) {
	originalRun := runCommandFunc
	originalLookPath := execLookPathFunc
	originalOutput := runCommandOutputFunc
	originalExe := osExecutableFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
		execLookPathFunc = originalLookPath
		runCommandOutputFunc = originalOutput
		osExecutableFunc = originalExe
	})

	execLookPathFunc = func(file string) (string, error) { return "/opt/homebrew/bin/brew", nil }
	runCommandOutputFunc = func(name string, args []string) (string, error) {
		return "/opt/homebrew", nil
	}
	osExecutableFunc = func() (string, error) { return "/opt/homebrew/bin/ballast", nil }
	runCommandFunc = func(name string, args []string) error { return nil }

	output := captureStdout(t, func() {
		exitCode := run([]string{"update"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if !strings.Contains(output, "ballast upgrade") {
		t.Fatalf("expected post-update hint to run ballast upgrade, got %q", output)
	}
}

func TestRunUpgradeDoesNotRunBrew(t *testing.T) {
	originalRun := runCommandFunc
	originalEnsure := ensureInstalledFunc
	originalExec := execToolFunc
	originalVersion := version
	originalLookPath := execLookPathFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
		ensureInstalledFunc = originalEnsure
		execToolFunc = originalExec
		version = originalVersion
		execLookPathFunc = originalLookPath
	})
	version = "5.0.6"

	// Even when brew is on PATH, upgrade should not invoke brew
	execLookPathFunc = func(file string) (string, error) { return "/opt/homebrew/bin/brew", nil }

	var commands [][]string
	runCommandFunc = func(name string, args []string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	}
	ensureInstalledFunc = func(tool toolConfig) error { return nil }
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		return 0, nil
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "ballastVersion":"5.0.5",
  "target":"claude",
  "agents":["local-dev"],
  "languages":["typescript"],
  "paths":{"typescript":["."]}
}`)

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"upgrade"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	for _, cmd := range commands {
		if cmd[0] == "brew" {
			t.Fatalf("expected no brew commands from upgrade, got %v", cmd)
		}
	}
}

func TestDetectBrewInstallReturnsFalseWhenPrefixError(t *testing.T) {
	originalLookPath := execLookPathFunc
	originalOutput := runCommandOutputFunc
	t.Cleanup(func() {
		execLookPathFunc = originalLookPath
		runCommandOutputFunc = originalOutput
	})

	execLookPathFunc = func(file string) (string, error) { return "/usr/local/bin/brew", nil }
	runCommandOutputFunc = func(name string, args []string) (string, error) {
		return "", errors.New("brew --prefix failed")
	}

	if detectBrewInstall() {
		t.Fatal("expected detectBrewInstall to return false when brew --prefix fails")
	}
}

func TestDetectBrewInstallReturnsFalseWhenPrefixEmpty(t *testing.T) {
	originalLookPath := execLookPathFunc
	originalOutput := runCommandOutputFunc
	t.Cleanup(func() {
		execLookPathFunc = originalLookPath
		runCommandOutputFunc = originalOutput
	})

	execLookPathFunc = func(file string) (string, error) { return "/usr/local/bin/brew", nil }
	runCommandOutputFunc = func(name string, args []string) (string, error) {
		return "", nil
	}

	if detectBrewInstall() {
		t.Fatal("expected detectBrewInstall to return false when brew --prefix returns empty")
	}
}

func TestDetectBrewInstallReturnsFalseWhenExecutableFails(t *testing.T) {
	originalLookPath := execLookPathFunc
	originalOutput := runCommandOutputFunc
	originalExe := osExecutableFunc
	t.Cleanup(func() {
		execLookPathFunc = originalLookPath
		runCommandOutputFunc = originalOutput
		osExecutableFunc = originalExe
	})

	execLookPathFunc = func(file string) (string, error) { return "/usr/local/bin/brew", nil }
	runCommandOutputFunc = func(name string, args []string) (string, error) {
		return "/opt/homebrew", nil
	}
	osExecutableFunc = func() (string, error) { return "", errors.New("cannot determine executable") }

	if detectBrewInstall() {
		t.Fatal("expected detectBrewInstall to return false when os.Executable fails")
	}
}

func TestDetectBrewInstallSafePathPrefixCheck(t *testing.T) {
	originalLookPath := execLookPathFunc
	originalOutput := runCommandOutputFunc
	originalExe := osExecutableFunc
	t.Cleanup(func() {
		execLookPathFunc = originalLookPath
		runCommandOutputFunc = originalOutput
		osExecutableFunc = originalExe
	})

	execLookPathFunc = func(file string) (string, error) { return "/opt/homebrew/bin/brew", nil }
	runCommandOutputFunc = func(name string, args []string) (string, error) {
		return "/opt/homebrew", nil
	}
	// Path starts with prefix string but is NOT under the directory
	osExecutableFunc = func() (string, error) { return "/opt/homebrew-old/bin/ballast", nil }

	if detectBrewInstall() {
		t.Fatal("expected detectBrewInstall to return false for path with shared string prefix but different directory")
	}
}

func TestBrewUpgradeArgsReturnsNonEmpty(t *testing.T) {
	args := brewUpgradeArgs()
	if len(args) == 0 {
		t.Fatal("expected brewUpgradeArgs to return non-empty args")
	}
	if args[0] != "upgrade" {
		t.Fatalf("expected first arg to be 'upgrade', got %q", args[0])
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

func TestRunInstallCLIGoUsesReleaseArchiveForPinnedVersion(t *testing.T) {
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
	mustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/test\n\ngo 1.24\n")
	withWorkingDir(t, root, func() {
		exitCode := run([]string{"install-cli", "--language", "go", "--version", "5.0.2"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(commands) != 1 {
		t.Fatalf("expected 1 install command, got %#v", commands)
	}
	got := strings.Join(commands[0], " ")
	if !strings.Contains(got, "https://github.com/everydaydevopsio/ballast/releases/download/v5.0.2/ballast-go_5.0.2_") {
		t.Fatalf("expected installer to use a release archive, got %q", got)
	}
	if !strings.Contains(got, "https://github.com/everydaydevopsio/ballast/releases/download/v5.0.2/ballast-go_checksums.txt") {
		t.Fatalf("expected installer to verify the published checksum asset, got %q", got)
	}
	if !strings.Contains(got, `destination="$3"`) || !strings.Contains(got, `install -m 0755 "$tmpdir/ballast-go" "$destination"`) {
		t.Fatalf("expected installer to preserve the destination path while verifying checksums, got %q", got)
	}
	if strings.Contains(got, "go install github.com/everydaydevopsio/ballast/packages/ballast-go/cmd/ballast-go@") {
		t.Fatalf("expected installer to avoid the invalid module path, got %q", got)
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

func TestRunInstallCLICommandInstallsGoBackendForAnsibleLanguage(t *testing.T) {
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
	mustWriteFile(t, filepath.Join(root, "ansible.cfg"), "[defaults]\n")
	withWorkingDir(t, root, func() {
		exitCode := run([]string{"install-cli", "--language", "ansible", "--version", "5.0.2"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(commands) != 1 {
		t.Fatalf("expected 1 install command, got %#v", commands)
	}
	got := strings.Join(commands[0], " ")
	if !strings.Contains(got, "ballast-go_5.0.2_") {
		t.Fatalf("expected ansible install-cli to reuse ballast-go backend, got %q", got)
	}
}

func TestRunInstallCLICommandInstallsGoBackendForTerraformLanguage(t *testing.T) {
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
	mustWriteFile(t, filepath.Join(root, ".terraform-version"), "1.8.5\n")
	withWorkingDir(t, root, func() {
		exitCode := run([]string{"install-cli", "--language", "terraform", "--version", "5.0.2"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(commands) != 1 {
		t.Fatalf("expected 1 install command, got %#v", commands)
	}
	got := strings.Join(commands[0], " ")
	if !strings.Contains(got, "ballast-go_5.0.2_") {
		t.Fatalf("expected terraform install-cli to reuse ballast-go backend, got %q", got)
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

func TestResolveLocalBackendCommandUsesGoBinaryForTerraform(t *testing.T) {
	repoRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(repoRoot, "packages", "ballast-go", "ballast-go"), "")

	resolved := resolveLocalBackendCommand(repoRoot, langTerraform)
	if resolved.Binary != filepath.Join(repoRoot, "packages", "ballast-go", "ballast-go") {
		t.Fatalf("expected terraform local backend to reuse ballast-go binary, got %#v", resolved)
	}
}

func TestProjectLocalBackendCommandUsesGoBinaryForTerraform(t *testing.T) {
	projectRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(projectRoot, ".ballast", "bin", "ballast-go"), "")

	resolved := projectLocalBackendCommand(projectRoot, langTerraform)
	if resolved.Binary != filepath.Join(projectRoot, ".ballast", "bin", "ballast-go") {
		t.Fatalf("expected terraform project-local backend to reuse ballast-go binary, got %#v", resolved)
	}
}

func TestSiblingBackendBinaryUsesGoBinaryForTerraform(t *testing.T) {
	originalExecutable := osExecutableFunc
	t.Cleanup(func() {
		osExecutableFunc = originalExecutable
	})

	binDir := t.TempDir()
	mustWriteFile(t, filepath.Join(binDir, "ballast-go"), "")
	osExecutableFunc = func() (string, error) {
		return filepath.Join(binDir, "ballast"), nil
	}

	got, ok := siblingBackendBinary(langTerraform)
	if !ok {
		t.Fatal("expected terraform sibling backend binary to be found")
	}
	if got != filepath.Join(binDir, "ballast-go") {
		t.Fatalf("expected terraform sibling backend to reuse ballast-go binary, got %q", got)
	}
}

func TestRunInstallCLIUsesPinnedReleaseVersionsInsideSourceCheckout(t *testing.T) {
	originalRun := runCommandFunc
	originalVersion := version
	originalExecutable := osExecutableFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
		version = originalVersion
		osExecutableFunc = originalExecutable
	})

	version = "5.0.6"
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
		exitCode := run([]string{"install-cli", "--version", "5.0.5"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if len(commands) != 3 {
		t.Fatalf("expected 3 install commands, got %#v", commands)
	}
	if got := strings.Join(commands[0], " "); got != "npm install --prefix "+filepath.Join(sourceRoot, ".ballast", "tools", "typescript")+" @everydaydevopsio/ballast@5.0.5" {
		t.Fatalf("expected pinned typescript release install, got %q", got)
	}
	if got := strings.Join(commands[1], " "); got != "env UV_TOOL_DIR="+filepath.Join(sourceRoot, ".ballast", "tools", "python")+" UV_TOOL_BIN_DIR="+filepath.Join(sourceRoot, ".ballast", "bin")+" uv tool install --reinstall --from https://github.com/everydaydevopsio/ballast/releases/download/v5.0.5/ballast_python-5.0.5-py3-none-any.whl ballast-python" {
		t.Fatalf("expected pinned python release install, got %q", got)
	}
	got := strings.Join(commands[2], " ")
	if !strings.Contains(got, "https://github.com/everydaydevopsio/ballast/releases/download/v5.0.5/ballast-go_5.0.5_") {
		t.Fatalf("expected pinned go release install, got %q", got)
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

func TestDetectRepoProfilesFindsAnsibleProfile(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "infra", "ansible", "ansible.cfg"), "[defaults]\n")
	mustWriteFile(t, filepath.Join(root, "infra", "ansible", "playbook.yml"), "---\n")

	profiles, err := detectRepoProfiles(root)
	if err != nil {
		t.Fatalf("detectRepoProfiles returned error: %v", err)
	}

	want := []repoProfile{
		{Language: langAnsible, Paths: []string{filepath.Join(root, "infra", "ansible")}},
	}
	if !reflect.DeepEqual(profiles, want) {
		t.Fatalf("expected ansible profile %#v, got %#v", want, profiles)
	}
}

func TestDetectRepoProfilesFindsAnsibleProfileFromRequirementsYaml(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "infra", "ansible", "requirements.yaml"), "---\n")

	profiles, err := detectRepoProfiles(root)
	if err != nil {
		t.Fatalf("detectRepoProfiles returned error: %v", err)
	}

	want := []repoProfile{
		{Language: langAnsible, Paths: []string{filepath.Join(root, "infra", "ansible")}},
	}
	if !reflect.DeepEqual(profiles, want) {
		t.Fatalf("expected ansible profile %#v, got %#v", want, profiles)
	}
}

func TestDetectRepoProfilesFindsTerraformProfile(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "infra", "terraform", ".terraform-version"), "1.8.5\n")
	mustWriteFile(t, filepath.Join(root, "infra", "terraform", "versions.tf"), "terraform {}\n")

	profiles, err := detectRepoProfiles(root)
	if err != nil {
		t.Fatalf("detectRepoProfiles returned error: %v", err)
	}

	want := []repoProfile{
		{Language: langTerraform, Paths: []string{filepath.Join(root, "infra", "terraform")}},
	}
	if !reflect.DeepEqual(profiles, want) {
		t.Fatalf("expected terraform profile %#v, got %#v", want, profiles)
	}
}

func TestDetectRepoProfilesSkipsTerraformCaches(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "infra", "terraform", ".terraform-version"), "1.8.5\n")
	mustWriteFile(t, filepath.Join(root, "infra", "terraform", "versions.tf"), "terraform {}\n")
	mustWriteFile(t, filepath.Join(root, "infra", "terraform", ".terraform", "modules", "cached", "main.tf"), "terraform {}\n")
	mustWriteFile(t, filepath.Join(root, "infra", "terraform", ".terragrunt-cache", "cached", "main.tf"), "terraform {}\n")

	profiles, err := detectRepoProfiles(root)
	if err != nil {
		t.Fatalf("detectRepoProfiles returned error: %v", err)
	}

	want := []repoProfile{
		{Language: langTerraform, Paths: []string{filepath.Join(root, "infra", "terraform")}},
	}
	if !reflect.DeepEqual(profiles, want) {
		t.Fatalf("expected cached terraform directories to be skipped; want %#v, got %#v", want, profiles)
	}
}

func TestDetectLanguageSupportsAnsibleRequirementsYaml(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "requirements.yaml"), "---\n")

	got := detectLanguage(root)
	if got != langAnsible {
		t.Fatalf("expected ansible detection, got %q", got)
	}
}

func TestDetectLanguagePrefersAnsibleMarkers(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "ansible.cfg"), "[defaults]\n")
	mustWriteFile(t, filepath.Join(root, "playbook.yml"), "---\n")

	got := detectLanguage(root)
	if got != langAnsible {
		t.Fatalf("expected ansible detection, got %q", got)
	}
}

func TestDetectLanguageSupportsTerraformMarkers(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".terraform-version"), "1.8.5\n")
	mustWriteFile(t, filepath.Join(root, "versions.tf"), "terraform {}\n")

	got := detectLanguage(root)
	if got != langTerraform {
		t.Fatalf("expected terraform detection, got %q", got)
	}
}

func TestDetectLanguageSupportsTerraformRulesConfig(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".rulesrc.terraform.json"), `{"target":"cursor","agents":["linting"]}`)

	got := detectLanguage(root)
	if got != langTerraform {
		t.Fatalf("expected terraform detection from legacy config, got %q", got)
	}
}

func TestDetectLanguageWarnsForJavaScriptPackageWithoutTsconfig(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), `{
  "name": "novnc-desktop",
  "private": true,
  "scripts": {
    "test": "playwright test"
  },
  "devDependencies": {
    "@playwright/test": "^1.45.0"
  }
}`)

	stderr := captureStderr(t, func() {
		got := detectLanguage(root)
		if got != langTypeScript {
			t.Fatalf("expected typescript detection from package.json, got %q", got)
		}
	})

	if !strings.Contains(stderr, "JavaScript package.json-based component or app") {
		t.Fatalf("expected JavaScript conversion warning, got %q", stderr)
	}
}

func TestResolveMonorepoPlanWarnsForJavaScriptRootWithoutTypeScriptProfile(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), `{
  "name": "novnc-desktop",
  "private": true,
  "scripts": {
    "test": "playwright test"
  },
  "devDependencies": {
    "@playwright/test": "^1.45.0"
  }
}`)
	mustWriteFile(t, filepath.Join(root, "ansible.cfg"), "[defaults]\n")
	mustWriteFile(t, filepath.Join(root, "infra", "terraform", "versions.tf"), "terraform {}\n")
	mustWriteFile(t, filepath.Join(root, "infra", "terraform", ".terraform-version"), "1.8.5\n")

	stderr := captureStderr(t, func() {
		plan, err := resolveMonorepoPlan(root, []string{"install", "--target", "cursor", "--all"})
		if err != nil {
			t.Fatalf("resolveMonorepoPlan returned error: %v", err)
		}
		if plan == nil {
			t.Fatal("expected monorepo plan, got nil")
		}
	})

	if !strings.Contains(stderr, "JavaScript package.json-based component or app") {
		t.Fatalf("expected JavaScript conversion warning, got %q", stderr)
	}
}

func TestResolveBackendCommandAddsAnsibleLanguageFlag(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "ansible.cfg"), "[defaults]\n")
	mustWriteFile(t, filepath.Join(root, ".ballast", "bin", "ballast-go"), "")

	withWorkingDir(t, root, func() {
		resolved := resolveBackendCommand(langAnsible, toolsByLanguage[langAnsible], []string{"install", "--target", "cursor", "--all"}, nil)
		if resolved.Binary != filepath.Join(root, ".ballast", "bin", "ballast-go") {
			t.Fatalf("expected project-local ballast-go backend, got %#v", resolved)
		}
		got := strings.Join(resolved.Args, " ")
		if !strings.Contains(got, "install --language ansible --target cursor --all") {
			t.Fatalf("expected ansible language forwarding, got %q", got)
		}
	})
}

func TestResolveBackendCommandAddsTerraformLanguageFlag(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".terraform-version"), "1.8.5\n")
	mustWriteFile(t, filepath.Join(root, ".ballast", "bin", "ballast-go"), "")

	withWorkingDir(t, root, func() {
		resolved := resolveBackendCommand(langTerraform, toolsByLanguage[langTerraform], []string{"install", "--target", "cursor", "--all"}, nil)
		if resolved.Binary != filepath.Join(root, ".ballast", "bin", "ballast-go") {
			t.Fatalf("expected project-local ballast-go backend, got %#v", resolved)
		}
		got := strings.Join(resolved.Args, " ")
		if !strings.Contains(got, "install --language terraform --target cursor --all") {
			t.Fatalf("expected terraform language forwarding, got %q", got)
		}
	})
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

func TestResolveMonorepoPlanAllIncludesExpectedCommonAgents(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "frontend", "tsconfig.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "services", "api", "pyproject.toml"), "[project]\nname='api'\n")

	plan, err := resolveMonorepoPlan(root, []string{"install", "--target", "cursor", "--all"})
	if err != nil {
		t.Fatalf("resolveMonorepoPlan returned error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected monorepo plan, got nil")
	}
	if len(plan.Invocations) == 0 {
		t.Fatal("expected at least one backend invocation")
	}
	got := strings.Join(plan.Invocations[0].Args, " ")
	if !strings.Contains(got, "--agent local-dev,docs,cicd,observability,publishing,git-hooks") {
		t.Fatalf("expected common invocation to include normalized common agents, got %q", got)
	}

	for _, expectedAgent := range []string{"docs", "publishing"} {
		if !slices.Contains(plan.Config.Agents, expectedAgent) {
			t.Fatalf("expected saved config agents to include %s, got %#v", expectedAgent, plan.Config.Agents)
		}
	}
}

func TestParseInstallSelectionIncludesSkills(t *testing.T) {
	agents, allAgents, skills, allSkills := parseInstallSelection([]string{
		"install",
		"--agent", "linting,testing",
		"--skill=owasp-security-scan",
		"--all-skills",
	})

	if allAgents {
		t.Fatal("expected --all to be false")
	}
	if !reflect.DeepEqual(agents, []string{"linting", "testing"}) {
		t.Fatalf("expected parsed agents, got %#v", agents)
	}
	if !reflect.DeepEqual(skills, []string{"owasp-security-scan"}) {
		t.Fatalf("expected parsed skills, got %#v", skills)
	}
	if !allSkills {
		t.Fatal("expected --all-skills to be true")
	}
}

func TestWithSkillSelectionReplacesExistingFlags(t *testing.T) {
	args := withSkillSelection(
		[]string{"install", "--target", "claude", "--skill", "old-skill", "--all-skills"},
		[]string{"owasp-security-scan"},
	)

	got := strings.Join(args, " ")
	if strings.Contains(got, "old-skill") {
		t.Fatalf("expected old skill selection to be removed, got %q", got)
	}
	if strings.Contains(got, "--all-skills") {
		t.Fatalf("expected --all-skills to be removed, got %q", got)
	}
	if !strings.Contains(got, "--skill owasp-security-scan") {
		t.Fatalf("expected normalized skill selection, got %q", got)
	}
}

func TestResolveMonorepoPlanSupportsSkillOnlyConfig(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "target": "claude",
  "skills": ["owasp-security-scan"],
  "languages": ["typescript", "python"],
  "paths": {
    "typescript": ["apps/frontend"],
    "python": ["services/api"]
  }
}`)

	plan, err := resolveMonorepoPlan(root, []string{"install"})
	if err != nil {
		t.Fatalf("resolveMonorepoPlan returned error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected monorepo plan, got nil")
	}
	if len(plan.Invocations) != 1 {
		t.Fatalf("expected a single common invocation for skill-only installs, got %#v", plan.Invocations)
	}
	got := strings.Join(plan.Invocations[0].Args, " ")
	if !strings.Contains(got, "--skill owasp-security-scan") {
		t.Fatalf("expected skill selection in common invocation, got %q", got)
	}
	if strings.Contains(got, "--agent") {
		t.Fatalf("expected no agent flags in skill-only invocation, got %q", got)
	}
	if !reflect.DeepEqual(plan.Config.Skills, []string{"owasp-security-scan"}) {
		t.Fatalf("expected saved config skills, got %#v", plan.Config.Skills)
	}
}

func TestResolveMonorepoPlanSkillOnlyInstallPreservesConfigAgents(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "target": "claude",
  "agents": ["local-dev", "linting"],
  "languages": ["typescript", "python"],
  "paths": {
    "typescript": ["apps/frontend"],
    "python": ["services/api"]
  }
}`)

	plan, err := resolveMonorepoPlan(root, []string{
		"install",
		"--target", "codex",
		"--skill", "owasp-security-scan",
	})
	if err != nil {
		t.Fatalf("resolveMonorepoPlan returned error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected monorepo plan, got nil")
	}
	if len(plan.Invocations) != 3 {
		t.Fatalf("expected common and per-language invocations, got %#v", plan.Invocations)
	}
	commonArgs := strings.Join(plan.Invocations[0].Args, " ")
	if !strings.Contains(commonArgs, "--agent local-dev") || !strings.Contains(commonArgs, "--skill owasp-security-scan") {
		t.Fatalf("expected common invocation to keep configured common agents and requested skill, got %q", commonArgs)
	}
	languageArgs := []string{
		strings.Join(plan.Invocations[1].Args, " "),
		strings.Join(plan.Invocations[2].Args, " "),
	}
	for _, got := range languageArgs {
		if !strings.Contains(got, "--agent linting") {
			t.Fatalf("expected language invocation to keep configured language agents, got %q", got)
		}
		if strings.Contains(got, "--skill") {
			t.Fatalf("expected language invocation to omit skill flags, got %q", got)
		}
	}
	if !reflect.DeepEqual(plan.Config.Agents, []string{"local-dev", "linting"}) {
		t.Fatalf("expected saved config to preserve existing agents, got %#v", plan.Config.Agents)
	}
	if !reflect.DeepEqual(plan.Config.Skills, []string{"owasp-security-scan"}) {
		t.Fatalf("expected saved config skills, got %#v", plan.Config.Skills)
	}
}

func TestResolveMonorepoPlanAgentOnlyInstallPreservesConfigSkills(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "target": "claude",
  "skills": ["owasp-security-scan"],
  "languages": ["typescript", "python"],
  "paths": {
    "typescript": ["apps/frontend"],
    "python": ["services/api"]
  }
}`)

	plan, err := resolveMonorepoPlan(root, []string{
		"install",
		"--target", "codex",
		"--agent", "local-dev",
	})
	if err != nil {
		t.Fatalf("resolveMonorepoPlan returned error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected monorepo plan, got nil")
	}
	got := strings.Join(plan.Invocations[0].Args, " ")
	if strings.Contains(got, "--skill") {
		t.Fatalf("expected configured skills not to be inherited, got %q", got)
	}
	if !strings.Contains(got, "--agent local-dev") {
		t.Fatalf("expected explicit agent selection, got %q", got)
	}
	if !reflect.DeepEqual(plan.Config.Skills, []string{"owasp-security-scan"}) {
		t.Fatalf("expected saved config to preserve existing skills, got %#v", plan.Config.Skills)
	}
}

func TestResolveMonorepoPlanSkillOnlyInstallRejectsInvalidPersistedAgents(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "target": "claude",
  "agents": ["not-an-agent"],
  "languages": ["typescript", "python"],
  "paths": {
    "typescript": ["apps/frontend"],
    "python": ["services/api"]
  }
}`)

	plan, err := resolveMonorepoPlan(root, []string{
		"install",
		"--target", "codex",
		"--skill", "owasp-security-scan",
	})
	if err == nil {
		t.Fatalf("expected unsupported agent error, got plan %#v", plan)
	}
	if !strings.Contains(err.Error(), "unsupported agent selection") {
		t.Fatalf("expected unsupported agent error, got %v", err)
	}
}

func TestResolveMonorepoPlanAgentOnlyInstallRejectsInvalidPersistedSkills(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "target": "claude",
  "skills": ["not-a-skill"],
  "languages": ["typescript", "python"],
  "paths": {
    "typescript": ["apps/frontend"],
    "python": ["services/api"]
  }
}`)

	plan, err := resolveMonorepoPlan(root, []string{
		"install",
		"--target", "codex",
		"--agent", "local-dev",
	})
	if err == nil {
		t.Fatalf("expected unsupported skill error, got plan %#v", plan)
	}
	if !strings.Contains(err.Error(), "unsupported skill selection") {
		t.Fatalf("expected unsupported skill error, got %v", err)
	}
}

func TestResolveMonorepoPlanSkillOnlyInstallRetainsConfiguredAgents(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "apps", "frontend", "tsconfig.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "services", "api", "pyproject.toml"), "[project]\nname='api'\n")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "targets": ["claude", "codex"],
  "agents": ["local-dev", "linting"],
  "skills": ["owasp-security-scan"],
  "languages": ["typescript", "python"],
  "paths": {
    "typescript": ["apps/frontend"],
    "python": ["services/api"]
  }
}`)

	plan, err := resolveMonorepoPlan(root, []string{
		"install",
		"--target", "claude,codex",
		"--skill", "github-health-check",
		"--patch",
		"--yes",
	})
	if err != nil {
		t.Fatalf("resolveMonorepoPlan returned error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected monorepo plan, got nil")
	}
	if len(plan.Invocations) == 0 {
		t.Fatal("expected backend invocations")
	}
	combinedArgs := make([]string, 0, len(plan.Invocations))
	for _, invocation := range plan.Invocations {
		combinedArgs = append(combinedArgs, strings.Join(invocation.Args, " "))
	}
	joined := strings.Join(combinedArgs, "\n")
	if !strings.Contains(joined, "--agent local-dev") || !strings.Contains(joined, "--agent linting") {
		t.Fatalf("expected configured agents in skill-only invocations, got %q", joined)
	}
	if !strings.Contains(joined, "--skill github-health-check") {
		t.Fatalf("expected requested skill in invocation, got %q", joined)
	}
}

func TestResolveMonorepoPlanRemoveLastTargetCleanupOnly(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "targets": ["codex"],
  "agents": ["local-dev", "linting"],
  "languages": ["typescript", "python"],
  "paths": {
    "typescript": ["apps/frontend"],
    "python": ["services/api"]
  }
}`)

	plan, err := resolveMonorepoPlan(root, []string{"install", "--remove-target", "codex", "--yes"})
	if err != nil {
		t.Fatalf("resolveMonorepoPlan returned error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected monorepo plan, got nil")
	}
	if len(plan.Invocations) != 0 {
		t.Fatalf("expected cleanup-only plan with no backend invocations, got %#v", plan.Invocations)
	}
	if len(plan.Config.Targets) != 0 {
		t.Fatalf("expected saved config targets to be empty after removing last target, got %#v", plan.Config.Targets)
	}
	if !reflect.DeepEqual(plan.Removed, []string{"codex"}) {
		t.Fatalf("expected removed codex target, got %#v", plan.Removed)
	}
}

func TestResolveMonorepoPlanRejectsInvalidConfiguredTargets(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "targets": ["cursor", "bogus"],
  "agents": ["local-dev", "linting"],
  "languages": ["typescript", "python"],
  "paths": {
    "typescript": ["apps/frontend"],
    "python": ["services/api"]
  }
}`)

	plan, err := resolveMonorepoPlan(root, []string{"install"})
	if err == nil {
		t.Fatalf("expected invalid target error, got plan %#v", plan)
	}
	if !strings.Contains(err.Error(), "unsupported target selection") {
		t.Fatalf("expected unsupported target error, got %v", err)
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
	if got := strings.Join(invocations[0].Args, " "); !strings.Contains(got, "--agent local-dev,docs,cicd,observability,publishing,git-hooks") {
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

func TestRunMonorepoInstallMergesRequestedTargetsIntoSavedConfig(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "frontend", "tsconfig.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "services", "api", "pyproject.toml"), "[project]\nname='api'\n")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "targets": ["cursor"],
  "agents": ["local-dev", "linting"],
  "languages": ["typescript", "python"],
  "paths": {
    "typescript": ["apps/frontend"],
    "python": ["services/api"]
  }
}`)

	originalEnsure := ensureInstalledFunc
	originalExec := execToolFunc
	t.Cleanup(func() {
		ensureInstalledFunc = originalEnsure
		execToolFunc = originalExec
	})

	ensureInstalledFunc = func(tool toolConfig) error { return nil }
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		return 0, nil
	}

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"install", "--target", "codex", "--all", "--yes"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	content, err := os.ReadFile(filepath.Join(root, ".rulesrc.json"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, `"cursor"`) || !strings.Contains(text, `"codex"`) {
		t.Fatalf("expected saved targets to retain existing entries and add codex, got %q", text)
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
	if got := strings.Join(invocations[2].Args, " "); got != "-m ballast install --yes --target claude --agent linting,logging,testing" {
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
		Targets:  []string{"claude"},
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

func TestRunMonorepoRemoveTargetCleansManagedRulesAndSupportFiles(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "frontend", "tsconfig.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "services", "api", "pyproject.toml"), "[project]\nname='api'\n")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "targets": ["claude", "codex"],
  "agents": ["local-dev", "linting"],
  "languages": ["typescript", "python"],
  "paths": {
    "typescript": ["apps/frontend"],
    "python": ["services/api"]
  }
}`)

	mustWriteFile(t, filepath.Join(root, ".codex", "rules", "common", "local-dev-env.md"), "managed")
	mustWriteFile(t, filepath.Join(root, ".codex", "rules", "typescript", "typescript-linting.md"), "managed")
	mustWriteFile(t, filepath.Join(root, ".claude", "rules", "common", "local-dev-env.md"), "managed")
	mustWriteFile(t, filepath.Join(root, ".claude", "rules", "typescript", "typescript-linting.md"), "managed")
	mustWriteFile(t, filepath.Join(root, "AGENTS.md"), "# AGENTS.md\n\n## Team Notes\n\nKeep this section.\n\n## Installed agent rules\n\nCreated by Ballast. Do not edit this section.\n\n- `.codex/rules/common/local-dev-env.md` — Rules for common/local-dev-env\n- `.codex/rules/typescript/typescript-linting.md` — Rules for typescript/linting\n")
	mustWriteFile(t, filepath.Join(root, "CLAUDE.md"), "# CLAUDE.md\n\n## Team Notes\n\nKeep this section.\n\n## Installed agent rules\n\nCreated by Ballast. Do not edit this section.\n\n- `.claude/rules/common/local-dev-env.md` — Rules for common/local-dev-env\n- `.claude/rules/typescript/typescript-linting.md` — Rules for typescript/linting\n")

	originalEnsure := ensureInstalledFunc
	originalExec := execToolFunc
	t.Cleanup(func() {
		ensureInstalledFunc = originalEnsure
		execToolFunc = originalExec
	})

	ensureInstalledFunc = func(tool toolConfig) error { return nil }
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		return 0, nil
	}

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"install", "--remove-target", "codex", "--yes"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if _, err := os.Stat(filepath.Join(root, ".codex", "rules", "common", "local-dev-env.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected codex common rule to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".codex", "rules", "typescript", "typescript-linting.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected codex language rule to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "rules", "common", "local-dev-env.md")); err != nil {
		t.Fatalf("expected claude rules to remain, got %v", err)
	}

	agentsMD, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if strings.Contains(string(agentsMD), ".codex/rules/") {
		t.Fatalf("expected codex installed-rules references to be removed, got %q", string(agentsMD))
	}
	if !strings.Contains(string(agentsMD), "## Team Notes") {
		t.Fatalf("expected AGENTS.md team notes to remain, got %q", string(agentsMD))
	}

	claudeMD, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(claudeMD), ".claude/rules/") {
		t.Fatalf("expected claude installed-rules references to remain, got %q", string(claudeMD))
	}

	config, err := os.ReadFile(filepath.Join(root, ".rulesrc.json"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	text := string(config)
	if strings.Contains(text, `"codex"`) || !strings.Contains(text, `"claude"`) {
		t.Fatalf("expected saved config targets to drop codex and keep claude, got %q", text)
	}
}

func TestRunMonorepoRemoveTargetDoesNotPersistConfigWhenCleanupFails(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "frontend", "tsconfig.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "services", "api", "pyproject.toml"), "[project]\nname='api'\n")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "targets": ["claude", "codex"],
  "agents": ["local-dev", "linting"],
  "languages": ["typescript", "python"],
  "paths": {
    "typescript": ["apps/frontend"],
    "python": ["services/api"]
  }
}`)

	blockingDir := filepath.Join(root, ".codex", "rules", "common", "local-dev-env.md")
	if err := os.MkdirAll(blockingDir, 0o755); err != nil {
		t.Fatalf("create blocking dir: %v", err)
	}
	mustWriteFile(t, filepath.Join(blockingDir, "child.txt"), "keep")

	originalEnsure := ensureInstalledFunc
	originalExec := execToolFunc
	t.Cleanup(func() {
		ensureInstalledFunc = originalEnsure
		execToolFunc = originalExec
	})

	ensureInstalledFunc = func(tool toolConfig) error { return nil }
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		return 0, nil
	}

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"install", "--remove-target", "codex", "--yes"})
		if exitCode == 0 {
			t.Fatal("expected cleanup failure")
		}
	})

	config, err := os.ReadFile(filepath.Join(root, ".rulesrc.json"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	text := string(config)
	if !strings.Contains(text, `"codex"`) || !strings.Contains(text, `"claude"`) {
		t.Fatalf("expected config to remain unchanged after cleanup failure, got %q", text)
	}
}

// TestRunMonorepoInstallPreservesTaskSystemWrittenByBackend asserts that a
// taskSystem value written to .rulesrc.json by a backend invocation (simulating
// what ballast-typescript does after prompting the user) is not clobbered by
// the final saveMonorepoConfig call.
func TestRunMonorepoInstallPreservesTaskSystemWrittenByBackend(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "frontend", "tsconfig.json"), "{}")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "targets": ["cursor"],
  "agents": ["tasks", "linting"],
  "languages": ["typescript"],
  "paths": {"typescript": ["apps/frontend"]}
}`)

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
		// Simulate the common backend invocation writing taskSystem to .rulesrc.json
		// after prompting the user (as ballast-typescript does).
		if callCount == 1 {
			mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "targets": ["cursor"],
  "agents": ["tasks", "linting"],
  "taskSystem": "github",
  "languages": ["typescript"],
  "paths": {"typescript": ["apps/frontend"]}
}`)
		}
		return 0, nil
	}

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"install", "--target", "cursor", "--all", "--yes"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	config, err := os.ReadFile(filepath.Join(root, ".rulesrc.json"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(config), `"taskSystem": "github"`) {
		t.Fatalf("expected taskSystem to be preserved in final .rulesrc.json, got %q", string(config))
	}
}

// TestRunMonorepoInstallPreservesTaskSystemFromExistingConfig asserts that a
// taskSystem already present in .rulesrc.json before the install runs is
// carried through into the final saved config.
func TestRunMonorepoInstallPreservesTaskSystemFromExistingConfig(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "frontend", "tsconfig.json"), "{}")
	mustWriteFile(t, filepath.Join(root, ".rulesrc.json"), `{
  "targets": ["cursor"],
  "agents": ["tasks", "linting"],
  "taskSystem": "linear",
  "languages": ["typescript"],
  "paths": {"typescript": ["apps/frontend"]}
}`)

	originalEnsure := ensureInstalledFunc
	originalExec := execToolFunc
	t.Cleanup(func() {
		ensureInstalledFunc = originalEnsure
		execToolFunc = originalExec
	})

	ensureInstalledFunc = func(tool toolConfig) error { return nil }
	execToolFunc = func(binary string, args []string, dir string, env map[string]string) (int, error) {
		return 0, nil
	}

	withWorkingDir(t, root, func() {
		exitCode := run([]string{"install", "--target", "cursor", "--all", "--yes"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	config, err := os.ReadFile(filepath.Join(root, ".rulesrc.json"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(config), `"taskSystem": "linear"`) {
		t.Fatalf("expected taskSystem to be preserved from existing config, got %q", string(config))
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

func TestPatchManagedSupportSectionsUpdatesInstalledSkills(t *testing.T) {
	existing := "# AGENTS.md\n\n## Team Notes\n\nKeep this section.\n\n## Installed agent rules\n\nCreated by Ballast. Do not edit this section.\n\n- `.codex/rules/typescript-linting.md` — Rules for typescript/linting\n\n## Installed skills\n\nCreated by Ballast. Do not edit this section.\n\n- `.codex/rules/old-skill.md` — Old skill\n"
	canonical := "# AGENTS.md\n\n## Installed agent rules\n\nCreated by Ballast. Do not edit this section.\n\n- `.codex/rules/typescript-linting.md` — Rules for typescript/linting\n\n## Installed skills\n\nCreated by Ballast. Do not edit this section.\n\n- `.codex/rules/owasp-security-scan.md` — run an OWASP-aligned security audit across Go, TypeScript, and Python projects\n"

	merged := patchManagedSupportSections(existing, canonical)
	if !strings.Contains(merged, "## Team Notes") {
		t.Fatalf("expected non-managed content to remain, got %q", merged)
	}
	if !strings.Contains(merged, "`.codex/rules/owasp-security-scan.md`") {
		t.Fatalf("expected installed skills section to be updated, got %q", merged)
	}
	if strings.Contains(merged, "`.codex/rules/old-skill.md`") {
		t.Fatalf("expected old installed skill entry to be replaced, got %q", merged)
	}
}

func TestBuildMonorepoSupportFileIncludesPublishingAndSkillsForCodex(t *testing.T) {
	plan := &monorepoPlan{
		Common:   []string{"local-dev", "publishing"},
		Language: []string{"linting"},
		Config: monorepoConfig{
			Languages: []string{"typescript"},
			Skills:    []string{"owasp-security-scan"},
		},
	}

	content := buildMonorepoSupportFile(plan, "codex")

	if !strings.Contains(content, "## Repository Facts") {
		t.Fatalf("expected repository facts section in codex support file, got %q", content)
	}
	if !strings.Contains(content, "Canonical GitHub repo: `<OWNER/REPO>`") {
		t.Fatalf("expected repository facts scaffold in codex support file, got %q", content)
	}
	if !strings.Contains(content, "`.codex/rules/common/publishing-libraries.md`") {
		t.Fatalf("expected publishing libraries rule in codex support file, got %q", content)
	}
	if !strings.Contains(content, "`.codex/rules/common/publishing-sdks.md`") {
		t.Fatalf("expected publishing sdks rule in codex support file, got %q", content)
	}
	if !strings.Contains(content, "`.codex/rules/common/publishing-apps.md`") {
		t.Fatalf("expected publishing apps rule in codex support file, got %q", content)
	}
	if !strings.Contains(content, "## Installed skills") {
		t.Fatalf("expected installed skills section in codex support file, got %q", content)
	}
	if !strings.Contains(content, "`.codex/rules/owasp-security-scan.md`") {
		t.Fatalf("expected codex skill entry in support file, got %q", content)
	}
}

func TestBuildMonorepoSupportFileIncludesSkillsForClaude(t *testing.T) {
	plan := &monorepoPlan{
		Common:   []string{"local-dev", "publishing"},
		Language: []string{"linting"},
		Config: monorepoConfig{
			Languages: []string{"typescript"},
			Skills:    []string{"owasp-security-scan"},
		},
	}

	content := buildMonorepoSupportFile(plan, "claude")

	if !strings.Contains(content, "## Installed skills") {
		t.Fatalf("expected installed skills section in claude support file, got %q", content)
	}
	if !strings.Contains(content, "`.claude/skills/owasp-security-scan.skill`") {
		t.Fatalf("expected claude skill entry in support file, got %q", content)
	}
}

func TestBuildMonorepoSupportFileIncludesDocsForCodex(t *testing.T) {
	plan := &monorepoPlan{
		Common:   []string{"docs"},
		Language: []string{"linting"},
		Config: monorepoConfig{
			Languages: []string{"typescript"},
		},
	}

	content := buildMonorepoSupportFile(plan, "codex")

	if !strings.Contains(content, "`.codex/rules/common/docs.md`") {
		t.Fatalf("expected docs rule entry in codex support file, got %q", content)
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

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	originalStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}

	os.Stderr = writer
	t.Cleanup(func() {
		os.Stderr = originalStderr
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
		t.Fatalf("read stderr: %v", copyErr)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}

	return buf.String()
}

func TestRegistryConsistency(t *testing.T) {
	// Every agentRegistry entry must have a non-empty ID.
	for _, e := range agentRegistry {
		if e.ID == "" {
			t.Error("agentRegistry contains an entry with an empty ID")
		}
	}
	// Every skillRegistry entry must have a non-empty ID and description.
	for _, e := range skillRegistry {
		if e.ID == "" {
			t.Error("skillRegistry contains an entry with an empty ID")
		}
		if e.Description == "" {
			t.Errorf("skillRegistry entry %q has no description", e.ID)
		}
	}
	// No duplicate IDs in agentRegistry.
	seen := map[string]bool{}
	for _, e := range agentRegistry {
		if seen[e.ID] {
			t.Errorf("agentRegistry has duplicate ID %q", e.ID)
		}
		seen[e.ID] = true
	}
	// No duplicate IDs in skillRegistry.
	seen = map[string]bool{}
	for _, e := range skillRegistry {
		if seen[e.ID] {
			t.Errorf("skillRegistry has duplicate ID %q", e.ID)
		}
		seen[e.ID] = true
	}
	// Deprecated entries must have a non-nil Deprecated field.
	for _, e := range agentRegistry {
		if e.Status == statusDeprecated && e.Deprecated == nil {
			t.Errorf("agent %q is deprecated but has no deprecation info", e.ID)
		}
	}
	for _, e := range skillRegistry {
		if e.Status == statusDeprecated && e.Deprecated == nil {
			t.Errorf("skill %q is deprecated but has no deprecation info", e.ID)
		}
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setUserSkillHomes(t *testing.T) (claudeDir, codexDir string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir = filepath.Join(home, "claude-config")
	codexDir = filepath.Join(home, "codex-home")
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("CODEX_HOME", codexDir)
	original := version
	t.Cleanup(func() { version = original })
	version = "9.9.9"
	return claudeDir, codexDir
}

func userSkillFile(base string) string {
	return filepath.Join(base, "skills", userSkillName, "SKILL.md")
}

func TestUserSkillInstallWritesClaudeAndCodexByDefault(t *testing.T) {
	claudeDir, codexDir := setUserSkillHomes(t)

	output := captureStdout(t, func() {
		if code := run([]string{"user-skill", "install"}); code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})

	for _, path := range []string{userSkillFile(claudeDir), userSkillFile(codexDir)} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected %s to be written: %v", path, err)
		}
		content := string(data)
		if !strings.HasPrefix(content, "---\nname: ballast\n") {
			t.Fatalf("expected skill front matter in %s, got %q", path, content[:40])
		}
		if !strings.Contains(content, "ballast-user-skill version=9.9.9") {
			t.Fatalf("expected version marker in %s", path)
		}
		if strings.Contains(content, "{{version}}") {
			t.Fatalf("expected version placeholder to be rendered in %s", path)
		}
		if !strings.Contains(output, path) {
			t.Fatalf("expected output to name %s, got %q", path, output)
		}
	}
}

func TestUserSkillDefaultsToInstall(t *testing.T) {
	claudeDir, _ := setUserSkillHomes(t)
	captureStdout(t, func() {
		if code := run([]string{"user-skill"}); code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})
	if _, err := os.Stat(userSkillFile(claudeDir)); err != nil {
		t.Fatalf("expected default action to install: %v", err)
	}
}

func TestUserSkillInstallFallsBackToHomeDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	captureStdout(t, func() {
		if code := run([]string{"user-skill", "install"}); code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})
	for _, dir := range []string{".claude", ".codex"} {
		if _, err := os.Stat(userSkillFile(filepath.Join(home, dir))); err != nil {
			t.Fatalf("expected skill under ~/%s: %v", dir, err)
		}
	}
}

func TestUserSkillInstallSingleTarget(t *testing.T) {
	claudeDir, codexDir := setUserSkillHomes(t)
	captureStdout(t, func() {
		if code := run([]string{"user-skill", "install", "--target", "codex"}); code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})
	if _, err := os.Stat(userSkillFile(codexDir)); err != nil {
		t.Fatalf("expected codex skill: %v", err)
	}
	if _, err := os.Stat(userSkillFile(claudeDir)); !os.IsNotExist(err) {
		t.Fatalf("expected no claude skill, got err=%v", err)
	}
}

func TestUserSkillRejectsUnknownTargetAndAction(t *testing.T) {
	setUserSkillHomes(t)
	cases := [][]string{
		{"user-skill", "install", "--target", "cursor"},
		{"user-skill", "frobnicate"},
		{"user-skill", "install", "--bogus"},
		{"user-skill", "install", "--target"},
	}
	for _, args := range cases {
		captureStdout(t, func() {
			if code := run(args); code == 0 {
				t.Fatalf("expected non-zero exit for %v", args)
			}
		})
	}
}

func TestUserSkillInstallRefusesToOverwriteUnmanagedSkill(t *testing.T) {
	claudeDir, _ := setUserSkillHomes(t)
	path := userSkillFile(claudeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nname: ballast\n---\nmine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		if code := run([]string{"user-skill", "install", "--target", "claude"}); code == 0 {
			t.Fatal("expected non-zero exit when an unmanaged skill exists")
		}
	})
	if !strings.Contains(output, "--force") {
		t.Fatalf("expected --force hint, got %q", output)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "---\nname: ballast\n---\nmine\n" {
		t.Fatal("expected unmanaged skill to be left untouched")
	}

	captureStdout(t, func() {
		if code := run([]string{"user-skill", "install", "--target", "claude", "--force"}); code != 0 {
			t.Fatalf("expected --force to overwrite, got exit %d", code)
		}
	})
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), userSkillMarker) {
		t.Fatal("expected --force to write the managed skill")
	}
}

func TestUserSkillReinstallOverwritesManagedSkill(t *testing.T) {
	claudeDir, _ := setUserSkillHomes(t)
	captureStdout(t, func() { run([]string{"user-skill", "install", "--target", "claude"}) })
	version = "10.0.0"
	captureStdout(t, func() {
		if code := run([]string{"user-skill", "install", "--target", "claude"}); code != 0 {
			t.Fatalf("expected reinstall to succeed, got %d", code)
		}
	})
	data, _ := os.ReadFile(userSkillFile(claudeDir))
	if !strings.Contains(string(data), "version=10.0.0") {
		t.Fatal("expected reinstall to refresh the version marker")
	}
}

func TestUserSkillStatusReportsEachTarget(t *testing.T) {
	claudeDir, codexDir := setUserSkillHomes(t)
	captureStdout(t, func() { run([]string{"user-skill", "install", "--target", "claude"}) })
	version = "10.0.0"

	output := captureStdout(t, func() {
		if code := run([]string{"user-skill", "status"}); code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})
	if !strings.Contains(output, "claude: installed 9.9.9 (outdated, running 10.0.0)") {
		t.Fatalf("expected outdated claude status, got %q", output)
	}
	if !strings.Contains(output, "codex: not installed") {
		t.Fatalf("expected codex not installed, got %q", output)
	}
	if !strings.Contains(output, claudeDir) || !strings.Contains(output, codexDir) {
		t.Fatalf("expected status to name both paths, got %q", output)
	}
}

func TestUserSkillRemoveDeletesOnlyManagedSkill(t *testing.T) {
	claudeDir, codexDir := setUserSkillHomes(t)
	captureStdout(t, func() { run([]string{"user-skill", "install", "--target", "claude"}) })

	foreign := userSkillFile(codexDir)
	if err := os.MkdirAll(filepath.Dir(foreign), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		if code := run([]string{"user-skill", "remove"}); code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})
	if _, err := os.Stat(filepath.Dir(userSkillFile(claudeDir))); !os.IsNotExist(err) {
		t.Fatalf("expected managed claude skill dir removed, err=%v", err)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("expected unmanaged codex skill kept: %v", err)
	}
	if !strings.Contains(output, "not managed by ballast") {
		t.Fatalf("expected skip message, got %q", output)
	}
}

func TestUsageMentionsUserSkill(t *testing.T) {
	output := captureStdout(t, func() { printUsage() })
	if !strings.Contains(output, "user-skill") {
		t.Fatalf("expected usage to document user-skill, got %q", output)
	}
}

func TestDoctorReportsUserSkillStatus(t *testing.T) {
	claudeDir, codexDir := setUserSkillHomes(t)
	captureStdout(t, func() { run([]string{"user-skill", "install", "--target", "claude"}) })
	version = "10.0.0"

	output := captureStdout(t, func() { printDoctorUserSkill() })

	if !strings.Contains(output, "User skill (ballast):") {
		t.Fatalf("expected user skill section, got %q", output)
	}
	if !strings.Contains(output, "- claude: installed 9.9.9 (outdated, running 10.0.0)") {
		t.Fatalf("expected outdated claude line, got %q", output)
	}
	if !strings.Contains(output, "- codex: not installed") {
		t.Fatalf("expected codex not installed line, got %q", output)
	}
	if !strings.Contains(output, claudeDir) || !strings.Contains(output, codexDir) {
		t.Fatalf("expected both paths, got %q", output)
	}
	if !strings.Contains(output, "Install the ballast user skill: ballast user-skill install --target codex") {
		t.Fatalf("expected install hint for codex, got %q", output)
	}
	if !strings.Contains(output, "Update the ballast user skill: ballast user-skill install --target claude") {
		t.Fatalf("expected update hint for claude, got %q", output)
	}
}

func TestDoctorUserSkillHintsCombineTargets(t *testing.T) {
	setUserSkillHomes(t)
	output := captureStdout(t, func() { printDoctorUserSkill() })
	if !strings.Contains(output, "Install the ballast user skill: ballast user-skill install --target claude,codex") {
		t.Fatalf("expected one combined install hint, got %q", output)
	}
	if strings.Contains(output, "Update the ballast user skill") {
		t.Fatalf("expected no update hint when nothing is installed, got %q", output)
	}
}

func TestDoctorUserSkillDoesNotHintForUnmanagedSkill(t *testing.T) {
	claudeDir, _ := setUserSkillHomes(t)
	captureStdout(t, func() { run([]string{"user-skill", "install", "--target", "codex"}) })
	path := userSkillFile(claudeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() { printDoctorUserSkill() })
	if strings.Contains(output, "Install the ballast user skill") || strings.Contains(output, "Update the ballast user skill") {
		t.Fatalf("expected no hint for a user's own ballast skill, got %q", output)
	}
	if !strings.Contains(output, "present but not managed by ballast") {
		t.Fatalf("expected unmanaged status, got %q", output)
	}
}

func TestDoctorUserSkillCurrentHasNoHint(t *testing.T) {
	setUserSkillHomes(t)
	captureStdout(t, func() { run([]string{"user-skill", "install"}) })

	output := captureStdout(t, func() { printDoctorUserSkill() })

	if strings.Contains(output, "the ballast user skill:") {
		t.Fatalf("expected no hint when every copy is current, got %q", output)
	}
	if strings.Count(output, "installed 9.9.9") != 2 {
		t.Fatalf("expected both targets current, got %q", output)
	}
}

func TestRunDoctorIncludesUserSkillSection(t *testing.T) {
	setUserSkillHomes(t)
	t.Chdir(t.TempDir())
	output := captureStdout(t, func() { run([]string{"doctor"}) })
	if !strings.Contains(output, "User skill (ballast):") {
		t.Fatalf("expected doctor to report the user skill, got %q", output)
	}
}

// Bosun review on #427, finding 1: remove must only delete the managed file.
func TestUserSkillRemoveKeepsAdjacentUserFiles(t *testing.T) {
	claudeDir, _ := setUserSkillHomes(t)
	captureStdout(t, func() { run([]string{"user-skill", "install", "--target", "claude"}) })
	extra := filepath.Join(filepath.Dir(userSkillFile(claudeDir)), "references", "notes.md")
	if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(extra, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	captureStdout(t, func() {
		if code := run([]string{"user-skill", "remove", "--target", "claude"}); code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})

	if _, err := os.Stat(userSkillFile(claudeDir)); !os.IsNotExist(err) {
		t.Fatalf("expected managed SKILL.md removed, err=%v", err)
	}
	if _, err := os.Stat(extra); err != nil {
		t.Fatalf("expected user file next to the skill to be kept: %v", err)
	}
}

// Bosun review on #427, finding 2: a newer installed skill is not outdated.
func TestUserSkillNewerInstallIsNotReportedOutdated(t *testing.T) {
	setUserSkillHomes(t)
	version = "10.0.0"
	captureStdout(t, func() { run([]string{"user-skill", "install"}) })
	version = "9.9.9"

	status := captureStdout(t, func() { run([]string{"user-skill", "status"}) })
	if !strings.Contains(status, "claude: installed 10.0.0 (newer than running 9.9.9)") {
		t.Fatalf("expected newer status, got %q", status)
	}
	if strings.Contains(status, "outdated") {
		t.Fatalf("expected no outdated label for a newer skill, got %q", status)
	}

	doctor := captureStdout(t, func() { printDoctorUserSkill() })
	if strings.Contains(doctor, "Update the ballast user skill") {
		t.Fatalf("expected no update hint that would downgrade, got %q", doctor)
	}
}

func TestUserSkillVersionComparisonIgnoresVPrefix(t *testing.T) {
	setUserSkillHomes(t)
	captureStdout(t, func() { run([]string{"user-skill", "install"}) })
	version = "v9.9.9"
	doctor := captureStdout(t, func() { printDoctorUserSkill() })
	if strings.Contains(doctor, "Update the ballast user skill") || strings.Contains(doctor, "outdated") {
		t.Fatalf("expected v-prefixed running version to match, got %q", doctor)
	}
}

func TestUserSkillFailsWithoutHomeDirectory(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	output := captureStdout(t, func() {
		if code := run([]string{"user-skill", "status"}); code == 0 {
			t.Fatal("expected non-zero exit without a home directory")
		}
	})
	if !strings.Contains(output, "cannot resolve home directory") {
		t.Fatalf("expected home directory error, got %q", output)
	}
	doctor := captureStdout(t, func() { printDoctorUserSkill() })
	if !strings.Contains(doctor, "- claude: cannot resolve home directory") {
		t.Fatalf("expected doctor to report the home directory error, got %q", doctor)
	}
}

func TestUserSkillReportsUnreadableSkill(t *testing.T) {
	claudeDir, _ := setUserSkillHomes(t)
	// A directory where SKILL.md should be makes the read fail.
	if err := os.MkdirAll(userSkillFile(claudeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() {
		if code := run([]string{"user-skill", "status", "--target", "claude"}); code == 0 {
			t.Fatal("expected non-zero exit for an unreadable skill")
		}
	})
	if !strings.Contains(output, "claude: cannot read") {
		t.Fatalf("expected read error, got %q", output)
	}
	doctor := captureStdout(t, func() { printDoctorUserSkill() })
	if !strings.Contains(doctor, "- claude: unreadable") {
		t.Fatalf("expected doctor unreadable line, got %q", doctor)
	}
}

func TestUserSkillAcceptsTargetEqualsForm(t *testing.T) {
	claudeDir, codexDir := setUserSkillHomes(t)
	captureStdout(t, func() {
		if code := run([]string{"user-skill", "install", "--target=codex,codex"}); code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})
	if _, err := os.Stat(userSkillFile(codexDir)); err != nil {
		t.Fatalf("expected codex skill: %v", err)
	}
	if _, err := os.Stat(userSkillFile(claudeDir)); !os.IsNotExist(err) {
		t.Fatalf("expected no claude skill, err=%v", err)
	}
}

func TestUserSkillInstallReportsCreateFailure(t *testing.T) {
	claudeDir, _ := setUserSkillHomes(t)
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A file named "skills" blocks creating the skill directory.
	if err := os.WriteFile(filepath.Join(claudeDir, "skills"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() {
		if code := run([]string{"user-skill", "install", "--target", "claude"}); code == 0 {
			t.Fatal("expected non-zero exit when the directory cannot be created")
		}
	})
	if !strings.Contains(output, "claude: failed to create") {
		t.Fatalf("expected create failure, got %q", output)
	}
}

func TestUserSkillInstallAndRemoveReportPermissionFailures(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	claudeDir, _ := setUserSkillHomes(t)
	captureStdout(t, func() { run([]string{"user-skill", "install", "--target", "claude"}) })
	path := userSkillFile(claudeDir)
	dir := filepath.Dir(path)

	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() {
		if code := run([]string{"user-skill", "install", "--target", "claude"}); code == 0 {
			t.Fatal("expected non-zero exit when the skill cannot be written")
		}
	})
	if !strings.Contains(output, "claude: failed to write") {
		t.Fatalf("expected write failure, got %q", output)
	}

	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	output = captureStdout(t, func() {
		if code := run([]string{"user-skill", "remove", "--target", "claude"}); code == 0 {
			t.Fatal("expected non-zero exit when the skill cannot be removed")
		}
	})
	if !strings.Contains(output, "claude: failed to remove") {
		t.Fatalf("expected remove failure, got %q", output)
	}
}

func TestUserSkillRemoveWhenNotInstalled(t *testing.T) {
	setUserSkillHomes(t)
	output := captureStdout(t, func() {
		if code := run([]string{"user-skill", "remove", "--target", "codex"}); code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})
	if !strings.Contains(output, "codex: not installed") {
		t.Fatalf("expected not installed, got %q", output)
	}
}

func TestUserSkillManagedWithoutVersionNeedsUpdate(t *testing.T) {
	claudeDir, _ := setUserSkillHomes(t)
	path := userSkillFile(claudeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(userSkillMarker+"-->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	doctor := captureStdout(t, func() { printDoctorUserSkill() })
	if !strings.Contains(doctor, "- claude: installed (unknown version)") {
		t.Fatalf("expected unknown version status, got %q", doctor)
	}
	if !strings.Contains(doctor, "Update the ballast user skill: ballast user-skill install --target claude") {
		t.Fatalf("expected update hint for an unversioned managed skill, got %q", doctor)
	}
}

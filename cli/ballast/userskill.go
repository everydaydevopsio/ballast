package main

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// The user skill is installed into the user's agent config, not a repository,
// so it can install Ballast into repositories that do not use it yet. It is a
// wrapper-only feature: no language backend writes user-scope files.

const (
	userSkillName    = "ballast"
	userSkillMarker  = "<!-- ballast-user-skill "
	userSkillVersion = "{{version}}"
)

//go:embed userskills/ballast/SKILL.md
var userSkillContent string

var userSkillTargets = []string{"claude", "codex"}

var userSkillVersionPattern = regexp.MustCompile(`<!-- ballast-user-skill version=(\S+?):`)

// userSkillBaseDir honors each agent's config-dir override so the skill lands
// where the agent actually reads it.
func userSkillBaseDir(target string) (string, error) {
	envVar, dir := "CLAUDE_CONFIG_DIR", ".claude"
	if target == "codex" {
		envVar, dir = "CODEX_HOME", ".codex"
	}
	if override := strings.TrimSpace(os.Getenv(envVar)); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve home directory: %w", err)
	}
	return filepath.Join(home, dir), nil
}

func userSkillPath(target string) (string, error) {
	base, err := userSkillBaseDir(target)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "skills", userSkillName, "SKILL.md"), nil
}

func renderUserSkill() string {
	return strings.Replace(userSkillContent, userSkillVersion, resolveVersion(), 1)
}

// readUserSkill reports whether a skill exists at path, whether Ballast owns
// it, and the version it was written by.
func readUserSkill(path string) (exists, managed bool, installedVersion string, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, false, "", nil
	}
	if err != nil {
		return false, false, "", err
	}
	content := string(data)
	if !strings.Contains(content, userSkillMarker) {
		return true, false, "", nil
	}
	if match := userSkillVersionPattern.FindStringSubmatch(content); match != nil {
		installedVersion = match[1]
	}
	return true, true, installedVersion, nil
}

func parseUserSkillArgs(args []string) (action string, targets []string, force bool, err error) {
	action = "install"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		action = args[0]
		args = args[1:]
	}
	if !slices.Contains([]string{"install", "remove", "status"}, action) {
		return "", nil, false, fmt.Errorf("unknown user-skill action: %s (expected install, remove, or status)", action)
	}
	targets = userSkillTargets
	for index := 0; index < len(args); index++ {
		arg := args[index]
		var value string
		switch {
		case arg == "--force" && action == "install":
			force = true
			continue
		case arg == "--target" || arg == "-t":
			if index+1 >= len(args) {
				return "", nil, false, fmt.Errorf("%s requires a value (%s)", arg, strings.Join(userSkillTargets, ", "))
			}
			index++
			value = args[index]
		case strings.HasPrefix(arg, "--target="):
			value = strings.TrimPrefix(arg, "--target=")
		default:
			return "", nil, false, fmt.Errorf("unknown user-skill option: %s", arg)
		}
		targets = nil
		for _, raw := range strings.Split(value, ",") {
			target := strings.ToLower(strings.TrimSpace(raw))
			if !slices.Contains(userSkillTargets, target) {
				return "", nil, false, fmt.Errorf("unsupported user-skill target: %q (supported: %s)", raw, strings.Join(userSkillTargets, ", "))
			}
			if !slices.Contains(targets, target) {
				targets = append(targets, target)
			}
		}
	}
	return action, targets, force, nil
}

func runUserSkill(args []string) int {
	action, targets, force, err := parseUserSkillArgs(args)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Usage: ballast user-skill [install|remove|status] [--target claude,codex] [--force]")
		return 1
	}

	failed := false
	for _, target := range targets {
		path, err := userSkillPath(target)
		if err != nil {
			fmt.Printf("%s: %v\n", target, err)
			failed = true
			continue
		}
		exists, managed, installedVersion, err := readUserSkill(path)
		if err != nil {
			fmt.Printf("%s: cannot read %s: %v\n", target, path, err)
			failed = true
			continue
		}

		switch action {
		case "status":
			fmt.Printf("%s: %s (%s)\n", target, userSkillStatus(exists, managed, installedVersion), path)
		case "remove":
			if !exists {
				fmt.Printf("%s: not installed (%s)\n", target, path)
				continue
			}
			if !managed {
				fmt.Printf("%s: skipped %s: not managed by ballast\n", target, path)
				continue
			}
			if err := os.RemoveAll(filepath.Dir(path)); err != nil {
				fmt.Printf("%s: failed to remove %s: %v\n", target, path, err)
				failed = true
				continue
			}
			fmt.Printf("%s: removed %s\n", target, path)
		case "install":
			if exists && !managed && !force {
				fmt.Printf("%s: %s exists and is not managed by ballast; rerun with --force to replace it\n", target, path)
				failed = true
				continue
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				fmt.Printf("%s: failed to create %s: %v\n", target, filepath.Dir(path), err)
				failed = true
				continue
			}
			if err := os.WriteFile(path, []byte(renderUserSkill()), 0o644); err != nil {
				fmt.Printf("%s: failed to write %s: %v\n", target, path, err)
				failed = true
				continue
			}
			fmt.Printf("%s: installed %s skill %s at %s\n", target, userSkillName, resolveVersion(), path)
		}
	}

	if action == "install" && !failed {
		fmt.Println("Restart Claude Code or Codex to load it, then ask: \"install ballast in this repo\", \"run a ballast health check\", or use /ballast.")
	}
	if failed {
		return 1
	}
	return 0
}

func userSkillStatus(exists, managed bool, installedVersion string) string {
	switch {
	case !exists:
		return "not installed"
	case !managed:
		return "present but not managed by ballast"
	case installedVersion == "":
		return "installed (unknown version)"
	case installedVersion != resolveVersion():
		return fmt.Sprintf("installed %s (outdated, running %s)", installedVersion, resolveVersion())
	default:
		return "installed " + installedVersion
	}
}

// printDoctorUserSkill reports the user-level skill for each agent. It is
// informational: the skill lives outside the repository, so doctor never
// treats a missing copy as a repository problem, and --fix does not write to
// the user's agent config.
func printDoctorUserSkill() {
	fmt.Printf("User skill (%s):\n", userSkillName)
	var missing, outdated []string
	for _, target := range userSkillTargets {
		path, err := userSkillPath(target)
		if err != nil {
			fmt.Printf("- %s: %v\n", target, err)
			continue
		}
		exists, managed, installedVersion, err := readUserSkill(path)
		if err != nil {
			fmt.Printf("- %s: unreadable (%v)\n", target, err)
			continue
		}
		fmt.Printf("- %s: %s (%s)\n", target, userSkillStatus(exists, managed, installedVersion), path)
		switch {
		case !exists:
			missing = append(missing, target)
		case managed && installedVersion != resolveVersion():
			outdated = append(outdated, target)
		}
	}
	if len(missing) > 0 {
		fmt.Printf("Install the ballast user skill: ballast user-skill install --target %s\n", strings.Join(missing, ","))
	}
	if len(outdated) > 0 {
		fmt.Printf("Update the ballast user skill: ballast user-skill install --target %s\n", strings.Join(outdated, ","))
	}
	fmt.Println()
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var targets = map[string]bool{"cursor": true, "claude": true, "opencode": true, "codex": true}
var agents = map[string]bool{"linting": true, "logging": true, "testing": true, "local-dev": true, "cicd": true, "observability": true}

const configFile = ".rulesrc.go.json"

type config struct {
	Target string   `json:"target"`
	Agents []string `json:"agents"`
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 || args[0] == "install" {
		return install(args)
	}
	fmt.Printf("Unknown command: %s\n", args[0])
	return 1
}

func install(args []string) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	target := fs.String("target", "", "cursor|claude|opencode|codex")
	fs.StringVar(target, "t", "", "cursor|claude|opencode|codex")
	agent := fs.String("agent", "", "comma-separated list")
	fs.StringVar(agent, "a", "", "comma-separated list")
	all := fs.Bool("all", false, "install all agents")
	force := fs.Bool("force", false, "overwrite files")
	fs.Bool("yes", false, "non-interactive mode")
	fs.Bool("y", false, "non-interactive mode")
	if err := fs.Parse(trimCommand(args)); err != nil {
		return 1
	}

	if !targets[*target] {
		fmt.Println("Invalid --target. Use: cursor, claude, opencode, codex")
		return 1
	}

	selected := resolveAgents(*agent, *all)
	if len(selected) == 0 {
		fmt.Println("No valid agents selected. Use --agent or --all")
		return 1
	}

	root, err := projectRoot()
	if err != nil {
		fmt.Println(err)
		return 1
	}

	if err := saveConfig(root, *target, selected); err != nil {
		fmt.Println(err)
		return 1
	}

	for _, a := range selected {
		dst := destination(root, *target, a)
		if _, err := os.Stat(dst); err == nil && !*force {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			fmt.Println(err)
			return 1
		}
		if err := os.WriteFile(dst, []byte(buildContent(a)), 0o644); err != nil {
			fmt.Println(err)
			return 1
		}
	}

	if *target == "codex" {
		agentsPath := filepath.Join(root, "AGENTS.md")
		if _, err := os.Stat(agentsPath); os.IsNotExist(err) || *force {
			lines := []string{"# AGENTS.md", "", "Installed by ballast-go.", "", "## Installed rules", ""}
			for _, a := range selected {
				lines = append(lines, fmt.Sprintf("- `.codex/rules/%s.md`", a))
			}
			content := strings.Join(lines, "\n") + "\n"
			if err := os.WriteFile(agentsPath, []byte(content), 0o644); err != nil {
				fmt.Println(err)
				return 1
			}
		}
	}

	return 0
}

func trimCommand(args []string) []string {
	if len(args) > 0 && args[0] == "install" {
		return args[1:]
	}
	return args
}

func resolveAgents(raw string, all bool) []string {
	if all {
		return sortedAgents()
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "all" {
			return sortedAgents()
		}
		if agents[v] {
			out = append(out, v)
		}
	}
	return out
}

func sortedAgents() []string {
	out := make([]string, 0, len(agents))
	for a := range agents {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

func projectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if exists(filepath.Join(dir, "package.json")) || exists(filepath.Join(dir, configFile)) {
			return dir, nil
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	return cwd, nil
}

func saveConfig(root, target string, selected []string) error {
	data, err := json.MarshalIndent(config{Target: target, Agents: selected}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, configFile), data, 0o644)
}

func destination(root, target, agent string) string {
	switch target {
	case "cursor":
		return filepath.Join(root, ".cursor", "rules", agent+".mdc")
	case "claude":
		return filepath.Join(root, ".claude", "rules", agent+".md")
	case "opencode":
		return filepath.Join(root, ".opencode", agent+".md")
	default:
		return filepath.Join(root, ".codex", "rules", agent+".md")
	}
}

func buildContent(agent string) string {
	return fmt.Sprintf("# Go %s Rules\n\nInstalled by ballast-go.\n", agent)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

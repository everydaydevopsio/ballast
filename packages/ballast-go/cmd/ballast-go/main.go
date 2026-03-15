package main

import (
	"bufio"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var targets = []string{"cursor", "claude", "opencode", "codex"}
var languages = []string{"typescript", "python", "go"}

var commonAgents = []string{"local-dev", "cicd", "observability"}
var languageAgents = []string{"linting", "logging", "testing"}

var descriptionRegex = regexp.MustCompile(`(?m)^description:\s*['\"]?(.+?)['\"]?\s*$`)

//go:embed agents/**
var embeddedAgentsFS embed.FS

type rulesConfig struct {
	Target string   `json:"target"`
	Agents []string `json:"agents"`
}

type installResult struct {
	installed           []string
	installedRules      []installedRule
	installedSupport    []string
	skipped             []string
	skippedSupportFiles []string
	errors              []agentError
}

type installedRule struct {
	agentID    string
	ruleSuffix string
}

type agentError struct {
	agent string
	err   string
}

type resolveOptions struct {
	projectRoot string
	target      string
	agents      []string
	all         bool
	yes         bool
	language    string
}

type installOptions struct {
	projectRoot string
	target      string
	agents      []string
	language    string
	force       bool
	saveConfig  bool
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 || args[0] == "install" {
		return runInstall(args)
	}
	fmt.Printf("Unknown command: %s\n", args[0])
	fmt.Println("Run ballast-go --help for usage.")
	return 1
}

func runInstall(args []string) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	target := fs.String("target", "", "cursor|claude|opencode|codex")
	fs.StringVar(target, "t", "", "cursor|claude|opencode|codex")
	language := fs.String("language", "go", "typescript|python|go")
	fs.StringVar(language, "l", "go", "typescript|python|go")
	agent := fs.String("agent", "", "comma-separated list")
	fs.StringVar(agent, "a", "", "comma-separated list")
	all := fs.Bool("all", false, "install all agents")
	force := fs.Bool("force", false, "overwrite files")
	yes := fs.Bool("yes", false, "non-interactive mode")
	fs.BoolVar(yes, "y", false, "non-interactive mode")
	if err := fs.Parse(trimCommand(args)); err != nil {
		return 1
	}

	if err := validateRepoRootOverride(); err != nil {
		fmt.Println(err)
		return 1
	}

	lang := strings.ToLower(strings.TrimSpace(*language))
	if !contains(languages, lang) {
		fmt.Printf("Invalid --language. Use: %s\n", strings.Join(languages, ", "))
		return 1
	}

	root, err := findProjectRoot("")
	if err != nil {
		fmt.Println(err)
		return 1
	}

	resolved, err := resolveTargetAndAgents(resolveOptions{
		projectRoot: root,
		target:      strings.ToLower(strings.TrimSpace(*target)),
		agents:      splitAgents(*agent),
		all:         *all,
		yes:         *yes,
		language:    lang,
	})
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if resolved == nil {
		fmt.Println("In CI/non-interactive mode (--yes or CI env), --target and --agent (or --all) are required when config is missing.")
		fmt.Println("Example: ballast-go install --yes --target cursor --agent linting")
		return 1
	}

	persist := strings.TrimSpace(*target) == "" && strings.TrimSpace(*agent) == "" && !*all
	result := install(installOptions{
		projectRoot: root,
		target:      resolved.Target,
		agents:      resolved.Agents,
		language:    lang,
		force:       *force,
		saveConfig:  persist,
	})

	if len(result.errors) > 0 {
		for _, item := range result.errors {
			fmt.Printf("Error installing %s: %s\n", item.agent, item.err)
		}
		return 1
	}

	if len(result.installedRules) > 0 {
		fmt.Printf("Installed for %s: %s\n", resolved.Target, strings.Join(result.installed, ", "))
		for _, rule := range result.installedRules {
			base := rule.agentID
			if rule.ruleSuffix != "" {
				base = base + "-" + rule.ruleSuffix
			}
			_, file := destination(root, resolved.Target, base)
			fmt.Printf("  %s -> %s\n", base, file)
		}
	}
	if len(result.installedSupport) > 0 {
		for _, file := range result.installedSupport {
			fmt.Printf("  AGENTS.md -> %s\n", file)
		}
	}
	if len(result.skipped) > 0 {
		fmt.Printf("Skipped (already present; use --force to overwrite): %s\n", strings.Join(result.skipped, ", "))
	}
	if len(result.skippedSupportFiles) > 0 {
		fmt.Printf(
			"Skipped support files (already present; use --force to overwrite): %s\n",
			strings.Join(result.skippedSupportFiles, ", "),
		)
	}
	if len(result.installed) == 0 && len(result.skipped) == 0 && len(result.errors) == 0 {
		fmt.Println("Nothing to install.")
	}

	return 0
}

func resolveTargetAndAgents(opts resolveOptions) (*rulesConfig, error) {
	config := loadConfig(opts.projectRoot, opts.language)
	ci := isCIMode() || opts.yes

	flagAgents := opts.agents
	if opts.all {
		flagAgents = []string{"all"}
	}

	if config != nil && opts.target == "" && len(flagAgents) == 0 {
		return config, nil
	}

	target := opts.target
	if target == "" && config != nil {
		target = config.Target
	}

	var resolvedAgents []string
	if len(flagAgents) > 0 {
		resolvedAgents = resolveAgents(flagAgents, opts.language)
	} else if config != nil {
		resolvedAgents = config.Agents
	}

	if target != "" && len(resolvedAgents) > 0 {
		return &rulesConfig{Target: target, Agents: resolvedAgents}, nil
	}

	if ci {
		return nil, nil
	}

	if target == "" {
		var err error
		target, err = promptTarget()
		if err != nil {
			return nil, err
		}
	}
	if len(resolvedAgents) == 0 {
		var err error
		resolvedAgents, err = promptAgents(opts.language)
		if err != nil {
			return nil, err
		}
	}

	return &rulesConfig{Target: target, Agents: resolvedAgents}, nil
}

func install(opts installOptions) installResult {
	result := installResult{}
	processed := map[string]struct{}{}

	if opts.saveConfig {
		if err := saveConfig(opts.projectRoot, opts.language, rulesConfig{Target: opts.target, Agents: opts.agents}); err != nil {
			result.errors = append(result.errors, agentError{agent: "config", err: err.Error()})
			return result
		}
	}

	for _, agentID := range opts.agents {
		if !isValidAgent(agentID, opts.language) {
			result.errors = append(result.errors, agentError{agent: agentID, err: "Unknown agent"})
			continue
		}

		suffixes, err := listRuleSuffixes(agentID, opts.language)
		if err != nil {
			result.errors = append(result.errors, agentError{agent: agentID, err: err.Error()})
			continue
		}

		agentInstalled := false
		agentSkipped := false
		agentProcessed := false
		for _, suffix := range suffixes {
			base := agentID
			if suffix != "" {
				base = agentID + "-" + suffix
			}
			dir, file := destination(opts.projectRoot, opts.target, base)
			if exists(file) && !opts.force {
				agentSkipped = true
				agentProcessed = true
				continue
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				result.errors = append(result.errors, agentError{agent: agentID, err: err.Error()})
				continue
			}
			content, err := buildContent(agentID, opts.target, opts.language, suffix)
			if err != nil {
				result.errors = append(result.errors, agentError{agent: agentID, err: err.Error()})
				continue
			}
			if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
				result.errors = append(result.errors, agentError{agent: agentID, err: err.Error()})
				continue
			}
			result.installedRules = append(result.installedRules, installedRule{agentID: agentID, ruleSuffix: suffix})
			agentInstalled = true
			agentProcessed = true
		}
		if agentProcessed {
			processed[agentID] = struct{}{}
		}
		if agentInstalled {
			result.installed = append(result.installed, agentID)
		}
		if agentSkipped && !agentInstalled {
			result.skipped = append(result.skipped, agentID)
		}
	}

	if opts.target == "codex" {
		agentsPath := codexAgentsMDPath(opts.projectRoot)
		if exists(agentsPath) && !opts.force {
			result.skippedSupportFiles = append(result.skippedSupportFiles, agentsPath)
		} else {
			ids := sortedKeys(processed)
			content, err := buildCodexAgentsMD(ids, opts.language)
			if err != nil {
				result.errors = append(result.errors, agentError{agent: "codex", err: err.Error()})
			} else if err := os.WriteFile(agentsPath, []byte(content), 0o644); err != nil {
				result.errors = append(result.errors, agentError{agent: "codex", err: err.Error()})
			} else {
				result.installedSupport = append(result.installedSupport, agentsPath)
			}
		}
	}

	return result
}

func buildCodexAgentsMD(agents []string, language string) (string, error) {
	lines := []string{
		"# AGENTS.md",
		"",
		"This file provides guidance to Codex (CLI and app) for working in this repository.",
		"",
		"## Installed agent rules",
		"",
		"Read and follow these rule files in `.codex/rules/` when they apply:",
		"",
	}
	for _, agentID := range agents {
		suffixes, err := listRuleSuffixes(agentID, language)
		if err != nil {
			return "", err
		}
		for _, suffix := range suffixes {
			base := agentID
			if suffix != "" {
				base = agentID + "-" + suffix
			}
			description, _ := codexRuleDescription(agentID, language, suffix)
			if description == "" {
				description = "Rules for " + base
			}
			lines = append(lines, fmt.Sprintf("- `.codex/rules/%s.md` — %s", base, description))
		}
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n"), nil
}

func codexRuleDescription(agentID, language, suffix string) (string, error) {
	frontmatter, err := readTemplate(agentID, language, "cursor-frontmatter.yaml", suffix)
	if err != nil {
		return "", err
	}
	match := descriptionRegex.FindStringSubmatch(frontmatter)
	if len(match) < 2 {
		return "", nil
	}
	return strings.TrimSpace(match[1]), nil
}

func buildContent(agentID, target, language, suffix string) (string, error) {
	content, err := readContent(agentID, language, suffix)
	if err != nil {
		return "", err
	}
	switch target {
	case "cursor":
		front, err := readTemplate(agentID, language, "cursor-frontmatter.yaml", suffix)
		if err != nil {
			return "", err
		}
		return front + "\n" + content, nil
	case "claude":
		header, err := readTemplate(agentID, language, "claude-header.md", suffix)
		if err != nil {
			return "", err
		}
		return header + content, nil
	case "opencode":
		front, err := readTemplate(agentID, language, "opencode-frontmatter.yaml", suffix)
		if err != nil {
			return "", err
		}
		return front + "\n" + content, nil
	case "codex":
		header, err := readTemplate(agentID, language, "codex-header.md", suffix)
		if err != nil {
			header, err = readTemplate(agentID, language, "claude-header.md", suffix)
			if err != nil {
				return "", err
			}
		}
		return header + content, nil
	default:
		return "", fmt.Errorf("unknown target: %s", target)
	}
}

func listRuleSuffixes(agentID, language string) ([]string, error) {
	dir := agentDir(agentID, language)
	entries, err := readDirEntries(dir)
	if err != nil {
		return nil, fmt.Errorf("agent %q has no content.md or content-*.md", agentID)
	}
	suffixes := make([]string, 0)
	if existsAgentFile(path.Join(dir, "content.md")) {
		suffixes = append(suffixes, "")
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "content-") || !strings.HasSuffix(name, ".md") {
			continue
		}
		suffix := strings.TrimSuffix(strings.TrimPrefix(name, "content-"), ".md")
		if suffix != "" {
			suffixes = append(suffixes, suffix)
		}
	}
	if len(suffixes) == 0 {
		return nil, fmt.Errorf("agent %q has no content.md or content-*.md", agentID)
	}
	sort.Strings(suffixes)
	return suffixes, nil
}

func readContent(agentID, language, suffix string) (string, error) {
	name := "content.md"
	if suffix != "" {
		name = "content-" + suffix + ".md"
	}
	bytes, err := readAgentFile(path.Join(agentDir(agentID, language), name))
	if err != nil {
		return "", fmt.Errorf("agent %q has no %s", agentID, name)
	}
	return string(bytes), nil
}

func readTemplate(agentID, language, filename, suffix string) (string, error) {
	dir := path.Join(agentDir(agentID, language), "templates")
	if suffix != "" {
		ext := path.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		ruleFile := path.Join(dir, base+"-"+suffix+ext)
		if existsAgentFile(ruleFile) {
			bytes, err := readAgentFile(ruleFile)
			if err != nil {
				return "", err
			}
			return string(bytes), nil
		}
	}
	main := path.Join(dir, filename)
	bytes, err := readAgentFile(main)
	if err != nil {
		return "", fmt.Errorf("agent %q missing template: %s", agentID, filename)
	}
	return string(bytes), nil
}

func findProjectRoot(cwd string) (string, error) {
	start := cwd
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	dir := start
	for {
		if exists(filepath.Join(dir, "package.json")) || hasAnyRulesConfig(dir) {
			return dir, nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	return start, nil
}

func hasAnyRulesConfig(dir string) bool {
	if exists(filepath.Join(dir, ".rulesrc.ts.json")) || exists(filepath.Join(dir, ".rulesrc.json")) {
		return true
	}
	for _, language := range languages {
		if exists(filepath.Join(dir, rulesrcFilename(language))) {
			return true
		}
	}
	return false
}

func loadConfig(projectRoot, language string) *rulesConfig {
	file := filepath.Join(projectRoot, rulesrcFilename(language))
	if language == "typescript" && !exists(file) {
		file = filepath.Join(projectRoot, ".rulesrc.json")
	}
	if !exists(file) {
		return nil
	}
	bytes, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var cfg rulesConfig
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		return nil
	}
	if cfg.Target == "" || len(cfg.Agents) == 0 {
		return nil
	}
	return &cfg
}

func saveConfig(projectRoot, language string, cfg rulesConfig) error {
	bytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(projectRoot, rulesrcFilename(language)), bytes, 0o644)
}

func rulesrcFilename(language string) string {
	if language == "typescript" {
		return ".rulesrc.ts.json"
	}
	return ".rulesrc." + language + ".json"
}

func isCIMode() bool {
	return os.Getenv("CI") == "true" ||
		os.Getenv("CI") == "1" ||
		os.Getenv("TF_BUILD") == "true" ||
		os.Getenv("GITHUB_ACTIONS") == "true" ||
		os.Getenv("GITLAB_CI") == "true"
}

func promptTarget() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("AI platform (%s): ", strings.Join(targets, ", "))
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, os.ErrClosed) {
			if len(strings.TrimSpace(line)) == 0 {
				return "", err
			}
		}
		value := strings.ToLower(strings.TrimSpace(line))
		if contains(targets, value) {
			return value, nil
		}
		fmt.Printf("Invalid target. Choose one of: %s\n", strings.Join(targets, ", "))
	}
}

func promptAgents(language string) ([]string, error) {
	allowed := listAgents(language)
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Agents (comma-separated or \"all\") [%s]: ", strings.Join(allowed, ", "))
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, os.ErrClosed) {
			if len(strings.TrimSpace(line)) == 0 {
				return nil, err
			}
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			return allowed, nil
		}
		resolved := resolveAgents(splitCSV(trimmed), language)
		if len(resolved) > 0 {
			return resolved, nil
		}
		fmt.Printf("Invalid agents. Use \"all\" or comma-separated: %s\n", strings.Join(allowed, ", "))
	}
}

func splitAgents(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return splitCSV(raw)
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, item := range parts {
		v := strings.TrimSpace(item)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func resolveAgents(tokens []string, language string) []string {
	if len(tokens) == 0 {
		return nil
	}
	for _, token := range tokens {
		if token == "all" {
			return listAgents(language)
		}
	}
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if isValidAgent(token, language) {
			out = append(out, token)
		}
	}
	return out
}

func listAgents(_ string) []string {
	agents := append([]string{}, commonAgents...)
	agents = append(agents, languageAgents...)
	return agents
}

func isValidAgent(agentID, language string) bool {
	for _, agent := range listAgents(language) {
		if agent == agentID {
			return true
		}
	}
	return false
}

func agentDir(agentID, language string) string {
	if contains(commonAgents, agentID) {
		return path.Join("agents", "common", agentID)
	}
	return path.Join("agents", language, agentID)
}

func readDirEntries(relativeDir string) ([]fs.DirEntry, error) {
	if overrideRoot := repoRootOverride(); overrideRoot != "" {
		entries, err := os.ReadDir(filepath.Join(overrideRoot, filepath.FromSlash(relativeDir)))
		if err != nil {
			return nil, err
		}
		out := make([]fs.DirEntry, 0, len(entries))
		for _, entry := range entries {
			out = append(out, entry)
		}
		return out, nil
	}
	return fs.ReadDir(embeddedAgentsFS, relativeDir)
}

func readAgentFile(relativePath string) ([]byte, error) {
	if overrideRoot := repoRootOverride(); overrideRoot != "" {
		return os.ReadFile(filepath.Join(overrideRoot, filepath.FromSlash(relativePath)))
	}
	return fs.ReadFile(embeddedAgentsFS, relativePath)
}

func existsAgentFile(relativePath string) bool {
	if overrideRoot := repoRootOverride(); overrideRoot != "" {
		return exists(filepath.Join(overrideRoot, filepath.FromSlash(relativePath)))
	}
	_, err := fs.Stat(embeddedAgentsFS, relativePath)
	return err == nil
}

func repoRootOverride() string {
	value := strings.TrimSpace(os.Getenv("BALLAST_REPO_ROOT"))
	if value == "" {
		return ""
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return ""
	}
	agentsPath := filepath.Join(abs, "agents")
	if !exists(agentsPath) {
		return ""
	}
	return abs
}

func validateRepoRootOverride() error {
	value := strings.TrimSpace(os.Getenv("BALLAST_REPO_ROOT"))
	if value == "" {
		return nil
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return fmt.Errorf("invalid BALLAST_REPO_ROOT: %w", err)
	}
	if !exists(filepath.Join(abs, "agents")) {
		return fmt.Errorf("BALLAST_REPO_ROOT does not contain agents/: %s", abs)
	}
	return nil
}

func destination(projectRoot, target, basename string) (string, string) {
	switch target {
	case "cursor":
		dir := filepath.Join(projectRoot, ".cursor", "rules")
		return dir, filepath.Join(dir, basename+".mdc")
	case "claude":
		dir := filepath.Join(projectRoot, ".claude", "rules")
		return dir, filepath.Join(dir, basename+".md")
	case "opencode":
		dir := filepath.Join(projectRoot, ".opencode")
		return dir, filepath.Join(dir, basename+".md")
	default:
		dir := filepath.Join(projectRoot, ".codex", "rules")
		return dir, filepath.Join(dir, basename+".md")
	}
}

func codexAgentsMDPath(projectRoot string) string {
	return filepath.Join(projectRoot, "AGENTS.md")
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func trimCommand(args []string) []string {
	if len(args) > 0 && args[0] == "install" {
		return args[1:]
	}
	return args
}

func sortedKeys(input map[string]struct{}) []string {
	out := make([]string, 0, len(input))
	for key := range input {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

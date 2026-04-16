package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
)

var targets = []string{"cursor", "claude", "opencode", "codex"}
var languages = []string{"typescript", "python", "go", "ansible", "terraform"}

var commonAgents = []string{"local-dev", "docs", "cicd", "observability", "publishing", "git-hooks"}
var languageAgents = []string{"linting", "logging", "testing"}
var commonSkills = []string{
	"owasp-security-scan",
	"aws-health-review",
	"aws-live-health-review",
	"aws-weekly-security-review",
	"github-health-check",
}

var descriptionRegex = regexp.MustCompile(`(?m)^description:\s*['\"]?(.+?)['\"]?\s*$`)
var ballastVersion = "dev"
var frontmatterRegex = regexp.MustCompile(`(?s)^\s*---\n(.*?)\n---\n?`)
var topLevelYAMLKeyRegex = regexp.MustCompile(`^([A-Za-z0-9_-]+):(.*)$`)
var gitHooksGuidanceToken = "{{BALLAST_GIT_HOOKS_GUIDANCE}}"

func withImplicitAgents(agents []string) []string {
	resolved := slices.Clone(agents)
	if contains(resolved, "linting") && !contains(resolved, "git-hooks") {
		resolved = append(resolved, "git-hooks")
	}
	return resolved
}

//go:embed agents/**
var embeddedAgentsFS embed.FS

//go:embed skills/**
var embeddedSkillsFS embed.FS

type rulesConfig struct {
	Targets        []string            `json:"targets,omitempty"`
	Agents         []string            `json:"agents"`
	Skills         []string            `json:"skills,omitempty"`
	BallastVersion string              `json:"ballastVersion,omitempty"`
	Languages      []string            `json:"languages,omitempty"`
	Paths          map[string][]string `json:"paths,omitempty"`
}

type rawRulesConfig struct {
	Target         string              `json:"target,omitempty"`
	Targets        []string            `json:"targets,omitempty"`
	Agents         []string            `json:"agents,omitempty"`
	Skills         []string            `json:"skills,omitempty"`
	BallastVersion string              `json:"ballastVersion,omitempty"`
	Languages      []string            `json:"languages,omitempty"`
	Paths          map[string][]string `json:"paths,omitempty"`
}

type installResult struct {
	installed           []string
	installedRules      []installedRule
	installedSkills     []string
	installedSupport    []string
	skipped             []string
	skippedSkills       []string
	skippedSupportFiles []string
	errors              []agentError
}

type installedRule struct {
	agentID    string
	ruleSuffix string
	target     string
}

type agentError struct {
	agent string
	err   string
}

type resolveOptions struct {
	projectRoot string
	targets     []string
	agents      []string
	skills      []string
	all         bool
	allSkills   bool
	yes         bool
	language    string
}

type installOptions struct {
	projectRoot string
	targets     []string
	agents      []string
	skills      []string
	language    string
	force       bool
	patch       bool
	patchClaude bool
	saveConfig  bool
}

type markdownSection struct {
	heading string
	text    string
}

type yamlBlock struct {
	key  string
	text string
}

type installedCLIStatus struct {
	Name    string
	Version string
	Path    string
}

type targetListFlag []string

func (f *targetListFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *targetListFlag) Set(value string) error {
	*f = append(*f, splitTargets(value)...)
	return nil
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if hasHelpFlag(args) || isHelpCommand(args) {
		printHelp()
		return 0
	}
	if hasVersionFlag(args) || isVersionCommand(args) {
		fmt.Println(resolveVersion())
		return 0
	}
	if len(args) > 0 && args[0] == "doctor" {
		return runDoctor()
	}
	if len(args) == 0 || args[0] == "install" {
		return runInstall(args)
	}
	fmt.Printf("Unknown command: %s\n", args[0])
	fmt.Println("Run ballast-go --help for usage.")
	return 1
}

func runInstall(args []string) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	var targetFlags targetListFlag
	fs.Var(&targetFlags, "target", "cursor|claude|opencode|codex")
	fs.Var(&targetFlags, "t", "cursor|claude|opencode|codex")
	language := fs.String("language", "go", "typescript|python|go|ansible|terraform")
	fs.StringVar(language, "l", "go", "typescript|python|go|ansible|terraform")
	agent := fs.String("agent", "", "comma-separated list")
	fs.StringVar(agent, "a", "", "comma-separated list")
	skill := fs.String("skill", "", "comma-separated list")
	fs.StringVar(skill, "s", "", "comma-separated list")
	all := fs.Bool("all", false, "install all agents")
	allSkills := fs.Bool("all-skills", false, "install all skills")
	force := fs.Bool("force", false, "overwrite files")
	patch := fs.Bool("patch", false, "merge upstream updates into existing files")
	fs.BoolVar(patch, "p", false, "merge upstream updates into existing files")
	yes := fs.Bool("yes", false, "non-interactive mode")
	fs.BoolVar(yes, "y", false, "non-interactive mode")
	if err := fs.Parse(trimCommand(args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
		targets:     targetFlags,
		agents:      splitAgents(*agent),
		skills:      splitCSV(*skill),
		all:         *all,
		allSkills:   *allSkills,
		yes:         *yes,
		language:    lang,
	})
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if resolved == nil {
		fmt.Println("In CI/non-interactive mode (--yes or CI env), --target and at least one of --agent/--all or --skill/--all-skills are required when config is missing.")
		fmt.Println("Example: ballast-go install --yes --target cursor --agent linting --skill owasp-security-scan")
		return 1
	}

	patchClaude := false
	for _, target := range resolved.Targets {
		if target != "claude" || !exists(claudeMDPath(root)) || *force {
			continue
		}
		if *patch {
			patchClaude = true
			break
		}
		if !*yes && !isCIMode() {
			approved, promptErr := promptYesNo(
				fmt.Sprintf(
					"Existing CLAUDE.md found at %s. Patch the Installed agent rules section?",
					claudeMDPath(root),
				),
				false,
			)
			if promptErr != nil {
				fmt.Println(promptErr)
				return 1
			}
			patchClaude = approved
			break
		}
	}
	result := install(installOptions{
		projectRoot: root,
		targets:     resolved.Targets,
		agents:      resolved.Agents,
		skills:      resolved.Skills,
		language:    lang,
		force:       *force,
		patch:       *patch,
		patchClaude: patchClaude,
		saveConfig:  true,
	})

	if len(result.errors) > 0 {
		for _, item := range result.errors {
			fmt.Printf("Error installing %s: %s\n", item.agent, item.err)
		}
		return 1
	}

	if len(result.installedRules) > 0 {
		fmt.Printf("Installed for %s: %s\n", strings.Join(resolved.Targets, ", "), strings.Join(result.installed, ", "))
		for _, rule := range result.installedRules {
			base := ruleBaseName(rule.agentID, lang, rule.ruleSuffix)
			_, file, err := destination(root, rule.target, base)
			if err != nil {
				fmt.Println(err)
				return 1
			}
			fmt.Printf("  %s -> %s\n", base, file)
		}
	}
	if len(result.installedSkills) > 0 {
		fmt.Printf("Installed skills for %s: %s\n", strings.Join(resolved.Targets, ", "), strings.Join(result.installedSkills, ", "))
		for _, skillID := range result.installedSkills {
			_, file, err := skillDestination(root, resolved.Targets[0], skillID)
			if err != nil {
				fmt.Println(err)
				return 1
			}
			fmt.Printf("  %s -> %s\n", skillID, file)
		}
	}
	if len(result.installedSupport) > 0 {
		for _, file := range result.installedSupport {
			label := "AGENTS.md"
			if strings.HasSuffix(file, "CLAUDE.md") {
				label = "CLAUDE.md"
			}
			fmt.Printf("  %s -> %s\n", label, file)
		}
	}
	if len(result.skipped) > 0 {
		fmt.Printf("Skipped (already present; use --force to overwrite): %s\n", strings.Join(result.skipped, ", "))
	}
	if len(result.skippedSkills) > 0 {
		fmt.Printf("Skipped skills (already present; use --force to overwrite): %s\n", strings.Join(result.skippedSkills, ", "))
	}
	if len(result.skippedSupportFiles) > 0 {
		fmt.Printf(
			"Skipped support files (already present; use --force to overwrite): %s\n",
			strings.Join(result.skippedSupportFiles, ", "),
		)
	}
	if len(result.installed) == 0 && len(result.installedSkills) == 0 && len(result.skipped) == 0 && len(result.skippedSkills) == 0 && len(result.errors) == 0 {
		fmt.Println("Nothing to install.")
	}

	return 0
}

func printHelp() {
	fmt.Printf(`
ballast-go v%s

Usage: ballast-go install [options]

Commands:
  install    Install agent rules for the chosen AI platform (default)
  doctor     Check local Ballast CLI versions and .rulesrc.json metadata

Options:
  --target, -t <platform>   AI platforms: %s (comma-separated or repeatable)
  --language, -l <lang>     Language profile: %s (default: go)
  --agent, -a <agents>      Agent(s): linting, local-dev, docs, cicd, observability, publishing, git-hooks, logging, testing (comma-separated)
  --skill, -s <skills>      Skill(s): owasp-security-scan, aws-health-review, aws-live-health-review, aws-weekly-security-review, github-health-check (comma-separated)
  --all                     Install all agents
  --all-skills              Install all skills
  --force                   Overwrite existing rule files
  --patch, -p               Merge upstream rule updates into existing files; ignored when --force is set
  --yes, -y                 Non-interactive; require --target and --agent/--all if no .rulesrc.json
  --help, -h                Show this help
  --version, -v             Show version

Examples:
  ballast-go install
  ballast-go install --target cursor --agent linting
  ballast-go install --target claude --skill owasp-security-scan
  ballast-go install --target codex --skill aws-health-review
  ballast-go install --target cursor,claude --agent linting
  ballast-go install --language python --target cursor --all
  ballast-go install --target claude --all --force
  ballast-go install --target cursor --agent linting --patch
  ballast-go install --yes --target cursor --target codex --all
`, resolveVersion(), strings.Join(targets, ", "), strings.Join(languages, ", "))
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func hasVersionFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--version" || arg == "-v" {
			return true
		}
	}
	return false
}

func isVersionCommand(args []string) bool {
	return len(args) == 1 && args[0] == "version"
}

func isHelpCommand(args []string) bool {
	return len(args) == 1 && args[0] == "help"
}

func resolveVersion() string {
	if strings.TrimSpace(ballastVersion) != "" && ballastVersion != "dev" {
		return ballastVersion
	}
	info, ok := debug.ReadBuildInfo()
	if ok {
		if strings.TrimSpace(info.Main.Version) != "" && info.Main.Version != "(devel)" {
			return strings.TrimPrefix(info.Main.Version, "v")
		}
	}
	return ballastVersion
}

func compareVersions(left, right string) int {
	if left == right {
		return 0
	}
	leftParts, leftOK := parseVersionParts(left)
	rightParts, rightOK := parseVersionParts(right)
	if leftOK && !rightOK {
		return 1
	}
	if !leftOK && rightOK {
		return -1
	}
	if !leftOK || !rightOK {
		if left < right {
			return -1
		}
		return 1
	}
	length := max(len(leftParts), len(rightParts))
	for index := 0; index < length; index++ {
		leftPart := 0
		rightPart := 0
		if index < len(leftParts) {
			leftPart = leftParts[index]
		}
		if index < len(rightParts) {
			rightPart = rightParts[index]
		}
		if leftPart < rightPart {
			return -1
		}
		if leftPart > rightPart {
			return 1
		}
	}
	return 0
}

func parseVersionParts(value string) ([]int, bool) {
	parts := strings.Split(value, ".")
	parsed := make([]int, 0, len(parts))
	for _, part := range parts {
		number := 0
		if _, err := fmt.Sscanf(part, "%d", &number); err != nil {
			return nil, false
		}
		parsed = append(parsed, number)
	}
	return parsed, true
}

func latestVersion(values ...string) string {
	best := resolveVersion()
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if compareVersions(value, best) > 0 {
			best = value
		}
	}
	return best
}

func detectInstalledCLI(name string) installedCLIStatus {
	cliPath, err := exec.LookPath(name)
	if err != nil {
		return installedCLIStatus{Name: name}
	}
	output, err := exec.Command(name, "--version").Output()
	if err != nil {
		return installedCLIStatus{Name: name, Path: cliPath}
	}
	return installedCLIStatus{
		Name:    name,
		Version: strings.TrimSpace(string(output)),
		Path:    cliPath,
	}
}

func upgradeCommand(name, version string) string {
	_ = name
	_ = version
	return "ballast doctor --fix"
}

func buildDoctorReport(currentCLI, currentVersion string, configPath string, config *rulesConfig, installed []installedCLIStatus) string {
	configVersion := ""
	if config != nil {
		configVersion = strings.TrimSpace(config.BallastVersion)
	}
	targetVersion := latestVersion(currentVersion, configVersion)
	for _, item := range installed {
		targetVersion = latestVersion(targetVersion, item.Version)
	}

	lines := []string{
		"Ballast doctor",
		fmt.Sprintf("Current CLI: %s %s", currentCLI, currentVersion),
		"",
		"Installed CLIs:",
	}
	recommendations := []string{}
	needsCLIFix := false
	for _, item := range installed {
		if item.Path == "" {
			lines = append(lines, fmt.Sprintf("- %s: not found", item.Name))
			needsCLIFix = true
			continue
		}
		version := item.Version
		if version == "" {
			version = "unknown"
		}
		lines = append(lines, fmt.Sprintf("- %s: %s (%s)", item.Name, version, item.Path))
		if item.Version == "" || compareVersions(item.Version, targetVersion) < 0 {
			needsCLIFix = true
		}
	}
	if needsCLIFix {
		recommendations = append(recommendations, "Run ballast doctor --fix to install or upgrade local Ballast CLIs.")
	}

	lines = append(lines, "", "Config:")
	if config == nil || configPath == "" {
		lines = append(lines, "- .rulesrc.json: not found")
	} else {
		lines = append(lines, fmt.Sprintf("- file: %s", configPath))
		if configVersion == "" {
			lines = append(lines, "- ballastVersion: missing")
		} else {
			lines = append(lines, fmt.Sprintf("- ballastVersion: %s", configVersion))
		}
		if len(config.Targets) > 0 {
			lines = append(lines, fmt.Sprintf("- targets: %s", strings.Join(config.Targets, ", ")))
		}
		if len(config.Agents) > 0 {
			lines = append(lines, fmt.Sprintf("- agents: %s", strings.Join(config.Agents, ", ")))
		}
		if len(config.Skills) > 0 {
			lines = append(lines, fmt.Sprintf("- skills: %s", strings.Join(config.Skills, ", ")))
		}
		if configVersion == "" || compareVersions(configVersion, targetVersion) < 0 {
			recommendations = append(
				recommendations,
				fmt.Sprintf("Refresh %s to Ballast %s: ballast install --refresh-config", filepath.Base(configPath), targetVersion),
			)
		}
	}

	lines = append(lines, "", "Recommendations:")
	if len(recommendations) == 0 {
		lines = append(lines, "- No action needed.")
	} else {
		for _, item := range recommendations {
			lines = append(lines, "- "+item)
		}
	}

	return strings.Join(lines, "\n") + "\n"
}

func runDoctor() int {
	root, err := findProjectRoot("")
	if err != nil {
		root = "."
	}
	configPath := filepath.Join(root, rulesrcFilename("go"))
	config := loadConfig(root, "go")
	if !exists(configPath) {
		configPath = ""
	}
	report := buildDoctorReport(
		"ballast-go",
		resolveVersion(),
		configPath,
		config,
		[]installedCLIStatus{
			detectInstalledCLI("ballast-typescript"),
			detectInstalledCLI("ballast-python"),
			detectInstalledCLI("ballast-go"),
		},
	)
	fmt.Print(report)
	return 0
}

func resolveTargetAndAgents(opts resolveOptions) (*rulesConfig, error) {
	config := loadConfig(opts.projectRoot, opts.language)
	ci := isCIMode() || opts.yes

	flagAgents := opts.agents
	if opts.all {
		flagAgents = []string{"all"}
	}
	flagSkills := opts.skills
	if opts.allSkills {
		flagSkills = []string{"all"}
	}

	if config != nil && len(opts.targets) == 0 && len(flagAgents) == 0 && len(flagSkills) == 0 {
		next := *config
		next.Agents = withImplicitAgents(config.Agents)
		return &next, nil
	}

	resolvedTargets, invalidTargets := normalizeTargetsDetailed(opts.targets)
	if len(invalidTargets) > 0 {
		return nil, fmt.Errorf("invalid --target: %s. Use: %s", strings.Join(invalidTargets, ", "), strings.Join(targets, ", "))
	}
	if len(resolvedTargets) == 0 && config != nil {
		resolvedTargets = normalizeTargets(config.Targets)
	}

	var resolvedAgents []string
	if len(flagAgents) > 0 {
		resolvedAgents = withImplicitAgents(resolveAgents(flagAgents, opts.language))
	} else if config != nil {
		resolvedAgents = withImplicitAgents(config.Agents)
	}
	resolvedSkills := []string{}
	if len(flagSkills) > 0 {
		resolvedSkills = resolveSkills(flagSkills, opts.language)
	} else if config != nil {
		resolvedSkills = slices.Clone(config.Skills)
	}

	if len(resolvedTargets) > 0 && (len(resolvedAgents) > 0 || len(resolvedSkills) > 0) {
		return &rulesConfig{Targets: resolvedTargets, Agents: resolvedAgents, Skills: resolvedSkills}, nil
	}

	if ci {
		return nil, nil
	}

	if len(resolvedTargets) == 0 {
		var err error
		resolvedTargets, err = promptTargets()
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
	if len(resolvedSkills) == 0 {
		var err error
		resolvedSkills, err = promptSkills(opts.language)
		if err != nil {
			return nil, err
		}
	}

	return &rulesConfig{Targets: resolvedTargets, Agents: resolvedAgents, Skills: resolvedSkills}, nil
}

func install(opts installOptions) installResult {
	result := installResult{}
	opts.agents = withImplicitAgents(opts.agents)
	disableSupportFiles := os.Getenv("BALLAST_DISABLE_SUPPORT_FILES") == "1"
	hookMode := resolveTsHookMode(opts.projectRoot, opts.language)
	targets := normalizeTargets(opts.targets)
	if len(targets) == 0 {
		result.errors = append(result.errors, agentError{agent: "target", err: "No targets selected"})
		return result
	}

	if err := ensureGitignoreEntry(opts.projectRoot, ".ballast/"); err != nil {
		result.errors = append(result.errors, agentError{agent: "gitignore", err: err.Error()})
	}

	if opts.saveConfig {
		if err := saveConfig(opts.projectRoot, opts.language, rulesConfig{
			Targets:   targets,
			Agents:    opts.agents,
			Skills:    opts.skills,
			Languages: []string{opts.language},
		}); err != nil {
			result.errors = append(result.errors, agentError{agent: "config", err: err.Error()})
			return result
		}
	}

	for _, target := range targets {
		processed := map[string]struct{}{}
		processedSkills := map[string]struct{}{}

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
				base := ruleBaseName(agentID, opts.language, suffix)
				dir, file, err := destination(opts.projectRoot, target, base)
				if err != nil {
					result.errors = append(result.errors, agentError{agent: agentID, err: err.Error()})
					continue
				}
				content, err := buildContent(agentID, target, opts.language, suffix, hookMode)
				if err != nil {
					result.errors = append(result.errors, agentError{agent: agentID, err: err.Error()})
					continue
				}
				if exists(file) && !opts.force && !opts.patch {
					agentSkipped = true
					agentProcessed = true
					continue
				}
				if err := os.MkdirAll(dir, 0o755); err != nil {
					result.errors = append(result.errors, agentError{agent: agentID, err: err.Error()})
					continue
				}
				nextContent := content
				if exists(file) && !opts.force && opts.patch {
					existing, err := os.ReadFile(file)
					if err != nil {
						result.errors = append(result.errors, agentError{agent: agentID, err: err.Error()})
						continue
					}
					nextContent = patchRuleContent(string(existing), content, target)
				}
				if err := os.WriteFile(file, []byte(nextContent), 0o644); err != nil {
					result.errors = append(result.errors, agentError{agent: agentID, err: err.Error()})
					continue
				}
				result.installedRules = append(result.installedRules, installedRule{target: target, agentID: agentID, ruleSuffix: suffix})
				agentInstalled = true
				agentProcessed = true
			}
			if agentProcessed {
				processed[agentID] = struct{}{}
			}
			if agentInstalled && !contains(result.installed, agentID) {
				result.installed = append(result.installed, agentID)
			}
			if agentSkipped && !agentInstalled && !contains(result.skipped, agentID) {
				result.skipped = append(result.skipped, agentID)
			}
		}

		for _, skillID := range opts.skills {
			if !isValidSkill(skillID, opts.language) {
				result.errors = append(result.errors, agentError{agent: skillID, err: "Unknown skill"})
				continue
			}
			dir, file, err := skillDestination(opts.projectRoot, target, skillID)
			if err != nil {
				result.errors = append(result.errors, agentError{agent: skillID, err: err.Error()})
				continue
			}
			if exists(file) && !opts.force {
				if !contains(result.skippedSkills, skillID) {
					result.skippedSkills = append(result.skippedSkills, skillID)
				}
				processedSkills[skillID] = struct{}{}
				continue
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				result.errors = append(result.errors, agentError{agent: skillID, err: err.Error()})
				continue
			}
			switch target {
			case "cursor":
				content, buildErr := buildCursorSkillFormat(skillID, opts.language)
				if buildErr != nil {
					result.errors = append(result.errors, agentError{agent: skillID, err: buildErr.Error()})
					continue
				}
				err = os.WriteFile(file, []byte(content), 0o644)
			case "claude":
				content, buildErr := buildClaudeSkill(skillID, opts.language)
				if buildErr != nil {
					result.errors = append(result.errors, agentError{agent: skillID, err: buildErr.Error()})
					continue
				}
				err = os.WriteFile(file, content, 0o644)
			case "opencode", "codex":
				content, buildErr := buildSkillMarkdown(skillID, opts.language)
				if buildErr != nil {
					result.errors = append(result.errors, agentError{agent: skillID, err: buildErr.Error()})
					continue
				}
				err = os.WriteFile(file, []byte(content), 0o644)
			default:
				err = fmt.Errorf("unknown target: %s", target)
			}
			if err != nil {
				result.errors = append(result.errors, agentError{agent: skillID, err: err.Error()})
				continue
			}
			if !contains(result.installedSkills, skillID) {
				result.installedSkills = append(result.installedSkills, skillID)
			}
			processedSkills[skillID] = struct{}{}
		}

		if target == "codex" && !disableSupportFiles {
			agentsPath := codexAgentsMDPath(opts.projectRoot)
			if exists(agentsPath) && !opts.force && !opts.patch {
				if !contains(result.skippedSupportFiles, agentsPath) {
					result.skippedSupportFiles = append(result.skippedSupportFiles, agentsPath)
				}
			} else {
				ids := sortedKeys(processed)
				content, err := buildCodexAgentsMD(ids, sortedKeys(processedSkills), opts.language)
				if err != nil {
					result.errors = append(result.errors, agentError{agent: "codex", err: err.Error()})
				} else {
					nextContent := content
					if exists(agentsPath) && !opts.force && opts.patch {
						existing, readErr := os.ReadFile(agentsPath)
						if readErr != nil {
							result.errors = append(result.errors, agentError{agent: "codex", err: readErr.Error()})
						} else {
							nextContent = patchCodexAgentsMD(string(existing), content)
						}
					}
					if err := os.WriteFile(agentsPath, []byte(nextContent), 0o644); err != nil {
						result.errors = append(result.errors, agentError{agent: "codex", err: err.Error()})
					} else if !contains(result.installedSupport, agentsPath) {
						result.installedSupport = append(result.installedSupport, agentsPath)
					}
				}
			}
		}

		if target == "claude" && !disableSupportFiles {
			claudePath := claudeMDPath(opts.projectRoot)
			shouldPatchClaude := opts.patch || opts.patchClaude
			if exists(claudePath) && !opts.force && !shouldPatchClaude {
				if !contains(result.skippedSupportFiles, claudePath) {
					result.skippedSupportFiles = append(result.skippedSupportFiles, claudePath)
				}
			} else {
				ids := sortedKeys(processed)
				content, err := buildClaudeMD(ids, sortedKeys(processedSkills), opts.language)
				if err != nil {
					result.errors = append(result.errors, agentError{agent: "claude", err: err.Error()})
				} else {
					nextContent := content
					if exists(claudePath) && !opts.force && shouldPatchClaude {
						existing, readErr := os.ReadFile(claudePath)
						if readErr != nil {
							result.errors = append(result.errors, agentError{agent: "claude", err: readErr.Error()})
						} else {
							nextContent = patchCodexAgentsMD(string(existing), content)
						}
					}
					if err := os.WriteFile(claudePath, []byte(nextContent), 0o644); err != nil {
						result.errors = append(result.errors, agentError{agent: "claude", err: err.Error()})
					} else if !contains(result.installedSupport, claudePath) {
						result.installedSupport = append(result.installedSupport, claudePath)
					}
				}
			}
		}
	}

	return result
}

func buildCodexAgentsMD(agents []string, skills []string, language string) (string, error) {
	lines := []string{
		"# AGENTS.md",
		"",
		"This file provides guidance to Codex (CLI and app) for working in this repository.",
		"",
	}
	lines = append(lines, repositoryFactsSection()...)
	lines = append(lines,
		"",
		"## Installed agent rules",
		"",
		ballastNotice(),
		"",
		"Read and follow these rule files in `.codex/rules/` when they apply:",
		"",
	)
	for _, agentID := range agents {
		suffixes, err := listRuleSuffixes(agentID, language)
		if err != nil {
			return "", err
		}
		for _, suffix := range suffixes {
			base := ruleBaseName(agentID, language, suffix)
			description, _ := codexRuleDescription(agentID, language, suffix)
			if description == "" {
				description = "Rules for " + base
			}
			lines = append(lines, fmt.Sprintf("- `.codex/rules/%s.md` — %s", base, description))
		}
	}
	if len(skills) > 0 {
		lines = append(lines,
			"",
			"## Installed skills",
			"",
			ballastNotice(),
			"",
			"Read and use these skill files in `.codex/rules/` when they are relevant:",
			"",
		)
		for _, skillID := range skills {
			lines = append(lines, fmt.Sprintf("- `.codex/rules/%s.md` — %s", skillID, skillDescription(skillID, language)))
		}
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n"), nil
}

func buildClaudeMD(agents []string, skills []string, language string) (string, error) {
	lines := []string{
		"# CLAUDE.md",
		"",
		"This file provides guidance to Claude Code for working in this repository.",
		"",
	}
	lines = append(lines, repositoryFactsSection()...)
	lines = append(lines,
		"",
		"## Installed agent rules",
		"",
		ballastNotice(),
		"",
		"Read and follow these rule files in `.claude/rules/` when they apply:",
		"",
	)
	for _, agentID := range agents {
		suffixes, err := listRuleSuffixes(agentID, language)
		if err != nil {
			return "", err
		}
		for _, suffix := range suffixes {
			base := ruleBaseName(agentID, language, suffix)
			description, _ := codexRuleDescription(agentID, language, suffix)
			if description == "" {
				description = "Rules for " + base
			}
			lines = append(lines, fmt.Sprintf("- `.claude/rules/%s.md` — %s", base, description))
		}
	}
	if len(skills) > 0 {
		lines = append(lines,
			"",
			"## Installed skills",
			"",
			ballastNotice(),
			"",
			"Read and use these skill files in `.claude/skills/` when they are relevant:",
			"",
		)
		for _, skillID := range skills {
			lines = append(lines, fmt.Sprintf("- `.claude/skills/%s.skill` — %s", skillID, skillDescription(skillID, language)))
		}
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n"), nil
}

func ballastNotice() string {
	return "Created by [Ballast](https://github.com/everydaydevopsio/ballast) v" + ballastVersion + ". Do not edit this section."
}

func repositoryFactsSection() []string {
	return []string{
		"## Repository Facts",
		"",
		"Use this section for durable repo-specific facts that agents repeatedly need. Prefer facts stored here over re-deriving them with shell commands on every task.",
		"",
		"Keep only stable, reviewable metadata here. Do not store secrets, credentials, or ephemeral runtime state.",
		"",
		"Suggested facts to record:",
		"",
		"- Canonical GitHub repo: `<OWNER/REPO>`",
		"- Default branch: `<main>`",
		"- Primary package manager: `<pnpm | npm | yarn | uv | go>`",
		"- Version-file locations agents should check first: `<.nvmrc, packageManager, pyproject.toml, go.mod, etc.>`",
		"- Canonical config files: `<paths agents should read before falling back to discovery>`",
		"- Primary CI workflows: `<workflow filenames>`",
		"- Primary release/publish workflows: `<workflow filenames>`",
		"- Preferred build/test/lint/format/coverage commands: `<commands>`",
		"- Coverage threshold: `<value>`",
		"- Generated or protected paths agents should avoid editing directly: `<paths>`",
		"",
		"Update this section when those facts change. If live runtime state is required, discover it separately instead of treating it as a durable repo fact.",
	}
}

func extractDescriptionFromFrontmatter(frontmatter string) *string {
	normalized := normalizeLineEndings(frontmatter)
	lines := strings.Split(normalized, "\n")
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "description:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
		if value == "" {
			return nil
		}
		if value == ">" || value == "|" || value == ">-" || value == "|-" || value == ">+" || value == "|+" {
			return extractFoldedDescription(lines, index+1, value)
		}
		description := strings.TrimSpace(strings.Trim(value, `"'`))
		if description == "" {
			return nil
		}
		return &description
	}
	match := descriptionRegex.FindStringSubmatch(normalized)
	if len(match) < 2 {
		return nil
	}
	description := strings.TrimSpace(strings.Trim(match[1], `"'`))
	if description == "" {
		return nil
	}
	return &description
}

func extractFoldedDescription(lines []string, start int, style string) *string {
	values := []string{}
	for index := start; index < len(lines); index++ {
		line := lines[index]
		if strings.TrimSpace(line) == "" {
			if len(values) == 0 {
				continue
			}
			values = append(values, "")
			continue
		}
		if len(line)-len(strings.TrimLeft(line, " ")) < 2 {
			break
		}
		values = append(values, strings.TrimSpace(line))
	}
	if len(values) == 0 {
		return nil
	}
	parts := []string{}
	if strings.HasPrefix(style, ">") {
		for _, value := range values {
			if value == "" {
				continue
			}
			parts = append(parts, value)
		}
		description := strings.TrimSpace(strings.Join(parts, " "))
		if description == "" {
			return nil
		}
		return &description
	}
	description := strings.TrimSpace(strings.Join(values, "\n"))
	if description == "" {
		return nil
	}
	return &description
}

func codexRuleDescription(agentID, language, suffix string) (string, error) {
	frontmatter, err := readTemplate(agentID, language, "cursor-frontmatter.yaml", suffix)
	if err != nil {
		return "", err
	}
	description := extractDescriptionFromFrontmatter(frontmatter)
	if description == nil {
		return "", nil
	}
	return *description, nil
}

func readSkillContent(skillID, language string) (string, error) {
	bytes, err := readSkillFile(path.Join(skillDir(skillID, language), "SKILL.md"))
	if err != nil {
		return "", fmt.Errorf("skill %q missing SKILL.md", skillID)
	}
	return string(bytes), nil
}

func splitSkillDocument(content string) (string, string) {
	normalized := normalizeLineEndings(content)
	match := frontmatterRegex.FindStringSubmatchIndex(normalized)
	if match == nil || match[0] != 0 {
		return "", strings.TrimLeft(normalized, "\n\t ")
	}
	frontmatter := strings.TrimRight(normalized[match[0]:match[1]], "\n")
	body := strings.TrimLeft(normalized[match[1]:], "\n\t ")
	return frontmatter, body
}

func skillDescription(skillID, language string) string {
	content, err := readSkillContent(skillID, language)
	if err != nil {
		return "Skill " + skillID
	}
	frontmatter, _ := splitSkillDocument(content)
	if frontmatter == "" {
		return "Skill " + skillID
	}
	description := extractDescriptionFromFrontmatter(frontmatter)
	if description == nil {
		return "Skill " + skillID
	}
	return *description
}

func buildCursorSkillFormat(skillID, language string) (string, error) {
	content, err := readSkillContent(skillID, language)
	if err != nil {
		return "", err
	}
	_, body := splitSkillDocument(content)
	return fmt.Sprintf("---\ndescription: %q\nalwaysApply: false\n---\n\n%s\n", skillDescription(skillID, language), strings.TrimRight(body, "\n")), nil
}

func buildSkillMarkdown(skillID, language string) (string, error) {
	content, err := readSkillContent(skillID, language)
	if err != nil {
		return "", err
	}
	_, body := splitSkillDocument(content)
	return strings.TrimRight(body, "\n") + "\n", nil
}

func buildClaudeSkill(skillID, language string) ([]byte, error) {
	content, err := readSkillContent(skillID, language)
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	writer, err := archive.Create("SKILL.md")
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		return nil, err
	}
	referencesDir := path.Join(skillDir(skillID, language), "references")
	if existsSkillFile(referencesDir) {
		if overrideRoot := repoRootOverride(); overrideRoot != "" {
			rootDir := filepath.Join(overrideRoot, filepath.FromSlash(referencesDir))
			err = filepath.WalkDir(rootDir, func(file string, d os.DirEntry, walkErr error) error {
				if walkErr != nil || d.IsDir() {
					return walkErr
				}
				relative, relErr := filepath.Rel(rootDir, file)
				if relErr != nil {
					return relErr
				}
				entry, createErr := archive.Create(path.Join("references", filepath.ToSlash(relative)))
				if createErr != nil {
					return createErr
				}
				data, readErr := os.ReadFile(file)
				if readErr != nil {
					return readErr
				}
				_, writeErr := entry.Write(data)
				return writeErr
			})
			if err != nil {
				return nil, err
			}
		} else {
			referenceEntries, readErr := fs.ReadDir(embeddedSkillsFS, referencesDir)
			if readErr == nil {
				for _, entry := range referenceEntries {
					if entry.IsDir() {
						continue
					}
					data, fileErr := readSkillFile(path.Join(referencesDir, entry.Name()))
					if fileErr != nil {
						return nil, fileErr
					}
					writer, createErr := archive.Create(path.Join("references", entry.Name()))
					if createErr != nil {
						return nil, createErr
					}
					if _, fileErr := writer.Write(data); fileErr != nil {
						return nil, fileErr
					}
				}
			}
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func normalizeLineEndings(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.ReplaceAll(content, "\r", "\n")
}

func splitFrontmatterDocument(content string) (string, string) {
	normalized := normalizeLineEndings(content)
	match := frontmatterRegex.FindStringSubmatchIndex(normalized)
	if match == nil || match[0] != 0 {
		return "", strings.TrimLeft(normalized, "\n\t ")
	}
	frontmatter := strings.TrimRight(normalized[match[0]:match[1]], "\n")
	body := strings.TrimLeft(normalized[match[1]:], "\n\t ")
	return frontmatter, body
}

func extractFrontmatterYAML(frontmatter string) string {
	match := frontmatterRegex.FindStringSubmatch(frontmatter)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func parseTopLevelYAMLBlocks(yamlContent string) (string, []yamlBlock) {
	lines := strings.Split(normalizeLineEndings(yamlContent), "\n")
	blocks := make([]yamlBlock, 0)
	preamble := make([]string, 0)
	currentKey := ""
	currentLines := make([]string, 0)
	flush := func() {
		if currentKey != "" {
			blocks = append(blocks, yamlBlock{
				key:  currentKey,
				text: strings.TrimRight(strings.Join(currentLines, "\n"), "\n"),
			})
		}
		currentKey = ""
		currentLines = currentLines[:0]
	}

	for _, line := range lines {
		if match := topLevelYAMLKeyRegex.FindStringSubmatch(line); len(match) == 3 {
			flush()
			currentKey = match[1]
			currentLines = append(currentLines, line)
			continue
		}
		if currentKey == "" {
			preamble = append(preamble, line)
			continue
		}
		currentLines = append(currentLines, line)
	}
	flush()

	return strings.TrimSpace(strings.Join(preamble, "\n")), blocks
}

func splitNestedYAMLBlock(block string) (string, string, int, bool) {
	lines := strings.Split(block, "\n")
	if len(lines) < 2 {
		return "", "", 0, false
	}
	bodyLines := lines[1:]
	nonEmpty := make([]string, 0, len(bodyLines))
	for _, line := range bodyLines {
		if strings.TrimSpace(line) != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}
	if len(nonEmpty) == 0 {
		return "", "", 0, false
	}
	indent := -1
	for _, line := range nonEmpty {
		currentIndent := len(line) - len(strings.TrimLeft(line, " "))
		if indent < 0 || currentIndent < indent {
			indent = currentIndent
		}
	}
	if indent <= 0 {
		return "", "", 0, false
	}
	dedented := make([]string, 0, len(bodyLines))
	for _, line := range bodyLines {
		if strings.TrimSpace(line) == "" {
			dedented = append(dedented, "")
			continue
		}
		dedented = append(dedented, line[indent:])
	}
	for _, line := range dedented {
		if strings.HasPrefix(line, "- ") {
			return "", "", 0, false
		}
	}
	return lines[0], strings.TrimRight(strings.Join(dedented, "\n"), "\n"), indent, true
}

func mergeYAMLBlocks(existingBlock, canonicalBlock string) string {
	existingHeader, existingBody, _, existingOK := splitNestedYAMLBlock(existingBlock)
	_, canonicalBody, canonicalIndent, canonicalOK := splitNestedYAMLBlock(canonicalBlock)
	if !existingOK || !canonicalOK {
		return existingBlock
	}
	mergedBody, ok := mergeYAMLMappingContent(existingBody, canonicalBody)
	if !ok {
		return existingBlock
	}
	lines := []string{existingHeader}
	for _, line := range strings.Split(mergedBody, "\n") {
		if line == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, strings.Repeat(" ", canonicalIndent)+line)
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func mergeYAMLMappingContent(existingYAML, canonicalYAML string) (string, bool) {
	existingPreamble, existingBlocks := parseTopLevelYAMLBlocks(existingYAML)
	canonicalPreamble, canonicalBlocks := parseTopLevelYAMLBlocks(canonicalYAML)
	if len(canonicalBlocks) == 0 {
		return "", false
	}
	existingByKey := make(map[string]string, len(existingBlocks))
	canonicalKeys := make(map[string]struct{}, len(canonicalBlocks))
	for _, block := range existingBlocks {
		existingByKey[block.key] = block.text
	}

	parts := make([]string, 0, len(existingBlocks)+len(canonicalBlocks)+1)
	preamble := canonicalPreamble
	if preamble == "" {
		preamble = existingPreamble
	}
	if preamble != "" {
		parts = append(parts, preamble)
	}
	for _, block := range canonicalBlocks {
		canonicalKeys[block.key] = struct{}{}
		if existing, ok := existingByKey[block.key]; ok {
			parts = append(parts, mergeYAMLBlocks(existing, block.text))
		} else {
			parts = append(parts, block.text)
		}
	}
	for _, block := range existingBlocks {
		if _, ok := canonicalKeys[block.key]; !ok {
			parts = append(parts, block.text)
		}
	}
	return strings.TrimRight(strings.Join(parts, "\n"), "\n"), true
}

func mergeFrontmatter(existingFrontmatter, canonicalFrontmatter string) string {
	switch {
	case canonicalFrontmatter == "":
		return existingFrontmatter
	case existingFrontmatter == "":
		return canonicalFrontmatter
	}

	existingYAML := extractFrontmatterYAML(existingFrontmatter)
	canonicalYAML := extractFrontmatterYAML(canonicalFrontmatter)
	if existingYAML == "" || canonicalYAML == "" {
		return existingFrontmatter
	}
	mergedYAML, ok := mergeYAMLMappingContent(existingYAML, canonicalYAML)
	if !ok {
		return existingFrontmatter
	}
	return "---\n" + mergedYAML + "\n---"
}

func parseMarkdownBody(content string) (string, []markdownSection) {
	lines := strings.Split(normalizeLineEndings(content), "\n")
	type pos struct {
		index   int
		heading string
	}
	var headings []pos
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			headings = append(headings, pos{index: i, heading: line})
		}
	}
	if len(headings) == 0 {
		return strings.TrimSpace(content), nil
	}
	intro := strings.TrimSpace(strings.Join(lines[:headings[0].index], "\n"))
	sections := make([]markdownSection, 0, len(headings))
	for i, item := range headings {
		end := len(lines)
		if i+1 < len(headings) {
			end = headings[i+1].index
		}
		text := strings.TrimSpace(strings.Join(lines[item.index:end], "\n"))
		sections = append(sections, markdownSection{heading: item.heading, text: text})
	}
	return intro, sections
}

func mergeMarkdownBodies(existing, canonical string) string {
	if strings.TrimSpace(existing) == "" {
		return normalizeLineEndings(canonical)
	}
	existingIntro, existingSections := parseMarkdownBody(existing)
	canonicalIntro, canonicalSections := parseMarkdownBody(canonical)
	existingByHeading := make(map[string]string, len(existingSections))
	canonicalHeadings := make(map[string]struct{}, len(canonicalSections))
	for _, section := range existingSections {
		existingByHeading[section.heading] = section.text
	}
	for _, section := range canonicalSections {
		canonicalHeadings[section.heading] = struct{}{}
	}
	parts := make([]string, 0, len(existingSections)+len(canonicalSections)+1)
	intro := existingIntro
	if intro == "" {
		intro = canonicalIntro
	}
	if intro != "" {
		parts = append(parts, intro)
	}
	for _, section := range canonicalSections {
		if existingText, ok := existingByHeading[section.heading]; ok {
			parts = append(parts, existingText)
		} else {
			parts = append(parts, section.text)
		}
	}
	for _, section := range existingSections {
		if _, ok := canonicalHeadings[section.heading]; !ok {
			parts = append(parts, section.text)
		}
	}
	return strings.TrimRight(strings.Join(parts, "\n\n"), "\n") + "\n"
}

func patchRuleContent(existing, canonical, target string) string {
	if strings.TrimSpace(existing) == "" {
		return normalizeLineEndings(canonical)
	}
	if target == "cursor" || target == "opencode" {
		existingFrontmatter, existingBody := splitFrontmatterDocument(existing)
		canonicalFrontmatter, canonicalBody := splitFrontmatterDocument(canonical)
		frontmatter := mergeFrontmatter(existingFrontmatter, canonicalFrontmatter)
		body := mergeMarkdownBodies(existingBody, canonicalBody)
		switch {
		case frontmatter != "":
			return frontmatter + "\n\n" + body
		default:
			return body
		}
	}
	return mergeMarkdownBodies(existing, canonical)
}

func findSectionRange(content, heading string) (int, int, bool) {
	normalized := normalizeLineEndings(content)
	lines := strings.SplitAfter(normalized, "\n")
	position := 0
	start := -1
	end := len(normalized)
	marker := "## " + heading
	inCodeFence := false

	for index, line := range lines {
		trimmed := strings.TrimSuffix(line, "\n")
		if strings.HasPrefix(strings.TrimSpace(trimmed), "```") {
			inCodeFence = !inCodeFence
		}
		if !inCodeFence && start < 0 {
			if trimmed == marker {
				start = position
			}
		} else if !inCodeFence && strings.HasPrefix(trimmed, "## ") {
			end = position
			return start, end, true
		}
		position += len(line)
		if index == len(lines)-1 && start >= 0 {
			return start, end, true
		}
	}
	return 0, 0, false
}

func patchCodexAgentsMD(existing, canonical string) string {
	if strings.TrimSpace(existing) == "" {
		return normalizeLineEndings(canonical)
	}
	existing = normalizeLineEndings(existing)
	canonical = normalizeLineEndings(canonical)
	current := existing
	for _, heading := range []string{"Installed agent rules", "Installed skills"} {
		canonicalStart, canonicalEnd, ok := findSectionRange(canonical, heading)
		if !ok {
			continue
		}
		canonicalSection := strings.TrimRight(canonical[canonicalStart:canonicalEnd], "\n")
		existingStart, existingEnd, ok := findSectionRange(current, heading)
		if !ok {
			current = strings.TrimRight(current, "\n") + "\n\n" + canonicalSection + "\n"
			continue
		}
		current = strings.TrimRight(current[:existingStart], "\n") +
			"\n\n" +
			canonicalSection +
			"\n\n" +
			strings.TrimLeft(current[existingEnd:], "\n") + "\n"
	}
	return current
}

func buildContent(agentID, target, language, suffix, hookMode string) (string, error) {
	content, err := readContent(agentID, language, suffix, hookMode)
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

func readContent(agentID, language, suffix, hookMode string) (string, error) {
	name := "content.md"
	if suffix != "" {
		name = "content-" + suffix + ".md"
	}
	bytes, err := readAgentFile(path.Join(agentDir(agentID, language), name))
	if err != nil {
		return "", fmt.Errorf("agent %q has no %s", agentID, name)
	}
	content := string(bytes)
	if agentID != "git-hooks" || !strings.Contains(content, gitHooksGuidanceToken) {
		return content, nil
	}
	return strings.ReplaceAll(content, gitHooksGuidanceToken, renderGitHooksGuidance(language, hookMode)), nil
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

func renderGitHooksGuidance(language, hookMode string) string {
	switch language {
	case "typescript":
		if hookMode == "monorepo" {
			return strings.Join([]string{
				"- Use Husky for this monorepo.",
				"- Install and initialize Husky.",
				"- Create `.husky/pre-commit` with the repo's fast lint command, such as `npx lint-staged`.",
				"- Create `.husky/pre-push` with the repo's unit test command, and for TypeScript monorepos run the build before the tests when the test command depends on generated output.",
				"- Keep the hook file executable with `chmod +x .husky/pre-commit`.",
				"- Keep `.husky/pre-push` executable with `chmod +x .husky/pre-push`.",
				"- Keep the hook in sync with the repo's linting workflow whenever the command changes.",
			}, "\n")
		}
		return strings.Join([]string{
			"- Use `pre-commit` for this repository layout.",
			"- Create `.pre-commit-config.yaml` at the repo root.",
			"- Install hooks with `pre-commit install`.",
			"- Install the pre-push hook with `pre-commit install --hook-type pre-push`.",
			"- Configure `.pre-commit-config.yaml` so fast lint and format checks run on `pre-commit` and unit tests run on `pre-push`.",
			"- Keep the configuration current with `pre-commit autoupdate`.",
			"- Verify the hook configuration with `pre-commit run --all-files`.",
		}, "\n")
	case "python":
		return strings.Join([]string{
			"- Use `pre-commit` for Python projects.",
			"- Create `.pre-commit-config.yaml` at the repo root.",
			"- Install hooks with `pre-commit install`.",
			"- Install the pre-push hook with `pre-commit install --hook-type pre-push`.",
			"- Configure `.pre-commit-config.yaml` so unit tests run on `pre-push`.",
			"- Keep the configuration current with `pre-commit autoupdate`.",
			"- Re-run `pre-commit run --all-files` after hook changes.",
		}, "\n")
	case "go":
		return strings.Join([]string{
			"- Use `pre-commit` for Go projects, and fan out to language-local configs with `sub-pre-commit` when needed.",
			"- Create or update `.pre-commit-config.yaml` at the repo root.",
			"- Use `sub-pre-commit` hooks to invoke nested `.pre-commit-config.yaml` files in Go subprojects.",
			"- Install hooks with `pre-commit install` and `pre-commit install --hook-type pre-push`.",
			"- Configure the pre-push stage to run Go unit tests for each module.",
			"- Keep the configuration current with `pre-commit autoupdate`.",
			"- Verify the hook configuration with `pre-commit run --all-files`.",
		}, "\n")
	case "ansible":
		return strings.Join([]string{
			"- Use `pre-commit` for Ansible repositories.",
			"- Create or update `.pre-commit-config.yaml` at the repo root.",
			"- Install hooks with `pre-commit install`.",
			"- Install the pre-push hook with `pre-commit install --hook-type pre-push`.",
			"- Run `ansible-lint`, `yamllint`, and `ansible-playbook --syntax-check` from the hook configuration.",
			"- Keep secrets out of logs and commits; prefer Ansible Vault or external secret stores.",
			"- Keep the configuration current with `pre-commit autoupdate`.",
		}, "\n")
	case "terraform":
		return strings.Join([]string{
			"- Use `pre-commit` for Terraform repositories.",
			"- Create or update `.pre-commit-config.yaml` at the repo root.",
			"- Commit `.terraform-version` and use `tfenv install` plus `tfenv use` before running Terraform commands.",
			"- Install hooks with `pre-commit install`.",
			"- Install the pre-push hook with `pre-commit install --hook-type pre-push`.",
			"- Run `terraform fmt -check -recursive`, `terraform validate`, `tflint`, and `tfsec` from the hook configuration.",
			"- Keep `.terraform/`, state files, and plan files out of Git.",
			"- Keep the configuration current with `pre-commit autoupdate`.",
		}, "\n")
	default:
		return ""
	}
}

func resolveTsHookMode(projectRoot, language string) string {
	if language != "typescript" {
		return "standalone"
	}

	configPath := filepath.Join(projectRoot, ".rulesrc.json")
	if exists(configPath) {
		var raw struct {
			Languages []string            `json:"languages"`
			Paths     map[string][]string `json:"paths"`
		}
		if content, err := os.ReadFile(configPath); err == nil {
			if err := json.Unmarshal(content, &raw); err == nil {
				if len(raw.Languages) > 1 || len(raw.Paths) > 1 {
					return "monorepo"
				}
			}
		}
	}

	if hasWorkspaceMonorepo(projectRoot) {
		return "monorepo"
	}
	return "standalone"
}

func hasWorkspaceMonorepo(projectRoot string) bool {
	root := filepath.Clean(projectRoot)
	if !exists(filepath.Join(root, "pnpm-workspace.yaml")) {
		rootPackageJSON := filepath.Join(root, "package.json")
		if !exists(rootPackageJSON) {
			return false
		}
		var raw map[string]any
		content, err := os.ReadFile(rootPackageJSON)
		if err != nil {
			return false
		}
		if err := json.Unmarshal(content, &raw); err != nil {
			return false
		}
		if _, ok := raw["workspaces"]; !ok {
			return false
		}
	}

	ignored := map[string]bool{
		".git":         true,
		"node_modules": true,
		"dist":         true,
		"build":        true,
		"coverage":     true,
		".next":        true,
		".turbo":       true,
		".pnpm-store":  true,
	}

	count := 0
	var walk func(string, int) bool
	walk = func(dir string, depth int) bool {
		if depth > 4 {
			return false
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				if ignored[name] {
					continue
				}
				if walk(filepath.Join(dir, name), depth+1) {
					return true
				}
				continue
			}
			if name == "package.json" {
				count++
				if count > 1 {
					return true
				}
			}
		}
		return false
	}

	return walk(root, 0)
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
		if exists(filepath.Join(dir, "package.json")) ||
			exists(filepath.Join(dir, "go.mod")) ||
			exists(filepath.Join(dir, "pyproject.toml")) ||
			exists(filepath.Join(dir, "ansible.cfg")) ||
			exists(filepath.Join(dir, "site.yml")) ||
			exists(filepath.Join(dir, "playbook.yml")) ||
			exists(filepath.Join(dir, "requirements.yml")) ||
			exists(filepath.Join(dir, "requirements.yaml")) ||
			exists(filepath.Join(dir, ".terraform-version")) ||
			exists(filepath.Join(dir, "main.tf")) ||
			exists(filepath.Join(dir, "providers.tf")) ||
			exists(filepath.Join(dir, "versions.tf")) ||
			exists(filepath.Join(dir, "terraform.tf")) ||
			hasAnyRulesConfig(dir) {
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
	if exists(filepath.Join(dir, ".rulesrc.json")) || exists(filepath.Join(dir, ".rulesrc.ts.json")) {
		return true
	}
	for _, language := range languages {
		if exists(filepath.Join(dir, legacyRulesrcFilename(language))) {
			return true
		}
	}
	return false
}

func loadConfig(projectRoot, language string) *rulesConfig {
	file := filepath.Join(projectRoot, rulesrcFilename(language))
	if !exists(file) {
		file = filepath.Join(projectRoot, legacyRulesrcFilename(language))
	}
	if !exists(file) {
		return nil
	}
	bytes, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var raw rawRulesConfig
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return nil
	}
	targets := normalizeTargets(append(append([]string{}, raw.Targets...), raw.Target))
	if len(targets) == 0 || (len(raw.Agents) == 0 && len(raw.Skills) == 0) {
		return nil
	}
	return &rulesConfig{
		Targets:        targets,
		Agents:         raw.Agents,
		Skills:         raw.Skills,
		BallastVersion: raw.BallastVersion,
		Languages:      raw.Languages,
		Paths:          raw.Paths,
	}
}

func saveConfig(projectRoot, language string, cfg rulesConfig) error {
	filePath := filepath.Join(projectRoot, rulesrcFilename(language))
	existing := loadConfig(projectRoot, language)
	cfg.Targets = normalizeTargets(cfg.Targets)
	if strings.TrimSpace(cfg.BallastVersion) == "" {
		cfg.BallastVersion = resolveVersion()
	}
	if existing != nil {
		if cfg.BallastVersion == "" {
			cfg.BallastVersion = existing.BallastVersion
		}
		cfg.Targets = mergeStringLists(existing.Targets, cfg.Targets)
		cfg.Languages = mergeLanguageList(existing.Languages, cfg.Languages)
		cfg.Paths = mergeLanguagePaths(existing.Paths, cfg.Languages)
	} else {
		cfg.Paths = mergeLanguagePaths(nil, cfg.Languages)
	}
	bytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, bytes, 0o644)
}

func ensureGitignoreEntry(projectRoot, entry string) error {
	normalized := strings.TrimSpace(entry)
	if normalized == "" {
		return nil
	}
	gitignorePath := filepath.Join(projectRoot, ".gitignore")
	if !exists(gitignorePath) {
		return os.WriteFile(gitignorePath, []byte(normalized+"\n"), 0o644)
	}
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		return err
	}
	lines := strings.Split(normalizeLineEndings(string(content)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == normalized {
			return nil
		}
	}
	separator := ""
	if len(content) > 0 && !strings.HasSuffix(string(content), "\n") {
		separator = "\n"
	}
	return os.WriteFile(gitignorePath, append(content, []byte(separator+normalized+"\n")...), 0o644)
}

func mergeLanguageList(existing, incoming []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	merged := make([]string, 0, len(existing)+len(incoming))
	for _, language := range existing {
		if _, ok := seen[language]; ok {
			continue
		}
		seen[language] = struct{}{}
		merged = append(merged, language)
	}
	for _, language := range incoming {
		if _, ok := seen[language]; ok {
			continue
		}
		seen[language] = struct{}{}
		merged = append(merged, language)
	}
	return merged
}

func mergeStringLists(existing, incoming []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	merged := make([]string, 0, len(existing)+len(incoming))
	for _, item := range existing {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		merged = append(merged, item)
	}
	for _, item := range incoming {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		merged = append(merged, item)
	}
	return merged
}

func mergeLanguagePaths(existing map[string][]string, languages []string) map[string][]string {
	merged := make(map[string][]string, len(existing)+len(languages))
	for key, paths := range existing {
		merged[key] = append([]string(nil), paths...)
	}
	for _, language := range languages {
		if len(merged[language]) == 0 {
			merged[language] = []string{"."}
		}
	}
	return merged
}

func rulesrcFilename(language string) string {
	return ".rulesrc.json"
}

func legacyRulesrcFilename(language string) string {
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

func promptTargets() ([]string, error) {
	allowed := strings.Join(targets, ", ")
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("AI platforms (comma-separated) [%s]: ", allowed)
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, os.ErrClosed) {
			if len(strings.TrimSpace(line)) == 0 {
				return nil, err
			}
		}
		trimmed := strings.TrimSpace(line)
		if strings.EqualFold(trimmed, "all") {
			return append([]string(nil), targets...), nil
		}
		resolved, invalid := normalizeTargetsDetailed(splitTargets(trimmed))
		if len(resolved) > 0 && len(invalid) == 0 {
			return resolved, nil
		}
		fmt.Printf("Invalid targets. Use comma-separated values from: %s\n", allowed)
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

func promptSkills(language string) ([]string, error) {
	allowed := listSkills(language)
	if len(allowed) == 0 {
		return nil, nil
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Skills (comma-separated, \"all\", or blank for none) [%s]: ", strings.Join(allowed, ", "))
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, os.ErrClosed) {
			if len(strings.TrimSpace(line)) == 0 {
				return nil, err
			}
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			return nil, nil
		}
		resolved := resolveSkills(splitCSV(trimmed), language)
		if len(resolved) > 0 {
			return resolved, nil
		}
		fmt.Printf("Invalid skills. Use \"all\" or comma-separated: %s\n", strings.Join(allowed, ", "))
	}
}

func promptYesNo(question string, defaultAnswer bool) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	suffix := " [y/N]: "
	if defaultAnswer {
		suffix = " [Y/n]: "
	}
	fmt.Print(question + suffix)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, os.ErrClosed) {
		if len(strings.TrimSpace(line)) == 0 {
			return false, err
		}
	}
	value := strings.ToLower(strings.TrimSpace(line))
	if value == "" {
		return defaultAnswer, nil
	}
	return value == "y" || value == "yes", nil
}

func splitAgents(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return splitCSV(raw)
}

func splitTargets(raw string) []string {
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

func normalizeTargets(values []string) []string {
	normalized, _ := normalizeTargetsDetailed(values)
	return normalized
}

func normalizeTargetsDetailed(values []string) ([]string, []string) {
	seen := map[string]struct{}{}
	invalidSeen := map[string]struct{}{}
	normalized := make([]string, 0, len(values))
	invalid := make([]string, 0)
	for _, raw := range values {
		for _, item := range splitTargets(raw) {
			target := strings.ToLower(strings.TrimSpace(item))
			if target == "" {
				continue
			}
			if !contains(targets, target) {
				if _, ok := invalidSeen[target]; !ok {
					invalidSeen[target] = struct{}{}
					invalid = append(invalid, target)
				}
				continue
			}
			if _, ok := seen[target]; ok {
				continue
			}
			seen[target] = struct{}{}
			normalized = append(normalized, target)
		}
	}
	return normalized, invalid
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

func resolveSkills(tokens []string, language string) []string {
	if len(tokens) == 0 {
		return nil
	}
	for _, token := range tokens {
		if token == "all" {
			return listSkills(language)
		}
	}
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if isValidSkill(token, language) {
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

func listSkills(_ string) []string {
	return append([]string{}, commonSkills...)
}

func isValidAgent(agentID, language string) bool {
	for _, agent := range listAgents(language) {
		if agent == agentID {
			return true
		}
	}
	return false
}

func isValidSkill(skillID, language string) bool {
	for _, skill := range listSkills(language) {
		if skill == skillID {
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

func skillDir(skillID, language string) string {
	if contains(commonSkills, skillID) {
		return path.Join("skills", "common", skillID)
	}
	return path.Join("skills", language, skillID)
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

func readSkillFile(relativePath string) ([]byte, error) {
	if overrideRoot := repoRootOverride(); overrideRoot != "" {
		return os.ReadFile(filepath.Join(overrideRoot, filepath.FromSlash(relativePath)))
	}
	return fs.ReadFile(embeddedSkillsFS, relativePath)
}

func existsSkillFile(relativePath string) bool {
	if overrideRoot := repoRootOverride(); overrideRoot != "" {
		return exists(filepath.Join(overrideRoot, filepath.FromSlash(relativePath)))
	}
	_, err := fs.Stat(embeddedSkillsFS, relativePath)
	return err == nil
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

func destination(projectRoot, target, basename string) (string, string, error) {
	ruleSubdir, err := validatedRuleSubdir()
	if err != nil {
		return "", "", err
	}
	scopedBasename := basename
	if ruleSubdir != "" && ruleSubdir != "common" && !strings.HasPrefix(basename, ruleSubdir+"-") {
		scopedBasename = ruleSubdir + "-" + basename
	}
	switch target {
	case "cursor":
		dir := filepath.Join(projectRoot, ".cursor", "rules")
		if ruleSubdir != "" {
			dir = filepath.Join(dir, ruleSubdir)
		}
		return dir, filepath.Join(dir, scopedBasename+".mdc"), nil
	case "claude":
		dir := filepath.Join(projectRoot, ".claude", "rules")
		if ruleSubdir != "" {
			dir = filepath.Join(dir, ruleSubdir)
		}
		return dir, filepath.Join(dir, scopedBasename+".md"), nil
	case "opencode":
		dir := filepath.Join(projectRoot, ".opencode")
		if ruleSubdir != "" {
			dir = filepath.Join(dir, ruleSubdir)
		}
		return dir, filepath.Join(dir, scopedBasename+".md"), nil
	default:
		dir := filepath.Join(projectRoot, ".codex", "rules")
		if ruleSubdir != "" {
			dir = filepath.Join(dir, ruleSubdir)
		}
		return dir, filepath.Join(dir, scopedBasename+".md"), nil
	}
}

func skillDestination(projectRoot, target, skillID string) (string, string, error) {
	root := filepath.Clean(projectRoot)
	switch target {
	case "cursor":
		dir := filepath.Join(root, ".cursor", "rules")
		return dir, filepath.Join(dir, skillID+".mdc"), nil
	case "claude":
		dir := filepath.Join(root, ".claude", "skills")
		return dir, filepath.Join(dir, skillID+".skill"), nil
	case "opencode":
		dir := filepath.Join(root, ".opencode", "skills")
		return dir, filepath.Join(dir, skillID+".md"), nil
	case "codex":
		dir := filepath.Join(root, ".codex", "rules")
		return dir, filepath.Join(dir, skillID+".md"), nil
	default:
		return "", "", fmt.Errorf("unknown target: %s", target)
	}
}

func validatedRuleSubdir() (string, error) {
	ruleSubdir := strings.TrimSpace(os.Getenv("BALLAST_RULE_SUBDIR"))
	if ruleSubdir == "" {
		return "", nil
	}
	if matched := regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(ruleSubdir); !matched {
		return "", fmt.Errorf("invalid BALLAST_RULE_SUBDIR %q: only [A-Za-z0-9_-] are allowed", ruleSubdir)
	}
	return ruleSubdir, nil
}

func ruleBaseName(agentID, language, suffix string) string {
	base := agentID
	if suffix != "" {
		base = agentID + "-" + suffix
	}
	if slices.Contains(commonAgents, agentID) {
		return base
	}
	return language + "-" + base
}

func codexAgentsMDPath(projectRoot string) string {
	return filepath.Join(projectRoot, "AGENTS.md")
}

func claudeMDPath(projectRoot string) string {
	return filepath.Join(projectRoot, "CLAUDE.md")
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

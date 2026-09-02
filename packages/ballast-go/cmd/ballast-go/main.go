package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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

	"golang.org/x/term"
)

var (
	targets   = []string{"cursor", "claude", "opencode", "codex", "gemini"}
	languages = []string{"typescript", "python", "go", "ansible", "terraform", "dart", "docker"}
)

var isStdinInteractiveFunc = isStdinInteractive

var (
	commonAgents   = []string{"local-dev", "docs", "cicd", "observability", "publishing", "git-hooks", "tasks", "plan-lifecycle", "spec-kit"}
	languageAgents = []string{"linting", "logging", "testing"}
	commonSkills   = []string{
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
)

var (
	descriptionRegex             = regexp.MustCompile(`(?m)^description:\s*['\"]?(.+?)['\"]?\s*$`)
	ballastVersion               = "dev"
	frontmatterRegex             = regexp.MustCompile(`(?s)^\s*---\n(.*?)\n---\n?`)
	topLevelYAMLKeyRegex         = regexp.MustCompile(`^([A-Za-z0-9_-]+):(.*)$`)
	gitHooksGuidanceToken        = "{{BALLAST_GIT_HOOKS_GUIDANCE}}"
	gitHooksPreCommitGlobToken   = "{{BALLAST_GIT_HOOKS_PRE_COMMIT_GLOB}}"
	deploymentModelGuidanceToken = "{{BALLAST_DEPLOYMENT_MODEL_GUIDANCE}}"
	taskSystemGuidanceToken      = "{{BALLAST_TASK_SYSTEM_GUIDANCE}}"
	taskSystemToken              = "{{taskSystem}}"
	taskSystems                  = []string{"github", "jira", "linear", "none"}
	deploymentModels             = []string{"none", "kubernetes", "serverless", "server", "docker", "hosted"}
	publishingProfiles           = []string{"cli", "apps", "web", "api", "libraries", "sdks", "apt", "brew"}
	publishingProfileAliases     = map[string]string{
		"app":     "apps",
		"library": "libraries",
		"sdk":     "sdks",
	}
)

var ruleMarkerRegex = regexp.MustCompile(`^<!-- ballast:rule\s+id="([^"]+)"\s+version="([^"]+)"\s+checksum="([a-fA-F0-9]{64})"\s*-->\r?\n?`)

type ruleMarker struct {
	ruleID   string
	version  string
	checksum string
}

var defaultLanguageTools = map[string][]string{
	"python":     {"uv", "pyenv"},
	"typescript": {"pnpm", "corepack"},
	"go":         {"go", "gofumpt", "golangci-lint"},
	"terraform":  {"tfenv", "tflint", "trivy"},
	"ansible":    {"ansible-lint", "molecule"},
	"dart":       {"flutter", "fvm"},
	"docker":     {"docker", "hadolint", "trivy"},
}

func withImplicitAgents(agents []string) []string {
	resolved := slices.Clone(agents)
	if contains(resolved, "linting") && !contains(resolved, "git-hooks") {
		resolved = append(resolved, "git-hooks")
	}
	return resolved
}

func uniqueStrings(s []string) []string {
	seen := make(map[string]struct{}, len(s))
	out := make([]string, 0, len(s))
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

//go:embed agents/**
var embeddedAgentsFS embed.FS

//go:embed skills/**
var embeddedSkillsFS embed.FS

type rulesConfig struct {
	Targets            []string            `json:"targets,omitempty"`
	Agents             []string            `json:"agents"`
	Skills             []string            `json:"skills,omitempty"`
	BallastVersion     string              `json:"ballastVersion,omitempty"`
	Languages          []string            `json:"languages,omitempty"`
	Paths              map[string][]string `json:"paths,omitempty"`
	Tools              map[string][]string `json:"tools,omitempty"`
	Discovery          *discoveryConfig    `json:"discovery,omitempty"`
	TaskSystem         string              `json:"taskSystem,omitempty"`
	DeploymentModel    string              `json:"deploymentModel,omitempty"`
	PublishingProfiles []string            `json:"publishingProfiles,omitempty"`
}

type discoveryConfig struct {
	ExcludePaths []string `json:"excludePaths,omitempty"`
}

type rawRulesConfig struct {
	Target             string              `json:"target,omitempty"`
	Targets            []string            `json:"targets,omitempty"`
	Agents             []string            `json:"agents,omitempty"`
	Skills             []string            `json:"skills,omitempty"`
	BallastVersion     string              `json:"ballastVersion,omitempty"`
	Languages          []string            `json:"languages,omitempty"`
	Paths              map[string][]string `json:"paths,omitempty"`
	Tools              map[string][]string `json:"tools,omitempty"`
	Discovery          json.RawMessage     `json:"discovery,omitempty"`
	TaskSystem         string              `json:"taskSystem,omitempty"`
	DeploymentModel    string              `json:"deploymentModel,omitempty"`
	PublishingProfiles []string            `json:"publishingProfiles,omitempty"`
}

type installResult struct {
	installed            []string
	installedRules       []installedRule
	installedSkills      []string
	installedSupport     []string
	skipped              []string
	skippedSupportFiles  []string
	declinedSupportFiles []string
	errors               []agentError
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
	projectRoot     string
	targets         []string
	agents          []string
	skills          []string
	all             bool
	allSkills       bool
	yes             bool
	language        string
	taskSystem      string
	deploymentModel string
}

type installOptions struct {
	projectRoot     string
	targets         []string
	agents          []string
	skills          []string
	language        string
	force           bool
	patch           bool
	patchClaude     bool
	patchGemini     bool
	skipSupport     map[string]struct{}
	saveConfig      bool
	taskSystem      string
	deploymentModel string
}

type buildOptions struct {
	tools map[string][]string
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
	fs.Var(&targetFlags, "target", "cursor|claude|opencode|codex|gemini")
	fs.Var(&targetFlags, "t", "cursor|claude|opencode|codex|gemini")
	language := fs.String("language", "go", "typescript|python|go|ansible|terraform|dart")
	fs.StringVar(language, "l", "go", "typescript|python|go|ansible|terraform|dart")
	agent := fs.String("agent", "", "comma-separated list")
	fs.StringVar(agent, "a", "", "comma-separated list")
	skill := fs.String("skill", "", "comma-separated list")
	fs.StringVar(skill, "s", "", "comma-separated list")
	all := fs.Bool("all", false, "install all agents")
	allSkills := fs.Bool("all-skills", false, "install all skills")
	force := fs.Bool("force", false, "overwrite existing rule and skill files; prompts before replacing support files")
	patch := fs.Bool("patch", false, "merge upstream rule and skill updates into existing files")
	fs.BoolVar(patch, "p", false, "merge upstream rule and skill updates into existing files")
	taskSystemFlag := fs.String("task-system", "", "task system for tasks: github|jira|linear|none")
	deploymentModelFlag := fs.String("deployment-model", "", "app/service deployment model for publishing; use none for CLI/library/SDK-only projects: none|kubernetes|serverless|server|docker|hosted")
	yes := fs.Bool("yes", false, "non-interactive mode")
	fs.BoolVar(yes, "y", false, "non-interactive mode")
	repositoryFactsFile := fs.String("repository-facts-file", "", "optional path to wrapper-generated repository facts JSON")
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
	if strings.TrimSpace(*repositoryFactsFile) != "" {
		_ = os.Setenv("BALLAST_REPOSITORY_FACTS_FILE", strings.TrimSpace(*repositoryFactsFile))
	}
	if !contains(languages, lang) {
		fmt.Printf("Invalid --language. Use: %s\n", strings.Join(languages, ", "))
		return 1
	}
	taskSystem := normalizeRequiredInstallOptionValue(*taskSystemFlag)
	if taskSystem != "" && !contains(taskSystems, taskSystem) {
		fmt.Printf("Invalid --task-system. Use: %s\n", strings.Join(taskSystems, ", "))
		return 1
	}
	deploymentModel := normalizeDeploymentModel(*deploymentModelFlag)
	if deploymentModel != "" && !contains(deploymentModels, deploymentModel) {
		fmt.Printf("Invalid --deployment-model. Use: %s\n", strings.Join(deploymentModels, ", "))
		return 1
	}

	root, err := findProjectRoot("")
	if err != nil {
		fmt.Println(err)
		return 1
	}

	resolved, err := resolveTargetAndAgents(resolveOptions{
		projectRoot:     root,
		targets:         targetFlags,
		agents:          splitAgents(*agent),
		skills:          splitCSV(*skill),
		all:             *all,
		allSkills:       *allSkills,
		yes:             *yes,
		language:        lang,
		taskSystem:      taskSystem,
		deploymentModel: deploymentModel,
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

	skippedSupport := map[string]struct{}{}
	for _, target := range resolved.Targets {
		supportPath := supportFilePath(root, target)
		if supportPath == "" || !*force || !exists(supportPath) {
			continue
		}
		if os.Getenv("BALLAST_DISABLE_SUPPORT_FILES") == "1" {
			continue
		}
		if *yes || isCIMode() || !isStdinInteractiveFunc() {
			fmt.Printf("Cannot overwrite existing support file %s in non-interactive mode. Re-run from an interactive terminal without --yes to confirm the destructive overwrite.\n", filepath.Base(supportPath))
			return 1
		}
		approved, promptErr := promptYesNo(
			fmt.Sprintf(
				"\n⚠️  %s already exists and may contain your customizations.\n    --force will replace it with the canonical template.\n    Your current file will be lost.\n\n    Continue?",
				filepath.Base(supportPath),
			),
			false,
		)
		if promptErr != nil {
			fmt.Println(promptErr)
			return 1
		}
		if !approved {
			skippedSupport[supportPath] = struct{}{}
			fmt.Printf("Skipped support file: %s (%s)\n", filepath.Base(supportPath), supportPath)
		}
	}
	result := install(installOptions{
		projectRoot:     root,
		targets:         resolved.Targets,
		agents:          resolved.Agents,
		skills:          resolved.Skills,
		language:        lang,
		force:           *force,
		patch:           *patch,
		patchClaude:     false,
		patchGemini:     false,
		skipSupport:     skippedSupport,
		saveConfig:      true,
		taskSystem:      resolved.TaskSystem,
		deploymentModel: resolved.DeploymentModel,
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
			fmt.Printf("  %s -> %s\n", filepath.Base(file), file)
		}
	}
	if len(result.skipped) > 0 {
		fmt.Printf("Skipped (already present; use --force to overwrite): %s\n", strings.Join(result.skipped, ", "))
	}
	if len(result.declinedSupportFiles) > 0 {
		fmt.Printf(
			"Skipped support files (overwrite declined): %s\n",
			strings.Join(result.declinedSupportFiles, ", "),
		)
	}
	if len(result.installed) == 0 && len(result.installedSkills) == 0 && len(result.skipped) == 0 && len(result.errors) == 0 {
		fmt.Println("Nothing to install.")
	}

	return 0
}

func supportFilePath(projectRoot, target string) string {
	switch target {
	case "codex":
		return codexAgentsMDPath(projectRoot)
	case "claude":
		return claudeMDPath(projectRoot)
	default:
		return ""
	}
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
  --agent, -a <agents>      Agent(s): linting, local-dev, docs, cicd, observability, publishing, git-hooks, tasks, logging, testing (comma-separated)
  --skill, -s <skills>      Skill(s): owasp-security-scan, aws-health-review, aws-live-health-review, aws-weekly-security-review, github-health-check, github-pr-copilot-cycle, ballast-audit, ballast-project-maintenance (comma-separated)
  --all                     Install all agents
  --all-skills              Install all skills
  --task-system <system>    Task system for tasks: %s
  --deployment-model <mode> App/service deployment model for publishing; use none for CLI/library/SDK-only projects: %s
  --force                   Overwrite existing rule/skill files; prompts before replacing AGENTS.md, CLAUDE.md, or GEMINI.md
  --patch, -p               Merge upstream rule/skill updates into existing files; ignored when --force is set
  --yes, -y                 Non-interactive; require --target and --agent/--all if no .rulesrc.json
  --repository-facts-file   Optional path to wrapper-generated repository facts JSON
  --help, -h                Show this help
  --version, -v             Show version

Examples:
  ballast-go install
  ballast-go install --target cursor --agent linting
  ballast-go install --target claude --skill owasp-security-scan
  ballast-go install --target codex --skill aws-health-review
  ballast-go install --target codex --agent publishing --deployment-model kubernetes
  ballast-go install --target cursor,claude,gemini --agent linting
  ballast-go install --language python --target cursor --all
  ballast-go install --target claude --all --force
  ballast-go install --target cursor --agent linting --patch
  ballast-go install --yes --target cursor --target codex --all
`, resolveVersion(), strings.Join(targets, ", "), strings.Join(languages, ", "), strings.Join(taskSystems, ", "), strings.Join(deploymentModels, ", "))
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
		if len(config.Languages) > 0 {
			lines = append(lines, fmt.Sprintf("- languages: %s", strings.Join(config.Languages, ", ")))
		}
		if formattedPaths := formatDoctorConfigPaths(config.Languages, config.Paths); formattedPaths != "" {
			lines = append(lines, fmt.Sprintf("- paths: %s", formattedPaths))
		}
		if formattedTools := formatDoctorConfigPaths(config.Languages, config.Tools); formattedTools != "" {
			lines = append(lines, fmt.Sprintf("- tools: %s", formattedTools))
		}
		if config.Discovery != nil && len(config.Discovery.ExcludePaths) > 0 {
			lines = append(lines, fmt.Sprintf("- discovery.excludePaths: %s", strings.Join(config.Discovery.ExcludePaths, ",")))
		}
		if strings.TrimSpace(config.TaskSystem) != "" {
			lines = append(lines, fmt.Sprintf("- taskSystem: %s", config.TaskSystem))
		}
		if strings.TrimSpace(config.DeploymentModel) != "" {
			lines = append(lines, fmt.Sprintf("- deploymentModel: %s", config.DeploymentModel))
		}
		if len(config.PublishingProfiles) > 0 {
			lines = append(lines, fmt.Sprintf("- publishingProfiles: %s", strings.Join(config.PublishingProfiles, ", ")))
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

func formatDoctorConfigPaths(languages []string, paths map[string][]string) string {
	orderedKeys := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, language := range languages {
		if len(paths[language]) == 0 {
			continue
		}
		orderedKeys = append(orderedKeys, language)
		seen[language] = true
	}
	remaining := make([]string, 0, len(paths))
	for language, values := range paths {
		if seen[language] || len(values) == 0 {
			continue
		}
		remaining = append(remaining, language)
	}
	sort.Strings(remaining)
	orderedKeys = append(orderedKeys, remaining...)
	entries := make([]string, 0, len(orderedKeys))
	for _, language := range orderedKeys {
		entries = append(entries, fmt.Sprintf("%s=%s", language, strings.Join(paths[language], ",")))
	}
	return strings.Join(entries, "; ")
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
	promptReader := bufio.NewReader(os.Stdin)

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
		if contains(next.Agents, "tasks") {
			taskSystem, err := resolveRequiredInstallOption(requiredInstallOptionResolution{
				Option:         taskSystemRequiredOption(),
				Requested:      opts.taskSystem,
				Saved:          next.TaskSystem,
				Selected:       true,
				NonInteractive: ci,
				Reader:         promptReader,
			})
			if err != nil {
				return nil, err
			}
			next.TaskSystem = taskSystem
		}
		if contains(next.Agents, "publishing") {
			deploymentModel, err := resolveRequiredInstallOption(requiredInstallOptionResolution{
				Option:         deploymentModelRequiredOption(),
				Requested:      opts.deploymentModel,
				Saved:          next.DeploymentModel,
				Selected:       true,
				NonInteractive: ci,
				Reader:         promptReader,
			})
			if err != nil {
				return nil, err
			}
			next.DeploymentModel = deploymentModel
		}
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
	taskSystem := opts.taskSystem
	deploymentModel := opts.deploymentModel

	if len(resolvedTargets) > 0 && (len(resolvedAgents) > 0 || len(resolvedSkills) > 0) {
		if contains(resolvedAgents, "tasks") {
			var err error
			taskSystem, err = resolveRequiredInstallOption(requiredInstallOptionResolution{
				Option:         taskSystemRequiredOption(),
				Requested:      opts.taskSystem,
				Saved:          configValue(config, "taskSystem"),
				Selected:       true,
				NonInteractive: ci,
				Reader:         promptReader,
			})
			if err != nil {
				return nil, err
			}
		}
		if contains(resolvedAgents, "publishing") {
			var err error
			deploymentModel, err = resolveRequiredInstallOption(requiredInstallOptionResolution{
				Option:         deploymentModelRequiredOption(),
				Requested:      opts.deploymentModel,
				Saved:          configValue(config, "deploymentModel"),
				Selected:       true,
				NonInteractive: ci,
				Reader:         promptReader,
			})
			if err != nil {
				return nil, err
			}
		}
		return &rulesConfig{Targets: resolvedTargets, Agents: resolvedAgents, Skills: resolvedSkills, TaskSystem: taskSystem, DeploymentModel: deploymentModel}, nil
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
	if contains(resolvedAgents, "tasks") {
		var err error
		taskSystem, err = resolveRequiredInstallOption(requiredInstallOptionResolution{
			Option:         taskSystemRequiredOption(),
			Requested:      opts.taskSystem,
			Saved:          configValue(config, "taskSystem"),
			Selected:       true,
			NonInteractive: ci,
			Reader:         promptReader,
		})
		if err != nil {
			return nil, err
		}
	}
	if contains(resolvedAgents, "publishing") {
		var err error
		deploymentModel, err = resolveRequiredInstallOption(requiredInstallOptionResolution{
			Option:         deploymentModelRequiredOption(),
			Requested:      opts.deploymentModel,
			Saved:          configValue(config, "deploymentModel"),
			Selected:       true,
			NonInteractive: ci,
			Reader:         promptReader,
		})
		if err != nil {
			return nil, err
		}
	}

	return &rulesConfig{Targets: resolvedTargets, Agents: resolvedAgents, Skills: resolvedSkills, TaskSystem: taskSystem, DeploymentModel: deploymentModel}, nil
}

func install(opts installOptions) installResult {
	result := installResult{}
	opts.agents = withImplicitAgents(opts.agents)
	disableSupportFiles := os.Getenv("BALLAST_DISABLE_SUPPORT_FILES") == "1"
	refreshManagedSkills := os.Getenv("BALLAST_REFRESH_SKILLS") == "1"
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
			Targets:         targets,
			Agents:          opts.agents,
			Skills:          opts.skills,
			Languages:       []string{opts.language},
			TaskSystem:      opts.taskSystem,
			DeploymentModel: opts.deploymentModel,
		}); err != nil {
			result.errors = append(result.errors, agentError{agent: "config", err: err.Error()})
			return result
		}
	}

	supportAgents := slices.Clone(opts.agents)
	supportSkills := slices.Clone(opts.skills)
	configForInstall := loadConfig(opts.projectRoot, opts.language)
	effectiveTools := map[string][]string{}
	if configForInstall != nil {
		effectiveTools = configForInstall.Tools
		supportAgents = withImplicitAgents(configForInstall.Agents)
		supportSkills = slices.Clone(configForInstall.Skills)
	}
	supportAgents = uniqueStrings(supportAgents)
	supportSkills = uniqueStrings(supportSkills)
	rulePublishingProfiles := []string{}
	if configForInstall != nil {
		rulePublishingProfiles = normalizePublishingProfiles(configForInstall.PublishingProfiles)
	}

	for _, target := range targets {
		processed := map[string]struct{}{}
		processedSkills := map[string]struct{}{}

		if target == "codex" && refreshManagedSkills {
			for _, skillID := range commonSkills {
				legacy := legacyCodexSkillDestination(opts.projectRoot, skillID)
				content, err := os.ReadFile(legacy)
				if err != nil {
					continue
				}
				if strings.Contains(string(content), "Created by [Ballast]") || strings.Contains(string(content), "Created by Ballast.") {
					_ = os.Remove(legacy)
				}
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
			suffixes = filterPublishingSuffixes(agentID, suffixes, rulePublishingProfiles)

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
				content, err := buildContent(agentID, target, opts.language, suffix, hookMode, opts.taskSystem, opts.deploymentModel, buildOptions{tools: effectiveTools})
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
			if err := os.MkdirAll(dir, 0o755); err != nil {
				result.errors = append(result.errors, agentError{agent: skillID, err: err.Error()})
				continue
			}
			if exists(file) && !opts.force && !opts.patch && !refreshManagedSkills {
				continue
			}
			switch target {
			case "cursor":
				content, buildErr := buildCursorSkillFormat(skillID, opts.language)
				if buildErr != nil {
					result.errors = append(result.errors, agentError{agent: skillID, err: buildErr.Error()})
					continue
				}
				nextContent := content
				if exists(file) && !opts.force && opts.patch {
					existing, readErr := os.ReadFile(file)
					if readErr != nil {
						result.errors = append(result.errors, agentError{agent: skillID, err: readErr.Error()})
						continue
					}
					nextContent = patchRuleContent(string(existing), content, target)
				}
				err = os.WriteFile(file, []byte(nextContent), 0o644)
			case "claude":
				skillContent, readErr := readSkillContent(skillID, opts.language)
				if readErr != nil {
					result.errors = append(result.errors, agentError{agent: skillID, err: readErr.Error()})
					continue
				}
				nextContent := skillContent
				if exists(file) && !opts.force && opts.patch {
					existing, readErr := readClaudeSkillContent(file)
					if readErr != nil {
						nextContent = skillContent
					} else {
						nextContent = patchRuleContent(existing, skillContent, target)
					}
				}
				content, buildErr := buildClaudeSkill(skillID, opts.language, nextContent)
				if buildErr != nil {
					result.errors = append(result.errors, agentError{agent: skillID, err: buildErr.Error()})
					continue
				}
				err = os.WriteFile(file, content, 0o644)
			case "opencode", "gemini":
				content, buildErr := buildSkillMarkdown(skillID, opts.language)
				if buildErr != nil {
					result.errors = append(result.errors, agentError{agent: skillID, err: buildErr.Error()})
					continue
				}
				nextContent := content
				if exists(file) && !opts.force && opts.patch {
					existing, readErr := os.ReadFile(file)
					if readErr != nil {
						result.errors = append(result.errors, agentError{agent: skillID, err: readErr.Error()})
						continue
					}
					nextContent = patchRuleContent(string(existing), content, target)
				}
				err = os.WriteFile(file, []byte(nextContent), 0o644)
			case "codex":
				content, buildErr := buildCodexSkillMarkdown(skillID, opts.language)
				if buildErr != nil {
					result.errors = append(result.errors, agentError{agent: skillID, err: buildErr.Error()})
					continue
				}
				nextContent := content
				if exists(file) && !opts.force && opts.patch {
					existing, readErr := os.ReadFile(file)
					if readErr != nil {
						result.errors = append(result.errors, agentError{agent: skillID, err: readErr.Error()})
						continue
					}
					nextContent = patchRuleContent(string(existing), content, target)
				}
				if err = os.WriteFile(file, []byte(nextContent), 0o644); err == nil {
					err = copyCodexSkillResources(skillID, opts.language, dir)
				}
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
			if _, skipped := opts.skipSupport[agentsPath]; skipped {
				if !contains(result.declinedSupportFiles, agentsPath) {
					result.declinedSupportFiles = append(result.declinedSupportFiles, agentsPath)
				}
			} else {
				content, err := buildCodexAgentsMD(supportAgents, supportSkills, opts.language, effectiveTools, rulePublishingProfiles)
				if err != nil {
					result.errors = append(result.errors, agentError{agent: "codex", err: err.Error()})
				} else {
					nextContent := content
					shouldWrite := true
					if exists(agentsPath) && !opts.force {
						existing, readErr := os.ReadFile(agentsPath)
						if readErr != nil {
							result.errors = append(result.errors, agentError{agent: "codex", err: readErr.Error()})
							shouldWrite = false
						} else {
							nextContent = patchCodexAgentsMDWithOptions(string(existing), content, opts.patch)
						}
					}
					if shouldWrite {
						if err := os.WriteFile(agentsPath, []byte(nextContent), 0o644); err != nil {
							result.errors = append(result.errors, agentError{agent: "codex", err: err.Error()})
						} else if !contains(result.installedSupport, agentsPath) {
							result.installedSupport = append(result.installedSupport, agentsPath)
						}
					}
				}
			}
		}

		if target == "claude" && !disableSupportFiles {
			claudePath := claudeMDPath(opts.projectRoot)
			shouldPatchClaude := (exists(claudePath) && !opts.force) || opts.patch || opts.patchClaude
			if _, skipped := opts.skipSupport[claudePath]; skipped {
				if !contains(result.declinedSupportFiles, claudePath) {
					result.declinedSupportFiles = append(result.declinedSupportFiles, claudePath)
				}
			} else {
				content, err := buildClaudeMD(supportAgents, supportSkills, opts.language, effectiveTools, rulePublishingProfiles)
				if err != nil {
					result.errors = append(result.errors, agentError{agent: "claude", err: err.Error()})
				} else {
					nextContent := content
					shouldWrite := true
					if exists(claudePath) && !opts.force && shouldPatchClaude {
						existing, readErr := os.ReadFile(claudePath)
						if readErr != nil {
							result.errors = append(result.errors, agentError{agent: "claude", err: readErr.Error()})
							shouldWrite = false
						} else {
							nextContent = patchCodexAgentsMDWithOptions(string(existing), content, opts.patch || opts.patchClaude)
						}
					}
					if shouldWrite {
						if err := os.WriteFile(claudePath, []byte(nextContent), 0o644); err != nil {
							result.errors = append(result.errors, agentError{agent: "claude", err: err.Error()})
						} else if !contains(result.installedSupport, claudePath) {
							result.installedSupport = append(result.installedSupport, claudePath)
						}
					}
				}
			}
		}

		if target == "gemini" && !disableSupportFiles {
			geminiPath := geminiMDPath(opts.projectRoot)
			shouldPatchGemini := (exists(geminiPath) && !opts.force) || opts.patch || opts.patchGemini
			if _, skipped := opts.skipSupport[geminiPath]; skipped {
				if !contains(result.declinedSupportFiles, geminiPath) {
					result.declinedSupportFiles = append(result.declinedSupportFiles, geminiPath)
				}
			} else {
				content, err := buildGeminiMD(supportAgents, supportSkills, opts.language, effectiveTools, rulePublishingProfiles)
				if err != nil {
					result.errors = append(result.errors, agentError{agent: "gemini", err: err.Error()})
				} else {
					nextContent := content
					shouldWrite := true
					if exists(geminiPath) && !opts.force && shouldPatchGemini {
						existing, readErr := os.ReadFile(geminiPath)
						if readErr != nil {
							result.errors = append(result.errors, agentError{agent: "gemini", err: readErr.Error()})
							shouldWrite = false
						} else {
							nextContent = patchCodexAgentsMDWithOptions(string(existing), content, opts.patch || opts.patchGemini)
						}
					}
					if shouldWrite {
						if err := os.WriteFile(geminiPath, []byte(nextContent), 0o644); err != nil {
							result.errors = append(result.errors, agentError{agent: "gemini", err: err.Error()})
						} else if !contains(result.installedSupport, geminiPath) {
							result.installedSupport = append(result.installedSupport, geminiPath)
						}
					}
				}
			}
		}
	}

	return result
}

func buildCodexAgentsMD(agents []string, skills []string, language string, tools map[string][]string, publishingProfiles []string) (string, error) {
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
	)
	lines = append(lines, renderRepositoryToolPolicyManifestLines(tools)...)
	lines = append(lines,
		"Read and follow these rule files in `.codex/rules/` when they apply:",
		"",
	)
	for _, agentID := range agents {
		suffixes, err := listRuleSuffixes(agentID, language)
		if err != nil {
			return "", err
		}
		suffixes = filterPublishingSuffixes(agentID, suffixes, publishingProfiles)
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
			"Read and use these skill files in `.codex/skills/` when they are relevant:",
			"",
		)
		for _, skillID := range skills {
			lines = append(lines, fmt.Sprintf("- `.codex/skills/%s/SKILL.md` — %s", skillID, skillDescription(skillID, language)))
		}
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n"), nil
}

func renderGeminiMandates() string {
	return strings.Join([]string{
		"## Gemini Mandates",
		"",
		"### Narrative Flow",
		"Always use the `update_topic` tool at the beginning of a task and when transitioning between major strategic phases. Provide a concise `title` and a detailed `summary` (5-10 sentences) that recaps completed work and outlines the immediate strategic intent.",
		"",
		"### Context Efficiency",
		"- **Surgical Reads:** Use `start_line` and `end_line` in `read_file` to minimize context usage.",
		"- **Parallelism:** Execute independent searches and reads in parallel whenever possible.",
		"- **Topic Search:** Use `grep_search` to identify points of interest before reading entire files.",
		"",
		"### Strategic Orchestration",
		"Delegate complex, repetitive, or high-volume tasks to specialized sub-agents (`codebase_investigator`, `generalist`) to keep the main session history lean and efficient.",
		"",
		"",
	}, "\n")
}

func buildGeminiMD(agents []string, skills []string, language string, tools map[string][]string, publishingProfiles []string) (string, error) {
	var sb strings.Builder
	sb.WriteString("# GEMINI.md\n\n")
	sb.WriteString("This file provides guidance to Gemini CLI for working in this repository.\n\n")

	for _, line := range repositoryFactsSection() {
		sb.WriteString(line + "\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Memory Tiering\n\n")
	sb.WriteString("Follow these routing rules for persisting long-lived facts and preferences:\n\n")
	sb.WriteString("- **Team-shared (Repository)**: Use this `GEMINI.md` file for architecture, workflows, and repo-wide rules.\n")
	sb.WriteString("- **Private (Local Setup)**: Use the private project memory (`MEMORY.md` in the ballast memory folder) for local machine notes or private workflows.\n")
	sb.WriteString("- **Global (Personal)**: Use the global personal memory (`~/.gemini/GEMINI.md`) for cross-project personal coding preferences.\n\n")

	sb.WriteString("---\n\n")
	sb.WriteString("## Installed agent rules\n\n")
	sb.WriteString(ballastNotice() + "\n\n")
	for _, line := range renderRepositoryToolPolicyManifestLines(tools) {
		sb.WriteString(line + "\n")
	}
	sb.WriteString("Read and follow these rule files in `.gemini/rules/` when they apply:\n\n")

	for _, agent := range agents {
		suffixes, err := listRuleSuffixes(agent, language)
		if err != nil {
			return "", err
		}
		suffixes = filterPublishingSuffixes(agent, suffixes, publishingProfiles)
		for _, suffix := range suffixes {
			basename := ruleBaseName(agent, language, suffix)
			description, _ := codexRuleDescription(agent, language, suffix)
			if description == "" {
				description = "Rules for " + basename
			}
			sb.WriteString(fmt.Sprintf("- `.gemini/rules/%s.md` — %s\n", basename, description))
		}
	}

	if len(skills) > 0 {
		sb.WriteString("\n## Installed skills\n\n")
		sb.WriteString(ballastNotice() + "\n\n")
		sb.WriteString("Read and use these skill files in `.gemini/rules/` when they are relevant:\n\n")

		for _, skill := range skills {
			desc := skillDescription(skill, language)
			sb.WriteString(fmt.Sprintf("- `.gemini/rules/%s.md` — %s\n", skill, desc))
		}
	}

	sb.WriteString("\n")
	return sb.String(), nil
}

func buildClaudeMD(agents []string, skills []string, language string, tools map[string][]string, publishingProfiles []string) (string, error) {
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
	)
	lines = append(lines, renderRepositoryToolPolicyManifestLines(tools)...)
	lines = append(lines,
		"Read and follow these rule files in `.claude/rules/` when they apply:",
		"",
	)
	for _, agentID := range agents {
		suffixes, err := listRuleSuffixes(agentID, language)
		if err != nil {
			return "", err
		}
		suffixes = filterPublishingSuffixes(agentID, suffixes, publishingProfiles)
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
	if overridePath := strings.TrimSpace(os.Getenv("BALLAST_REPOSITORY_FACTS_FILE")); overridePath != "" {
		type factsPayload struct {
			RepositoryFactsSection []string `json:"repositoryFactsSection"`
		}
		content, err := os.ReadFile(overridePath)
		if err == nil {
			var payload factsPayload
			if json.Unmarshal(content, &payload) == nil && len(payload.RepositoryFactsSection) > 0 {
				return payload.RepositoryFactsSection
			}
		}
	}
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
	return fmt.Sprintf("---\ndescription: %q\nalwaysApply: false\n---\n\n<!-- %s -->\n\n%s\n", skillDescription(skillID, language), ballastNotice(), strings.TrimRight(body, "\n")), nil
}

func buildSkillMarkdown(skillID, language string) (string, error) {
	content, err := readSkillContent(skillID, language)
	if err != nil {
		return "", err
	}
	_, body := splitSkillDocument(content)
	return "<!-- " + ballastNotice() + " -->\n\n" + strings.TrimRight(body, "\n") + "\n", nil
}

func buildCodexSkillMarkdown(skillID, language string) (string, error) {
	content, err := readSkillContent(skillID, language)
	if err != nil {
		return "", err
	}
	frontmatter, body := splitSkillDocument(content)
	if frontmatter == "" {
		return "<!-- " + ballastNotice() + " -->\n\n" + strings.TrimRight(body, "\n") + "\n", nil
	}
	return frontmatter + "\n\n<!-- " + ballastNotice() + " -->\n\n" + strings.TrimRight(body, "\n") + "\n", nil
}

func copyCodexSkillResources(skillID, language, destinationDir string) error {
	sourceDir := skillDir(skillID, language)
	return copyCodexSkillResourceDir(sourceDir, destinationDir)
}

func copyCodexSkillResourceDir(sourceDir, destinationDir string) error {
	entries, err := fs.ReadDir(embeddedSkillsFS, sourceDir)
	if overrideRoot := repoRootOverride(); overrideRoot != "" {
		entries, err = os.ReadDir(filepath.Join(overrideRoot, filepath.FromSlash(sourceDir)))
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == "SKILL.md" || entry.Name() == "claude-settings.json" {
			continue
		}
		source := path.Join(sourceDir, entry.Name())
		destination := filepath.Join(destinationDir, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return err
			}
			if err := copyCodexSkillResourceDir(source, destination); err != nil {
				return err
			}
			continue
		}
		data, err := readSkillFile(source)
		if err != nil {
			return err
		}
		if err := os.WriteFile(destination, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func buildClaudeSkill(skillID, language string, skillContent ...string) ([]byte, error) {
	content := ""
	if len(skillContent) > 0 {
		content = skillContent[0]
	} else {
		var err error
		content, err = readSkillContent(skillID, language)
		if err != nil {
			return nil, err
		}
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

func readClaudeSkillContent(archivePath string) (string, error) {
	content, err := os.ReadFile(archivePath)
	if err != nil {
		return "", err
	}
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", err
	}
	for _, file := range reader.File {
		if file.Name != "SKILL.md" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return "", fmt.Errorf("skill archive missing SKILL.md")
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
		if _, ok := canonicalHeadings[section.heading]; ok {
			continue
		}
		// The tool policy is Ballast-generated; when canonical output no longer
		// carries it (it moved to the target manifest), drop the stale copy
		// instead of preserving it like a user-authored section. Match on the
		// generated body signature so a user-authored section that merely
		// reuses the heading is preserved.
		if section.heading == "## Repository Tool Policy" &&
			strings.Contains(section.text, "Check `.rulesrc.json` `tools` before adding, installing, or running language tooling.") {
			continue
		}
		parts = append(parts, section.text)
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
	ranges := findSectionRanges(content, heading)
	if len(ranges) == 0 {
		return 0, 0, false
	}
	return ranges[0].start, ranges[0].end, true
}

type sectionRange struct {
	start int
	end   int
}

func findSectionRanges(content, heading string) []sectionRange {
	normalized := normalizeLineEndings(content)
	lines := strings.SplitAfter(normalized, "\n")
	position := 0
	marker := "## " + heading
	inCodeFence := false
	type headingPosition struct {
		line  string
		start int
	}
	headings := []headingPosition{}

	for _, line := range lines {
		trimmed := strings.TrimSuffix(line, "\n")
		if strings.HasPrefix(strings.TrimSpace(trimmed), "```") {
			inCodeFence = !inCodeFence
		}
		if !inCodeFence && strings.HasPrefix(trimmed, "## ") {
			headings = append(headings, headingPosition{line: trimmed, start: position})
		}
		position += len(line)
	}

	ranges := []sectionRange{}
	for index, headingPosition := range headings {
		if headingPosition.line != marker {
			continue
		}
		end := len(normalized)
		if index+1 < len(headings) {
			end = headings[index+1].start
		}
		ranges = append(ranges, sectionRange{start: headingPosition.start, end: end})
	}
	return ranges
}

func patchCodexAgentsMD(existing, canonical string) string {
	return patchCodexAgentsMDWithOptions(existing, canonical, true)
}

func hasBallastManagedNotice(section string) bool {
	return (strings.Contains(section, "Created by [Ballast]") || strings.Contains(section, "Created by Ballast.")) &&
		strings.Contains(section, "Do not edit this section.")
}

var repositoryFactLineRegex = regexp.MustCompile("^- ([^:`]+): `([^`]*)`$")

func isPlaceholderFactValue(value string) bool {
	return strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">")
}

// fillPlaceholderRepositoryFacts fills still-unfilled `<placeholder>` fact
// lines in the existing Repository Facts section with discovered values from
// the canonical section. Lines a user has populated are never touched, and
// facts discovery could not resolve stay as placeholders.
func fillPlaceholderRepositoryFacts(existing, canonical string) string {
	existingStart, existingEnd, existingOK := findSectionRange(existing, "Repository Facts")
	canonicalStart, canonicalEnd, canonicalOK := findSectionRange(canonical, "Repository Facts")
	if !existingOK || !canonicalOK {
		return existing
	}

	discovered := map[string]string{}
	for _, line := range strings.Split(canonical[canonicalStart:canonicalEnd], "\n") {
		parts := repositoryFactLineRegex.FindStringSubmatch(line)
		if len(parts) != 3 || isPlaceholderFactValue(parts[2]) {
			continue
		}
		discovered[parts[1]] = line
	}
	if len(discovered) == 0 {
		return existing
	}

	lines := strings.Split(existing[existingStart:existingEnd], "\n")
	for i, line := range lines {
		parts := repositoryFactLineRegex.FindStringSubmatch(line)
		if len(parts) != 3 || !isPlaceholderFactValue(parts[2]) {
			continue
		}
		if replacement, ok := discovered[parts[1]]; ok {
			lines[i] = replacement
		}
	}
	return existing[:existingStart] + strings.Join(lines, "\n") + existing[existingEnd:]
}

func patchCodexAgentsMDWithOptions(existing, canonical string, replaceUnmanagedSections bool) string {
	if strings.TrimSpace(existing) == "" {
		return normalizeLineEndings(canonical)
	}
	existing = normalizeLineEndings(existing)
	canonical = normalizeLineEndings(canonical)
	current := fillPlaceholderRepositoryFacts(existing, canonical)
	for _, heading := range []string{"Installed agent rules", "Installed skills"} {
		canonicalStart, canonicalEnd, ok := findSectionRange(canonical, heading)
		if !ok {
			continue
		}
		canonicalSection := strings.TrimRight(canonical[canonicalStart:canonicalEnd], "\n")
		var existingRange sectionRange
		found := false
		for _, candidate := range findSectionRanges(current, heading) {
			if replaceUnmanagedSections || hasBallastManagedNotice(current[candidate.start:candidate.end]) {
				existingRange = candidate
				found = true
				break
			}
		}
		if !found {
			current = strings.TrimRight(current, "\n") + "\n\n" + canonicalSection + "\n"
			continue
		}
		current = joinSupportSectionParts(
			current[:existingRange.start],
			canonicalSection,
			current[existingRange.end:],
		)
	}
	return current
}

func joinSupportSectionParts(prefix, section, suffix string) string {
	return strings.TrimRight(
		strings.TrimRight(prefix, "\n")+"\n\n"+
			section+"\n\n"+
			strings.TrimLeft(suffix, "\n"),
		"\n",
	) + "\n"
}

func buildContent(agentID, target, language, suffix, hookMode, taskSystem, deploymentModel string, options ...buildOptions) (string, error) {
	content, err := readContent(agentID, language, suffix, hookMode, taskSystem, deploymentModel)
	if err != nil {
		return "", err
	}
	buildOpts := buildOptions{}
	if len(options) > 0 {
		buildOpts = options[0]
	}
	// Manifest-bearing targets (claude, codex, gemini) get the tool policy once
	// in their manifest's "Installed agent rules" section instead of per rule.
	withToolPolicy := func(rendered string) string {
		return insertRepositoryToolPolicy(rendered, buildOpts.tools)
	}
	switch target {
	case "cursor":
		front, err := readTemplate(agentID, language, "cursor-frontmatter.yaml", suffix)
		if err != nil {
			return "", err
		}
		front = applyHookTemplateVariables(front, agentID, language, hookMode)
		front = applyTaskSystemVariables(front, agentID, taskSystem)
		return addRuleMarker(withToolPolicy(front+"\n"+content), ruleMarkerID(agentID, language, suffix)), nil
	case "claude":
		header, err := readTemplate(agentID, language, "claude-header.md", suffix)
		if err != nil {
			return "", err
		}
		header = applyTaskSystemVariables(header, agentID, taskSystem)
		return addRuleMarker(header+content, ruleMarkerID(agentID, language, suffix)), nil
	case "gemini":
		header, err := readTemplate(agentID, language, "gemini-header.md", suffix)
		if err != nil {
			header, err = readTemplate(agentID, language, "claude-header.md", suffix)
			if err != nil {
				header, err = readTemplate(agentID, language, "codex-header.md", suffix)
				if err != nil {
					return "", err
				}
			}
		}
		header = applyTaskSystemVariables(header, agentID, taskSystem)
		return addRuleMarker(header+"\n---\n\n"+renderGeminiMandates()+content, ruleMarkerID(agentID, language, suffix)), nil
	case "opencode":
		front, err := readTemplate(agentID, language, "opencode-frontmatter.yaml", suffix)
		if err != nil {
			return "", err
		}
		front = applyTaskSystemVariables(front, agentID, taskSystem)
		return addRuleMarker(withToolPolicy(front+"\n"+content), ruleMarkerID(agentID, language, suffix)), nil
	case "codex":
		header, err := readTemplate(agentID, language, "codex-header.md", suffix)
		if err != nil {
			header, err = readTemplate(agentID, language, "claude-header.md", suffix)
			if err != nil {
				return "", err
			}
		}
		header = applyTaskSystemVariables(header, agentID, taskSystem)
		return addRuleMarker(header+content, ruleMarkerID(agentID, language, suffix)), nil
	default:
		return "", fmt.Errorf("unknown target: %s", target)
	}
}

func renderRepositoryToolPolicy(tools map[string][]string) string {
	normalized := normalizeLanguageTools(tools)
	if len(normalized) == 0 {
		return ""
	}
	languages := make([]string, 0, len(normalized))
	for language := range normalized {
		languages = append(languages, language)
	}
	sort.Strings(languages)

	formatted := make([]string, 0, len(languages))
	for _, language := range languages {
		formatted = append(formatted, language+"="+strings.Join(normalized[language], ","))
	}

	lines := []string{
		"## Repository Tool Policy",
		"",
		"- Check `.rulesrc.json` `tools` before adding, installing, or running language tooling.",
		"- Configured tools: " + strings.Join(formatted, "; ") + ".",
	}
	if slices.Contains(normalized["python"], "uv") {
		lines = append(lines, "- For Python commands, prefer `uv run <command>` and `uv add ...` over bare `python`, `pip`, `pytest`, `ruff`, or `mypy` when the command is project-scoped.")
	}
	if slices.Contains(normalized["typescript"], "pnpm") {
		lines = append(lines, "- For TypeScript commands, prefer `pnpm`/`pnpm exec` over `npm`/`npx` when the command is project-scoped.")
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n") + "\n"
}

func insertRepositoryToolPolicy(content string, tools map[string][]string) string {
	policy := renderRepositoryToolPolicy(tools)
	if policy == "" || strings.Contains(content, "## Repository Tool Policy") {
		return content
	}

	prefix := ""
	body := content
	if match := frontmatterRegex.FindString(content); match != "" {
		prefix = match
		body = strings.TrimPrefix(content, match)
	}
	insertAt := strings.Index(body, "\n## ")
	if insertAt == -1 {
		return prefix + policy + body
	}
	return prefix + body[:insertAt] + "\n\n" + policy + body[insertAt+1:]
}

// renderRepositoryToolPolicyManifestLines renders the tool policy as manifest
// lines nested under the managed "Installed agent rules" section (### heading
// so section patching keeps treating it as part of that section).
func renderRepositoryToolPolicyManifestLines(tools map[string][]string) []string {
	policy := renderRepositoryToolPolicy(tools)
	if policy == "" {
		return nil
	}
	lines := strings.Split(strings.TrimRight(policy, "\n"), "\n")
	lines[0] = "### Repository Tool Policy"
	return append(lines, "")
}

func ruleMarkerID(agentID, language, suffix string) string {
	parts := []string{language, agentID}
	if strings.TrimSpace(suffix) != "" {
		parts = append(parts, suffix)
	}
	return strings.Join(parts, "/")
}

func parseRuleMarker(content string) (*ruleMarker, bool) {
	matches := ruleMarkerRegex.FindStringSubmatch(content[ruleMarkerHeaderStart(content):])
	if len(matches) != 4 {
		return nil, false
	}
	return &ruleMarker{
		ruleID:   matches[1],
		version:  matches[2],
		checksum: strings.ToLower(matches[3]),
	}, true
}

func stripRuleMarker(content string) string {
	start := ruleMarkerHeaderStart(content)
	match := ruleMarkerRegex.FindStringIndex(content[start:])
	if match == nil || match[0] != 0 {
		return content
	}
	return content[:start] + content[start+match[1]:]
}

func ruleMarkerHeaderStart(content string) int {
	frontmatterRegex := regexp.MustCompile(`(?s)^---\r?\n.*?\r?\n---\r?\n?`)
	if match := frontmatterRegex.FindStringIndex(content); match != nil && match[0] == 0 {
		return match[1]
	}
	return 0
}

func calculateRuleChecksum(content string) string {
	sum := sha256.Sum256([]byte(stripRuleMarker(content)))
	return hex.EncodeToString(sum[:])
}

func verifyRuleChecksum(content string) bool {
	marker, ok := parseRuleMarker(content)
	return ok && calculateRuleChecksum(content) == marker.checksum
}

func addRuleMarker(content, ruleID string) string {
	body := stripRuleMarker(content)
	marker := fmt.Sprintf(
		`<!-- ballast:rule id="%s" version="%s" checksum="%s" -->`,
		ruleID,
		resolveVersion(),
		calculateRuleChecksum(body),
	)
	frontmatterRegex := regexp.MustCompile(`(?s)^---\r?\n.*?\r?\n---\r?\n?`)
	if match := frontmatterRegex.FindStringIndex(body); match != nil && match[0] == 0 {
		return body[:match[1]] + marker + "\n" + body[match[1]:]
	}
	return marker + "\n" + body
}

func renderGitHooksPreCommitGlob(agentID, language, hookMode string) string {
	if agentID != "git-hooks" {
		return ""
	}
	if language == "typescript" && hookMode == "husky" {
		return ""
	}
	return "  - '.pre-commit-config.yaml'"
}

func normalizeDeploymentModel(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeRequiredInstallOptionValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func renderDeploymentModelGuidance(deploymentModel string) string {
	switch normalizeDeploymentModel(deploymentModel) {
	case "kubernetes":
		return strings.Join([]string{
			"Deployment guidance is active (`deploymentModel: kubernetes`). Apply web/API deployment workflow guidance for repositories that own this deployment model.",
			"",
			"Kubernetes deployment model:",
			"- Treat app deployment ownership as Kubernetes-native unless repo docs say otherwise.",
			"- Keep application Helm charts in the app repository under `charts/<app>/` with chart tests and schema validation.",
			"- Keep ArgoCD `Application` or `ApplicationSet` resources, environment values, and promotion state in the configured GitOps repository.",
			"- CI should publish immutable images and charts; GitOps changes should promote those versions by environment.",
		}, "\n")
	case "serverless":
		return strings.Join([]string{
			"Deployment guidance is active (`deploymentModel: serverless`). Apply web/API deployment workflow guidance for repositories that own this deployment model.",
			"",
			"Serverless deployment model:",
			"- Treat deployable apps as functions, jobs, queues, event rules, and managed cloud resources.",
			"- Keep infrastructure definitions close to the owning service unless the repo documents a shared infrastructure boundary.",
			"- CI should package immutable artifacts, run provider validation, and promote environment-specific configuration explicitly.",
		}, "\n")
	case "server":
		return strings.Join([]string{
			"Deployment guidance is active (`deploymentModel: server`). Apply web/API deployment workflow guidance for repositories that own this deployment model.",
			"",
			"Self-managed server deployment model:",
			"- Treat deployable apps as services on provisioned hosts, VMs, or bare-metal servers.",
			"- Keep systemd, process manager, reverse proxy, secrets, and rollback instructions aligned with the runtime environment.",
			"- CI should build immutable artifacts and deployment automation should perform health checks before traffic cutover.",
		}, "\n")
	case "docker":
		return strings.Join([]string{
			"Deployment guidance is active (`deploymentModel: docker`). Apply container image publishing and runtime handoff guidance for repositories that own Docker images.",
			"",
			"Docker deployment model:",
			"- Treat the Docker or OCI image as the primary deployable artifact.",
			"- Build images in CI from release tags or protected branches, publish immutable tags, and capture the digest for deployment handoff.",
			"- Support GHCR and Docker Hub explicitly; choose public or private visibility from repo policy and the image audience.",
			"- Run Dockerfile linting, image build smoke tests, and image vulnerability scans before publishing.",
			"- Do not assume systemd, SSH, Kubernetes, hosted-platform, or serverless rollout ownership unless repo docs explicitly add that layer.",
		}, "\n")
	case "hosted":
		return strings.Join([]string{
			"Deployment guidance is active (`deploymentModel: hosted`). Apply web/API deployment workflow guidance for repositories that own this deployment model.",
			"",
			"Hosted platform deployment model:",
			"- Treat deployable apps as hosted-platform workloads such as Vercel, Netlify, Railway, Render, Fly.io, or similar services.",
			"- Keep provider configuration, environment variables, preview environments, and production promotion rules documented with the app.",
			"- CI should validate builds and let the hosted platform own rollout mechanics unless the repo defines a separate release gate.",
		}, "\n")
	default:
		return "No app deployment model is configured (`deploymentModel: none`). Deployment guidance is reference-only. Deployment is inactive: keep library, SDK, CLI, and optional container publishing guidance active, but do not create deploy-on-main workflows, deployment-state updates, Kubernetes, serverless, hosted-platform, Docker registry, or self-managed server deployment ownership until the repository sets an active `deploymentModel`."
	}
}

var deploymentConditionalRegex = regexp.MustCompile(`\{\{BALLAST_IF_DEPLOYMENT:([a-z, -]+)\}\}\r?\n?([\s\S]*?)\{\{BALLAST_END_IF_DEPLOYMENT\}\}\r?\n?`)

// applyDeploymentConditionalBlocks strips {{BALLAST_IF_DEPLOYMENT:<models>}}
// blocks whose model list does not match the configured deployment model. The
// special name "active" matches any model except "none".
func applyDeploymentConditionalBlocks(content, deploymentModel string) string {
	if !strings.Contains(content, "{{BALLAST_IF_DEPLOYMENT:") {
		return content
	}
	model := strings.ToLower(strings.TrimSpace(deploymentModel))
	if model == "" {
		model = "none"
	}
	return deploymentConditionalRegex.ReplaceAllStringFunc(content, func(match string) string {
		parts := deploymentConditionalRegex.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		for _, name := range strings.Split(parts[1], ",") {
			name = strings.TrimSpace(name)
			if name == model || (name == "active" && model != "none") {
				return parts[2]
			}
		}
		return ""
	})
}

func applyDeploymentModelGuidance(content, agentID, deploymentModel string) string {
	content = applyDeploymentConditionalBlocks(content, deploymentModel)
	if agentID != "publishing" || !strings.Contains(content, deploymentModelGuidanceToken) {
		return content
	}
	return strings.ReplaceAll(content, deploymentModelGuidanceToken, renderDeploymentModelGuidance(deploymentModel))
}

func applyTaskSystemVariables(content, agentID, taskSystem string) string {
	if agentID != "tasks" {
		return content
	}
	normalized := normalizeRequiredInstallOptionValue(taskSystem)
	if strings.Contains(content, taskSystemGuidanceToken) {
		if normalized == "none" {
			return content[:strings.Index(content, taskSystemGuidanceToken)] + strings.Join([]string{
				"## Activation",
				"",
				"External issue tracking is disabled (`taskSystem: none`). This repository has no external task system configured. Do not require GitHub Issues, Jira, Linear, or MCP-backed ticket creation for routine branch work.",
				"",
				"Use `tasks/todo.md` as the structured branch-local task artifact. If work must survive beyond the current branch, ask the user where they want durable follow-up tracked before creating external issues or tickets.",
				"",
				"## MCP Server Setup",
				"",
				"No task-system MCP server is required while `taskSystem` is `none`. Configure GitHub Issues, Jira, or Linear MCP only after the repository changes its saved `taskSystem` value or the user explicitly asks for that integration.",
				"",
				"## Using Work Items",
				"",
				"- Do not create external issues or tickets by default.",
				"- When preparing a PR, triage `tasks/todo.md` and either resolve items, keep them as branch-local evidence, or ask the user where durable follow-up belongs.",
				"- Keep credentials out of committed files; use environment variables or platform secret stores if a task-system integration is added later.",
			}, "\n")
		}
		content = strings.ReplaceAll(content, taskSystemGuidanceToken, strings.Join([]string{
			"## Activation",
			"",
			fmt.Sprintf("External issue tracking is active (`taskSystem: %s`). This repository uses **%s** as the system of record for all planned work, follow-up tasks, bugs, and feature requests. All durable work items must be created there, not left only in local notes or branch files.", normalized, taskSystemDisplayName(normalized)),
		}, "\n"))
	}
	if strings.Contains(content, taskSystemToken) {
		return strings.ReplaceAll(content, taskSystemToken, taskSystemDisplayName(normalized))
	}
	return content
}

func taskSystemDisplayName(taskSystem string) string {
	switch taskSystem {
	case "github":
		return "GitHub"
	case "jira":
		return "Jira"
	case "linear":
		return "Linear"
	default:
		return taskSystem
	}
}

func applyHookTemplateVariables(content, agentID, language, hookMode string) string {
	if !strings.Contains(content, gitHooksPreCommitGlobToken) {
		return content
	}
	return strings.ReplaceAll(content, gitHooksPreCommitGlobToken, renderGitHooksPreCommitGlob(agentID, language, hookMode))
}

var optInPublishingProfiles = []string{"apt", "brew"}

// filterPublishingSuffixes narrows publishing rule suffixes to the configured
// profiles, or excludes reference-only opt-in variants when no profiles are
// configured.
func filterPublishingSuffixes(agentID string, suffixes, profiles []string) []string {
	if agentID != "publishing" {
		return suffixes
	}
	if len(profiles) > 0 {
		available := map[string]struct{}{}
		for _, suffix := range suffixes {
			available[suffix] = struct{}{}
		}
		selected := make([]string, 0, len(profiles))
		for _, profile := range profiles {
			if _, ok := available[profile]; ok {
				selected = append(selected, profile)
			}
		}
		return selected
	}
	filtered := make([]string, 0, len(suffixes))
	for _, suffix := range suffixes {
		if contains(optInPublishingProfiles, suffix) {
			continue
		}
		filtered = append(filtered, suffix)
	}
	return filtered
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

var includeTokenRegex = regexp.MustCompile(`\{\{include:([^}]+)\}\}`)

const maxIncludeDepth = 10

// resolveContentIncludes resolves {{include:<path>.md}} tokens against the
// agents content root. The fragment body is inserted with trailing whitespace
// trimmed so tokens can sit inline in a content file. Fragments may include
// other fragments; recursion and missing files fail the build with a clear
// error.
func resolveContentIncludes(content string, stack []string) (string, error) {
	if !strings.Contains(content, "{{include:") {
		return content, nil
	}
	var resolveErr error
	resolved := includeTokenRegex.ReplaceAllStringFunc(content, func(match string) string {
		if resolveErr != nil {
			return match
		}
		parts := includeTokenRegex.FindStringSubmatch(match)
		includePath := strings.TrimSpace(parts[1])
		if !strings.HasSuffix(includePath, ".md") || strings.Contains(includePath, "..") || strings.HasPrefix(includePath, "/") {
			resolveErr = fmt.Errorf("invalid include path %q: must be a relative .md path under agents/", includePath)
			return match
		}
		chain := strings.Join(append(slices.Clone(stack), includePath), " -> ")
		if contains(stack, includePath) {
			resolveErr = fmt.Errorf("recursive include detected for %q (chain: %s)", includePath, chain)
			return match
		}
		if len(stack) >= maxIncludeDepth {
			resolveErr = fmt.Errorf("include depth exceeded (max %d) at %q (chain: %s)", maxIncludeDepth, includePath, chain)
			return match
		}
		fragment, err := readAgentFile(path.Join("agents", includePath))
		if err != nil {
			resolveErr = fmt.Errorf("missing include fragment: %s", includePath)
			return match
		}
		expanded, err := resolveContentIncludes(string(fragment), append(slices.Clone(stack), includePath))
		if err != nil {
			resolveErr = err
			return match
		}
		return strings.TrimRight(expanded, " \t\r\n")
	})
	if resolveErr != nil {
		return "", resolveErr
	}
	return resolved, nil
}

func readContent(agentID, language, suffix, hookMode, taskSystem, deploymentModel string) (string, error) {
	name := "content.md"
	if suffix != "" {
		name = "content-" + suffix + ".md"
	}
	bytes, err := readAgentFile(path.Join(agentDir(agentID, language), name))
	if err != nil {
		return "", fmt.Errorf("agent %q has no %s", agentID, name)
	}
	raw, err := resolveContentIncludes(string(bytes), nil)
	if err != nil {
		return "", err
	}
	content := applyDeploymentModelGuidance(raw, agentID, deploymentModel)
	content = applyTaskSystemVariables(content, agentID, taskSystem)
	if agentID == "git-hooks" && strings.Contains(content, gitHooksGuidanceToken) {
		content = strings.ReplaceAll(content, gitHooksGuidanceToken, renderGitHooksGuidance(language, hookMode))
	}
	return content, nil
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
	gitleaksHookGuidance := "- Add the official `gitleaks` pre-commit hook in `.pre-commit-config.yaml` for secret detection; do not generate or call a repo-local no-secrets shell script."
	switch language {
	case "typescript":
		if hookMode == "husky" {
			return strings.Join([]string{
				"- Use Husky for TypeScript-only repositories.",
				"- Install and initialize Husky.",
				"- Create `.husky/pre-commit` with the repo's fast lint command, such as `npx lint-staged`.",
				"- Create `.husky/pre-push` with the repo's unit test command, and for TypeScript repositories run the build before the tests when the test command depends on generated output.",
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
			gitleaksHookGuidance,
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
			gitleaksHookGuidance,
			"- Keep Bandit and `pip-audit` in CI or explicit security-review workflows unless this repository opts into running them from hooks.",
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
			gitleaksHookGuidance,
			"- Keep `govulncheck`, fuzzing, and `go test -race` in CI, pre-push, or explicit security-review workflows unless this repository opts into running them at commit time.",
			"- Keep the configuration current with `pre-commit autoupdate`.",
			"- Verify the hook configuration with `pre-commit run --all-files`.",
		}, "\n")
	case "ansible":
		return strings.Join([]string{
			"- Use `pre-commit` for Ansible repositories.",
			"- Create or update `.pre-commit-config.yaml` at the repo root.",
			"- Install hooks with `pre-commit install`.",
			"- Install the pre-push hook with `pre-commit install --hook-type pre-push`.",
			"- Run `ansible-lint` and `yamllint` from the pre-commit stage.",
			"- Run `ansible-playbook --syntax-check` for representative top-level playbooks from the pre-push stage.",
			gitleaksHookGuidance,
			"- Prefer `ansible-lint --profile=safety` in CI or explicit security-review workflows when the repository is ready for safety-oriented rules.",
			"- Keep secrets out of logs and commits; prefer Ansible Vault or external secret stores.",
			"- Keep the configuration current with `pre-commit autoupdate`; rerun `pre-commit run --all-files` after hook changes.",
		}, "\n")
	case "terraform":
		return strings.Join([]string{
			"- Use `pre-commit` for Terraform repositories.",
			"- Create or update `.pre-commit-config.yaml` at the repo root.",
			"- Commit `.terraform-version` and use `tfenv install` plus `tfenv use` before running Terraform commands.",
			"- Install hooks with `pre-commit install`.",
			"- Install the pre-push hook with `pre-commit install --hook-type pre-push`.",
			"- Run `terraform fmt -check -recursive`, `terraform init -backend=false`, `terraform validate`, `tflint --init`, `tflint --recursive`, and `trivy config .` from the hook configuration; keep `tfsec` only for legacy-compatible pipelines.",
			"- Keep `.terraform/`, state files, and plan files out of Git.",
			gitleaksHookGuidance,
			"- Keep deeper IaC static analysis, policy checks, and cloud/runtime posture scanning in CI or operational review workflows.",
			"- Keep the configuration current with `pre-commit autoupdate`.",
		}, "\n")
	case "dart":
		return strings.Join([]string{
			"- Use `pre-commit` for Dart and Flutter repositories.",
			"- Create or update `.pre-commit-config.yaml` at the Flutter app root or monorepo root.",
			"- Install hooks with `pre-commit install`.",
			"- Install the pre-push hook with `pre-commit install --hook-type pre-push`.",
			"- Run `dart format --set-exit-if-changed .` and `flutter analyze` on `pre-commit`.",
			"- Run `flutter test` on `pre-push`; keep `flutter test integration_test` in CI or device-backed jobs when emulators are required.",
			"- Keep `.dart_tool/`, `build/`, and platform build output out of Git.",
			gitleaksHookGuidance,
			"- Keep the configuration current with `pre-commit autoupdate`.",
		}, "\n")
	case "docker":
		return strings.Join([]string{
			"- Use `pre-commit` for Dockerfile and container configuration repositories.",
			"- Create or update `.pre-commit-config.yaml` at the repo root.",
			"- Install hooks with `pre-commit install`.",
			"- Install the pre-push hook with `pre-commit install --hook-type pre-push`.",
			"- Run `hadolint` for Dockerfiles and `docker compose config` for Compose files from the pre-commit stage.",
			"- Run image build and smoke checks from the pre-push stage only when they are deterministic and do not require registry credentials.",
			gitleaksHookGuidance,
			"- Keep image vulnerability scans in CI or pre-push when local Docker availability is reliable.",
			"- Keep the configuration current with `pre-commit autoupdate`.",
		}, "\n")
	default:
		return ""
	}
}

func resolveTsHookMode(projectRoot, language string) string {
	if language != "typescript" {
		return "pre-commit"
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
					return "pre-commit"
				}
			}
		}
	}

	return "husky"
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
	start = filepath.Clean(start)
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
			exists(filepath.Join(dir, "pubspec.yaml")) ||
			exists(filepath.Join(dir, "analysis_options.yaml")) ||
			exists(filepath.Join(dir, ".metadata")) ||
			hasDockerfileMarker(dir) ||
			exists(filepath.Join(dir, "compose.yaml")) ||
			exists(filepath.Join(dir, "compose.yml")) ||
			exists(filepath.Join(dir, "docker-compose.yaml")) ||
			exists(filepath.Join(dir, "docker-compose.yml")) ||
			hasAnyRulesConfig(dir) {
			if dir != start && !isGitBoundary(dir) {
				return start, nil
			}
			return dir, nil
		}
		if isGitBoundary(dir) {
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

func hasDockerfileMarker(dir string) bool {
	for _, pattern := range []string{"Dockerfile*", "Containerfile*"} {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err == nil && len(matches) > 0 {
			return true
		}
	}
	return false
}

func isGitBoundary(dir string) bool {
	gitPath := filepath.Join(dir, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return true
	}
	return exists(filepath.Join(gitPath, "HEAD")) || exists(filepath.Join(gitPath, "config"))
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
		Targets:            targets,
		Agents:             raw.Agents,
		Skills:             raw.Skills,
		BallastVersion:     raw.BallastVersion,
		Languages:          raw.Languages,
		Paths:              raw.Paths,
		Tools:              normalizeLanguageTools(raw.Tools),
		Discovery:          normalizeDiscovery(raw.Discovery),
		TaskSystem:         normalizeRequiredInstallOptionValue(raw.TaskSystem),
		DeploymentModel:    normalizeDeploymentModel(raw.DeploymentModel),
		PublishingProfiles: normalizePublishingProfiles(raw.PublishingProfiles),
	}
}

func normalizePublishingProfiles(values []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		profile := strings.ToLower(strings.TrimSpace(value))
		if alias, ok := publishingProfileAliases[profile]; ok {
			profile = alias
		}
		if !contains(publishingProfiles, profile) {
			continue
		}
		if _, ok := seen[profile]; ok {
			continue
		}
		seen[profile] = struct{}{}
		normalized = append(normalized, profile)
	}
	return normalized
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
		if strings.TrimSpace(cfg.TaskSystem) == "" {
			cfg.TaskSystem = existing.TaskSystem
		}
		if strings.TrimSpace(cfg.DeploymentModel) == "" {
			cfg.DeploymentModel = existing.DeploymentModel
		}
		if cfg.Discovery == nil {
			cfg.Discovery = existing.Discovery
		}
		cfg.Targets = mergeStringLists(existing.Targets, cfg.Targets)
		cfg.Languages = mergeLanguageList(existing.Languages, cfg.Languages)
		cfg.Paths = mergeLanguagePaths(existing.Paths, cfg.Languages)
		cfg.Tools = mergeLanguageTools(existing.Tools, cfg.Tools, cfg.Languages)
	} else {
		cfg.Paths = mergeLanguagePaths(nil, cfg.Languages)
		cfg.Tools = mergeLanguageTools(nil, cfg.Tools, cfg.Languages)
	}
	cfg.DeploymentModel = normalizeDeploymentModel(cfg.DeploymentModel)
	cfg.TaskSystem = normalizeRequiredInstallOptionValue(cfg.TaskSystem)
	if cfg.TaskSystem != "" && !contains(taskSystems, cfg.TaskSystem) {
		return fmt.Errorf("invalid taskSystem %q; use one of: %s", cfg.TaskSystem, strings.Join(taskSystems, ", "))
	}
	if cfg.DeploymentModel != "" && !contains(deploymentModels, cfg.DeploymentModel) {
		return fmt.Errorf("invalid deploymentModel %q; use one of: %s", cfg.DeploymentModel, strings.Join(deploymentModels, ", "))
	}
	bytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, bytes, 0o644)
}

func normalizeDiscovery(raw json.RawMessage) *discoveryConfig {
	if len(raw) == 0 {
		return nil
	}
	var parsed struct {
		ExcludePaths []string `json:"excludePaths"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil
	}
	excludePaths := uniqueToolList(parsed.ExcludePaths)
	if len(excludePaths) == 0 {
		return nil
	}
	return &discoveryConfig{ExcludePaths: excludePaths}
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

func mergeLanguageTools(existing map[string][]string, incoming map[string][]string, languages []string) map[string][]string {
	merged := normalizeLanguageTools(existing)
	for language, tools := range normalizeLanguageTools(incoming) {
		merged[language] = tools
	}
	for _, language := range languages {
		normalizedLanguage := strings.ToLower(strings.TrimSpace(language))
		if normalizedLanguage == "" || len(merged[normalizedLanguage]) > 0 {
			continue
		}
		merged[normalizedLanguage] = append([]string(nil), defaultLanguageTools[normalizedLanguage]...)
	}
	for language, tools := range merged {
		if len(tools) == 0 {
			delete(merged, language)
		}
	}
	return merged
}

func normalizeLanguageTools(raw map[string][]string) map[string][]string {
	normalized := make(map[string][]string, len(raw))
	for language, tools := range raw {
		normalizedLanguage := strings.ToLower(strings.TrimSpace(language))
		normalizedTools := uniqueToolList(tools)
		if normalizedLanguage == "" || len(normalizedTools) == 0 {
			continue
		}
		normalized[normalizedLanguage] = normalizedTools
	}
	return normalized
}

func uniqueToolList(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
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

func isStdinInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func promptTarget() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("AI platform (%s): ", strings.Join(targets, ", "))
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
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

type requiredInstallOption struct {
	FieldName    string
	PromptLabel  string
	Allowed      []string
	DefaultValue string
}

type requiredInstallOptionResolution struct {
	Option         requiredInstallOption
	Requested      string
	Saved          string
	Selected       bool
	NonInteractive bool
	Reader         *bufio.Reader
}

func taskSystemRequiredOption() requiredInstallOption {
	return requiredInstallOption{
		FieldName:    "taskSystem",
		PromptLabel:  "Task system for tasks",
		Allowed:      taskSystems,
		DefaultValue: "github",
	}
}

func deploymentModelRequiredOption() requiredInstallOption {
	return requiredInstallOption{
		FieldName:    "deploymentModel",
		PromptLabel:  "App deployment model for publishing (use none for CLI/library/SDK-only projects)",
		Allowed:      deploymentModels,
		DefaultValue: "none",
	}
}

func resolveRequiredInstallOption(resolution requiredInstallOptionResolution) (string, error) {
	if resolution.Requested != "" {
		return resolution.Requested, nil
	}
	saved := normalizeRequiredInstallOptionValue(resolution.Saved)
	if saved != "" {
		if contains(resolution.Option.Allowed, saved) {
			return saved, nil
		}
		return "", fmt.Errorf("invalid %s %q; use one of: %s", resolution.Option.FieldName, resolution.Saved, strings.Join(resolution.Option.Allowed, ", "))
	}
	if !resolution.Selected {
		return "", nil
	}
	if resolution.NonInteractive {
		return resolution.Option.DefaultValue, nil
	}
	return promptRequiredInstallOption(resolution.Option, resolution.Reader)
}

func promptRequiredInstallOption(option requiredInstallOption, reader *bufio.Reader) (string, error) {
	if reader == nil {
		reader = bufio.NewReader(os.Stdin)
	}
	allowed := strings.Join(option.Allowed, ", ")
	for {
		fmt.Printf("%s [%s] (default: %s): ", option.PromptLabel, allowed, option.DefaultValue)
		response, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
				if strings.TrimSpace(response) != "" {
					value := normalizeRequiredInstallOptionValue(response)
					if contains(option.Allowed, value) {
						return value, nil
					}
					fmt.Printf("Invalid %s. Choose one of: %s\n", option.FieldName, allowed)
					continue
				}
				return option.DefaultValue, nil
			}
			return "", err
		}
		value := normalizeRequiredInstallOptionValue(response)
		if value == "" {
			return option.DefaultValue, nil
		}
		if contains(option.Allowed, value) {
			return value, nil
		}
		fmt.Printf("Invalid %s. Choose one of: %s\n", option.FieldName, allowed)
	}
}

func configValue(config *rulesConfig, field string) string {
	if config == nil {
		return ""
	}
	switch field {
	case "taskSystem":
		return config.TaskSystem
	case "deploymentModel":
		return config.DeploymentModel
	default:
		return ""
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
	case "gemini":
		dir := filepath.Join(projectRoot, ".gemini", "rules")
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
		dir := filepath.Join(root, ".codex", "skills", skillID)
		return dir, filepath.Join(dir, "SKILL.md"), nil
	case "gemini":
		dir := filepath.Join(root, ".gemini", "rules")
		return dir, filepath.Join(dir, skillID+".md"), nil
	default:
		return "", "", fmt.Errorf("unknown target: %s", target)
	}
}

func legacyCodexSkillDestination(projectRoot, skillID string) string {
	return filepath.Join(filepath.Clean(projectRoot), ".codex", "rules", skillID+".md")
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

func geminiMDPath(projectRoot string) string {
	return filepath.Join(projectRoot, "GEMINI.md")
}

func geminiRuleDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".gemini", "rules")
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

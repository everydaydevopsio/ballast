package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
)

type language string

const (
	langTypeScript language = "typescript"
	langPython     language = "python"
	langGo         language = "go"
)

var supportedLanguages = []language{langTypeScript, langPython, langGo}

type toolConfig struct {
	binary         string
	installCommand []string
}

var toolsByLanguage = map[language]toolConfig{
	langTypeScript: {
		binary:         "ballast-typescript",
		installCommand: []string{"npm", "install", "-g", "@everydaydevopsio/ballast"},
	},
	langPython: {
		binary:         "ballast-python",
		installCommand: []string{"uv", "tool", "install", "ballast-python"},
	},
	langGo: {
		binary:         "ballast-go",
		installCommand: []string{"go", "install", "github.com/everydaydevopsio/ballast/packages/ballast-go/cmd/ballast-go@latest"},
	},
}

var version = "dev"

var ensureInstalledFunc = ensureInstalled
var execToolFunc = execTool
var walkDirFunc = filepath.WalkDir

var commonAgents = []string{"local-dev", "cicd", "observability"}
var languageAgents = []string{"linting", "logging", "testing"}
var supportedAgents = append(slices.Clone(commonAgents), languageAgents...)

type monorepoConfig struct {
	Target    string              `json:"target,omitempty"`
	Agents    []string            `json:"agents,omitempty"`
	Languages []string            `json:"languages,omitempty"`
	Paths     map[string][]string `json:"paths,omitempty"`
}

type repoProfile struct {
	Language language
	Paths    []string
}

type backendInvocation struct {
	Language language
	Binary   string
	Dir      string
	Args     []string
}

type monorepoPlan struct {
	Invocations []backendInvocation
	Config      monorepoConfig
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	selectedLanguage, forwardedArgs, err := parseLanguageArg(args)
	if err != nil {
		fmt.Println(err)
		printUsage()
		return 1
	}

	if hasVersionFlag(forwardedArgs) {
		printVersion()
		return 0
	}

	if isVersionCommand(forwardedArgs) {
		printVersion()
		return 0
	}

	if len(forwardedArgs) == 0 {
		printUsage()
		return 0
	}

	if isHelpCommand(forwardedArgs) {
		printUsage()
		return 0
	}

	if selectedLanguage == "" {
		root := findProjectRoot("")
		plan, err := resolveMonorepoPlan(root, forwardedArgs)
		if err != nil {
			fmt.Println(err)
			return 1
		}
		if plan != nil {
			for _, invocation := range plan.Invocations {
				tool, ok := toolsByLanguage[invocation.Language]
				if !ok {
					fmt.Printf("Unsupported language: %s\n", invocation.Language)
					return 1
				}
				if err := ensureInstalledFunc(tool); err != nil {
					fmt.Println(err)
					return 1
				}
				exitCode, err := execToolFunc(invocation.Binary, invocation.Args, invocation.Dir)
				if err != nil {
					fmt.Println(err)
					return 1
				}
				if exitCode != 0 {
					return exitCode
				}
			}
			if err := saveMonorepoConfig(root, plan.Config); err != nil {
				fmt.Println(err)
				return 1
			}
			return 0
		}

		selectedLanguage = detectLanguage(root)
		if selectedLanguage == "" {
			fmt.Printf(
				"Could not detect repository language. Use --language %s.\n",
				strings.Join(languageNames(), "|"),
			)
			return 1
		}
	}

	tool, ok := toolsByLanguage[selectedLanguage]
	if !ok {
		fmt.Printf("Unsupported language: %s\n", selectedLanguage)
		return 1
	}

	if err := ensureInstalledFunc(tool); err != nil {
		fmt.Println(err)
		return 1
	}

	exitCode, err := execToolFunc(tool.binary, forwardedArgs, "")
	if err != nil {
		fmt.Println(err)
		return 1
	}
	return exitCode
}

func parseLanguageArg(args []string) (language, []string, error) {
	var selected language
	forwarded := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--help" || arg == "-h" {
			printUsage()
			os.Exit(0)
		}
		if strings.HasPrefix(arg, "--language=") {
			raw := strings.TrimSpace(strings.TrimPrefix(arg, "--language="))
			lang := language(strings.ToLower(raw))
			if !isSupportedLanguage(lang) {
				return "", nil, fmt.Errorf("invalid --language: %s", raw)
			}
			selected = lang
			continue
		}
		if arg == "--language" || arg == "-l" {
			if i+1 >= len(args) {
				return "", nil, errors.New("missing value for --language")
			}
			raw := strings.TrimSpace(args[i+1])
			lang := language(strings.ToLower(raw))
			if !isSupportedLanguage(lang) {
				return "", nil, fmt.Errorf("invalid --language: %s", raw)
			}
			selected = lang
			i++
			continue
		}
		forwarded = append(forwarded, arg)
	}

	return selected, forwarded, nil
}

func printUsage() {
	fmt.Println("ballast installs AI agent rules for Cursor, Claude Code, OpenCode, and Codex.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ballast [flags] <command> [command flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  install     Install agent rules for the detected or selected language")
	fmt.Println("  help        Show help for ballast")
	fmt.Println("  version     Print the ballast wrapper version")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Printf("  -l, --language string   Force the language backend (%s)\n", strings.Join(languageNames(), "|"))
	fmt.Println("  -h, --help              Show help")
	fmt.Println("  -v, --version           Print version")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  ballast")
	fmt.Println("  ballast install --target cursor --all")
	fmt.Println("  ballast install --target cursor --all --yes   # auto-detect and install across a TypeScript/Python/Go monorepo")
	fmt.Println("  ballast --language python install --target codex --agent linting")
	fmt.Println("  ballast --version")
	fmt.Println()
	fmt.Println("When --language is omitted, ballast detects the repository layout.")
	fmt.Println("Single-language repos are forwarded to the matching backend CLI.")
	fmt.Println("Mixed TypeScript/Python/Go monorepos install common rules at the root and language-specific rules in each detected project directory.")
}

func printVersion() {
	fmt.Println(resolveVersion())
}

func resolveVersion() string {
	if strings.TrimSpace(version) != "" && version != "dev" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if ok {
		if strings.TrimSpace(info.Main.Version) != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}

	return version
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

func isSupportedLanguage(lang language) bool {
	for _, candidate := range supportedLanguages {
		if candidate == lang {
			return true
		}
	}
	return false
}

func languageNames() []string {
	names := make([]string, 0, len(supportedLanguages))
	for _, lang := range supportedLanguages {
		names = append(names, string(lang))
	}
	return names
}

func ensureInstalled(tool toolConfig) error {
	if _, err := exec.LookPath(tool.binary); err == nil {
		return nil
	}

	if len(tool.installCommand) == 0 {
		return fmt.Errorf("%s is not installed and no installer is configured", tool.binary)
	}

	fmt.Printf("%s not found. Installing...\n", tool.binary)
	cmd := exec.Command(tool.installCommand[0], tool.installCommand[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install %s: %w", tool.binary, err)
	}

	if _, err := exec.LookPath(tool.binary); err != nil {
		return fmt.Errorf("installed dependencies but %s is still not on PATH", tool.binary)
	}
	return nil
}

func execTool(binary string, args []string, dir string) (int, error) {
	cmd := exec.Command(binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("failed to run %s: %w", binary, err)
	}
	return 0, nil
}

func resolveMonorepoPlan(root string, args []string) (*monorepoPlan, error) {
	if len(args) == 0 || args[0] != "install" {
		return nil, nil
	}

	config, err := loadMonorepoConfig(root)
	if err != nil {
		return nil, err
	}

	profiles, err := detectRepoProfiles(root)
	if err != nil {
		return nil, err
	}
	if config != nil && len(config.Languages) > 0 {
		profiles, err = profilesFromConfig(root, *config)
		if err != nil {
			return nil, err
		}
	}

	if len(profiles) < 2 {
		return nil, nil
	}

	installTarget := findFlagValue(args, "--target", "-t")
	installAgents, installAll := parseInstallSelection(args)
	if installTarget == "" && config != nil {
		installTarget = config.Target
	}
	if len(installAgents) == 0 && !installAll && config != nil {
		installAgents = slices.Clone(config.Agents)
	}
	if installTarget == "" || (len(installAgents) == 0 && !installAll) {
		return nil, errors.New("monorepo install requires --target and --agent/--all, or a root .rulesrc.json with target, agents, languages, and paths")
	}

	selectedAgents := installAgents
	if installAll {
		selectedAgents = append(slices.Clone(commonAgents), languageAgents...)
	}
	if err := validateSelectedAgents(selectedAgents); err != nil {
		return nil, err
	}

	configToSave := monorepoConfig{
		Target:    installTarget,
		Agents:    selectedAgents,
		Languages: make([]string, 0, len(profiles)),
		Paths:     map[string][]string{},
	}
	for _, profile := range profiles {
		configToSave.Languages = append(configToSave.Languages, string(profile.Language))
		configToSave.Paths[string(profile.Language)] = relativePaths(root, profile.Paths)
	}

	commonSelection := filterAgents(selectedAgents, commonAgents)
	languageSelection := filterAgents(selectedAgents, languageAgents)
	baseArgs := stripMonorepoFlags(args)

	plan := make([]backendInvocation, 0, len(profiles)+1)
	if len(commonSelection) > 0 {
		commonLanguage := profiles[0].Language
		if hasLanguage(profiles, langTypeScript) {
			commonLanguage = langTypeScript
		}
		tool := toolsByLanguage[commonLanguage]
		plan = append(plan, backendInvocation{
			Language: commonLanguage,
			Binary:   tool.binary,
			Dir:      root,
			Args:     withAgentSelection(baseArgs, commonSelection),
		})
	}
	for _, profile := range profiles {
		if len(languageSelection) == 0 {
			continue
		}
		tool := toolsByLanguage[profile.Language]
		for _, installPath := range profile.Paths {
			plan = append(plan, backendInvocation{
				Language: profile.Language,
				Binary:   tool.binary,
				Dir:      installPath,
				Args:     withAgentSelection(baseArgs, languageSelection),
			})
		}
	}

	if len(plan) == 0 {
		return nil, fmt.Errorf(
			"no supported agents selected for monorepo install; supported agents: %s",
			strings.Join(supportedAgents, ", "),
		)
	}

	return &monorepoPlan{
		Invocations: plan,
		Config:      configToSave,
	}, nil
}

func loadMonorepoConfig(root string) (*monorepoConfig, error) {
	path := filepath.Join(root, ".rulesrc.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var config monorepoConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(config.Languages) == 0 || len(config.Paths) == 0 {
		return nil, nil
	}
	return &config, nil
}

func saveMonorepoConfig(root string, config monorepoConfig) error {
	bytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal monorepo config: %w", err)
	}
	return os.WriteFile(filepath.Join(root, ".rulesrc.json"), append(bytes, '\n'), 0o644)
}

func profilesFromConfig(root string, config monorepoConfig) ([]repoProfile, error) {
	profiles := make([]repoProfile, 0, len(config.Languages))
	for _, rawLanguage := range config.Languages {
		lang := language(strings.ToLower(strings.TrimSpace(rawLanguage)))
		if !isSupportedLanguage(lang) {
			continue
		}
		rawPaths := config.Paths[string(lang)]
		paths := make([]string, 0, len(rawPaths))
		for _, rawPath := range rawPaths {
			if strings.TrimSpace(rawPath) == "" {
				continue
			}
			safePath, err := resolveScopedPath(root, rawPath)
			if err != nil {
				return nil, fmt.Errorf("invalid %s path %q in .rulesrc.json: %w", lang, rawPath, err)
			}
			paths = append(paths, safePath)
		}
		if len(paths) == 0 {
			continue
		}
		profiles = append(profiles, repoProfile{Language: lang, Paths: uniqueStrings(paths)})
	}
	return profiles, nil
}

func detectRepoProfiles(root string) ([]repoProfile, error) {
	pathsByLanguage := map[language][]string{
		langTypeScript: {},
		langPython:     {},
		langGo:         {},
	}

	if err := walkDirFunc(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == ".venv" || name == "dist" || name == "build" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		dir := filepath.Dir(path)
		switch d.Name() {
		case "tsconfig.json":
			pathsByLanguage[langTypeScript] = append(pathsByLanguage[langTypeScript], dir)
		case "pyproject.toml", "requirements.txt":
			pathsByLanguage[langPython] = append(pathsByLanguage[langPython], dir)
		case "go.mod":
			pathsByLanguage[langGo] = append(pathsByLanguage[langGo], dir)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan repo for language profiles: %w", err)
	}

	profiles := make([]repoProfile, 0, len(pathsByLanguage))
	for _, lang := range supportedLanguages {
		paths := uniqueStrings(pathsByLanguage[lang])
		if len(paths) == 0 {
			continue
		}
		profiles = append(profiles, repoProfile{Language: lang, Paths: paths})
	}
	return profiles, nil
}

func parseInstallSelection(args []string) ([]string, bool) {
	agents := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--all" {
			return nil, true
		}
		if strings.HasPrefix(arg, "--agent=") {
			agents = append(agents, splitAgentValues(strings.TrimPrefix(arg, "--agent="))...)
			continue
		}
		if arg == "--agent" || arg == "-a" {
			if i+1 >= len(args) {
				continue
			}
			agents = append(agents, splitAgentValues(args[i+1])...)
			i++
		}
	}
	return uniqueStrings(agents), false
}

func splitAgentValues(raw string) []string {
	parts := strings.Split(raw, ",")
	agents := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		agents = append(agents, trimmed)
	}
	return agents
}

func findFlagValue(args []string, longFlag string, shortFlag string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, longFlag+"=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, longFlag+"="))
		}
		if arg == longFlag || arg == shortFlag {
			if i+1 >= len(args) {
				return ""
			}
			return strings.TrimSpace(args[i+1])
		}
	}
	return ""
}

func stripMonorepoFlags(args []string) []string {
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--language" || arg == "-l" {
			i++
			continue
		}
		if strings.HasPrefix(arg, "--language=") {
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}

func withAgentSelection(baseArgs []string, agents []string) []string {
	filtered := make([]string, 0, len(baseArgs))
	for i := 0; i < len(baseArgs); i++ {
		arg := baseArgs[i]
		if arg == "--all" {
			continue
		}
		if arg == "--agent" || arg == "-a" {
			i++
			continue
		}
		if strings.HasPrefix(arg, "--agent=") {
			continue
		}
		filtered = append(filtered, arg)
	}
	if len(agents) > 0 {
		filtered = append(filtered, "--agent", strings.Join(agents, ","))
	}
	return filtered
}

func filterAgents(selected []string, allowed []string) []string {
	allowedSet := map[string]struct{}{}
	for _, agent := range allowed {
		allowedSet[agent] = struct{}{}
	}
	filtered := []string{}
	for _, agent := range selected {
		if _, ok := allowedSet[agent]; ok {
			filtered = append(filtered, agent)
		}
	}
	return uniqueStrings(filtered)
}

func hasLanguage(profiles []repoProfile, target language) bool {
	for _, profile := range profiles {
		if profile.Language == target {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

func relativePaths(root string, paths []string) []string {
	relative := make([]string, 0, len(paths))
	for _, currentPath := range paths {
		rel, err := filepath.Rel(root, currentPath)
		if err != nil {
			relative = append(relative, currentPath)
			continue
		}
		relative = append(relative, filepath.Clean(rel))
	}
	return uniqueStrings(relative)
}

func validateSelectedAgents(agents []string) error {
	invalid := []string{}
	for _, agent := range uniqueStrings(agents) {
		if !slices.Contains(supportedAgents, agent) {
			invalid = append(invalid, agent)
		}
	}
	if len(invalid) > 0 {
		return fmt.Errorf(
			"unsupported agent selection: %s (supported agents: %s)",
			strings.Join(invalid, ", "),
			strings.Join(supportedAgents, ", "),
		)
	}
	return nil
}

func resolveScopedPath(root string, rawPath string) (string, error) {
	if filepath.IsAbs(rawPath) {
		return "", errors.New("absolute paths are not allowed")
	}

	candidate := filepath.Clean(filepath.Join(root, rawPath))
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes repository root")
	}
	return candidate, nil
}

func findProjectRoot(cwd string) string {
	dir := cwd
	if strings.TrimSpace(dir) == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "."
		}
	}
	dir = filepath.Clean(dir)

	for {
		if hasRootMarker(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return cwdOrDot(cwd)
		}
		dir = parent
	}
}

func cwdOrDot(cwd string) string {
	if strings.TrimSpace(cwd) == "" {
		return "."
	}
	return cwd
}

func hasRootMarker(dir string) bool {
	markers := []string{".git", "go.mod", "pyproject.toml", "package.json", "pnpm-lock.yaml", "uv.lock"}
	for _, marker := range markers {
		if fileExists(filepath.Join(dir, marker)) {
			return true
		}
	}
	return false
}

func detectLanguage(root string) language {
	scores := map[language]int{
		langTypeScript: 0,
		langPython:     0,
		langGo:         0,
	}

	applyMarkerScores(root, scores)
	applyConfigScores(root, scores)

	best := language("")
	bestScore := 0
	tie := false
	for _, candidate := range supportedLanguages {
		score := scores[candidate]
		if score > bestScore {
			best = candidate
			bestScore = score
			tie = false
			continue
		}
		if score == bestScore && score > 0 {
			tie = true
		}
	}

	if bestScore == 0 || tie {
		return ""
	}
	return best
}

func applyMarkerScores(root string, scores map[language]int) {
	if fileExists(filepath.Join(root, "go.mod")) {
		scores[langGo] += 10
	}
	if fileExists(filepath.Join(root, "pyproject.toml")) || fileExists(filepath.Join(root, "requirements.txt")) || fileExists(filepath.Join(root, "uv.lock")) {
		scores[langPython] += 10
	}
	if fileExists(filepath.Join(root, "tsconfig.json")) {
		scores[langTypeScript] += 10
	}
	if fileExists(filepath.Join(root, "package.json")) {
		scores[langTypeScript] += 6
	}
}

func applyConfigScores(root string, scores map[language]int) {
	if fileExists(filepath.Join(root, ".rulesrc.go.json")) {
		scores[langGo] += 20
	}
	if fileExists(filepath.Join(root, ".rulesrc.python.json")) {
		scores[langPython] += 20
	}
	if fileExists(filepath.Join(root, ".rulesrc.ts.json")) || fileExists(filepath.Join(root, ".rulesrc.json")) {
		scores[langTypeScript] += 20
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

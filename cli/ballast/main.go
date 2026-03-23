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
	installCommand func(version string, projectRoot string) []string
}

func localSourceRoot() string {
	return findBallastSourceRoot()
}

var toolsByLanguage = map[language]toolConfig{
	langTypeScript: {
		binary: "ballast-typescript",
		installCommand: func(version string, projectRoot string) []string {
			toolRoot := filepath.Join(projectRoot, ".ballast", "tools", "typescript")
			if releaseVersion(version) == "" {
				if sourceRoot := localSourceRoot(); sourceRoot != "" {
					return []string{"npm", "install", "--prefix", toolRoot, filepath.Join(sourceRoot, "packages", "ballast-typescript")}
				}
			}
			pkg := "@everydaydevopsio/ballast"
			if releaseVersion(version) != "" {
				pkg += "@" + releaseVersion(version)
			}
			return []string{"npm", "install", "--prefix", toolRoot, pkg}
		},
	},
	langPython: {
		binary: "ballast-python",
		installCommand: func(version string, projectRoot string) []string {
			binDir := filepath.Join(projectRoot, ".ballast", "bin")
			toolDir := filepath.Join(projectRoot, ".ballast", "tools", "python")
			release := releaseVersion(version)
			if release == "" {
				if sourceRoot := localSourceRoot(); sourceRoot != "" {
					return []string{"env", "UV_TOOL_DIR=" + toolDir, "UV_TOOL_BIN_DIR=" + binDir, "uv", "tool", "install", "--reinstall", filepath.Join(sourceRoot, "packages", "ballast-python")}
				}
				release = releaseVersion(resolveVersion())
			}
			wheel := fmt.Sprintf(
				"https://github.com/everydaydevopsio/ballast/releases/download/v%s/ballast_python-%s-py3-none-any.whl",
				release,
				release,
			)
			return []string{"env", "UV_TOOL_DIR=" + toolDir, "UV_TOOL_BIN_DIR=" + binDir, "uv", "tool", "install", "--reinstall", "--from", wheel, "ballast-python"}
		},
	},
	langGo: {
		binary: "ballast-go",
		installCommand: func(version string, projectRoot string) []string {
			if releaseVersion(version) == "" {
				if sourceRoot := localSourceRoot(); sourceRoot != "" {
					moduleRoot := filepath.Join(sourceRoot, "packages", "ballast-go")
					return []string{"go", "build", "-C", moduleRoot, "-o", filepath.Join(projectRoot, ".ballast", "bin", "ballast-go"), "./cmd/ballast-go"}
				}
			}
			target := "latest"
			if release := releaseVersion(version); release != "" {
				target = "v" + release
			}
			return []string{"env", "GOBIN=" + filepath.Join(projectRoot, ".ballast", "bin"), "go", "install", "github.com/everydaydevopsio/ballast/packages/ballast-go/cmd/ballast-go@" + target}
		},
	},
}

var version = "dev"

var ensureInstalledFunc = ensureInstalled
var execToolFunc = execTool
var walkDirFunc = filepath.WalkDir
var osExecutableFunc = os.Executable
var execLookPathFunc = exec.LookPath
var runCommandFunc = runCommand
var resolveInstalledVersionFunc = resolveInstalledVersion
var collectDoctorBackendsFunc = collectDoctorBackends

var commonAgents = []string{"local-dev", "cicd", "observability"}
var languageAgents = []string{"linting", "logging", "testing"}
var supportedAgents = append(slices.Clone(commonAgents), languageAgents...)

type monorepoConfig struct {
	Target         string              `json:"target,omitempty"`
	Agents         []string            `json:"agents,omitempty"`
	BallastVersion string              `json:"ballastVersion,omitempty"`
	Languages      []string            `json:"languages,omitempty"`
	Paths          map[string][]string `json:"paths,omitempty"`
}

type repoProfile struct {
	Language language
	Paths    []string
}

type backendInvocation struct {
	Language language
	Binary   string
	Dir      string
	Env      map[string]string
	Args     []string
}

type doctorBackendStatus struct {
	Name     string
	Version  string
	Location string
	Found    bool
}

type monorepoPlan struct {
	Invocations []backendInvocation
	Config      monorepoConfig
	Target      string
	Common      []string
	Language    []string
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

	if len(forwardedArgs) > 0 && forwardedArgs[0] == "install-cli" {
		return runInstallCLI(selectedLanguage, forwardedArgs[1:])
	}
	if len(forwardedArgs) > 0 && forwardedArgs[0] == "doctor" {
		return runDoctor(selectedLanguage, forwardedArgs[1:])
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

	root := findProjectRoot("")
	normalizedArgs, err := normalizeInstallArgs(forwardedArgs, root)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	forwardedArgs = normalizedArgs

	if selectedLanguage == "" {
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
				resolved := resolveBackendCommand(invocation.Language, tool, invocation.Args, invocation.Env)
				if !resolved.UseLocal {
					if err := ensureInstalledFunc(tool); err != nil {
						fmt.Println(err)
						return 1
					}
					resolved = resolveBackendCommand(invocation.Language, tool, invocation.Args, invocation.Env)
				}
				exitCode, err := execToolFunc(resolved.Binary, resolved.Args, invocation.Dir, resolved.Env)
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
			if err := updateMonorepoSupportFiles(root, plan, forwardedArgs); err != nil {
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

	resolved := resolveBackendCommand(selectedLanguage, tool, forwardedArgs, nil)
	if !resolved.UseLocal {
		if err := ensureInstalledFunc(tool); err != nil {
			fmt.Println(err)
			return 1
		}
		resolved = resolveBackendCommand(selectedLanguage, tool, forwardedArgs, nil)
	}

	exitCode, err := execToolFunc(resolved.Binary, resolved.Args, "", resolved.Env)
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
	fmt.Println("  install     Install agent rules for the detected or selected language (`--refresh-config` reuses saved .rulesrc.json settings)")
	fmt.Println("  install-cli Install or upgrade backend CLIs (latest by default, or a specific --version)")
	fmt.Println("  doctor      Check local Ballast CLI versions and .rulesrc.json metadata (`--fix` installs/upgrades CLIs and refreshes config)")
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
	fmt.Println("  ballast install --refresh-config")
	fmt.Println("  ballast install-cli --language python")
	fmt.Println("  ballast doctor")
	fmt.Println("  ballast doctor --fix")
	fmt.Println("  ballast install-cli --language go --version 5.0.2")
	fmt.Println("  ballast install --target cursor --all --yes   # auto-detect and install across a TypeScript/Python/Go monorepo")
	fmt.Println("  ballast --language python install --target codex --agent linting")
	fmt.Println("  ballast --version")
	fmt.Println()
	fmt.Println("When --language is omitted, ballast detects the repository layout.")
	fmt.Println("Single-language repos are forwarded to the matching backend CLI.")
	fmt.Println("Mixed TypeScript/Python/Go monorepos install all rules at the repo root under per-language directories (for example `.claude/rules/typescript/` and `.codex/rules/python/`).")
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

func releaseVersion(raw string) string {
	trimmed := normalizeVersion(raw)
	if trimmed == "" || trimmed == "dev" || trimmed == "(devel)" {
		return ""
	}
	return trimmed
}

func normalizeVersion(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "v")
	return trimmed
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
	root := findProjectRoot("")
	lang, ok := languageForBinary(tool.binary)
	if !ok {
		return fmt.Errorf("unsupported tool binary: %s", tool.binary)
	}
	local := projectLocalBackendCommand(root, lang)
	desiredVersion := resolveVersion()
	desiredRelease := releaseVersion(desiredVersion)
	if local.Binary != "" {
		if desiredRelease == "" {
			return nil
		}
		currentVersion, versionErr := resolveCommandVersion(local.Binary)
		if versionErr == nil && normalizeVersion(currentVersion) == normalizeVersion(desiredRelease) {
			return nil
		}
		if versionErr == nil {
			fmt.Printf(
				"%s %s does not match ballast %s. Reinstalling...\n",
				tool.binary,
				currentVersion,
				desiredRelease,
			)
		} else {
			fmt.Printf(
				"Could not determine %s version (%v). Reinstalling to match ballast %s...\n",
				tool.binary,
				versionErr,
				desiredRelease,
			)
		}
	} else {
		fmt.Printf("%s not found in %s. Installing...\n", tool.binary, filepath.Join(root, ".ballast"))
	}

	if tool.installCommand == nil {
		return fmt.Errorf("%s is not installed and no installer is configured", tool.binary)
	}

	if err := os.MkdirAll(filepath.Join(root, ".ballast", "bin"), 0o755); err != nil {
		return fmt.Errorf("create local ballast bin dir: %w", err)
	}
	installCommand := tool.installCommand(desiredVersion, root)
	if len(installCommand) == 0 {
		return fmt.Errorf("%s is not installed and no installer is configured", tool.binary)
	}

	if err := runCommandFunc(installCommand[0], installCommand[1:]); err != nil {
		return fmt.Errorf("failed to install %s: %w", tool.binary, err)
	}

	local = projectLocalBackendCommand(root, lang)
	if local.Binary == "" {
		return fmt.Errorf("installed dependencies but %s is still not available in %s", tool.binary, filepath.Join(root, ".ballast"))
	}
	return nil
}

func runInstallCLI(selectedLanguage language, args []string) int {
	version, err := parseInstallCLIVersion(args)
	if err != nil {
		fmt.Println(err)
		return 1
	}

	return installCLIs(selectedLanguage, version)
}

func installCLIs(selectedLanguage language, version string) int {
	root := findProjectRoot("")
	var languagesToInstall []language
	if selectedLanguage != "" {
		languagesToInstall = []language{selectedLanguage}
	} else {
		languagesToInstall = slices.Clone(supportedLanguages)
	}

	for _, lang := range languagesToInstall {
		tool, ok := toolsByLanguage[lang]
		if !ok {
			fmt.Printf("Unsupported language: %s\n", lang)
			return 1
		}
		command := tool.installCommand(version, root)
		if len(command) == 0 {
			fmt.Printf("No installer configured for %s\n", lang)
			return 1
		}
		if err := runCommandFunc(command[0], command[1:]); err != nil {
			fmt.Printf("failed to install %s: %v\n", tool.binary, err)
			return 1
		}
	}

	return 0
}

func runDoctor(selectedLanguage language, args []string) int {
	fix, err := parseDoctorFix(args)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	root := findProjectRoot("")
	printDoctorSummary(root, selectedLanguage, fix)

	if fix {
		if exitCode := installCLIs(selectedLanguage, resolveVersion()); exitCode != 0 {
			return exitCode
		}
		if fileExists(filepath.Join(root, ".rulesrc.json")) {
			refreshArgs := []string{"install", "--refresh-config"}
			if selectedLanguage != "" {
				refreshArgs = append([]string{"--language", string(selectedLanguage)}, refreshArgs...)
			}
			return run(refreshArgs)
		}
		return 0
	}
	return 0
}

func printDoctorSummary(root string, selectedLanguage language, fix bool) {
	fmt.Println("Ballast doctor")
	fmt.Printf("Project root: %s\n", root)
	if selectedLanguage != "" {
		fmt.Printf("Fix target: %s\n", selectedLanguage)
	}
	if fix {
		fmt.Println("Mode: fix")
	}
	fmt.Println()

	fmt.Println("Installed backends:")
	for _, status := range collectDoctorBackendsFunc(root) {
		if !status.Found {
			fmt.Printf("- %s: not found\n", status.Name)
			continue
		}
		version := status.Version
		if strings.TrimSpace(version) == "" {
			version = "unknown"
		}
		fmt.Printf("- %s: %s (%s)\n", status.Name, version, status.Location)
	}
	fmt.Println()

	fmt.Println("Config:")
	configPath := filepath.Join(root, ".rulesrc.json")
	config, err := loadDoctorConfig(root)
	if err != nil {
		fmt.Printf("- .rulesrc.json: unreadable (%v)\n", err)
		fmt.Println()
		return
	}
	if config == nil {
		fmt.Println("- .rulesrc.json: not found")
		fmt.Println()
		return
	}
	fmt.Printf("- file: %s\n", configPath)
	if strings.TrimSpace(config.BallastVersion) == "" {
		fmt.Println("- ballastVersion: missing")
	} else {
		fmt.Printf("- ballastVersion: %s\n", config.BallastVersion)
	}
	if strings.TrimSpace(config.Target) != "" {
		fmt.Printf("- target: %s\n", config.Target)
	}
	if len(config.Agents) > 0 {
		fmt.Printf("- agents: %s\n", strings.Join(config.Agents, ", "))
	}
	fmt.Println()
}

func parseDoctorFix(args []string) (bool, error) {
	fix := false
	for _, arg := range args {
		switch arg {
		case "--fix":
			fix = true
		default:
			return false, fmt.Errorf("unknown doctor option: %s", arg)
		}
	}
	return fix, nil
}

func parseInstallCLIVersion(args []string) (string, error) {
	version := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case strings.HasPrefix(arg, "--version="):
			version = strings.TrimSpace(strings.TrimPrefix(arg, "--version="))
		case arg == "--version":
			if index+1 >= len(args) {
				return "", errors.New("missing value for --version")
			}
			version = strings.TrimSpace(args[index+1])
			index++
		default:
			return "", fmt.Errorf("unknown install-cli option: %s", arg)
		}
	}
	return version, nil
}

func normalizeInstallArgs(args []string, root string) ([]string, error) {
	if len(args) == 0 || args[0] != "install" {
		return args, nil
	}

	filtered := []string{args[0]}
	refreshConfig := false
	for index := 1; index < len(args); index++ {
		arg := args[index]
		if arg == "--refresh-config" {
			refreshConfig = true
			continue
		}
		filtered = append(filtered, arg)
	}

	if !refreshConfig {
		return filtered, nil
	}
	if !fileExists(filepath.Join(root, ".rulesrc.json")) {
		return nil, errors.New("--refresh-config requires an existing .rulesrc.json")
	}
	if findFlagValue(filtered, "--target", "-t") == "" && !hasFlag(filtered, "--yes", "-y") {
		filtered = append(filtered, "--yes")
	}
	return filtered, nil
}

func hasFlag(args []string, longFlag string, shortFlag string) bool {
	for _, arg := range args {
		if arg == longFlag || arg == shortFlag {
			return true
		}
	}
	return false
}

func runCommand(name string, args []string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func resolveCommandVersion(binary string) (string, error) {
	output, err := exec.Command(binary, "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run --version: %w", err)
	}
	versionText := strings.TrimSpace(string(output))
	if versionText == "" {
		return "", errors.New("empty version output")
	}
	return versionText, nil
}

func resolveInstalledVersion(tool toolConfig) (string, error) {
	return resolveCommandVersion(tool.binary)
}

func resolveCommandVersionWithArgs(binary string, args []string, env map[string]string) (string, error) {
	cmd := exec.Command(binary, append(append([]string(nil), args...), "--version")...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), envPairs(env)...)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run --version: %w", err)
	}
	versionText := strings.TrimSpace(string(output))
	if versionText == "" {
		return "", errors.New("empty version output")
	}
	return versionText, nil
}

func collectDoctorBackends(root string) []doctorBackendStatus {
	statuses := make([]doctorBackendStatus, 0, len(supportedLanguages))
	for _, lang := range supportedLanguages {
		tool := toolsByLanguage[lang]
		resolved := resolveBackendCommand(lang, tool, nil, nil)
		statuses = append(statuses, detectDoctorBackendStatus(resolved, tool))
	}
	return statuses
}

func detectDoctorBackendStatus(resolved resolvedBackendCommand, tool toolConfig) doctorBackendStatus {
	location := resolvedCommandLocation(resolved)
	if resolved.UseLocal {
		version, err := resolveCommandVersionWithArgs(resolved.Binary, resolved.Args, resolved.Env)
		if err != nil {
			return doctorBackendStatus{Name: tool.binary, Version: "", Location: location, Found: true}
		}
		return doctorBackendStatus{Name: tool.binary, Version: version, Location: location, Found: true}
	}

	binaryPath, err := execLookPathFunc(resolved.Binary)
	if err != nil {
		return doctorBackendStatus{Name: tool.binary}
	}

	version, err := resolveCommandVersionWithArgs(resolved.Binary, resolved.Args, resolved.Env)
	if err != nil {
		return doctorBackendStatus{Name: tool.binary, Version: "", Location: binaryPath, Found: true}
	}
	return doctorBackendStatus{Name: tool.binary, Version: version, Location: binaryPath, Found: true}
}

func resolvedCommandLocation(resolved resolvedBackendCommand) string {
	switch resolved.Binary {
	case "node":
		if len(resolved.Args) > 0 {
			return resolved.Args[0]
		}
	case "python3":
		if pythonPath := strings.TrimSpace(resolved.Env["PYTHONPATH"]); pythonPath != "" {
			return pythonPath
		}
	}
	return resolved.Binary
}

func execTool(binary string, args []string, dir string, env map[string]string) (int, error) {
	cmd := exec.Command(binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), envPairs(env)...)
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

func loadDoctorConfig(root string) (*monorepoConfig, error) {
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
	return &config, nil
}

type resolvedBackendCommand struct {
	Binary   string
	Args     []string
	Env      map[string]string
	UseLocal bool
}

func resolveBackendCommand(lang language, tool toolConfig, args []string, env map[string]string) resolvedBackendCommand {
	mergedEnv := cloneEnvMap(env)
	repoRoot := findBallastSourceRoot()
	if repoRoot == "" {
		projectRoot := findProjectRoot("")
		local := projectLocalBackendCommand(projectRoot, lang)
		if local.Binary != "" {
			local.Args = append(local.Args, args...)
			local.Env = mergeResolvedEnv(mergedEnv, local.Env)
			local.UseLocal = true
			return local
		}
		return resolvedBackendCommand{
			Binary: tool.binary,
			Args:   append([]string(nil), args...),
			Env:    mergedEnv,
		}
	}

	local := resolveLocalBackendCommand(repoRoot, lang)
	if local.Binary == "" {
		return resolvedBackendCommand{
			Binary: tool.binary,
			Args:   append([]string(nil), args...),
			Env:    mergedEnv,
		}
	}

	projectRoot := findProjectRoot("")
	projectLocal := projectLocalBackendCommand(projectRoot, lang)
	if projectLocal.Binary != "" {
		projectLocal.Args = append(projectLocal.Args, args...)
		projectLocal.Env = mergeResolvedEnv(mergedEnv, projectLocal.Env)
		projectLocal.UseLocal = true
		return projectLocal
	}

	if mergedEnv == nil {
		mergedEnv = map[string]string{}
	}
	mergedEnv["BALLAST_REPO_ROOT"] = repoRoot
	for key, value := range local.Env {
		mergedEnv[key] = value
	}

	return resolvedBackendCommand{
		Binary:   local.Binary,
		Args:     append(append([]string(nil), local.Args...), args...),
		Env:      mergedEnv,
		UseLocal: true,
	}
}

func mergeResolvedEnv(base map[string]string, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	merged := cloneEnvMap(base)
	if merged == nil {
		merged = map[string]string{}
	}
	for key, value := range extra {
		merged[key] = value
	}
	return merged
}

func resolveLocalBackendCommand(repoRoot string, lang language) resolvedBackendCommand {
	if siblingBinary, ok := siblingBackendBinary(lang); ok {
		return resolvedBackendCommand{
			Binary: siblingBinary,
		}
	}
	switch lang {
	case langTypeScript:
		cliPath := filepath.Join(repoRoot, "packages", "ballast-typescript", "dist", "cli.js")
		if fileExists(cliPath) {
			return resolvedBackendCommand{
				Binary: "node",
				Args:   []string{cliPath},
			}
		}
	case langPython:
		modulePath := filepath.Join(repoRoot, "packages", "ballast-python", "ballast", "__main__.py")
		if fileExists(modulePath) {
			return resolvedBackendCommand{
				Binary: "python3",
				Args:   []string{"-m", "ballast"},
				Env: map[string]string{
					"PYTHONPATH": filepath.Join(repoRoot, "packages", "ballast-python"),
				},
			}
		}
	case langGo:
		binaryPath := filepath.Join(repoRoot, "packages", "ballast-go", "ballast-go")
		if fileExists(binaryPath) {
			return resolvedBackendCommand{
				Binary: binaryPath,
			}
		}
	}
	return resolvedBackendCommand{}
}

func projectLocalBackendCommand(projectRoot string, lang language) resolvedBackendCommand {
	switch lang {
	case langTypeScript:
		binary := filepath.Join(projectRoot, ".ballast", "tools", "typescript", "node_modules", ".bin", "ballast-typescript")
		if fileExists(binary) {
			return resolvedBackendCommand{Binary: binary}
		}
	case langPython:
		binary := filepath.Join(projectRoot, ".ballast", "bin", "ballast-python")
		if fileExists(binary) {
			return resolvedBackendCommand{Binary: binary}
		}
	case langGo:
		binary := filepath.Join(projectRoot, ".ballast", "bin", "ballast-go")
		if fileExists(binary) {
			return resolvedBackendCommand{Binary: binary}
		}
	}
	return resolvedBackendCommand{}
}

func languageForBinary(binary string) (language, bool) {
	switch binary {
	case "ballast-typescript":
		return langTypeScript, true
	case "ballast-python":
		return langPython, true
	case "ballast-go":
		return langGo, true
	default:
		return "", false
	}
}

func siblingBackendBinary(lang language) (string, bool) {
	executable, err := osExecutableFunc()
	if err != nil {
		return "", false
	}
	dir := filepath.Dir(executable)
	var name string
	switch lang {
	case langTypeScript:
		name = "ballast-typescript"
	case langPython:
		name = "ballast-python"
	case langGo:
		name = "ballast-go"
	default:
		return "", false
	}
	path := filepath.Join(dir, name)
	if !fileExists(path) {
		return "", false
	}
	return path, true
}

func findBallastSourceRoot() string {
	executable, err := osExecutableFunc()
	if err != nil {
		return ""
	}
	current := filepath.Dir(executable)
	for {
		if isBallastSourceRoot(current) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func isBallastSourceRoot(root string) bool {
	return fileExists(filepath.Join(root, "packages", "ballast-typescript", "package.json")) &&
		fileExists(filepath.Join(root, "packages", "ballast-python", "pyproject.toml")) &&
		fileExists(filepath.Join(root, "packages", "ballast-go", "go.mod"))
}

func cloneEnvMap(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(env))
	for key, value := range env {
		cloned[key] = value
	}
	return cloned
}

func resolveMonorepoPlan(root string, args []string) (*monorepoPlan, error) {
	if len(args) == 0 || args[0] != "install" {
		return nil, nil
	}

	config, err := loadMonorepoConfig(root)
	if err != nil {
		return nil, err
	}

	detectedProfiles, err := detectRepoProfiles(root)
	if err != nil {
		return nil, err
	}

	profiles := detectedProfiles
	if config != nil && len(config.Languages) > 0 {
		configProfiles, err := profilesFromConfig(root, *config)
		if err != nil {
			return nil, err
		}
		if len(detectedProfiles) == 0 || profilesMatchRepo(detectedProfiles, configProfiles) {
			profiles = configProfiles
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
		Target:         installTarget,
		Agents:         selectedAgents,
		BallastVersion: normalizeVersion(resolveVersion()),
		Languages:      make([]string, 0, len(profiles)),
		Paths:          map[string][]string{},
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
			Env:      monorepoInvocationEnv("common"),
			Args:     withAgentSelection(baseArgs, commonSelection),
		})
	}
	for _, profile := range profiles {
		if len(languageSelection) == 0 {
			continue
		}
		tool := toolsByLanguage[profile.Language]
		plan = append(plan, backendInvocation{
			Language: profile.Language,
			Binary:   tool.binary,
			Dir:      root,
			Env:      monorepoInvocationEnv(string(profile.Language)),
			Args:     withAgentSelection(baseArgs, languageSelection),
		})
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
		Target:      installTarget,
		Common:      commonSelection,
		Language:    languageSelection,
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

func profilesMatchRepo(detected []repoProfile, configured []repoProfile) bool {
	if len(detected) != len(configured) {
		return false
	}

	detectedByLanguage := make(map[language][]string, len(detected))
	for _, profile := range detected {
		detectedByLanguage[profile.Language] = profile.Paths
	}

	configuredByLanguage := make(map[language][]string, len(configured))
	for _, profile := range configured {
		configuredByLanguage[profile.Language] = profile.Paths
	}

	if len(detectedByLanguage) != len(configuredByLanguage) {
		return false
	}

	for lang, detectedPaths := range detectedByLanguage {
		configuredPaths, ok := configuredByLanguage[lang]
		if !ok || !sameStringSet(detectedPaths, configuredPaths) {
			return false
		}
	}

	return true
}

func sameStringSet(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	counts := make(map[string]int, len(left))
	for _, value := range left {
		counts[value]++
	}

	for _, value := range right {
		count := counts[value]
		if count == 0 {
			return false
		}
		if count == 1 {
			delete(counts, value)
			continue
		}
		counts[value] = count - 1
	}

	return len(counts) == 0
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

func monorepoInvocationEnv(subdir string) map[string]string {
	return map[string]string{
		"BALLAST_RULE_SUBDIR":           subdir,
		"BALLAST_DISABLE_SUPPORT_FILES": "1",
	}
}

func envPairs(env map[string]string) []string {
	pairs := make([]string, 0, len(env))
	for key, value := range env {
		pairs = append(pairs, key+"="+value)
	}
	return pairs
}

func updateMonorepoSupportFiles(root string, plan *monorepoPlan, args []string) error {
	if plan.Target != "claude" && plan.Target != "codex" {
		return nil
	}

	path := supportFilePath(root, plan.Target)
	content := buildMonorepoSupportFile(plan)
	if !fileExists(path) {
		return os.WriteFile(path, []byte(content), 0o644)
	}
	if hasPatchFlag(args) {
		return os.WriteFile(path, []byte(patchInstalledRulesSection(readFile(path), content)), 0o644)
	}
	if isInteractiveInstall(args) {
		approved, err := promptSupportFilePatch(path)
		if err != nil {
			return err
		}
		if approved {
			return os.WriteFile(path, []byte(patchInstalledRulesSection(readFile(path), content)), 0o644)
		}
	}
	return nil
}

func supportFilePath(root string, target string) string {
	if target == "claude" {
		return filepath.Join(root, "CLAUDE.md")
	}
	return filepath.Join(root, "AGENTS.md")
}

func buildMonorepoSupportFile(plan *monorepoPlan) string {
	title := "# AGENTS.md"
	intro := "This file provides guidance to Codex (CLI and app) for working in this repository."
	rulesDir := ".codex/rules"
	extension := ".md"
	if plan.Target == "claude" {
		title = "# CLAUDE.md"
		intro = "This file provides guidance to Claude Code for working in this repository."
		rulesDir = ".claude/rules"
	}

	lines := []string{
		title,
		"",
		intro,
		"",
		"## Installed agent rules",
		"",
		"Created by Ballast. Do not edit this section.",
		"",
		fmt.Sprintf("Read and follow these rule files in `%s/` when they apply:", rulesDir),
		"",
	}

	for _, agent := range plan.Common {
		for _, suffix := range ruleSuffixesForAgent(agent) {
			base := agentBaseName(agent, suffix)
			lines = append(lines, fmt.Sprintf("- `.%s/common/%s%s` — Rules for common/%s", strings.TrimPrefix(rulesDir, "."), base, extension, base))
		}
	}
	for _, lang := range plan.Config.Languages {
		for _, agent := range plan.Language {
			for _, suffix := range ruleSuffixesForAgent(agent) {
				base := agentBaseName(agent, suffix)
				lines = append(lines, fmt.Sprintf("- `.%s/%s/%s-%s%s` — Rules for %s/%s", strings.TrimPrefix(rulesDir, "."), lang, lang, base, extension, lang, base))
			}
		}
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func ruleSuffixesForAgent(agent string) []string {
	if agent == "local-dev" {
		return []string{"badges", "env", "license", "mcp"}
	}
	return []string{""}
}

func agentBaseName(agent string, suffix string) string {
	if suffix == "" {
		return agent
	}
	return agent + "-" + suffix
}

func hasPatchFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--patch" || arg == "-p" {
			return true
		}
	}
	return false
}

func isInteractiveInstall(args []string) bool {
	for _, arg := range args {
		if arg == "--yes" || arg == "-y" {
			return false
		}
	}
	return true
}

func promptSupportFilePatch(path string) (bool, error) {
	fmt.Printf("Existing %s found. Patch the Installed agent rules section? [y/N]: ", filepath.Base(path))
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		if errors.Is(err, os.ErrClosed) || strings.Contains(err.Error(), "unexpected newline") || strings.Contains(err.Error(), "expected newline") {
			return false, nil
		}
		return false, err
	}
	value := strings.ToLower(strings.TrimSpace(response))
	return value == "y" || value == "yes", nil
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func patchInstalledRulesSection(existing string, canonical string) string {
	canonicalRange := findSectionRange(canonical, "Installed agent rules")
	if canonicalRange == nil {
		return existing
	}
	canonicalSection := strings.TrimRight(canonical[canonicalRange[0]:canonicalRange[1]], "\n")

	existingRange := findSectionRange(existing, "Installed agent rules")
	if existingRange == nil {
		return strings.TrimRight(existing, "\n") + "\n\n" + canonicalSection + "\n"
	}

	return strings.TrimRight(existing[:existingRange[0]], "\n") + "\n\n" +
		canonicalSection + "\n\n" +
		strings.TrimLeft(existing[existingRange[1]:], "\n")
}

func findSectionRange(content string, heading string) []int {
	match := indexHeading(content, "## "+heading)
	if match == nil {
		return nil
	}
	next := indexNextHeading(content[match[1]:])
	end := len(content)
	if next >= 0 {
		end = match[1] + next
	}
	return []int{match[0], end}
}

func indexHeading(content string, heading string) []int {
	lines := strings.Split(content, "\n")
	offset := 0
	inFence := false
	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
		}
		if !inFence && line == heading {
			return []int{offset, offset + len(line)}
		}
		offset += len(line) + 1
	}
	return nil
}

func indexNextHeading(content string) int {
	lines := strings.Split(content, "\n")
	offset := 0
	inFence := false
	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
		}
		if !inFence && strings.HasPrefix(line, "## ") {
			return offset
		}
		offset += len(line) + 1
	}
	return -1
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

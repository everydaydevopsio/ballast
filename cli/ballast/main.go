package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
)

type language string

const (
	langTypeScript language = "typescript"
	langPython     language = "python"
	langGo         language = "go"
	langAnsible    language = "ansible"
	langTerraform  language = "terraform"
)

var supportedLanguages = []language{langTypeScript, langPython, langGo, langAnsible, langTerraform}
var installableBackendLanguages = []language{langTypeScript, langPython, langGo}
var supportedTargets = []string{"cursor", "claude", "opencode", "codex", "gemini"}

type toolConfig struct {
	binary         string
	installCommand func(version string, projectRoot string) ([]string, error)
}

func localSourceRoot() string {
	return findBallastSourceRoot()
}

func preferredSourceRoot(projectRoot string) string {
	for root := filepath.Clean(projectRoot); root != "" && root != string(filepath.Separator); {
		if isBallastSourceRoot(root) {
			return root
		}
		parent := filepath.Dir(root)
		if parent == root {
			break
		}
		root = parent
	}
	return localSourceRoot()
}

func preferredInstallSourceRoot(projectRoot string, version string) string {
	if releaseVersion(version) != "" {
		return ""
	}
	return preferredSourceRoot(projectRoot)
}

var typeScriptTool = toolConfig{
	binary: "ballast-typescript",
	installCommand: func(version string, projectRoot string) ([]string, error) {
		toolRoot := filepath.Join(projectRoot, ".ballast", "tools", "typescript")
		if sourceRoot := preferredInstallSourceRoot(projectRoot, version); sourceRoot != "" {
			return []string{"npm", "install", "--prefix", toolRoot, filepath.Join(sourceRoot, "packages", "ballast-typescript")}, nil
		}
		pkg := "@everydaydevopsio/ballast"
		if releaseVersion(version) != "" {
			pkg += "@" + releaseVersion(version)
		}
		return []string{"npm", "install", "--prefix", toolRoot, pkg}, nil
	},
}

var pythonTool = toolConfig{
	binary: "ballast-python",
	installCommand: func(version string, projectRoot string) ([]string, error) {
		binDir := filepath.Join(projectRoot, ".ballast", "bin")
		toolDir := filepath.Join(projectRoot, ".ballast", "tools", "python")
		release := releaseVersion(version)
		if sourceRoot := preferredInstallSourceRoot(projectRoot, version); sourceRoot != "" {
			return []string{"env", "UV_TOOL_DIR=" + toolDir, "UV_TOOL_BIN_DIR=" + binDir, "uv", "tool", "install", "--reinstall", filepath.Join(sourceRoot, "packages", "ballast-python")}, nil
		}
		if release == "" {
			release = releaseVersion(resolveVersion())
			if release == "" {
				return nil, errors.New("ballast-python install requires a release version or a ballast source checkout")
			}
		}
		wheel := fmt.Sprintf(
			"https://github.com/everydaydevopsio/ballast/releases/download/v%s/ballast_python-%s-py3-none-any.whl",
			release,
			release,
		)
		return []string{"env", "UV_TOOL_DIR=" + toolDir, "UV_TOOL_BIN_DIR=" + binDir, "uv", "tool", "install", "--reinstall", "--from", wheel, "ballast-python"}, nil
	},
}

var goTool = toolConfig{
	binary: "ballast-go",
	installCommand: func(version string, projectRoot string) ([]string, error) {
		if sourceRoot := preferredInstallSourceRoot(projectRoot, version); sourceRoot != "" {
			moduleRoot := filepath.Join(sourceRoot, "packages", "ballast-go")
			return []string{"go", "build", "-C", moduleRoot, "-o", filepath.Join(projectRoot, ".ballast", "bin", "ballast-go"), "./cmd/ballast-go"}, nil
		}
		return releasedGoInstallCommand(version, projectRoot)
	},
}

var toolsByLanguage = map[language]toolConfig{
	langTypeScript: typeScriptTool,
	langPython:     pythonTool,
	langGo:         goTool,
	langAnsible:    goTool,
	langTerraform:  goTool,
}

var version = "dev"

var ensureInstalledFunc = ensureInstalled
var execToolFunc = execTool
var walkDirFunc = filepath.WalkDir
var osExecutableFunc = os.Executable
var execLookPathFunc = exec.LookPath
var runCommandFunc = runCommand
var runCommandOutputFunc = runCommandOutput
var resolveInstalledVersionFunc = resolveInstalledVersion
var collectDoctorBackendsFunc = collectDoctorBackends

var supportedTaskSystems = []string{"github", "jira", "linear"}

type monorepoConfig struct {
	Target         string              `json:"target,omitempty"`
	Targets        []string            `json:"targets,omitempty"`
	Agents         []string            `json:"agents,omitempty"`
	Skills         []string            `json:"skills,omitempty"`
	BallastVersion string              `json:"ballastVersion,omitempty"`
	Languages      []string            `json:"languages,omitempty"`
	Paths          map[string][]string `json:"paths,omitempty"`
	TaskSystem     string              `json:"taskSystem,omitempty"`
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
	Targets     []string
	Common      []string
	Language    []string
	Removed     []string
	Previous    *monorepoConfig
}

type repositoryFactsPayload struct {
	Version                int      `json:"version"`
	RepositoryFactsSection []string `json:"repositoryFactsSection"`
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
	if len(forwardedArgs) > 0 && forwardedArgs[0] == "upgrade" {
		return runUpgrade(selectedLanguage, forwardedArgs[1:])
	}
	if len(forwardedArgs) > 0 && forwardedArgs[0] == "update" {
		return runUpdate(forwardedArgs[1:])
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
	repositoryFactsEnv, cleanupRepositoryFacts, err := prepareRepositoryFactsEnv(root)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	defer cleanupRepositoryFacts()
	refreshConfigRequested := len(forwardedArgs) > 0 && forwardedArgs[0] == "install" && hasFlag(forwardedArgs, "--refresh-config", "")
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
				if refreshConfigRequested {
					invocation.Env = cloneEnvMap(invocation.Env)
					invocation.Env["BALLAST_REFRESH_SKILLS"] = "1"
					invocation.Env["BALLAST_REFRESH_TASK_RULES"] = "1"
				}
				invocation.Env = mergeResolvedEnv(invocation.Env, repositoryFactsEnv)
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
			if err := cleanupRemovedMonorepoTargets(root, plan); err != nil {
				fmt.Println(err)
				return 1
			}
			if err := cleanupStaleManagedSelections(root, plan); err != nil {
				fmt.Println(err)
				return 1
			}
			if err := updateMonorepoSupportFiles(root, plan, forwardedArgs); err != nil {
				fmt.Println(err)
				return 1
			}
			// Merge taskSystem written by backend invocations (e.g. on a fresh
			// install where plan.Config.TaskSystem starts empty).
			if plan.Config.TaskSystem == "" {
				plan.Config.TaskSystem = readTaskSystem(root)
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

	singleEnv := map[string]string(nil)
	if refreshConfigRequested {
		singleEnv = map[string]string{
			"BALLAST_REFRESH_SKILLS":     "1",
			"BALLAST_REFRESH_TASK_RULES": "1",
		}
	}
	singleEnv = mergeResolvedEnv(singleEnv, repositoryFactsEnv)
	resolved := resolveBackendCommand(selectedLanguage, tool, forwardedArgs, singleEnv)
	if !resolved.UseLocal {
		if err := ensureInstalledFunc(tool); err != nil {
			fmt.Println(err)
			return 1
		}
		resolved = resolveBackendCommand(selectedLanguage, tool, forwardedArgs, singleEnv)
	}

	exitCode, err := execToolFunc(resolved.Binary, resolved.Args, "", resolved.Env)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if exitCode == 0 && refreshConfigRequested {
		if err := cleanupSingleLanguageManagedSelections(root, selectedLanguage); err != nil {
			fmt.Println(err)
			return 1
		}
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
	fmt.Println("ballast installs AI agent rules for Cursor, Claude Code, OpenCode, Codex, and Gemini CLI.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ballast [flags] <command> [command flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  install     Install agent rules for the detected or selected language (`--refresh-config` reuses saved .rulesrc.json settings)")
	fmt.Println("  install-cli Install or upgrade backend CLIs (latest by default, or a specific --version)")
	fmt.Println("  doctor      Check local Ballast CLI versions and .rulesrc.json metadata (`--fix` installs/upgrades CLIs and refreshes config; add `--patch` with `--fix` to merge backend file updates during refresh)")
	fmt.Println("  upgrade     Rewrite .rulesrc.json to the running ballast version and sync backend CLIs (`--patch` and `--force` forward to the backend refresh)")
	fmt.Println("  update      Upgrade the ballast CLI itself via Homebrew (`brew update && brew upgrade ...`)")
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
	fmt.Println("  ballast install --target cursor,claude --all")
	fmt.Println("  ballast install --target gemini --all")
	fmt.Println("  ballast install --remove-target codex")
	fmt.Println("  ballast install --remove-language python")
	fmt.Println("  ballast install --target claude --skill owasp-security-scan")
	fmt.Println("  ballast install --refresh-config")
	fmt.Println("  ballast install-cli --language python")
	fmt.Println("  ballast doctor")
	fmt.Println("  ballast doctor --fix")
	fmt.Println("  ballast update")
	fmt.Println("  ballast upgrade")
	fmt.Println("  ballast upgrade --patch")
	fmt.Println("  ballast upgrade --force")
	fmt.Println("  ballast install-cli --language go --version 5.0.2")
	fmt.Println("  ballast install --target cursor --all --yes   # auto-detect and install across a TypeScript/Python/Go/Ansible/Terraform repo")
	fmt.Println("  ballast --language python install --target codex --agent linting")
	fmt.Println("  ballast --language ansible install --target cursor --all")
	fmt.Println("  ballast --language terraform install --target cursor --all")
	fmt.Println("  ballast --version")
	fmt.Println()
	fmt.Println("When --language is omitted, ballast detects the repository layout.")
	fmt.Println("Install target behavior: `--target` adds to the saved targets in `.rulesrc.json`; use `--remove-target` to stop managing a target and clean up Ballast-managed files for it.")
	fmt.Println("Install language behavior: `--remove-language` removes languages from `.rulesrc.json`, removes their `paths`, and prunes stale Ballast-managed rule files.")
	fmt.Println("Single-language repos are forwarded to the matching backend CLI.")
	fmt.Println("Mixed TypeScript/Python/Go/Ansible/Terraform repos install all rules at the repo root under per-language directories (for example `.claude/rules/typescript/`, `.gemini/rules/python/`, and `.codex/rules/terraform/`).")
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

	if err := ensureLocalToolDirs(root); err != nil {
		return err
	}
	installCommand, err := tool.installCommand(desiredVersion, root)
	if err != nil {
		return fmt.Errorf("prepare install for %s: %w", tool.binary, err)
	}
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
	if err := ensureLocalToolDirs(root); err != nil {
		fmt.Println(err)
		return 1
	}
	var languagesToInstall []language
	if selectedLanguage != "" {
		languagesToInstall = []language{selectedLanguage}
	} else {
		languagesToInstall = slices.Clone(installableBackendLanguages)
	}

	for _, lang := range languagesToInstall {
		tool, ok := toolsByLanguage[lang]
		if !ok {
			fmt.Printf("Unsupported language: %s\n", lang)
			return 1
		}
		command, err := tool.installCommand(version, root)
		if err != nil {
			fmt.Printf("failed to prepare %s install: %v\n", tool.binary, err)
			return 1
		}
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
	fix, patch, err := parseDoctorFix(args)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	root := findProjectRoot("")
	printDoctorSummary(root, selectedLanguage, fix)

	if fix {
		return runDoctorFix(root, selectedLanguage, patch, false)
	}
	return 0
}

func runUpdate(args []string) int {
	if len(args) > 0 {
		fmt.Printf("unknown update option: %s\n", args[0])
		return 1
	}
	if !detectBrewInstall() {
		fmt.Println("ballast update is only supported for Homebrew installations.")
		fmt.Println("To upgrade a non-Homebrew install, download the latest release from GitHub.")
		return 1
	}
	fmt.Println("Updating Homebrew...")
	if err := runCommandFunc("brew", []string{"update"}); err != nil {
		fmt.Printf("brew update failed: %v\n", err)
		return 1
	}
	fmt.Println("Upgrading ballast via Homebrew...")
	if err := runCommandFunc("brew", brewUpgradeArgs()); err != nil {
		fmt.Printf("brew upgrade failed: %v\n", err)
		return 1
	}
	fmt.Println("ballast upgraded. Run `ballast upgrade` to update .rulesrc.json and sync backend CLIs.")
	return 0
}

func runUpgrade(selectedLanguage language, args []string) int {
	patch, force, err := parseUpgradeOptions(args)
	if err != nil {
		fmt.Println(err)
		return 1
	}

	root := findProjectRoot("")
	config, err := loadDoctorConfig(root)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if config == nil {
		fmt.Println("upgrade requires an existing .rulesrc.json; run ballast install first")
		return 1
	}

	config.BallastVersion = normalizeVersion(resolveVersion())
	if err := saveMonorepoConfig(root, *config); err != nil {
		fmt.Println(err)
		return 1
	}
	printDoctorSummary(root, selectedLanguage, true)
	installVersion := config.BallastVersion
	if preferredSourceRoot(root) != "" {
		installVersion = ""
	}
	return runDoctorFixWithVersion(root, selectedLanguage, patch, force, installVersion)
}

func runDoctorFix(root string, selectedLanguage language, patch bool, force bool) int {
	return runDoctorFixWithVersion(
		root,
		selectedLanguage,
		patch,
		force,
		normalizeVersion(desiredDoctorInstallVersion(root)),
	)
}

func runDoctorFixWithVersion(root string, selectedLanguage language, patch bool, force bool, desiredVersion string) int {
	if exitCode := installCLIs(selectedLanguage, desiredVersion); exitCode != 0 {
		return exitCode
	}
	if !fileExists(filepath.Join(root, ".rulesrc.json")) {
		return 0
	}

	refreshArgs := []string{"install", "--refresh-config"}
	if patch {
		refreshArgs = append(refreshArgs, "--patch")
	}
	if force {
		refreshArgs = append(refreshArgs, "--force")
	}
	if selectedLanguage != "" {
		refreshArgs = append([]string{"--language", string(selectedLanguage)}, refreshArgs...)
	}
	exitCode := run(refreshArgs)
	if exitCode != 0 {
		return exitCode
	}
	if desiredVersion != "" {
		if err := rewriteDoctorConfigVersion(root, desiredVersion); err != nil {
			fmt.Println(err)
			return 1
		}
	}
	return 0
}

func desiredDoctorInstallVersion(root string) string {
	config, err := loadDoctorConfig(root)
	if err == nil && config != nil {
		if release := releaseVersion(config.BallastVersion); release != "" {
			return release
		}
	}
	return resolveVersion()
}

func rewriteDoctorConfigVersion(root string, version string) error {
	config, err := loadDoctorConfig(root)
	if err != nil {
		return err
	}
	if config == nil {
		return nil
	}
	config.BallastVersion = normalizeVersion(version)
	return saveMonorepoConfig(root, *config)
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
	if len(config.Targets) > 0 {
		fmt.Printf("- targets: %s\n", strings.Join(config.Targets, ", "))
	}
	if len(config.Agents) > 0 {
		fmt.Printf("- agents: %s\n", strings.Join(config.Agents, ", "))
	}
	if len(config.Skills) > 0 {
		fmt.Printf("- skills: %s\n", strings.Join(config.Skills, ", "))
	}
	if len(config.Languages) > 0 {
		fmt.Printf("- languages: %s\n", strings.Join(config.Languages, ", "))
	}
	if formattedPaths := formatDoctorConfigPaths(config.Languages, config.Paths); formattedPaths != "" {
		fmt.Printf("- paths: %s\n", formattedPaths)
	}
	if strings.TrimSpace(config.TaskSystem) != "" {
		fmt.Printf("- taskSystem: %s\n", config.TaskSystem)
	}
	fmt.Println()
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

func parseDoctorFix(args []string) (bool, bool, error) {
	fix := false
	patch := false
	for _, arg := range args {
		switch arg {
		case "--fix":
			fix = true
		case "--patch":
			patch = true
		default:
			return false, false, fmt.Errorf("unknown doctor option: %s", arg)
		}
	}
	if patch && !fix {
		return false, false, errors.New("--patch requires --fix")
	}
	return fix, patch, nil
}

func parseUpgradeOptions(args []string) (bool, bool, error) {
	patch := false
	force := false
	for _, arg := range args {
		switch arg {
		case "--patch":
			patch = true
		case "--force":
			force = true
		default:
			return false, false, fmt.Errorf("unknown upgrade option: %s", arg)
		}
	}
	return patch, force, nil
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
	if len(findFlagValues(filtered, "--target", "-t")) == 0 && !hasFlag(filtered, "--yes", "-y") {
		filtered = append(filtered, "--yes")
	}
	return filtered, nil
}

func parseRemoveLanguageValues(args []string) []string {
	values := findFlagValues(args, "--remove-language", "")
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		token := strings.ToLower(strings.TrimSpace(value))
		if token == "" {
			continue
		}
		normalized = append(normalized, token)
	}
	return uniqueStrings(normalized)
}

func validateSelectedLanguages(values []string) error {
	if len(values) == 0 {
		return nil
	}
	allowed := map[string]struct{}{}
	for _, lang := range supportedLanguages {
		allowed[string(lang)] = struct{}{}
	}
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			return fmt.Errorf(
				"invalid --remove-language: %s (valid: %s)",
				value,
				strings.Join(languageNames(), ", "),
			)
		}
	}
	return nil
}

func filterProfilesByLanguage(profiles []repoProfile, removed []string) []repoProfile {
	if len(removed) == 0 {
		return profiles
	}
	removedSet := map[string]struct{}{}
	for _, lang := range removed {
		removedSet[lang] = struct{}{}
	}
	filtered := make([]repoProfile, 0, len(profiles))
	for _, profile := range profiles {
		if _, remove := removedSet[string(profile.Language)]; remove {
			continue
		}
		filtered = append(filtered, profile)
	}
	return filtered
}

func ensureLocalToolDirs(root string) error {
	if err := ensureGitignoreEntry(root, ".ballast/"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore for .ballast/: %v\n", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".ballast", "bin"), 0o755); err != nil {
		return fmt.Errorf("create local ballast bin dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".ballast", "tools"), 0o755); err != nil {
		return fmt.Errorf("create local ballast tools dir: %w", err)
	}
	return nil
}

func ensureGitignoreEntry(root string, entry string) error {
	normalized := strings.TrimSpace(entry)
	if normalized == "" {
		return nil
	}
	gitignorePath := filepath.Join(root, ".gitignore")
	if !fileExists(gitignorePath) {
		return os.WriteFile(gitignorePath, []byte(normalized+"\n"), 0o644)
	}
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
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

func releasedGoInstallCommand(version string, projectRoot string) ([]string, error) {
	release := releaseVersion(version)
	if release == "" {
		release = releaseVersion(resolveVersion())
	}
	if release == "" {
		return nil, errors.New("ballast-go install requires a release version or a ballast source checkout")
	}

	url, err := releasedGoArchiveURL(release)
	if err != nil {
		return nil, err
	}
	checksumURL := releasedGoChecksumURL(release)
	destination := filepath.Join(projectRoot, ".ballast", "bin", "ballast-go")

	if runtime.GOOS == "windows" {
		script := `$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString()); ` +
			`New-Item -ItemType Directory -Path $tmp | Out-Null; ` +
			`try { ` +
			`$archive = Join-Path $tmp 'ballast-go.zip'; ` +
			`$checksums = Join-Path $tmp 'ballast-go_checksums.txt'; ` +
			`Invoke-WebRequest -Uri $args[0] -OutFile $archive; ` +
			`Invoke-WebRequest -Uri $args[1] -OutFile $checksums; ` +
			`$archiveName = [System.IO.Path]::GetFileName($args[0]); ` +
			`$expected = Select-String -Path $checksums -Pattern ("  " + [regex]::Escape($archiveName) + "$") | ForEach-Object { ($_ -split '\s+')[0] } | Select-Object -First 1; ` +
			`if (-not $expected) { throw "missing checksum for $archiveName" }; ` +
			`$actual = (Get-FileHash -Path $archive -Algorithm SHA256).Hash.ToLowerInvariant(); ` +
			`if ($actual -ne $expected.ToLowerInvariant()) { throw "checksum mismatch for $archiveName" }; ` +
			`Expand-Archive -Path $archive -DestinationPath $tmp -Force; ` +
			`Copy-Item (Join-Path $tmp 'ballast-go.exe') $args[2] -Force ` +
			`} finally { ` +
			`Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue ` +
			`}`
		return []string{"powershell", "-NoProfile", "-Command", script, url, checksumURL, destination + ".exe"}, nil
	}

	script := `set -e
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
archive="$tmpdir/ballast-go.tar.gz"
checksums="$tmpdir/ballast-go_checksums.txt"
destination="$3"
curl -fsSL "$1" -o "$archive"
curl -fsSL "$2" -o "$checksums"
archive_name="$(basename "$1")"
set -- $(grep "  $archive_name$" "$checksums")
[ "${1:-}" != "" ]
expected_checksum="$1"
set -- $(openssl dgst -sha256 -r "$archive")
[ "${1:-}" = "$expected_checksum" ]
tar -xzf "$archive" -C "$tmpdir"
install -m 0755 "$tmpdir/ballast-go" "$destination"`
	return []string{"sh", "-c", script, "sh", url, checksumURL, destination}, nil
}

func releasedGoArchiveURL(release string) (string, error) {
	goos, goarch, archiveExt, err := releasedGoArchiveParts(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	filename := fmt.Sprintf("ballast-go_%s_%s_%s.%s", release, goos, goarch, archiveExt)
	return fmt.Sprintf("https://github.com/everydaydevopsio/ballast/releases/download/v%s/%s", release, filename), nil
}

func releasedGoChecksumURL(release string) string {
	return fmt.Sprintf("https://github.com/everydaydevopsio/ballast/releases/download/v%s/ballast-go_checksums.txt", release)
}

func releasedGoArchiveParts(goos string, goarch string) (string, string, string, error) {
	switch goos {
	case "linux", "darwin":
		switch goarch {
		case "amd64", "arm64":
			return goos, goarch, "tar.gz", nil
		}
	case "windows":
		switch goarch {
		case "amd64":
			return goos, goarch, "zip", nil
		}
	}
	return "", "", "", fmt.Errorf("unsupported ballast-go release platform: %s/%s", goos, goarch)
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

func runCommandOutput(name string, args []string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

// detectBrewInstall reports whether the running ballast binary was installed
// via Homebrew by checking that (a) brew is on PATH and (b) the resolved
// executable path lives under the Homebrew prefix.
func detectBrewInstall() bool {
	if _, err := execLookPathFunc("brew"); err != nil {
		return false
	}
	prefix, err := runCommandOutputFunc("brew", []string{"--prefix"})
	if err != nil || prefix == "" {
		return false
	}
	execPath, err := osExecutableFunc()
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = resolved
	}
	prefix = filepath.Clean(prefix)
	execPath = filepath.Clean(execPath)
	return execPath == prefix || strings.HasPrefix(execPath, prefix+string(os.PathSeparator))
}

// brewUpgradeArgs returns the brew subcommand arguments needed to upgrade
// the ballast CLI: formula on Linux, cask on macOS.
func brewUpgradeArgs() []string {
	if runtime.GOOS == "darwin" {
		return []string{"upgrade", "--cask", "ballast"}
	}
	return []string{"upgrade", "--formula", "everydaydevopsio/ballast/ballast"}
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
	statuses := make([]doctorBackendStatus, 0, len(installableBackendLanguages))
	for _, lang := range installableBackendLanguages {
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
	if len(config.Targets) == 0 && strings.TrimSpace(config.Target) != "" {
		config.Targets = []string{strings.TrimSpace(config.Target)}
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
	dispatchedArgs := backendArgs(lang, args)
	repoRoot := findBallastSourceRoot()
	if repoRoot == "" {
		projectRoot := findProjectRoot("")
		local := projectLocalBackendCommand(projectRoot, lang)
		if local.Binary != "" {
			local.Args = append(local.Args, dispatchedArgs...)
			local.Env = mergeResolvedEnv(mergedEnv, local.Env)
			local.UseLocal = true
			return local
		}
		return resolvedBackendCommand{
			Binary: tool.binary,
			Args:   append([]string(nil), dispatchedArgs...),
			Env:    mergedEnv,
		}
	}

	local := resolveLocalBackendCommand(repoRoot, lang)
	if local.Binary == "" {
		return resolvedBackendCommand{
			Binary: tool.binary,
			Args:   append([]string(nil), dispatchedArgs...),
			Env:    mergedEnv,
		}
	}

	projectRoot := findProjectRoot("")
	projectLocal := projectLocalBackendCommand(projectRoot, lang)
	if projectLocal.Binary != "" {
		projectLocal.Args = append(projectLocal.Args, dispatchedArgs...)
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
		Args:     append(append([]string(nil), local.Args...), dispatchedArgs...),
		Env:      mergedEnv,
		UseLocal: true,
	}
}

func backendArgs(lang language, args []string) []string {
	dispatched := append([]string(nil), args...)
	if lang != langAnsible && lang != langTerraform {
		return dispatched
	}
	languageName := string(lang)
	for i := 0; i < len(dispatched); i++ {
		arg := dispatched[i]
		if arg == "--language" || arg == "-l" {
			return dispatched
		}
		if strings.HasPrefix(arg, "--language=") {
			return dispatched
		}
	}
	if len(dispatched) > 0 && !strings.HasPrefix(dispatched[0], "-") {
		return append([]string{dispatched[0], "--language", languageName}, dispatched[1:]...)
	}
	return append([]string{"--language", languageName}, dispatched...)
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
	case langAnsible:
		binaryPath := filepath.Join(repoRoot, "packages", "ballast-go", "ballast-go")
		if fileExists(binaryPath) {
			return resolvedBackendCommand{
				Binary: binaryPath,
			}
		}
	case langTerraform:
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
	case langAnsible:
		binary := filepath.Join(projectRoot, ".ballast", "bin", "ballast-go")
		if fileExists(binary) {
			return resolvedBackendCommand{Binary: binary}
		}
	case langTerraform:
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
	case langAnsible:
		name = "ballast-go"
	case langTerraform:
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
	if root := strings.TrimSpace(os.Getenv("BALLAST_REPO_ROOT")); root != "" {
		if abs, err := filepath.Abs(root); err == nil && isBallastSourceRoot(abs) {
			return abs
		}
	}
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

func prepareRepositoryFactsEnv(root string) (map[string]string, func(), error) {
	section := discoverRepositoryFactsSection(root)
	payload := repositoryFactsPayload{
		Version:                1,
		RepositoryFactsSection: section,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, func() {}, fmt.Errorf("marshal repository facts payload: %w", err)
	}
	file, err := os.CreateTemp("", "ballast-repository-facts-*.json")
	if err != nil {
		return nil, func() {}, fmt.Errorf("create repository facts file: %w", err)
	}
	path := file.Name()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, func() {}, fmt.Errorf("write repository facts file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, func() {}, fmt.Errorf("close repository facts file: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = os.Remove(path)
		return nil, func() {}, fmt.Errorf("set repository facts file permissions: %w", err)
	}
	return map[string]string{"BALLAST_REPOSITORY_FACTS_FILE": path}, func() {
		_ = os.Remove(path)
	}, nil
}

func discoverRepositoryFactsSection(root string) []string {
	canonicalRepo := detectCanonicalRepo(root)
	defaultBranch := detectDefaultBranch(root)
	packageManager := detectPackageManager(root)
	versionFiles := detectVersionFiles(root)
	configFiles := detectConfigFiles(root)
	ciWorkflows, releaseWorkflows := detectWorkflows(root)
	preferredCommands := detectPreferredCommands(root)
	coverageThreshold := detectCoverageThreshold(root)
	generatedPaths := detectGeneratedPaths(root)

	withPlaceholder := func(value string, placeholder string) string {
		if strings.TrimSpace(value) == "" {
			return "<" + placeholder + ">"
		}
		return value
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
		fmt.Sprintf("- Canonical GitHub repo: `%s`", withPlaceholder(canonicalRepo, "OWNER/REPO")),
		fmt.Sprintf("- Default branch: `%s`", withPlaceholder(defaultBranch, "main")),
		fmt.Sprintf("- Primary package manager: `%s`", withPlaceholder(packageManager, "pnpm | npm | yarn | uv | go")),
		fmt.Sprintf("- Version-file locations agents should check first: `%s`", withPlaceholder(versionFiles, ".nvmrc, packageManager, pyproject.toml, go.mod, etc.")),
		fmt.Sprintf("- Canonical config files: `%s`", withPlaceholder(configFiles, "paths agents should read before falling back to discovery")),
		fmt.Sprintf("- Primary CI workflows: `%s`", withPlaceholder(ciWorkflows, "workflow filenames")),
		fmt.Sprintf("- Primary release/publish workflows: `%s`", withPlaceholder(releaseWorkflows, "workflow filenames")),
		fmt.Sprintf("- Preferred build/test/lint/format/coverage commands: `%s`", withPlaceholder(preferredCommands, "commands")),
		fmt.Sprintf("- Coverage threshold: `%s`", withPlaceholder(coverageThreshold, "value")),
		fmt.Sprintf("- Generated or protected paths agents should avoid editing directly: `%s`", withPlaceholder(generatedPaths, "paths")),
		"",
		"Update this section when those facts change. If live runtime state is required, discover it separately instead of treating it as a durable repo fact.",
	}
}

func detectCanonicalRepo(root string) string {
	output, err := runCommandOutputFunc("git", []string{"-C", root, "remote", "get-url", "origin"})
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(output)
	url = strings.TrimSuffix(url, ".git")
	if strings.HasPrefix(url, "git@github.com:") {
		return strings.TrimPrefix(url, "git@github.com:")
	}
	if strings.HasPrefix(url, "https://github.com/") {
		return strings.TrimPrefix(url, "https://github.com/")
	}
	if strings.HasPrefix(url, "ssh://git@github.com/") {
		return strings.TrimPrefix(url, "ssh://git@github.com/")
	}
	return ""
}

func detectDefaultBranch(root string) string {
	output, err := runCommandOutputFunc("git", []string{"-C", root, "symbolic-ref", "refs/remotes/origin/HEAD"})
	if err != nil {
		return ""
	}
	trimmed := strings.TrimSpace(output)
	const prefix = "refs/remotes/origin/"
	if strings.HasPrefix(trimmed, prefix) {
		return strings.TrimPrefix(trimmed, prefix)
	}
	return trimmed
}

func detectPackageManager(root string) string {
	switch {
	case fileExists(filepath.Join(root, "pnpm-lock.yaml")):
		return "pnpm"
	case fileExists(filepath.Join(root, "yarn.lock")):
		return "yarn"
	case fileExists(filepath.Join(root, "package-lock.json")):
		return "npm"
	case fileExists(filepath.Join(root, "uv.lock")):
		return "uv"
	case fileExists(filepath.Join(root, "go.mod")):
		return "go"
	}
	metadata, ok := loadPackageJSONMetadata(root)
	if !ok {
		return ""
	}
	packageManager := strings.TrimSpace(metadata.PackageManager)
	if packageManager == "" {
		return ""
	}
	if index := strings.Index(packageManager, "@"); index > 0 {
		return strings.TrimSpace(packageManager[:index])
	}
	return packageManager
}

func detectVersionFiles(root string) string {
	candidates := []string{".nvmrc", "package.json", "pyproject.toml", "uv.lock", "go.mod", ".python-version"}
	found := []string{}
	for _, candidate := range candidates {
		if fileExists(filepath.Join(root, candidate)) {
			found = append(found, candidate)
		}
	}
	return strings.Join(found, ", ")
}

func detectConfigFiles(root string) string {
	candidates := []string{"eslint.config.mjs", ".eslintrc", ".prettierrc", "tsconfig.json", "pyproject.toml", "ruff.toml", "go.mod"}
	found := []string{}
	for _, candidate := range candidates {
		if fileExists(filepath.Join(root, candidate)) {
			found = append(found, candidate)
		}
	}
	return strings.Join(found, ", ")
}

func detectWorkflows(root string) (string, string) {
	workflowDir := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		return "", ""
	}
	ci := []string{}
	release := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.Contains(name, "ci") || strings.Contains(name, "test") || strings.Contains(name, "lint") || strings.Contains(name, "build") {
			ci = append(ci, entry.Name())
		}
		if strings.Contains(name, "release") || strings.Contains(name, "publish") {
			release = append(release, entry.Name())
		}
	}
	sort.Strings(ci)
	sort.Strings(release)
	return strings.Join(ci, ", "), strings.Join(release, ", ")
}

func detectPreferredCommands(root string) string {
	commands := []string{}
	targets := parseMakeTargets(root)
	for _, target := range []string{"test", "lint", "build"} {
		if containsString(targets, target) {
			commands = append(commands, "make "+target)
		}
	}
	if metadata, ok := loadPackageJSONMetadata(root); ok {
		for _, script := range []string{"test", "lint", "build", "format"} {
			if _, exists := metadata.Scripts[script]; exists {
				commands = append(commands, "package.json:"+script)
			}
		}
	}
	return strings.Join(uniqueStrings(commands), ", ")
}

func detectCoverageThreshold(root string) string {
	for _, candidate := range []string{
		filepath.Join(root, "vitest.config.ts"),
		filepath.Join(root, "vitest.config.js"),
		filepath.Join(root, "jest.config.ts"),
		filepath.Join(root, "jest.config.js"),
	} {
		content, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		if threshold := parseCoverageThreshold(string(content)); threshold != "" {
			return threshold
		}
	}
	return ""
}

func parseCoverageThreshold(content string) string {
	for _, pattern := range []string{
		`(?m)lines\s*:\s*([0-9]{1,3})`,
		`(?m)statements\s*:\s*([0-9]{1,3})`,
		`(?m)functions\s*:\s*([0-9]{1,3})`,
	} {
		match := regexp.MustCompile(pattern).FindStringSubmatch(content)
		if len(match) == 2 {
			return match[1] + "%"
		}
	}
	return ""
}

func parseMakeTargets(root string) []string {
	content, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		return nil
	}
	targets := []string{}
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ".") || strings.Contains(trimmed, "=") {
			continue
		}
		index := strings.Index(trimmed, ":")
		if index <= 0 {
			continue
		}
		name := strings.TrimSpace(trimmed[:index])
		if name == "" || strings.Contains(name, " ") {
			continue
		}
		targets = append(targets, name)
	}
	return uniqueStrings(targets)
}

func containsString(values []string, value string) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}

func detectGeneratedPaths(root string) string {
	candidates := []string{"dist/", "build/", "coverage/", "generated/", ".ballast/"}
	found := []string{}
	for _, candidate := range candidates {
		if fileExists(filepath.Join(root, strings.TrimSuffix(candidate, "/"))) {
			found = append(found, candidate)
		}
	}
	return strings.Join(found, ", ")
}

type packageJSONMetadata struct {
	Scripts              map[string]any `json:"scripts"`
	PackageManager       string         `json:"packageManager"`
	Dependencies         map[string]any `json:"dependencies"`
	DevDependencies      map[string]any `json:"devDependencies"`
	PeerDependencies     map[string]any `json:"peerDependencies"`
	OptionalDependencies map[string]any `json:"optionalDependencies"`
	Main                 string         `json:"main"`
	Module               string         `json:"module"`
	Browser              any            `json:"browser"`
	Bin                  any            `json:"bin"`
	Exports              any            `json:"exports"`
}

func loadPackageJSONMetadata(root string) (packageJSONMetadata, bool) {
	packageJSONPath := filepath.Join(root, "package.json")
	if !fileExists(packageJSONPath) {
		return packageJSONMetadata{}, false
	}
	content, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return packageJSONMetadata{}, false
	}
	var metadata packageJSONMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		return packageJSONMetadata{}, false
	}
	return metadata, true
}

func javascriptComponentWarning(root string) string {
	if fileExists(filepath.Join(root, "tsconfig.json")) {
		return ""
	}

	packageJSONPath := filepath.Join(root, "package.json")
	if !fileExists(packageJSONPath) {
		return ""
	}

	content, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return ""
	}

	var metadata packageJSONMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		return ""
	}

	if !looksLikeJavaScriptComponent(metadata) {
		return ""
	}

	return "detected a JavaScript package.json-based component or app without tsconfig.json; convert it to TypeScript or add tsconfig.json so Ballast can track it as a TypeScript profile."
}

func looksLikeJavaScriptComponent(metadata packageJSONMetadata) bool {
	if len(metadata.Scripts) > 0 {
		return true
	}
	if len(metadata.Dependencies) > 0 ||
		len(metadata.DevDependencies) > 0 ||
		len(metadata.PeerDependencies) > 0 ||
		len(metadata.OptionalDependencies) > 0 {
		return true
	}
	if strings.TrimSpace(metadata.Main) != "" || strings.TrimSpace(metadata.Module) != "" {
		return true
	}
	if metadata.Browser != nil || metadata.Bin != nil || metadata.Exports != nil {
		return true
	}
	return false
}

func profilesIncludeLanguage(profiles []repoProfile, target language) bool {
	for _, profile := range profiles {
		if profile.Language == target {
			return true
		}
	}
	return false
}

func resolveMonorepoPlan(root string, args []string) (*monorepoPlan, error) {
	if len(args) == 0 || args[0] != "install" {
		return nil, nil
	}

	requestedTaskSystem, err := parseTaskSystemFlag(args)
	if err != nil {
		return nil, err
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
	removeLanguages := parseRemoveLanguageValues(args)
	if err := validateSelectedLanguages(removeLanguages); err != nil {
		return nil, err
	}
	profiles = filterProfilesByLanguage(profiles, removeLanguages)

	if len(profiles) < 2 {
		allowLanguageRemovalPlan := len(removeLanguages) > 0 && config != nil && len(config.Languages) > 1
		if !allowLanguageRemovalPlan {
			return nil, nil
		}
	}
	if warning := javascriptComponentWarning(root); warning != "" && !profilesIncludeLanguage(profiles, langTypeScript) {
		fmt.Fprintln(os.Stderr, "warning:", warning)
	}

	installTargets := findFlagValues(args, "--target", "-t")
	removeTargets := findFlagValues(args, "--remove-target", "")
	if err := validateSelectedTargets(removeTargets); err != nil {
		return nil, err
	}
	installAgents, installAll, installSkills, installAllSkills := parseInstallSelection(args)
	explicitAgentSelection := len(installAgents) > 0 || installAll
	explicitSkillSelection := len(installSkills) > 0 || installAllSkills
	existingTargets := []string{}
	if config != nil {
		existingTargets = slices.Clone(config.Targets)
	}
	if err := validateSelectedTargets(installTargets); err != nil {
		return nil, err
	}
	requestedTargets := installTargets
	if len(requestedTargets) == 0 && config != nil {
		requestedTargets = slices.Clone(config.Targets)
	}
	requestedTargets = subtractStrings(requestedTargets, removeTargets)
	if err := validateSelectedTargets(requestedTargets); err != nil {
		return nil, err
	}
	savedTargets := subtractStrings(uniqueStrings(append(slices.Clone(existingTargets), installTargets...)), removeTargets)
	if err := validateSelectedTargets(savedTargets); err != nil {
		return nil, err
	}
	if !explicitAgentSelection && config != nil {
		installAgents = slices.Clone(config.Agents)
		if !explicitSkillSelection {
			installSkills = slices.Clone(config.Skills)
		}
	}
	cleanupOnly := len(removeTargets) > 0 && len(requestedTargets) == 0 && !explicitAgentSelection && !explicitSkillSelection
	languageCleanupOnly := len(removeLanguages) > 0 && !explicitAgentSelection && !explicitSkillSelection
	if !cleanupOnly && !languageCleanupOnly && (len(requestedTargets) == 0 || ((len(installAgents) == 0 && !installAll) && (len(installSkills) == 0 && !installAllSkills))) {
		return nil, errors.New("monorepo install requires --target and at least one of --agent/--all or --skill/--all-skills, or a root .rulesrc.json with target, agents/skills, languages, and paths")
	}

	selectedAgents := installAgents
	if installAll {
		selectedAgents = append(commonAgentIDs(), languageAgentIDs()...)
	}
	selectedSkills := installSkills
	if installAllSkills {
		selectedSkills = supportedSkillIDs()
	}
	if err := validateSelectedAgents(selectedAgents); err != nil {
		return nil, err
	}
	for _, agent := range selectedAgents {
		if w := deprecationWarningForAgent(agent); w != "" {
			fmt.Fprintln(os.Stderr, "warning:", w)
		}
	}
	for _, skill := range selectedSkills {
		if w := deprecationWarningForSkill(skill); w != "" {
			fmt.Fprintln(os.Stderr, "warning:", w)
		}
	}

	// Compute agents and skills to persist: merge explicit selection with existing
	// config so that installing only agents never drops saved skills and vice versa.
	persistAgents := selectedAgents
	if !installAll && config != nil {
		persistAgents = uniqueStrings(append(slices.Clone(config.Agents), selectedAgents...))
	}
	persistSkills := selectedSkills
	if !installAllSkills && config != nil {
		persistSkills = uniqueStrings(append(slices.Clone(config.Skills), selectedSkills...))
	}

	// Validate merged agents and skills before saving to prevent re-persisting
	// typos or unsupported entries from the existing config.
	if err := validateSelectedAgents(persistAgents); err != nil {
		return nil, err
	}
	if err := validateSelectedSkills(persistSkills); err != nil {
		return nil, err
	}

	var savedTaskSystem string
	if config != nil {
		savedTaskSystem = config.TaskSystem
	}
	if requestedTaskSystem != "" {
		savedTaskSystem = requestedTaskSystem
	}
	configToSave := monorepoConfig{
		Targets:        savedTargets,
		Agents:         persistAgents,
		Skills:         persistSkills,
		BallastVersion: normalizeVersion(resolveVersion()),
		Languages:      make([]string, 0, len(profiles)),
		Paths:          map[string][]string{},
		TaskSystem:     savedTaskSystem,
	}
	for _, profile := range profiles {
		configToSave.Languages = append(configToSave.Languages, string(profile.Language))
		configToSave.Paths[string(profile.Language)] = relativePaths(root, profile.Paths)
	}
	if cleanupOnly || languageCleanupOnly {
		return &monorepoPlan{
			Invocations: nil,
			Config:      configToSave,
			Targets:     requestedTargets,
			Common:      nil,
			Language:    nil,
			Removed:     removeTargets,
			Previous:    config,
		}, nil
	}
	if len(profiles) == 0 {
		return nil, errors.New("no languages remain after --remove-language; run with only --remove-language to clean up and persist config, or select a language with --language for single-language installs")
	}

	commonSelection := filterAgents(selectedAgents, commonAgentIDs())
	languageSelection := filterAgents(selectedAgents, languageAgentIDs())
	baseArgs := withTargetSelection(stripMonorepoFlags(args), requestedTargets)
	plan := make([]backendInvocation, 0, len(profiles)+1)
	commonArgs := withSkillSelection(withAgentSelection(baseArgs, commonSelection), selectedSkills)
	if requestedTaskSystem != "" && slices.Contains(configToSave.Agents, "tasks") {
		commonArgs = append(commonArgs, "--task-system", requestedTaskSystem)
	}
	if len(commonSelection) > 0 || len(selectedSkills) > 0 {
		commonLanguage := profiles[0].Language
		if hasLanguage(profiles, langTypeScript) {
			commonLanguage = langTypeScript
		}
		tool := toolsByLanguage[commonLanguage]
		env := monorepoInvocationEnv("common")
		if requestedTaskSystem != "" && slices.Contains(commonSelection, "tasks") {
			env["BALLAST_REFRESH_TASK_RULES"] = "1"
		}
		plan = append(plan, backendInvocation{
			Language: commonLanguage,
			Binary:   tool.binary,
			Dir:      root,
			Env:      env,
			Args:     commonArgs,
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
			Args:     withSkillSelection(withAgentSelection(baseArgs, languageSelection), nil),
		})
	}

	if len(plan) == 0 {
		return nil, fmt.Errorf(
			"no supported agents selected for monorepo install; supported agents: %s",
			strings.Join(supportedAgentIDs(), ", "),
		)
	}

	return &monorepoPlan{
		Invocations: plan,
		Config:      configToSave,
		Targets:     requestedTargets,
		Common:      filterAgents(configToSave.Agents, commonAgentIDs()),
		Language:    filterAgents(configToSave.Agents, languageAgentIDs()),
		Removed:     removeTargets,
		Previous:    config,
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
	if len(config.Targets) == 0 && strings.TrimSpace(config.Target) != "" {
		config.Targets = []string{strings.TrimSpace(config.Target)}
	}
	if len(config.Languages) == 0 || len(config.Paths) == 0 {
		return nil, nil
	}
	return &config, nil
}

func cleanupSingleLanguageManagedSelections(root string, selectedLanguage language) error {
	config, err := loadDoctorConfig(root)
	if err != nil {
		return err
	}
	if config == nil {
		return nil
	}
	if len(config.Targets) == 0 {
		return nil
	}
	if len(config.Languages) == 0 && selectedLanguage != "" {
		config.Languages = []string{string(selectedLanguage)}
	}
	for _, target := range config.Targets {
		if err := removeStaleManagedFiles(root, target, nil, config); err != nil {
			return err
		}
	}
	return nil
}

// readTaskSystem reads only the taskSystem field from .rulesrc.json.
// It does not validate Languages/Paths so it works on configs written by a
// single-language backend invocation during a fresh monorepo install.
func readTaskSystem(root string) string {
	data, err := os.ReadFile(filepath.Join(root, ".rulesrc.json"))
	if err != nil {
		return ""
	}
	var raw struct {
		TaskSystem string `json:"taskSystem"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	return raw.TaskSystem
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
		langAnsible:    {},
		langTerraform:  {},
	}

	if err := walkDirFunc(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			if name == ".git" || name == "node_modules" || name == ".venv" || name == "dist" || name == "build" || name == "vendor" || name == ".terraform" || name == ".terragrunt-cache" {
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
		case "ansible.cfg", "site.yml", "playbook.yml", "requirements.yml", "requirements.yaml":
			pathsByLanguage[langAnsible] = append(pathsByLanguage[langAnsible], dir)
		case ".terraform-version", "main.tf", "providers.tf", "versions.tf", "terraform.tf":
			pathsByLanguage[langTerraform] = append(pathsByLanguage[langTerraform], dir)
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

func parseInstallSelection(args []string) ([]string, bool, []string, bool) {
	agents := []string{}
	skills := []string{}
	allAgents := false
	allSkills := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--all" {
			allAgents = true
			continue
		}
		if arg == "--all-skills" {
			allSkills = true
			continue
		}
		if strings.HasPrefix(arg, "--agent=") {
			agents = append(agents, splitAgentValues(strings.TrimPrefix(arg, "--agent="))...)
			continue
		}
		if strings.HasPrefix(arg, "--skill=") {
			skills = append(skills, splitAgentValues(strings.TrimPrefix(arg, "--skill="))...)
			continue
		}
		if arg == "--agent" || arg == "-a" {
			if i+1 >= len(args) {
				continue
			}
			agents = append(agents, splitAgentValues(args[i+1])...)
			i++
			continue
		}
		if arg == "--skill" || arg == "-s" {
			if i+1 >= len(args) {
				continue
			}
			skills = append(skills, splitAgentValues(args[i+1])...)
			i++
		}
	}
	return uniqueStrings(agents), allAgents, uniqueStrings(skills), allSkills
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

func parseTaskSystemFlag(args []string) (string, error) {
	raw := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--task-system=") {
			raw = strings.TrimSpace(strings.TrimPrefix(arg, "--task-system="))
			continue
		}
		if arg != "--task-system" {
			continue
		}
		if i+1 >= len(args) {
			return "", errors.New("missing value for --task-system")
		}
		candidate := strings.TrimSpace(args[i+1])
		if candidate == "" || strings.HasPrefix(candidate, "-") {
			return "", errors.New("missing value for --task-system")
		}
		raw = candidate
		i++
	}
	if raw == "" {
		return "", nil
	}
	normalized := strings.ToLower(raw)
	if slices.Contains(supportedTaskSystems, normalized) {
		return normalized, nil
	}
	return "", fmt.Errorf(
		"invalid --task-system: %s (valid: %s)",
		raw,
		strings.Join(supportedTaskSystems, ", "),
	)
}

func findFlagValue(args []string, longFlag string, shortFlag string) string {
	values := findFlagValues(args, longFlag, shortFlag)
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func findFlagValues(args []string, longFlag string, shortFlag string) []string {
	values := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, longFlag+"=") {
			values = append(values, splitAgentValues(strings.TrimPrefix(arg, longFlag+"="))...)
			continue
		}
		if arg == longFlag || arg == shortFlag {
			if i+1 >= len(args) {
				break
			}
			values = append(values, splitAgentValues(args[i+1])...)
		}
	}
	return uniqueStrings(values)
}

func stripMonorepoFlags(args []string) []string {
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--language" || arg == "-l" {
			i++
			continue
		}
		if arg == "--remove-target" {
			i++
			continue
		}
		if arg == "--remove-language" {
			i++
			continue
		}
		if arg == "--task-system" {
			i++
			continue
		}
		if strings.HasPrefix(arg, "--language=") {
			continue
		}
		if strings.HasPrefix(arg, "--remove-target=") {
			continue
		}
		if strings.HasPrefix(arg, "--remove-language=") {
			continue
		}
		if strings.HasPrefix(arg, "--task-system=") {
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

func withSkillSelection(baseArgs []string, skills []string) []string {
	filtered := make([]string, 0, len(baseArgs))
	for i := 0; i < len(baseArgs); i++ {
		arg := baseArgs[i]
		if arg == "--all-skills" {
			continue
		}
		if arg == "--skill" || arg == "-s" {
			i++
			continue
		}
		if strings.HasPrefix(arg, "--skill=") {
			continue
		}
		filtered = append(filtered, arg)
	}
	if len(skills) > 0 {
		filtered = append(filtered, "--skill", strings.Join(skills, ","))
	}
	return filtered
}

func withTargetSelection(baseArgs []string, targets []string) []string {
	filtered := make([]string, 0, len(baseArgs))
	for i := 0; i < len(baseArgs); i++ {
		arg := baseArgs[i]
		if arg == "--target" || arg == "-t" {
			i++
			continue
		}
		if strings.HasPrefix(arg, "--target=") {
			continue
		}
		filtered = append(filtered, arg)
	}
	if len(targets) > 0 {
		filtered = append(filtered, "--target", strings.Join(targets, ","))
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

func validateSelectedTargets(targets []string) error {
	invalid := []string{}
	for _, target := range uniqueStrings(targets) {
		if !slices.Contains(supportedTargets, target) {
			invalid = append(invalid, target)
		}
	}
	if len(invalid) > 0 {
		return fmt.Errorf("unsupported target selection: %s (supported targets: %s)", strings.Join(invalid, ", "), strings.Join(supportedTargets, ", "))
	}
	return nil
}

func subtractStrings(values []string, remove []string) []string {
	if len(remove) == 0 {
		return uniqueStrings(values)
	}
	removeSet := map[string]struct{}{}
	for _, value := range remove {
		removeSet[value] = struct{}{}
	}
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := removeSet[value]; ok {
			continue
		}
		filtered = append(filtered, value)
	}
	return uniqueStrings(filtered)
}

func stringDifference(values []string, remove []string) []string {
	if len(values) == 0 {
		return nil
	}
	removeSet := map[string]struct{}{}
	for _, value := range remove {
		removeSet[value] = struct{}{}
	}
	filtered := make([]string, 0, len(values))
	for _, value := range uniqueStrings(values) {
		if _, ok := removeSet[value]; ok {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
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
	allowReplaceUnmanaged := hasPatchFlag(args)
	interactive := isInteractiveInstall(args)
	for _, target := range plan.Targets {
		if target != "claude" && target != "codex" && target != "gemini" {
			continue
		}
		path := supportFilePath(root, target)
		content := buildMonorepoSupportFile(root, plan, target)
		if !fileExists(path) {
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return err
			}
			continue
		}
		existing := readFile(path)
		allowUnmanaged := allowReplaceUnmanaged
		if !allowUnmanaged && interactive && supportFileHasUnmanagedManagedSections(existing) {
			confirmed, err := promptSupportFilePatch(path)
			if err != nil {
				return err
			}
			allowUnmanaged = confirmed
		}
		merged := mergeManagedSupportSections(existing, content, allowUnmanaged)
		if err := os.WriteFile(path, []byte(merged), 0o644); err != nil {
			return err
		}
		if target == "gemini" {
			agentsPath := supportFilePath(root, "codex")
			if !fileExists(agentsPath) {
				if err := os.WriteFile(agentsPath, []byte(buildMonorepoSupportFile(root, plan, "codex")), 0o644); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func cleanupRemovedMonorepoTargets(root string, plan *monorepoPlan) error {
	if plan == nil || len(plan.Removed) == 0 || plan.Previous == nil {
		return nil
	}
	for _, target := range plan.Removed {
		if err := removeManagedTargetFiles(root, target, plan.Previous); err != nil {
			return err
		}
		if target == "claude" || target == "codex" || target == "gemini" {
			path := supportFilePath(root, target)
			if fileExists(path) {
				if err := os.WriteFile(path, []byte(removeManagedSections(readFile(path))), 0o644); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func cleanupStaleManagedSelections(root string, plan *monorepoPlan) error {
	if plan == nil {
		return nil
	}
	for _, target := range plan.Targets {
		if err := removeStaleManagedFiles(root, target, plan.Previous, &plan.Config); err != nil {
			return err
		}
	}
	return nil
}

func removeStaleManagedFiles(root string, target string, previous *monorepoConfig, next *monorepoConfig) error {
	if next == nil {
		return nil
	}
	trackedPaths := ballastManagedPathsFromSupportFile(root, target)
	for _, file := range stringDifference(allManagedRulePaths(root, target), managedRulePaths(root, target, next)) {
		if !ballastOwnsManagedFile(file) && !trackedPaths[file] {
			continue
		}
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		pruneEmptyParents(filepath.Dir(file), targetRootDir(root, target))
	}
	for _, file := range stringDifference(allManagedSkillPaths(root, target), managedSkillPaths(root, target, next.Skills)) {
		if !ballastOwnsManagedFile(file) && !trackedPaths[file] && !allowConfigBackedStaleSkillRemoval(root, target, file, previous) {
			continue
		}
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		pruneEmptyParents(filepath.Dir(file), targetRootDir(root, target))
	}
	return nil
}

func allowConfigBackedStaleSkillRemoval(root string, target string, path string, previous *monorepoConfig) bool {
	if target != "cursor" && target != "opencode" {
		return false
	}
	if previous == nil {
		return false
	}
	for _, previousPath := range managedSkillPaths(root, target, previous.Skills) {
		if previousPath == path {
			return true
		}
	}
	return false
}

func allManagedRulePaths(root string, target string) []string {
	languages := make([]string, 0, len(supportedLanguages))
	for _, lang := range supportedLanguages {
		languages = append(languages, string(lang))
	}
	return managedRulePaths(root, target, &monorepoConfig{
		Agents:    supportedAgentIDs(),
		Languages: languages,
	})
}

func allManagedSkillPaths(root string, target string) []string {
	return managedSkillPaths(root, target, supportedSkillIDs())
}

func removeManagedTargetFiles(root string, target string, config *monorepoConfig) error {
	if config == nil {
		return nil
	}
	for _, file := range managedRulePaths(root, target, config) {
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		pruneEmptyParents(filepath.Dir(file), targetRootDir(root, target))
	}
	for _, file := range managedSkillPaths(root, target, config.Skills) {
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		pruneEmptyParents(filepath.Dir(file), targetRootDir(root, target))
	}
	return nil
}

func managedRulePaths(root string, target string, config *monorepoConfig) []string {
	paths := []string{}
	ext := targetRuleExtension(target)
	rulesRoot := targetRulesRoot(root, target)
	commonSelection := filterAgents(config.Agents, commonAgentIDs())
	languageSelection := filterAgents(config.Agents, languageAgentIDs())
	for _, agent := range commonSelection {
		for _, suffix := range ruleSuffixesForAgent(agent) {
			base := agentBaseName(agent, suffix)
			paths = append(paths, filepath.Join(rulesRoot, "common", base+ext))
		}
	}
	for _, lang := range config.Languages {
		for _, agent := range languageSelection {
			for _, suffix := range ruleSuffixesForAgent(agent) {
				base := agentBaseName(agent, suffix)
				paths = append(paths, filepath.Join(rulesRoot, lang, lang+"-"+base+ext))
			}
		}
	}
	return uniqueStrings(paths)
}

func managedSkillPaths(root string, target string, skills []string) []string {
	paths := []string{}
	for _, skill := range skills {
		path := targetSkillPath(root, target, skill)
		if strings.TrimSpace(path) != "" {
			paths = append(paths, path)
		}
	}
	return uniqueStrings(paths)
}

func targetRootDir(root string, target string) string {
	switch target {
	case "cursor":
		return filepath.Join(root, ".cursor")
	case "claude":
		return filepath.Join(root, ".claude")
	case "gemini":
		return filepath.Join(root, ".gemini")
	case "opencode":
		return filepath.Join(root, ".opencode")
	case "codex":
		return filepath.Join(root, ".codex")
	default:
		return root
	}
}

func targetRulesRoot(root string, target string) string {
	switch target {
	case "cursor":
		return filepath.Join(root, ".cursor", "rules")
	case "claude":
		return filepath.Join(root, ".claude", "rules")
	case "gemini":
		return filepath.Join(root, ".gemini", "rules")
	case "opencode":
		return filepath.Join(root, ".opencode", "rules")
	case "codex":
		return filepath.Join(root, ".codex", "rules")
	default:
		return root
	}
}

func targetRuleExtension(target string) string {
	if target == "cursor" {
		return ".mdc"
	}
	return ".md"
}

func targetSkillPath(root string, target string, skill string) string {
	switch target {
	case "cursor":
		return filepath.Join(root, ".cursor", "rules", skill+".mdc")
	case "claude":
		return filepath.Join(root, ".claude", "skills", skill+".skill")
	case "gemini":
		return filepath.Join(root, ".gemini", "rules", skill+".md")
	case "opencode":
		return filepath.Join(root, ".opencode", "skills", skill+".md")
	case "codex":
		return filepath.Join(root, ".codex", "rules", skill+".md")
	default:
		return ""
	}
}

func pruneEmptyParents(dir string, stop string) {
	current := filepath.Clean(dir)
	limit := filepath.Clean(stop)
	for strings.HasPrefix(current, limit) {
		entries, err := os.ReadDir(current)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(current); err != nil {
			return
		}
		if current == limit {
			return
		}
		parent := filepath.Dir(current)
		if parent == current {
			return
		}
		current = parent
	}
}

func supportFilePath(root string, target string) string {
	if target == "claude" {
		return filepath.Join(root, "CLAUDE.md")
	}
	if target == "gemini" {
		return filepath.Join(root, "GEMINI.md")
	}
	return filepath.Join(root, "AGENTS.md")
}

func buildMonorepoSupportFile(root string, plan *monorepoPlan, target string) string {
	title := "# AGENTS.md"
	intro := "This file provides shared repository guidance for agent tools that read AGENTS.md."
	rulesDir := ".codex/rules"
	extension := ".md"
	if target == "claude" {
		title = "# CLAUDE.md"
		intro = "This file provides guidance to Claude Code for working in this repository."
		rulesDir = ".claude/rules"
	}
	if target == "gemini" {
		title = "# GEMINI.md"
		intro = "This file provides guidance to Gemini CLI for working in this repository."
		rulesDir = ".gemini/rules"
	}

	lines := []string{
		title,
		"",
		intro,
		"",
	}

	if target == "gemini" {
		lines = append(lines, "@./AGENTS.md", "")
	} else {
		lines = append(lines, discoverRepositoryFactsSection(root)...)
		lines = append(lines, "")
	}

	lines = append(lines,
		"## Installed agent rules",
		"",
		"Created by Ballast. Do not edit this section.",
		"",
		fmt.Sprintf("Read and follow these rule files in `%s/` when they apply:", rulesDir),
		"",
	)

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
	if len(plan.Config.Skills) > 0 {
		lines = append(lines,
			"",
			"## Installed skills",
			"",
			"Created by Ballast. Do not edit this section.",
			"",
			fmt.Sprintf("Read and use these skill files in `%s/` when they are relevant:", strings.TrimPrefix(filepath.Dir(targetSkillPath(rootPlaceholder, target, "example")), rootPlaceholder+"/")),
			"",
		)
		for _, skill := range plan.Config.Skills {
			skillPath := fmt.Sprintf("`%s`", strings.TrimPrefix(targetSkillPath(rootPlaceholder, target, skill), rootPlaceholder+"/"))
			lines = append(lines, fmt.Sprintf("- %s — %s", skillPath, skillDescription(skill)))
		}
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func ruleSuffixesForAgent(agent string) []string {
	if agent == "local-dev" {
		return []string{"badges", "env", "license", "mcp"}
	}
	if agent == "publishing" {
		return []string{"libraries", "sdks", "apps"}
	}
	return []string{""}
}

func agentBaseName(agent string, suffix string) string {
	if suffix == "" {
		return agent
	}
	return agent + "-" + suffix
}

func skillDescription(skill string) string {
	return skillDescriptionFromRegistry(skill)
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

const rootPlaceholder = "__BALLAST_ROOT__"
const ballastManagedMarker = "Created by [Ballast]"
const ballastManagedSectionNotice = "Created by Ballast. Do not edit this section."

func patchManagedSupportSections(existing string, canonical string) string {
	next := existing
	for _, heading := range []string{"Installed agent rules", "Installed skills"} {
		next = patchSupportSection(next, canonical, heading)
	}
	return next
}

func patchSupportSection(existing string, canonical string, heading string) string {
	canonicalRange := findSectionRange(canonical, heading)
	if canonicalRange == nil {
		return existing
	}
	canonicalSection := strings.TrimRight(canonical[canonicalRange[0]:canonicalRange[1]], "\n")

	existingRange := findSectionRange(existing, heading)
	if existingRange == nil {
		return strings.TrimRight(existing, "\n") + "\n\n" + canonicalSection + "\n"
	}

	return strings.TrimRight(existing[:existingRange[0]], "\n") + "\n\n" +
		canonicalSection + "\n\n" +
		strings.TrimLeft(existing[existingRange[1]:], "\n")
}

func mergeManagedSupportSections(existing string, canonical string, allowUnmanaged bool) string {
	next := existing
	for _, heading := range []string{"Installed agent rules", "Installed skills"} {
		next = mergeSupportSection(next, canonical, heading, allowUnmanaged)
	}
	return next
}

func mergeSupportSection(existing string, canonical string, heading string, allowUnmanaged bool) string {
	canonicalRange := findSectionRange(canonical, heading)
	if canonicalRange == nil {
		return existing
	}
	canonicalSection := strings.TrimRight(canonical[canonicalRange[0]:canonicalRange[1]], "\n")

	existingRange := findSectionRange(existing, heading)
	if existingRange == nil {
		return strings.TrimRight(existing, "\n") + "\n\n" + canonicalSection + "\n"
	}
	existingSection := existing[existingRange[0]:existingRange[1]]
	if !allowUnmanaged && !strings.Contains(existingSection, ballastManagedSectionNotice) {
		return existing
	}

	return strings.TrimRight(existing[:existingRange[0]], "\n") + "\n\n" +
		canonicalSection + "\n\n" +
		strings.TrimLeft(existing[existingRange[1]:], "\n")
}

func supportFileHasUnmanagedManagedSections(existing string) bool {
	for _, heading := range []string{"Installed agent rules", "Installed skills"} {
		section := findSectionRange(existing, heading)
		if section == nil {
			continue
		}
		if !strings.Contains(existing[section[0]:section[1]], ballastManagedSectionNotice) {
			return true
		}
	}
	return false
}

func removeManagedSections(existing string) string {
	next := existing
	for _, heading := range []string{"Installed agent rules", "Installed skills"} {
		section := findSectionRange(next, heading)
		if section == nil {
			continue
		}
		next = strings.TrimRight(next[:section[0]], "\n") + "\n\n" + strings.TrimLeft(next[section[1]:], "\n")
	}
	return strings.TrimRight(next, "\n") + "\n"
}

func ballastOwnsManagedFile(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if strings.HasSuffix(path, ".skill") {
		reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
		if err != nil {
			return false
		}
		for _, file := range reader.File {
			if file.Name != "SKILL.md" {
				continue
			}
			rc, err := file.Open()
			if err != nil {
				return false
			}
			data, readErr := io.ReadAll(rc)
			_ = rc.Close()
			if readErr != nil {
				return false
			}
			return strings.Contains(string(data), ballastManagedMarker)
		}
		return false
	}
	return strings.Contains(string(content), ballastManagedMarker)
}

func ballastManagedPathsFromSupportFile(root string, target string) map[string]bool {
	if target != "claude" && target != "codex" && target != "gemini" {
		return nil
	}
	path := supportFilePath(root, target)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	tracked := map[string]bool{}
	for _, heading := range []string{"Installed agent rules", "Installed skills"} {
		section := findSectionRange(string(content), heading)
		if section == nil {
			continue
		}
		sectionText := string(content[section[0]:section[1]])
		if !strings.Contains(sectionText, ballastManagedSectionNotice) {
			continue
		}
		for _, line := range strings.Split(sectionText, "\n") {
			start := strings.Index(line, "`")
			if start < 0 {
				continue
			}
			rest := line[start+1:]
			end := strings.Index(rest, "`")
			if end < 0 {
				continue
			}
			ref := strings.TrimSpace(rest[:end])
			if ref == "" || filepath.IsAbs(ref) {
				continue
			}
			tracked[filepath.Clean(filepath.Join(root, ref))] = true
		}
	}
	return tracked
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
		if !isValidAgent(agent) {
			invalid = append(invalid, agent)
		}
	}
	if len(invalid) > 0 {
		return fmt.Errorf(
			"unsupported agent selection: %s (supported agents: %s)",
			strings.Join(invalid, ", "),
			strings.Join(supportedAgentIDs(), ", "),
		)
	}
	return nil
}

func validateSelectedSkills(skills []string) error {
	invalid := []string{}
	for _, skill := range uniqueStrings(skills) {
		if !isValidSkill(skill) {
			invalid = append(invalid, skill)
		}
	}
	if len(invalid) > 0 {
		return fmt.Errorf(
			"unsupported skill selection: %s (supported skills: %s)",
			strings.Join(invalid, ", "),
			strings.Join(supportedSkillIDs(), ", "),
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
	markers := []string{
		".git",
		".rulesrc.json",
		".rulesrc.ts.json",
		".rulesrc.python.json",
		".rulesrc.go.json",
		".rulesrc.ansible.json",
		".rulesrc.terraform.json",
		"go.mod",
		"pyproject.toml",
		"package.json",
		"pnpm-lock.yaml",
		"uv.lock",
		"ansible.cfg",
		"site.yml",
		"playbook.yml",
		"requirements.yml",
		"requirements.yaml",
		".terraform-version",
		"main.tf",
		"providers.tf",
		"versions.tf",
		"terraform.tf",
	}
	for _, marker := range markers {
		if fileExists(filepath.Join(dir, marker)) {
			return true
		}
	}
	return false
}

func detectLanguage(root string) language {
	if warning := javascriptComponentWarning(root); warning != "" {
		fmt.Fprintln(os.Stderr, "warning:", warning)
	}

	scores := map[language]int{
		langTypeScript: 0,
		langPython:     0,
		langGo:         0,
		langAnsible:    0,
		langTerraform:  0,
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
	if fileExists(filepath.Join(root, "ansible.cfg")) {
		scores[langAnsible] += 10
	}
	if fileExists(filepath.Join(root, "site.yml")) || fileExists(filepath.Join(root, "playbook.yml")) {
		scores[langAnsible] += 8
	}
	if fileExists(filepath.Join(root, "requirements.yml")) {
		scores[langAnsible] += 6
	}
	if fileExists(filepath.Join(root, "requirements.yaml")) {
		scores[langAnsible] += 6
	}
	if fileExists(filepath.Join(root, ".terraform-version")) {
		scores[langTerraform] += 10
	}
	if fileExists(filepath.Join(root, "versions.tf")) || fileExists(filepath.Join(root, "providers.tf")) {
		scores[langTerraform] += 8
	}
	if fileExists(filepath.Join(root, "main.tf")) || fileExists(filepath.Join(root, "terraform.tf")) {
		scores[langTerraform] += 6
	}
}

func applyConfigScores(root string, scores map[language]int) {
	if fileExists(filepath.Join(root, ".rulesrc.go.json")) {
		scores[langGo] += 20
	}
	if fileExists(filepath.Join(root, ".rulesrc.ansible.json")) {
		scores[langAnsible] += 20
	}
	if fileExists(filepath.Join(root, ".rulesrc.terraform.json")) {
		scores[langTerraform] += 20
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

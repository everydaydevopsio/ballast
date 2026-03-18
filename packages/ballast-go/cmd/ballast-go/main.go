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
var ballastVersion = "dev"
var frontmatterRegex = regexp.MustCompile(`(?s)^\s*---\n(.*?)\n---\n?`)
var topLevelYAMLKeyRegex = regexp.MustCompile(`^([A-Za-z0-9_-]+):(.*)$`)

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
	patch := fs.Bool("patch", false, "merge upstream updates into existing files")
	fs.BoolVar(patch, "p", false, "merge upstream updates into existing files")
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
	patchClaude := false
	if resolved.Target == "claude" && !*yes && !isCIMode() && !*force && !*patch && exists(claudeMDPath(root)) {
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
	}
	result := install(installOptions{
		projectRoot: root,
		target:      resolved.Target,
		agents:      resolved.Agents,
		language:    lang,
		force:       *force,
		patch:       *patch,
		patchClaude: patchClaude,
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
			content, err := buildContent(agentID, opts.target, opts.language, suffix)
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
				nextContent = patchRuleContent(string(existing), content, opts.target)
			}
			if err := os.WriteFile(file, []byte(nextContent), 0o644); err != nil {
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
		if exists(agentsPath) && !opts.force && !opts.patch {
			result.skippedSupportFiles = append(result.skippedSupportFiles, agentsPath)
		} else {
			ids := sortedKeys(processed)
			content, err := buildCodexAgentsMD(ids, opts.language)
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
				} else {
					result.installedSupport = append(result.installedSupport, agentsPath)
				}
			}
		}
	}

	if opts.target == "claude" {
		claudePath := claudeMDPath(opts.projectRoot)
		if exists(claudePath) && !opts.force && !opts.patchClaude {
			result.skippedSupportFiles = append(result.skippedSupportFiles, claudePath)
		} else {
			ids := sortedKeys(processed)
			content, err := buildClaudeMD(ids, opts.language)
			if err != nil {
				result.errors = append(result.errors, agentError{agent: "claude", err: err.Error()})
			} else {
				nextContent := content
				if exists(claudePath) && !opts.force && opts.patchClaude {
					existing, readErr := os.ReadFile(claudePath)
					if readErr != nil {
						result.errors = append(result.errors, agentError{agent: "claude", err: readErr.Error()})
					} else {
						nextContent = patchCodexAgentsMD(string(existing), content)
					}
				}
				if err := os.WriteFile(claudePath, []byte(nextContent), 0o644); err != nil {
					result.errors = append(result.errors, agentError{agent: "claude", err: err.Error()})
				} else {
					result.installedSupport = append(result.installedSupport, claudePath)
				}
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
		"Created by Ballast v" + ballastVersion + ". Do not edit this section.",
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

func buildClaudeMD(agents []string, language string) (string, error) {
	lines := []string{
		"# CLAUDE.md",
		"",
		"This file provides guidance to Claude Code for working in this repository.",
		"",
		"## Installed agent rules",
		"",
		"Created by Ballast v" + ballastVersion + ". Do not edit this section.",
		"",
		"Read and follow these rule files in `.claude/rules/` when they apply:",
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
			lines = append(lines, fmt.Sprintf("- `.claude/rules/%s.md` — %s", base, description))
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
	canonicalStart, canonicalEnd, ok := findSectionRange(canonical, "Installed agent rules")
	if !ok {
		return existing
	}
	canonicalSection := strings.TrimRight(canonical[canonicalStart:canonicalEnd], "\n")
	existingStart, existingEnd, ok := findSectionRange(existing, "Installed agent rules")
	if !ok {
		return strings.TrimRight(existing, "\n") + "\n\n" + canonicalSection + "\n"
	}
	return strings.TrimRight(existing[:existingStart], "\n") +
		"\n\n" +
		canonicalSection +
		"\n\n" +
		strings.TrimLeft(existing[existingEnd:], "\n") + "\n"
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
		if exists(filepath.Join(dir, "package.json")) ||
			exists(filepath.Join(dir, "go.mod")) ||
			exists(filepath.Join(dir, "pyproject.toml")) ||
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

package main

// registry.go — single source of truth for all supported agents and skills.
//
// When adding, removing, or deprecating an agent or skill:
//   1. Add/update its entry here.
//   2. For skills, update skillDescription in main.go accordingly.
//   3. Keep packages/ballast-typescript/src/agents.ts in sync:
//      - COMMON_AGENT_IDS / LANGUAGE_AGENT_IDS must match agentRegistry entries.
//      - COMMON_SKILL_IDS must match skillRegistry entries (non-removed).

// entryStatus tracks the lifecycle state of a registry entry.
type entryStatus string

const (
	statusActive     entryStatus = "active"
	statusDeprecated entryStatus = "deprecated" // warn but still allow
	statusRemoved    entryStatus = "removed"    // reject at validation
)

// agentKind controls how an agent is dispatched in monorepo installs.
// common agents are installed once at the repo root; language agents are
// installed once per language sub-project.
type agentKind string

const (
	kindCommon   agentKind = "common"
	kindLanguage agentKind = "language"
)

type deprecationNote struct {
	Since   string // semver version when deprecated
	Remove  string // semver version when it will be removed (empty = TBD)
	Message string // human-readable migration hint
}

type agentEntry struct {
	ID         string
	Kind       agentKind
	Status     entryStatus
	Deprecated *deprecationNote // non-nil when Status == statusDeprecated
}

type skillEntry struct {
	ID          string
	Description string
	Status      entryStatus
	Deprecated  *deprecationNote // non-nil when Status == statusDeprecated
}

// agentRegistry is the authoritative list of all agents.
// Keep in sync with COMMON_AGENT_IDS / LANGUAGE_AGENT_IDS in
// packages/ballast-typescript/src/agents.ts.
var agentRegistry = []agentEntry{
	// Common agents — installed once at repo root, language-agnostic.
	{ID: "local-dev", Kind: kindCommon, Status: statusActive},
	{ID: "docs", Kind: kindCommon, Status: statusActive},
	{ID: "cicd", Kind: kindCommon, Status: statusActive},
	{ID: "observability", Kind: kindCommon, Status: statusActive},
	{ID: "publishing", Kind: kindCommon, Status: statusActive},
	{ID: "git-hooks", Kind: kindCommon, Status: statusActive},

	// Language agents — installed once per language sub-project.
	{ID: "linting", Kind: kindLanguage, Status: statusActive},
	{ID: "logging", Kind: kindLanguage, Status: statusActive},
	{ID: "testing", Kind: kindLanguage, Status: statusActive},
}

// skillRegistry is the authoritative list of all skills.
// Keep in sync with COMMON_SKILL_IDS in
// packages/ballast-typescript/src/agents.ts.
var skillRegistry = []skillEntry{
	{
		ID:          "owasp-security-scan",
		Description: "run an OWASP-aligned security audit across Go, TypeScript, and Python projects",
		Status:      statusActive,
	},
	{
		ID:          "aws-health-review",
		Description: "run a weekly read-only AWS health review covering configuration, performance, errors, and warnings",
		Status:      statusActive,
	},
	{
		ID:          "aws-live-health-review",
		Description: "run a read-only AWS live health review for current EC2, RDS, ALB, CloudWatch alarms, and logs",
		Status:      statusActive,
	},
	{
		ID:          "aws-weekly-security-review",
		Description: "run a weekly read-only AWS security baseline review and generate a prioritized findings report",
		Status:      statusActive,
	},
	{
		ID:          "github-health-check",
		Description: "run a comprehensive GitHub repository health check covering CI status, branch hygiene, and repo configuration",
		Status:      statusActive,
	},
}

// — Agent registry helpers —————————————————————————————————————————————————

// activeAgentIDs returns all non-removed agent IDs.
func activeAgentIDs() []string {
	var ids []string
	for _, e := range agentRegistry {
		if e.Status != statusRemoved {
			ids = append(ids, e.ID)
		}
	}
	return ids
}

// commonAgentIDs returns non-removed agent IDs with kindCommon.
func commonAgentIDs() []string {
	var ids []string
	for _, e := range agentRegistry {
		if e.Kind == kindCommon && e.Status != statusRemoved {
			ids = append(ids, e.ID)
		}
	}
	return ids
}

// languageAgentIDs returns non-removed agent IDs with kindLanguage.
func languageAgentIDs() []string {
	var ids []string
	for _, e := range agentRegistry {
		if e.Kind == kindLanguage && e.Status != statusRemoved {
			ids = append(ids, e.ID)
		}
	}
	return ids
}

// isValidAgent reports whether id is a known, non-removed agent.
func isValidAgent(id string) bool {
	for _, e := range agentRegistry {
		if e.ID == id && e.Status != statusRemoved {
			return true
		}
	}
	return false
}

// deprecationWarningForAgent returns a non-empty warning string when the agent
// is deprecated, empty string otherwise.
func deprecationWarningForAgent(id string) string {
	for _, e := range agentRegistry {
		if e.ID == id && e.Status == statusDeprecated && e.Deprecated != nil {
			msg := "agent " + id + " is deprecated since " + e.Deprecated.Since
			if e.Deprecated.Remove != "" {
				msg += " and will be removed in " + e.Deprecated.Remove
			}
			if e.Deprecated.Message != "" {
				msg += ": " + e.Deprecated.Message
			}
			return msg
		}
	}
	return ""
}

// — Skill registry helpers —————————————————————————————————————————————————

// activeSkillIDs returns all non-removed skill IDs.
func activeSkillIDs() []string {
	var ids []string
	for _, e := range skillRegistry {
		if e.Status != statusRemoved {
			ids = append(ids, e.ID)
		}
	}
	return ids
}

// isValidSkill reports whether id is a known, non-removed skill.
func isValidSkill(id string) bool {
	for _, e := range skillRegistry {
		if e.ID == id && e.Status != statusRemoved {
			return true
		}
	}
	return false
}

// skillDescriptionFromRegistry returns the description for a skill from the
// registry, falling back to the skill ID if not found.
func skillDescriptionFromRegistry(id string) string {
	for _, e := range skillRegistry {
		if e.ID == id {
			return e.Description
		}
	}
	return id
}

// deprecationWarningForSkill returns a non-empty warning string when the skill
// is deprecated, empty string otherwise.
func deprecationWarningForSkill(id string) string {
	for _, e := range skillRegistry {
		if e.ID == id && e.Status == statusDeprecated && e.Deprecated != nil {
			msg := "skill " + id + " is deprecated since " + e.Deprecated.Since
			if e.Deprecated.Remove != "" {
				msg += " and will be removed in " + e.Deprecated.Remove
			}
			if e.Deprecated.Message != "" {
				msg += ": " + e.Deprecated.Message
			}
			return msg
		}
	}
	return ""
}

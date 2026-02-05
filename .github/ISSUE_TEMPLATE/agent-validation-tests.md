---
name: Agent validation tests
about: Dockerfiles + rule validation per AI platform
title: Add agent validation tests: Dockerfiles + rule validation per AI platform
assignees: ''
---

## Summary

Create automated tests that validate the rules generated for each AI agent (Cursor, Claude, OpenCode, Codex) by running the agent itself against the generated rules. Use ballast as the example project. Each agent gets its own Dockerfile with appropriate API token handling.

## Requirements

### Test approach

- **Validate rules by using the agent**: For each target platform, install ballast rules into the ballast project, then invoke the agent to perform a simple task that exercises those rules. Assert the agent's output or behavior conforms to the rules.
- **Example project**: Use the ballast repo itself as the test project (run `ballast install` for each target, then run the agent).

### Per-agent setup

| Platform     | Execution         | API / model                      | Notes                                                                                                                            |
| ------------ | ----------------- | -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| **Codex**    | CLI tool          | Env var for API token            | Use Codex CLI                                                                                                                    |
| **Claude**   | CLI tool          | Env var for API token            | Use Claude CLI                                                                                                                   |
| **OpenCode** | CLI tool          | **Ollama** (local, no API token) | Use OpenCode CLI with Ollama backend                                                                                             |
| **Cursor**   | IDE (headless/CI) | Env var for API token            | Use generated rules in a **devcontainer**; Cursor runs from IDE, so validation runs inside devcontainer with rules pre-installed |

### Dockerfiles

- One Dockerfile per agent: `Dockerfile.codex`, `Dockerfile.claude`, `Dockerfile.opencode`, `Dockerfile.cursor`.
- Each Dockerfile:
  - Installs the agent CLI (or devcontainer tooling for Cursor)
  - Accepts API tokens via environment variables (e.g. `CODEX_API_KEY`, `ANTHROPIC_API_KEY`, `CURSOR_API_KEY`).
  - For OpenCode: use Ollama (no external API token).
  - Runs the validation test for that agent.

### Environment variables

- `CODEX_API_KEY` (or equivalent) for Codex
- `ANTHROPIC_API_KEY` (or equivalent) for Claude
- `CURSOR_API_KEY` (or equivalent) for Cursor
- OpenCode: no API key; use Ollama (local model)

### Cursor-specific

- Cursor runs from the IDE, so the test cannot invoke Cursor directly from a script.
- Use a **devcontainer** that:
  - Has ballast rules pre-installed (e.g. `.cursor/rules/*.mdc` from `ballast install --target cursor`)
  - Provides a script or documented steps to validate rules from within the IDE
  - Can be used in CI to build the devcontainer and verify the rules are present and correctly formatted

### Acceptance criteria

- [ ] Dockerfile for Codex with env-based API token
- [ ] Dockerfile for Claude with env-based API token
- [ ] Dockerfile for OpenCode using Ollama (no API token)
- [ ] Dockerfile/devcontainer for Cursor with env-based API token and pre-installed rules
- [ ] Validation test(s) that run each agent (or devcontainer) against ballast with installed rules
- [ ] CI workflow (optional) to run these validation tests
- [ ] Documentation for running validation locally (including required env vars)

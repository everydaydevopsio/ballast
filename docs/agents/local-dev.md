# Local Development Agent

The **local-dev** agent helps set up and maintain a consistent local development environment for language-specific projects.

## What It Sets Up

The local-dev agent installs multiple rules:

- **Base** — Reproducible environments, nvm, Docker Compose, env files, dev scripts
- **Badges** — README badges (CI, Release, License, GitHub Release, npm/Python/Go as applicable)
- **License** — `LICENSE` file, `package.json` license field, README reference (MIT default)
- **MCP** — Optional integration with GitHub MCP and issues MCP (Jira, Linear, GitHub Issues)

## What It Provides

- `.nvmrc` for consistent Node versions
- Dockerfile, `docker-compose.yaml`, and `docker-compose.local.yaml` with `develop.watch` for hot reload
- A `Makefile` with `up`, `down`, and `logs` targets for both the base stack and local watch-mode stack
- `.env.example` and env validation
- PR hygiene guidance: verify Copilot/reviewer assignment, use a sub-agent to watch checks, inspect failures with `gh`, and reply directly on resolved review threads
- License setup (MIT by default, configurable)
- README badges for CI, release, license, and package registries
- Optional PR/issue context when MCP servers are enabled

## Prompts to Improve Your App

### Environment and Node

- **"Add .nvmrc with the Node version from our CI"** — Version consistency
- **"Update the README with nvm setup instructions for new contributors"** — Onboarding
- **"Create .env.example with all required environment variables"** — Env documentation

### Docker

- **"Create a Dockerfile, docker-compose.yaml, docker-compose.local.yaml, and Makefile for local development"** — Full container setup
- **"Add develop.watch to docker-compose.local.yaml so code changes sync without full rebuilds"** — Hot reload
- **"Add Makefile targets for compose up/down/logs in normal and watch mode"** — Developer workflow
- **"Our Docker build is slow—add a .dockerignore and optimize the Dockerfile"** — Build performance

### License and Documentation

- **"Set up MIT license for this project"** — License files
- **"Add standard badges to our README (CI, Release, License)"** — Badges
- **"We use Apache-2.0—update AGENTS.md and create the LICENSE file"** — Custom license

### MCP (when enabled)

- **"Summarize my open PRs"** — Uses GitHub MCP
- **"What am I working on? List my assigned issues"** — Uses issues MCP
- **"Correlate PROJ-123 with the current branch"** — Ticket + code context

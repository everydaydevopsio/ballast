# Local Development Agent

The **local-dev** agent helps set up and maintain a consistent local development environment for TypeScript/JavaScript projects.

## What It Sets Up

The local-dev agent installs multiple rules:

- **Base** — Reproducible environments, nvm, Docker Compose, env files, dev scripts
- **Badges** — README badges (CI, Release, License, GitHub Release, npm)
- **License** — `LICENSE` file, `package.json` license field, README reference (MIT default)
- **MCP** — Optional integration with GitHub MCP and issues MCP (Jira, Linear, GitHub Issues)

## What It Provides

- `.nvmrc` for consistent Node versions
- Dockerfile and docker-compose.yml with `develop.watch` for hot reload
- `.env.example` and env validation
- License setup (MIT by default, configurable)
- README badges for CI, release, license, npm
- Optional PR/issue context when MCP servers are enabled

## Prompts to Improve Your App

### Environment and Node

- **"Add .nvmrc with the Node version from our CI"** — Version consistency
- **"Update the README with nvm setup instructions for new contributors"** — Onboarding
- **"Create .env.example with all required environment variables"** — Env documentation

### Docker

- **"Create a Dockerfile and docker-compose for local development"** — Full container setup
- **"Add develop.watch to docker-compose so code changes sync without full rebuilds"** — Hot reload
- **"Our Docker build is slow—add a .dockerignore and optimize the Dockerfile"** — Build performance

### License and Documentation

- **"Set up MIT license for this project"** — License files
- **"Add standard badges to our README (CI, Release, License)"** — Badges
- **"We use Apache-2.0—update AGENTS.md and create the LICENSE file"** — Custom license

### MCP (when enabled)

- **"Summarize my open PRs"** — Uses GitHub MCP
- **"What am I working on? List my assigned issues"** — Uses issues MCP
- **"Correlate PROJ-123 with the current branch"** — Ticket + code context

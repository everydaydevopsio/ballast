# Publishing Agent

The **publishing** agent adds release and distribution guidance for reusable libraries, SDKs, and installable apps.

## What It Sets Up

The publishing agent installs multiple rules:

- **Libraries** — release workflows for reusable packages
- **SDKs** — publish guidance for generated or hand-written client SDKs
- **Apps** — publish guidance for CLIs and installable applications

## What It Provides

- A release workflow shape modeled on Ballast `publish.yml`
- `workflow_dispatch` release flows with `patch`, `minor`, and `major` version selection
- TypeScript publishing guidance for npmjs
- Python publishing guidance for PyPI
- Go publishing guidance for GitHub tags and GitHub Releases
- Web app publishing guidance for Docker images in GHCR or Docker Hub plus deployment-model-specific release flow
- Release validation, tag-based publishing, and least-privilege permission patterns
- Packaged-command smoke tests for installable CLIs: install or execute the built artifact, check `--help`, check `--version`, and run one representative command before publishing

## Deployment Model

When the publishing agent is installed, Ballast records `deploymentModel` in `.rulesrc.json`. Valid values are:

- `none` — no app deployment assumptions; keep library, SDK, and CLI publishing guidance active.
- `kubernetes` — app repo owns `charts/<app>/`; a separate GitOps repo owns ArgoCD `Application` or `ApplicationSet` configuration and environment-specific deployment state.
- `serverless` — managed function or container platforms such as AWS Lambda, Cloud Run, or Azure Functions.
- `server` — self-managed VM, VPS, or bare-metal deployment with a repeatable artifact transfer, service restart, health check, and rollback path.
- `hosted` — app platforms such as Vercel, Netlify, Render, Railway, or Fly.io.

Use `ballast install --agent publishing --deployment-model kubernetes` for non-interactive app/service setup. Interactive installs prompt for the app deployment model only when the publishing agent is selected; choose `none` for CLI, library, or SDK-only projects.

## CLI Smoke Placement

- Local: run the packaged-command smoke check when changing command startup, packaging metadata, or release scripts.
- Pre-push: run the smoke check when the artifact can be built deterministically without publishing.
- CI: require the smoke check before publish jobs and before any release artifact is promoted.

## Prompts to Improve Your App

### Libraries

- **"Create a publish workflow for our TypeScript library to npmjs"** — npm trusted publishing
- **"Create a publish workflow with a workflow_dispatch patch/minor/major selector"** — Tag-driven version bumping
- **"Set up PyPI publishing for our Python package with GitHub Actions"** — PyPI trusted publishing
- **"Publish our Go module from tags and add GitHub release notes"** — Go release flow

### SDKs

- **"Add a release workflow for our generated SDKs in TypeScript, Python, and Go"** — Multi-language SDK release design
- **"Fail the release if generated SDK code is stale"** — Generation guardrail
- **"Document semver expectations for our SDK surface"** — Compatibility policy

### Apps

- **"Publish our Node CLI to npm with provenance"** — npm app distribution
- **"Publish our Python CLI to PyPI with console entry points"** — Python app packaging
- **"Release our Go CLI binaries to GitHub with checksums"** — Binary release automation
- **"Publish our web app image to GHCR and update the GitOps repo"** — Container + Kubernetes GitOps release flow
- **"Publish our web app to Docker Hub and pin production to the image digest"** — Immutable deployment release flow

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
- Web app publishing guidance for Docker images in GHCR or Docker Hub plus Helm charts in a separate repo
- Release validation, tag-based publishing, and least-privilege permission patterns

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
- **"Publish our web app image to GHCR and update the Helm chart repo"** — Container + chart release flow
- **"Publish our web app to Docker Hub and pin the Helm chart to the image digest"** — Immutable deployment release flow

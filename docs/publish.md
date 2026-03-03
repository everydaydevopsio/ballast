# Publishing Setup

This guide explains how to configure GitHub Actions publishing for:

- npm (`@everydaydevopsio/ballast`)
- PyPI (`ballast-python`)
- Go CLI release assets (`ballast-go`)

## Workflow Map

- Unified release workflow: `.github/workflows/publish.yml`
- TypeScript publish workflow: `.github/workflows/publish.typescript.yml`
- Python publish workflow: `.github/workflows/publish-python.yml`
- Go publish workflow: `.github/workflows/publish-go.yml`

## npmjs (OIDC Trusted Publishing)

### 1. Configure npm trusted publisher

In npm package settings for `@everydaydevopsio/ballast`, add a trusted publisher for this GitHub repository.

Use:

- GitHub owner: `everydaydevopsio`
- Repository: `ballast`
- Workflow file: `.github/workflows/publish.typescript.yml`
- Branch: `main`

If you publish TypeScript via `.github/workflows/publish.yml`, also add that workflow as a trusted publisher.

### 2. Required workflow permissions

The publish job must include:

- `id-token: write`

### 3. Publish command

Use `npm publish --provenance` from `packages/ballast-typescript`.

This repository already does that in `publish.typescript.yml`.

## PyPI (Trusted Publishing)

### 1. Configure PyPI trusted publisher

In PyPI project settings for `ballast-python`, add a trusted publisher:

- Owner: `everydaydevopsio`
- Repository: `ballast`
- Workflow: `.github/workflows/publish-python.yml`
- Environment: leave empty unless your workflow job uses a GitHub environment

### 2. Important limitation: reusable workflows

PyPI Trusted Publishing currently does not officially support reusable workflow calls (`workflow_call`) and may fail with:

- `invalid-publisher`
- claim mismatch warnings showing `workflow_ref` and `job_workflow_ref`

### 3. Recommended approach for this repo

For PyPI trusted publishing reliability:

1. Trigger `.github/workflows/publish-python.yml` directly (standalone `workflow_dispatch`).
2. Do not rely on Python publish through reusable workflow calls from `publish.yml` if using Trusted Publishing.

### 4. Alternative if you must publish via reusable workflow

Use a PyPI API token secret instead of Trusted Publishing for that path.

## Go (GitHub Releases)

Go publishing in this repository is release-asset publishing (GitHub Releases), not an external package registry publish.

### 1. Required settings

- Workflow permission: `contents: write`
- Go toolchain in workflow (`actions/setup-go`)
- GoReleaser config in `packages/ballast-go/.goreleaser.yaml`

### 2. Release assets

Upload unique archive artifacts to avoid duplicate-name collisions:

- `*.tar.gz`
- `*.zip`
- `checksums.txt`

This repository already limits uploaded files in `publish-go.yml`.

## Quick Checklist

- npm trusted publisher configured for this repo and publish workflow.
- PyPI trusted publisher configured for `publish-python.yml`.
- PyPI publish triggered from standalone Python workflow when using Trusted Publishing.
- `id-token: write` enabled for npm/PyPI trusted publishing jobs.
- `contents: write` enabled for tag/release creation jobs.

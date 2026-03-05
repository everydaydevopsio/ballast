# Publishing Setup

This guide explains how to configure GitHub Actions publishing for:

- npm (`@everydaydevopsio/ballast`)
- Python artifacts on GitHub Releases (`ballast-python`)
- Go CLI release assets (`ballast-go`)

## Workflow Map

- Unified release workflow: `.github/workflows/publish.yml`
- TypeScript publish workflow: `.github/workflows/publish.typescript.yml`
- Python publish workflow: `.github/workflows/publish-python.yml`
- Go publish workflow: `.github/workflows/publish-go.yml`

## npmjs (OIDC Trusted Publishing)

### 1. Configure npm trusted publisher

In npm package settings for `@everydaydevopsio/ballast`, add a trusted publisher for:

- GitHub owner: `everydaydevopsio`
- Repository: `ballast`
- Workflow file: `.github/workflows/publish.typescript.yml`
- Branch: `main`

If you publish TypeScript via `.github/workflows/publish.yml`, add that workflow as a trusted publisher too.

### 2. Required workflow permissions

- `id-token: write`

### 3. Publish command

- `npm publish --access public --provenance`

## Python (GitHub Releases Artifacts)

Python is published as wheel/sdist assets on the GitHub Release tag (not PyPI Trusted Publishing).

### 1. Required workflow permissions

- `contents: write`

### 2. Publish behavior

`.github/workflows/publish-python.yml` builds:

- `packages/ballast-python/dist/*.whl`
- `packages/ballast-python/dist/*.tar.gz`

Then uploads them to release tag `v<version>`.

### 3. Install from GitHub Releases

```bash
VERSION=4.0.0
uv tool install --from "https://github.com/everydaydevopsio/ballast/releases/download/v${VERSION}/ballast_python-${VERSION}-py3-none-any.whl" ballast
```

Or run once without global install:

```bash
VERSION=4.0.0
uvx --from "https://github.com/everydaydevopsio/ballast/releases/download/v${VERSION}/ballast_python-${VERSION}-py3-none-any.whl" ballast install --target cursor --all
```

## Go (GitHub Releases)

Go publishing in this repository is release-asset publishing (GitHub Releases), not an external package registry publish.

### 1. Required settings

- Workflow permission: `contents: write`
- Go toolchain in workflow (`actions/setup-go`)
- GoReleaser config in `packages/ballast-go/.goreleaser.yaml`

### 2. Release assets

Upload unique archive artifacts:

- `*.tar.gz`
- `*.zip`
- `checksums.txt`

## Quick Checklist

- npm trusted publisher configured for this repo/workflow.
- Python workflow uploads wheel/sdist assets to GitHub Release tag `v<version>`.
- Go workflow uploads archives/checksums to GitHub Release.
- `id-token: write` enabled for npm trusted publish jobs.
- `contents: write` enabled for release asset upload jobs.

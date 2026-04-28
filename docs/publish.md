# Publishing Setup

This guide explains how to configure GitHub Actions publishing for:

- npm (`@everydaydevopsio/ballast`)
- Python artifacts on GitHub Releases (`ballast-python`)
- Go CLI release assets (`ballast-go`)
- Homebrew formula publishing for Linux `brew install ballast`
- Homebrew cask publishing for the `ballast` wrapper CLI

## Workflow Map

- Unified release workflow: `.github/workflows/publish.yml`
- TypeScript publish workflow: `.github/workflows/publish.typescript.yml`
- Python publish workflow: `.github/workflows/publish-python.yml`
- Go package workflow: `.github/workflows/publish-go.yml`
- CLI wrapper workflow: `.github/workflows/publish-cli.yml`
- Shared cross-language validation workflow: `.github/workflows/cross-language-validate.yml`

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
VERSION=5.2.0
uv tool install --from "https://github.com/everydaydevopsio/ballast/releases/download/v${VERSION}/ballast_python-${VERSION}-py3-none-any.whl" ballast-python
```

Or run once without global install:

```bash
VERSION=5.2.0
uvx --from "https://github.com/everydaydevopsio/ballast/releases/download/v${VERSION}/ballast_python-${VERSION}-py3-none-any.whl" ballast-python install --target cursor --all
```

## Go (GitHub Releases)

Go publishing in this repository is release-asset publishing (GitHub Releases), not an external package registry publish.

### 1. Required settings

- Workflow permission: `contents: write`
- Go toolchain in workflow (`actions/setup-go`)
- GoReleaser config in `cli/ballast/.goreleaser.yaml`
- Repository secret: `HOMEBREW_TAP_GITHUB_TOKEN`

### 2. Release assets

Upload unique archive artifacts:

- `*.tar.gz`
- `*.zip`
- `ballast-go_checksums.txt`

### 3. Homebrew tap setup with GoReleaser

Create a separate tap repository named `homebrew-ballast` under the `everydaydevopsio` GitHub org or user. The repository should contain a `Casks/` directory and allow pushes from a token stored as the `HOMEBREW_TAP_GITHUB_TOKEN` Actions secret in this repo.

`cli/ballast/.goreleaser.yaml` publishes both a formula and a cask named `ballast` into that tap:

- Homebrew tap repo: `everydaydevopsio/homebrew-ballast`
- Formula path: `Formula/ballast.rb`
- Cask path: `Casks/ballast.rb`
- Cask test: `ballast --help`

The Go publish workflows run:

```bash
goreleaser release --clean --config .goreleaser.yaml
```

That single GoReleaser release step:

- builds the `ballast` archives
- uploads release artifacts to GitHub Releases
- writes or updates `Formula/ballast.rb` in the tap repository
- writes or updates `Casks/ballast.rb` in the tap repository

### 4. Homebrew install commands

After the tap repo exists and the release workflow has run for a tagged version:

Linux:

```bash
brew tap everydaydevopsio/ballast
brew install ballast
```

macOS:

```bash
brew tap everydaydevopsio/ballast
brew install --cask ballast
```

## macOS Code Signing and Notarization

macOS release artifacts are signed with a Developer ID Application certificate and notarized with Apple before the Homebrew cask is updated. This is required for Gatekeeper to accept the binary on macOS.

### One-time Apple Developer setup

1. Enroll in the [Apple Developer Program](https://developer.apple.com/programs/enroll) ($99/year)
2. In **Keychain Access**, create a CSR: Keychain Access → Certificate Assistant → Request a Certificate From a Certificate Authority → save to disk
3. At `developer.apple.com` → Certificates, IDs & Profiles → `+` → **Developer ID Application** → upload the CSR → download and double-click the `.cer` to install into the **login** keychain
4. In Keychain Access → login → My Certificates, right-click the cert → **Export** → save as `.p12` with a strong password
5. At `appstoreconnect.apple.com` → Users and Access → Integrations → App Store Connect API → `+` → Name: `Ballast Release`, Access: **Developer** → download the `.p8` (only available once)

### Required GitHub secrets

Add these to `everydaydevopsio/ballast` → Settings → Secrets and variables → Actions:

| Secret | How to get it |
|---|---|
| `APPLE_CERTIFICATE` | `base64 -i certificate.p12` |
| `APPLE_CERTIFICATE_PASSWORD` | password set when exporting the `.p12` |
| `APPLE_SIGNING_IDENTITY` | `Developer ID Application: Mark C Allen (TEAMID)` |
| `APPLE_API_KEY_ID` | Key ID from App Store Connect |
| `APPLE_API_KEY_ISSUER_ID` | Issuer ID from App Store Connect |
| `APPLE_API_KEY_CONTENT` | `base64 -i AuthKey_KEYID.p8` |

### How it works in CI

The `publish_cli` job in `publish.yml` and both jobs in `publish-cli.yml` run on `macos-latest` so that signing tools are available. The sequence is:

1. **Import certificate** — `apple-actions/import-codesign-certs` installs the `.p12` into the runner keychain
2. **GoReleaser build** — a `hooks.post` in `.goreleaser.yaml` calls `codesign` on each darwin binary immediately after compilation, before archiving
3. **GoReleaser publish** — signed binaries are archived, SHA256 checksums computed, artifacts uploaded to GitHub Releases, and `Formula/ballast.rb` and `Casks/ballast.rb` updated with correct checksums in the Homebrew tap
4. **Notarization** — each darwin archive is extracted and submitted to Apple via `xcrun notarytool submit --wait`; Apple validates the signature and registers a ticket in their CDN so Gatekeeper's online check passes. Notarization runs after the Homebrew tap is updated; there is a short window (typically under five minutes) between the tap update and notarization completing during which a `brew install --cask ballast` would succeed but Gatekeeper's online check might not yet find the notarization ticket.

### Verifying a signed release locally

```bash
# Check signature
codesign --verify --verbose $(which ballast)

# Check Gatekeeper acceptance
spctl --assess --verbose $(which ballast)
```

Both should succeed after a signed release is installed.

## Quick Checklist

- Single-language publish workflows call the shared cross-language validation workflow before publishing.
- Release validation runs `scripts/release-cross-language-check.sh` to verify TypeScript, Python, Go, and unified monorepo installs.
- npm trusted publisher configured for this repo/workflow.
- Python workflow uploads wheel/sdist assets to GitHub Release tag `v<version>`.
- CLI workflow uploads archives/checksums to GitHub Release and updates the Homebrew tap formula and cask.
- `id-token: write` enabled for npm trusted publish jobs.
- `contents: write` enabled for release asset upload jobs.
- `HOMEBREW_TAP_GITHUB_TOKEN` configured for tap repository pushes.

<!-- ballast:rule id="typescript/local-dev/badges" version="5.18.3" checksum="c61fea82e93ae164b0bbc418d7c36a01c138719559d12e0bc5730133b77bfad5" -->
# Local Development: README Badges

These rules are intended for Codex (CLI and app).

Add standard badges (CI, Release, License, GitHub Release; plus npm for published packages) to the top of README.md.

---
# README Badges

When setting up or improving project documentation, add standard badges near the top of `README.md` for quick visibility into CI status, releases, license, and (for npm packages) registry info.

## Your Responsibilities

1. Place badges immediately after the main title (or project description), before the first `##` heading, on one compact line separated by spaces.
2. Include the standard set:
   - **CI** and **Release**: GitHub Actions badges (`https://github.com/OWNER/REPO/actions/workflows/<file>/badge.svg`) linking to the workflow. Use the actual filenames in `.github/workflows/` (e.g. `ci.yml`, `release.yml`, `publish.yml`) — never assume names.
   - **License**: `https://img.shields.io/github/license/OWNER/REPO` linking to `LICENSE`.
   - **GitHub Release**: `https://img.shields.io/github/v/release/OWNER/REPO` linking to the releases page.
3. For published npm packages, add **npm version** (`https://img.shields.io/npm/v/PACKAGE.svg`) and optionally **npm downloads** (`.../npm/dm/PACKAGE.svg`), both linking to the package page.

## Implementation Order

1. Determine `OWNER/REPO` from the git remote or `package.json` repository field.
2. List `.github/workflows/` to identify the real CI and release workflow filenames.
3. Check `package.json` for a published npm package name.
4. Add the badges after the README title.

## When to Apply

- When creating a project README, when a README lacks badges, or when new CI/release workflows or npm publishing are added without matching badges.

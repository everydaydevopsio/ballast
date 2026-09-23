<!-- ballast:rule id="typescript/publishing/libraries" version="5.21.0" checksum="95f4bec59a20df0d0fec66ebfb5ab3a618ffeb151dc0b3e8621381ac3e0d9be2" -->
# Publishing Libraries Agent

## Goals

- Ship reproducible releases from tagged source, not from an arbitrary branch state.
- Publish TypeScript libraries to npmjs, Python libraries to PyPI, and Go libraries through Git tags and GitHub releases.

Follow the shared publishing release pattern (`publishing` rule) for the bump-and-tag workflow, version and tag rules, concurrency, and per-registry publish guidance. This rule adds only the artifact-specific requirements.

## Library-Specific Requirements

- The workflow-dispatch `release_type` input must be the only manual version selector unless the user explicitly asks for a different release process.
- TypeScript: publish to npmjs, not only GitHub Releases; require typed exports, a clean build step, and tests before publish.
- Python: keep TestPyPI available for dry runs when the maintainer wants a staging path.
- Go: if the repository also ships example binaries, attach them to GitHub Releases, but the module tag stays the source of truth for library consumers.
- Registry credentials or trusted-publishing permissions must be scoped to only the job that needs them.

## When to Apply

- When creating or updating release workflows for reusable libraries published to npmjs or PyPI, or consumed as Go modules from GitHub tags.
- When a repo currently publishes from branch state instead of tagged, validated source.

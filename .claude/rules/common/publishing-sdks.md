<!-- ballast:rule id="typescript/publishing/sdks" version="5.19.0" checksum="fcb239455363d0772910f9ff21f9d199ab6ae2a0b554035859300ef0fb8eb630" -->
# Publishing SDKs Agent

## Goals

- Publish SDKs with clear API-version compatibility and stable semantic versioning.
- Publish TypeScript SDKs to npmjs, Python SDKs to PyPI, and Go SDKs through GitHub tags and releases.

Follow the shared publishing release pattern (`publishing` rule) for the bump-and-tag workflow, version and tag rules, concurrency, and per-registry publish guidance. This rule adds only the artifact-specific requirements.

## SDK-Specific Requirements

- Match generated package metadata to the computed release version before creating the tag.
- For generated SDKs, check generation reproducibly in CI and fail the release if generated output is stale relative to the source API description.
- Document the upstream API or schema version the SDK targets; keep examples and generated docs in sync with the released package.
- Avoid breaking renames or regenerated surface changes without a semver-major release; record deprecations before removal.
- Changelogs must describe both API compatibility and package-level changes.

## When to Apply

- When a repository publishes reusable API clients, generated clients, or framework SDKs.
- When code generation is part of the release path.

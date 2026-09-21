<!-- ballast:rule id="typescript/publishing/apps" version="5.19.0" checksum="1eeb1d916634d9ca9a62b60707e659dc83081fd177f1c09dbbc2c9a3cb8339ed" -->
# Publishing Apps Agent

## Goals

- Publish installable applications from validated release tags.
- Publish TypeScript apps to npmjs when they are distributed as Node packages, Python apps to PyPI when they are installed as Python packages, and Go apps to GitHub Releases (prefer GoReleaser for multi-OS/arch binaries).

Follow the shared publishing release pattern (`publishing` rule) for the bump-and-tag workflow, version and tag rules, concurrency, and per-registry publish guidance. This rule adds only the artifact-specific requirements.

## App Deployment Model

No app deployment model is configured (`deploymentModel: none`). Deployment guidance is reference-only. Deployment is inactive: keep library, SDK, CLI, and optional container publishing guidance active, but do not create deploy-on-main workflows, deployment-state updates, Kubernetes, serverless, hosted-platform, Docker registry, or self-managed server deployment ownership until the repository sets an active `deploymentModel`.

### Container Publishing

When an app releases a container image (GHCR `ghcr.io/<owner>/<image>` or Docker Hub):

- Build the production image from the checked-out release tag; push only after tests and build verification pass.
- Tag with the app version from the `v<version>` tag and the git SHA; add `latest` only when the team explicitly wants a mutable tag; prefer immutable deploy references using the published digest.
- Use `docker/login-action` (GHCR: `GITHUB_TOKEN` with `packages: write`; Docker Hub: `DOCKERHUB_USERNAME`/`DOCKERHUB_TOKEN` secrets), `docker/build-push-action` to build/push and emit the digest, and `docker/setup-buildx-action` when multi-platform builds are needed.
- Keep the app version and deployment-manifest version distinct; make target environments explicit; do not hardcode registry passwords or hide deployment-state changes in scripts without a visible diff.

## App-Specific Requirements

- Smoke-test the installed app or CLI from the built artifact before publish, and ensure its version output matches the release tag.
- Publish checksums for downloadable binaries when the app ships archives.
- Keep `README.md` installation instructions aligned with the actual release channel.
- The workflow-dispatch `release_type` input decides whether the next app tag is patch, minor, or major.

## When to Apply

- When a repository publishes a CLI, desktop helper, or installable service package intended for direct installation by end users.
- When a web app is released as a container image and deployed through the configured deployment model.

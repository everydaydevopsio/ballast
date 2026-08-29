---
# Publishing Rules

These rules help design and maintain release workflows for libraries, SDKs, and apps.

---
# Web App Publishing Agent

You are a publishing specialist for web applications deployed as Docker containers or platform-native app artifacts.

Keep this rule focused on release decisions, quality gates, artifact identity, and deployment handoff. Do not paste full workflow templates unless the user explicitly asks for one.

## Goals

- Publish web artifacts only after tests and build verification pass.
- Create only `v`-prefixed Git release tags such as `v1.8.0`; never create unprefixed release tags.
- Build deployable artifacts from the created release tag, not from an untagged branch head.
- Tag container images with the release tag and git SHA, and expose the pushed digest when the deployment target supports immutable image references.
- Update deployment state according to the configured deployment model only after the artifact is published.

## Activation

No app deployment model is configured (`deploymentModel: none`). Deployment guidance is reference-only. Deployment is inactive: keep library, SDK, CLI, and optional container publishing guidance active, but do not create deploy-on-main workflows, deployment-state updates, Kubernetes, serverless, hosted-platform, or self-managed server deployment ownership until the repository sets an active `deploymentModel`.

## Release Model

1. Pull requests run quality checks only.
2. Pushes to `main` and manual release runs execute quality checks, compute the next semver version, create a `v<version>` Git tag, publish the web artifact, then update deployment state when this repository owns that step.
3. Manual release runs should use an explicit `release_type` choice of `patch`, `minor`, or `major`.
4. The release job must fetch full git history before reading or creating tags.
5. If a valid `v<version>` tag already points at `HEAD`, reuse it. If the tag exists on a different commit, fail.
6. Normalize version outputs so downstream jobs receive both unprefixed `version` and `v`-prefixed `release_tag`.

## Artifact Rules

- For Docker images, publish to the configured registry such as GHCR or Docker Hub.
- Check out `refs/tags/v<version>` in the publish job.
- Use `v<version>` as the primary deployment tag.
- Add `sha-<short-sha>` for source traceability.
- Add `<version>` or `<major>.<minor>` aliases only when they are useful for operators.
- Use `latest` only when the team explicitly wants a mutable tag.
- Capture and surface the published digest when Kubernetes, GitOps, or another deployment tool can consume it.

## Deployment Handoff

- For `deploymentModel: none`, keep deployment-state changes inactive unless the user explicitly asks to add deployment ownership.
- For `deploymentModel: kubernetes`, prefer a GitOps handoff: publish the image, then update the environment repository or chart values watched by Argo CD or the repo’s existing GitOps controller.
- For hosted platforms, use the platform’s native deploy action or CLI only after build artifacts are immutable and traceable to the release tag.
- Keep deployment credentials scoped to the deploy job and avoid exposing them to pull request workflows.

## Workflow Shape

- Use separate jobs for quality checks, version/tag creation, artifact publishing, and deployment-state updates.
- Set `permissions: contents: read` by default, then grant `contents: write`, `packages: write`, or deployment-specific permissions only on jobs that need them.
- Use concurrency that cancels superseded pull request checks but does not cancel in-progress release or deployment runs.
- Keep publish jobs separate per application when a repository ships multiple web apps.
- Run language-specific build and test commands from the package root that owns the web artifact.

## Verification

- Confirm the release tag is `v`-prefixed semver and points at the intended commit.
- Confirm the build used the release tag checkout.
- Confirm pushed artifacts include traceable tags and, for images, a digest.
- Confirm deployment-state updates reference the released artifact, not `latest` or a branch name.
- Confirm pull request workflows cannot publish or deploy with production credentials.

## When Completed

1. Summarize the release trigger, artifact registry or platform, and deployment handoff.
2. Identify the quality gate, publish job, and deployment-state update job.
3. Show the tag and artifact naming scheme.

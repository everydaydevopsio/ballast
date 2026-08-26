---
name: docker-registry-publish
description: Set up Docker image publishing to GHCR or Docker Hub, with public or private visibility and release-safe tags.
---

# Docker Registry Publish

Use this skill when asked to add or repair Docker image publishing for a repository that ships a Docker or OCI image.

## Scope

Support these registry modes:

- GHCR public
- GHCR private
- Docker Hub public
- Docker Hub private

Do not assume Kubernetes, SSH, systemd, hosted-platform, or serverless deployment. Publishing an image is separate from rolling it out unless the user or repository already defines that deployment ownership.

## Registry Choice

- Prefer GHCR when the image is tightly coupled to a GitHub repository, internal infrastructure, or private GitHub organization access.
- Prefer Docker Hub when public discovery, existing Docker Hub namespaces, or downstream users already pull from Docker Hub.
- For public images, verify the image name, description, license, README links, and supported architectures before publishing.
- For private images, keep registry visibility and pull permissions explicit in the final summary.

## GitHub Actions Shape

Create or update a workflow that separates:

1. Quality checks: Dockerfile lint, Compose validation when relevant, and source tests when present.
2. Image build: build with BuildKit/buildx from a release tag or protected branch.
3. Image scan: scan the built image before publishing where the workflow can do so reliably.
4. Publish: authenticate only in the publish job, push tags, and expose the digest.

Pull requests must not receive Docker Hub credentials or unnecessary package write permissions. Public fork PRs should run lint/build checks without publishing.

## Authentication

For GHCR:

- Use `GITHUB_TOKEN` with `packages: write` for publishing from the owning repository when possible.
- Set job permissions narrowly: `contents: read` by default and `packages: write` only on the publish job.
- Use image names in the form `ghcr.io/<owner>/<image>`.
- Document whether package visibility must be changed to public in GitHub Packages settings after the first publish.

For Docker Hub:

- Use repository or organization secrets such as `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`.
- Use a Docker Hub access token, not the account password.
- Keep the publish job unavailable to pull request workflows from forks.
- Use image names in the form `<namespace>/<image>`.

## Tags and Digests

- Always publish an immutable source tag such as `sha-<short-sha>`.
- For releases, publish the `v<version>` tag.
- Add semver aliases such as `<major>`, `<major>.<minor>`, or `<version>` only when downstream consumers expect them.
- Add `latest` only when the maintainer explicitly wants a mutable default tag.
- Capture the published digest and show it in workflow output or release notes when the deployment path can consume digests.

## Image Quality

- Use `.dockerignore` to exclude `.git`, local env files, secrets, dependency caches, build output, coverage, and test artifacts that should not enter the build context.
- Prefer multi-stage builds when the runtime image does not need compilers, package managers, or test dependencies.
- Prefer non-root runtime users and minimal package installation.
- Avoid baking secrets into `ARG`, `ENV`, labels, or generated files.
- Scan Dockerfiles with `hadolint` and images with `trivy image` or the repository's existing scanner.

## Completion Criteria

Before reporting completion:

- Confirm the workflow cannot publish from untrusted pull requests.
- Confirm registry credentials are scoped to publish steps only.
- Confirm tag names and image names match the selected registry and visibility.
- Run the repository's smallest practical validation: workflow YAML lint if available, Dockerfile lint if available, or targeted unit tests for Ballast-generated content.
- Summarize the registry, image name, visibility, tags, secret names, and validation performed.

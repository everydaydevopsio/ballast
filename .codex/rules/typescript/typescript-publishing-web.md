---
# Publishing Rules

These rules are intended for Codex (CLI and app).

These rules help design and maintain release workflows for libraries, SDKs, and apps.

---
# Web App Publishing Agent

You are a publishing specialist for web applications deployed as Docker containers or platform-native app artifacts.

## Goals

- Publish Docker images to GHCR or Docker Hub only after tests and build verification pass.
- For `deploymentModel: kubernetes`, use a versioned GitOps release flow: quality gates, `bump_and_tag`, image publish, then GitOps values update for Argo CD.
- Create only `v`-prefixed Git release tags such as `v1.8.0`; never create unprefixed release tags.
- Build images from the created release tag, tag images with the release tag and git SHA, and expose the pushed digest for immutable deployments.
- Update deployment state according to the configured deployment model only after the tagged image is pushed.

## Activation

No app deployment model is configured (`deploymentModel: none`). Deployment guidance is reference-only. Deployment is inactive: keep library, SDK, CLI, and optional container publishing guidance active, but do not create deploy-on-main workflows, deployment-state updates, Kubernetes, serverless, hosted-platform, or self-managed server deployment ownership until the repository sets an active `deploymentModel`.

## Release Model

Web Docker publishing is a release flow. For Kubernetes deployments, model it after this sequence:

1. Pull requests run quality checks only.
2. Pushes to `main` and manual `workflow_dispatch` runs execute quality checks, compute the next semver version, create a `v<version>` Git tag, publish the image, then update the GitOps repository watched by Argo CD.
3. `workflow_dispatch` must use a required `release_type` choice input of `patch`, `minor`, or `major`.
4. The `bump_and_tag` job must:
   - fetch full git history
   - detect an existing `v<version>` tag already pointing at `HEAD`
   - fetch the previous tag with `WyriHaximus/github-action-get-previous-tag@v2`
   - calculate the next patch, minor, and major versions with `WyriHaximus/github-action-next-semvers`
   - normalize the computed version to an unprefixed semver output and a separate `v`-prefixed `release_tag` output
   - update app version files when the app has them
   - create and push only `v<version>`, never `<version>`
5. The Docker publish job must check out `refs/tags/v<version>`, not the branch head.
6. Image tags should include:
   - `v<version>` as the primary deployment tag
   - `<version>` and `<major>.<minor>` aliases when useful for operators
   - `sha-<short-sha>` for source traceability
   - `latest` only when the team explicitly wants a mutable tag
7. Kubernetes GitOps updates should write the image tag and, when the chart supports it, the image digest.

## Workflow Trigger and Concurrency

Use this workflow trigger when this repository publishes and deploys a Kubernetes web image. If `deploymentModel` is `none`, keep deployment-state update jobs inactive unless the user explicitly asks to introduce deployment ownership.

```yaml
on:
  pull_request:
    branches: [main]
  push:
    branches: [main]
  workflow_dispatch:
    inputs:
      release_type:
        description: 'Release type (patch/minor/major)'
        type: choice
        options:
          - patch
          - minor
          - major
        default: 'patch'
        required: true

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: false
```

Do not cancel in-progress release, image publish, or GitOps deployment runs. If the project needs aggressively cancellable pull request checks, put PR-only CI in a separate workflow with `cancel-in-progress: true`.

## Kubernetes Workflow Template (`deploy.yml`)

This template is conditional. Apply it when `deploymentModel` is `kubernetes` and this repository owns the application image while a separate GitOps repository owns environment state.

```yaml
name: CI, Image, and GitOps Deploy

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]
  workflow_dispatch:
    inputs:
      release_type:
        description: 'Release type (patch/minor/major)'
        type: choice
        options:
          - patch
          - minor
          - major
        default: 'patch'
        required: true

permissions:
  contents: read

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: false

jobs:
  quality:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v4
      # Set up the project runtime and package manager here.
      - run: make test
      - run: make build

  bump_and_tag:
    needs:
      - quality
    if: ${{ github.event_name != 'pull_request' && github.ref == 'refs/heads/main' }}
    runs-on: ubuntu-latest
    permissions:
      contents: write
    outputs:
      version: ${{ steps.version.outputs.version }}
      release_tag: ${{ steps.version.outputs.release_tag }}
      major_minor: ${{ steps.version_parts.outputs.major_minor }}
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
          fetch-depth: 0

      - name: Find existing version tag on HEAD
        id: existing_tag
        shell: bash
        run: |
          set -euo pipefail
          existing_tag="$(git tag --points-at HEAD --list 'v[0-9]*.[0-9]*.[0-9]*' | sort -V | tail -n 1)"
          echo "tag=${existing_tag}" >> "$GITHUB_OUTPUT"

      - name: Get previous tag
        id: previoustag
        uses: WyriHaximus/github-action-get-previous-tag@v2
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Get next version
        id: semvers
        uses: WyriHaximus/github-action-next-semvers@v1
        with:
          version: ${{ steps.previoustag.outputs.tag }}

      - name: Set version
        id: version
        shell: bash
        run: |
          set -euo pipefail

          if [ -n "${{ steps.existing_tag.outputs.tag }}" ]; then
            release_tag="${{ steps.existing_tag.outputs.tag }}"
            version="${release_tag#v}"
          else
            case "${{ github.event.inputs.release_type || 'patch' }}" in
              major) version="${{ steps.semvers.outputs.major }}" ;;
              minor) version="${{ steps.semvers.outputs.minor }}" ;;
              patch) version="${{ steps.semvers.outputs.patch }}" ;;
              *) echo "Unsupported release_type: ${{ github.event.inputs.release_type }}" >&2; exit 1 ;;
            esac
            version="${version#v}"
            release_tag="v${version}"
          fi

          if ! [[ "${release_tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "Release tag must be v-prefixed semver: ${release_tag}" >&2
            exit 1
          fi

          echo "version=${version}" >> "$GITHUB_OUTPUT"
          echo "release_tag=${release_tag}" >> "$GITHUB_OUTPUT"

      - name: Set version parts
        id: version_parts
        shell: bash
        run: |
          set -euo pipefail
          major_minor="$(cut -d. -f1,2 <<< "${{ steps.version.outputs.version }}")"
          echo "major_minor=${major_minor}" >> "$GITHUB_OUTPUT"

      - name: Bump app version
        shell: bash
        run: |
          set -euo pipefail
          VERSION="${{ steps.version.outputs.version }}"
          # Update package.json, pyproject.toml, Chart.yaml, or another app-owned
          # version file here when the application stores a release version.
          echo "Set application version to ${VERSION}"

      - name: Commit version changes and create tag
        shell: bash
        run: |
          set -euo pipefail
          release_tag="${{ steps.version.outputs.release_tag }}"

          if git rev-parse --verify --quiet "refs/tags/${release_tag}" >/dev/null; then
            tagged_commit="$(git rev-list -n 1 "${release_tag}")"
            head_commit="$(git rev-parse HEAD)"
            if [ "${tagged_commit}" != "${head_commit}" ]; then
              echo "Tag ${release_tag} already exists but does not point at HEAD." >&2
              exit 1
            fi
            echo "Tag ${release_tag} already exists on HEAD; skipping creation."
            exit 0
          fi

          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"

          if ! git diff --quiet; then
            # Replace this list with the exact version files changed above.
            git add package.json package-lock.json pnpm-lock.yaml pyproject.toml uv.lock Chart.yaml charts/*/Chart.yaml 2>/dev/null || true
            git commit -m "chore(release): bump version to ${release_tag} [skip ci]"
          fi

          git tag -a "${release_tag}" -m "Release ${release_tag}"
          git push origin HEAD:${{ github.ref_name }}
          git push origin "refs/tags/${release_tag}"

  image:
    needs:
      - quality
      - bump_and_tag
    if: ${{ github.event_name != 'pull_request' && github.ref == 'refs/heads/main' }}
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write        # required for GHCR; remove for Docker Hub
    outputs:
      image_tag: ${{ needs.bump_and_tag.outputs.release_tag }}
      image_digest: ${{ steps.push.outputs.digest }}
    steps:
      - name: Checkout release tag
        uses: actions/checkout@v4
        with:
          ref: refs/tags/${{ needs.bump_and_tag.outputs.release_tag }}

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      # For Docker Hub instead:
      # - uses: docker/login-action@v3
      #   with:
      #     username: ${{ secrets.DOCKERHUB_USERNAME }}
      #     password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ghcr.io/${{ github.repository }}
          # For Docker Hub: images: docker.io/NAMESPACE/IMAGE
          tags: |
            type=raw,value=${{ needs.bump_and_tag.outputs.release_tag }}
            type=raw,value=${{ needs.bump_and_tag.outputs.version }}
            type=raw,value=${{ needs.bump_and_tag.outputs.major_minor }}
            type=sha,prefix=sha-,format=short
            # Add latest only when the team explicitly wants a mutable tag:
            # type=raw,value=latest

      - name: Build and push
        id: push
        uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

      - name: Report image digest
        run: echo "Published image digest ${{ steps.push.outputs.digest }}"

  deploy:
    needs:
      - image
    if: ${{ github.event_name != 'pull_request' && github.ref == 'refs/heads/main' }}
    uses: ./.github/workflows/gitops-deploy.yml
    with:
      tag: ${{ needs.image.outputs.image_tag }}
      digest: ${{ needs.image.outputs.image_digest }}
    secrets:
      GITOPS_TOKEN: ${{ secrets.GITOPS_TOKEN }}
```

## GitOps Deploy Workflow Template (`gitops-deploy.yml`)

Use a separate reusable workflow for GitOps updates. It should be callable by the image workflow and manually runnable for an operator-provided image tag.

```yaml
name: GitOps Deploy

on:
  workflow_call:
    inputs:
      tag:
        description: Image tag to deploy. Must be v-prefixed semver.
        required: true
        type: string
      digest:
        description: Image digest produced by the image publish job.
        required: false
        type: string
    secrets:
      GITOPS_TOKEN:
        required: true
  workflow_dispatch:
    inputs:
      tag:
        description: Image tag to deploy. Must be v-prefixed semver.
        required: true
        type: string
      digest:
        description: Optional image digest to pin.
        required: false
        type: string

permissions:
  contents: read

concurrency:
  group: ${{ github.workflow }}-${{ inputs.tag }}
  cancel-in-progress: false

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Validate image tag
        shell: bash
        run: |
          set -euo pipefail
          if ! [[ "${{ inputs.tag }}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "Image tag must be v-prefixed semver: ${{ inputs.tag }}" >&2
            exit 1
          fi

      - name: Checkout GitOps repository
        uses: actions/checkout@v4
        with:
          repository: OWNER/GITOPS_REPO
          ref: main
          path: gitops
          token: ${{ secrets.GITOPS_TOKEN }}

      - name: Install yq
        uses: mikefarah/yq@v4

      - name: Update GitOps image reference
        shell: bash
        run: |
          set -euo pipefail
          export IMAGE_REPOSITORY="ghcr.io/${{ github.repository }}"
          export IMAGE_TAG="${{ inputs.tag }}"
          export IMAGE_DIGEST="${{ inputs.digest }}"
          values_file="gitops/path/to/app/values.yaml"

          yq -i '.image.repository = strenv(IMAGE_REPOSITORY) | .image.tag = strenv(IMAGE_TAG)' "${values_file}"
          if [ -n "${IMAGE_DIGEST}" ]; then
            yq -i '.image.digest = strenv(IMAGE_DIGEST)' "${values_file}"
          fi

      - name: Commit GitOps deployment update
        shell: bash
        run: |
          set -euo pipefail
          cd gitops
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add path/to/app/values.yaml
          git diff --staged --quiet && echo "No deployment update needed." && exit 0
          git commit -m "chore: deploy <app> ${{ inputs.tag }}"
          git push
```

## Kubernetes Chart Rules

For Kubernetes, keep the Helm chart in the application repository and keep Argo CD `Application` or environment values in the GitOps repository.

If digest pinning is enabled, render the image by digest when `.Values.image.digest` is set, while keeping `.Values.image.tag` for readability:

```yaml
image:
  repository: ghcr.io/OWNER/IMAGE
  tag: v1.2.3
  digest: ""
```

```yaml
image: "{{ .Values.image.repository }}{{ if .Values.image.digest }}@{{ .Values.image.digest }}{{ else }}:{{ .Values.image.tag }}{{ end }}"
```

If the platform cannot use digest pinning, deploy `v<version>` tags rather than `latest`.

## Container Registry Targets

Choose one registry per deployment. Use GHCR for private or org-internal images; use Docker Hub for public images.

### GHCR (`ghcr.io`)

- Authenticate with `GITHUB_TOKEN` for same-repository images.
- Grant `packages: write` permission only to the image publish job.
- Image URL: `ghcr.io/<owner>/<repo>` or `ghcr.io/<owner>/<image-name>`.

### Docker Hub (`docker.io`)

- Add `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` as repository secrets.
- Remove the `packages: write` permission from the job.
- Image URL: `docker.io/<namespace>/<image>`.

## Deployment State Update Rules

These rules apply only when the configured deployment model has deployment state to update. With `deploymentModel: none`, omit deployment-state update jobs.

- Prefer digest pinning (`image.digest`) over tag pinning for production deploys when the chart and platform support it.
- Keep the `image.tag` field for human readability alongside the digest.
- Do not overwrite unrelated environment values in the automation step.
- If a deployment state repo is private, use a fine-grained PAT or GitHub App credential scoped to Contents: Read and write on that repo only.
- For Kubernetes, bump the chart `version` field when chart templates in `charts/<app>/` change, not on every image update.
- Operators may manually run the GitOps deploy workflow with a previously published `v<version>` image tag to redeploy or roll forward without rebuilding an image.

## Required Secrets and Permissions

| Secret | Required for |
|--------|-------------|
| `GITHUB_TOKEN` | GHCR push (automatic) and release tag creation |
| `DOCKERHUB_USERNAME` | Docker Hub push |
| `DOCKERHUB_TOKEN` | Docker Hub push |
| `GITOPS_TOKEN` | External GitOps repo write access |

## README Badge

Add a badge for the deployment workflow:

```markdown
[![Deploy](https://github.com/OWNER/REPO/actions/workflows/deploy.yml/badge.svg)](https://github.com/OWNER/REPO/actions/workflows/deploy.yml)
```

## Important Notes

- Never create unprefixed Git release tags. Normalize action outputs to `version=<major>.<minor>.<patch>` and `release_tag=v<major>.<minor>.<patch>`.
- Check out the release tag, not the branch head, before building the Docker image.
- Run tests and build verification before pushing the image.
- Do not cancel in-progress release, image publish, or GitOps update jobs.
- Do not push mutable `latest` tags by default; always include the `v<version>` tag and SHA tag.
- Use `docker/setup-buildx-action` and `cache-from: type=gha` to speed up repeated builds.
- The deployment state update job should be omitted when the deployment model does not use an external state repository.
- When present, the deployment state update job should be a no-op when there are no changes, to avoid empty commits.
- If multiple environments exist, make the target environment explicit in workflow inputs or use separate workflows.

## When to Apply

- When a web application is deployed from a container image or platform-native app artifact.
- When `deploymentModel` is `kubernetes` and Argo CD deploys from a GitOps repository.
- When the team wants versioned image releases with auditable deployment-state changes.

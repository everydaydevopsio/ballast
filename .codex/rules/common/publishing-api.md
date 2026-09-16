<!-- ballast:rule id="typescript/publishing/api" version="5.18.3" checksum="afdde1e9231f897e28b4b11255de73be540b5880105183f534ef90e978431723" -->
# REST API Publishing Agent

## Goals

- Use the same container publishing and deployment model as web apps.
- Ensure the API exposes health and readiness endpoints that the configured runtime can use for rollout safety.
- Scope Kubernetes probes and Helm chart templates to repositories with `deploymentModel: kubernetes`.
- Scope registry-only image publishing to repositories with `deploymentModel: docker`.
- Distinguish private (GHCR) vs public (Docker Hub) image publishing based on the API's audience.

## Activation

No app deployment model is configured (`deploymentModel: none`). Deployment guidance is reference-only. Deployment is inactive: keep library, SDK, CLI, and optional container publishing guidance active, but do not create deploy-on-main workflows, deployment-state updates, Kubernetes, serverless, hosted-platform, Docker registry, or self-managed server deployment ownership until the repository sets an active `deploymentModel`.

## Release Model

REST API Docker publishing uses the same versioned release model as web app Docker publishing. For Kubernetes, use the web app publishing rule's quality, `bump_and_tag`, image publish, and GitOps deploy workflow. The API-specific differences are health endpoint requirements and any deployment-model-specific runtime configuration.

## Docker Publish Workflow

Use the same Kubernetes `deploy.yml` and `gitops-deploy.yml` templates as the web app publishing rule when this repository publishes an API Docker image:

- Pull requests run quality checks only.
- Pushes to `main` and `workflow_dispatch` runs create only `v`-prefixed Git tags such as `v1.8.0`; never create unprefixed tags.
- Use `concurrency: cancel-in-progress: false` for release, image publish, and GitOps update workflows so an in-flight publish can complete.
- Check out `refs/tags/v<version>` before building the image.
- Build and push the Docker image tagged with `v<version>`, optional semver aliases, and the git SHA.
- Add `latest` only when the team explicitly wants a mutable tag.
- Update the GitOps repository only after the tagged image is pushed, and include the image digest when the chart supports digest pinning.

Name the workflow file `deploy-api.yml` (or keep `deploy.yml` if there is only one service).

If `deploymentModel` is `docker`, publish the API image to GHCR or Docker Hub and expose the digest, but do not add Kubernetes, SSH, systemd, hosted-platform, or serverless deployment-state jobs unless the repository already owns that runtime layer.

If `deploymentModel` is `none`, do not add deployment-state update jobs unless the user explicitly asks to introduce API deployment ownership. Container publishing may still be valid for installable or local runtime images, but deployment-state updates are inactive.

## Health Endpoint Requirements

### Recommended Endpoints

- `/health` or `/healthz` (liveness): return `200 OK` while the process is up; non-2xx only when the process is broken and should be restarted.
- `/ready` or `/readyz` (readiness): return `200 OK` only when all critical dependencies (DB, cache, downstream services) are reachable; `503` during startup or dependency outage.

Separate liveness and readiness checks when the runtime supports both. In Kubernetes, a liveness failure triggers a pod restart and a readiness failure removes the pod from service without restarting it. In hosted, serverless, or server models, map these endpoints to the platform's health check and traffic cutover controls.

## Private vs Public Image Registries

- Internal/org-only APIs: GHCR (`ghcr.io`) with the automatic `GITHUB_TOKEN`; grant `packages: write` to the build job.
- Public/open-source APIs: Docker Hub (`docker.io`) with `DOCKERHUB_USERNAME` + `DOCKERHUB_TOKEN` secrets (no `packages: write`).
- External deployment-state or GitOps repo writes need a scoped `DEPLOYMENT_STATE_REPO_TOKEN`.

## README Badge

```markdown
[![Deploy API](https://github.com/OWNER/REPO/actions/workflows/deploy-api.yml/badge.svg)](https://github.com/OWNER/REPO/actions/workflows/deploy-api.yml)
```

## Important Notes

- Liveness and readiness have different semantics — do not reuse one handler for both unless the runtime supports only a single health check; readiness covers critical dependencies, liveness only process health.
- Never create unprefixed Git release tags. Normalize action outputs to `version=<major>.<minor>.<patch>` and `release_tag=v<major>.<minor>.<patch>`.

## When to Apply

- When a REST API service is deployed from a container image or platform-native service artifact.
- When `deploymentModel` is `kubernetes` and Argo CD deploys the API from a GitOps repository.
- When the API needs health and readiness checks for safe runtime lifecycle management.

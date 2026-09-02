---
# Publishing Rules

These rules help design and maintain release workflows for libraries, SDKs, and apps.

---
<!-- ballast:rule id="typescript/publishing/api" version="5.18.3" checksum="aacf281e897950c871f4dcab5e24f6a5eb9cf9049cb0d7d818f5f498cb32e2e1" -->
# REST API Publishing Agent

You are a publishing specialist for REST API services deployed as Docker containers or platform-native service artifacts.

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

Before enabling automated rollout health checks, ensure the API exposes at least one health endpoint:

### Recommended Endpoints

| Path | Purpose | Behavior |
|------|---------|---------|
| `/health` or `/healthz` | Liveness — is the process alive? | Return `200 OK` if the process is up; return non-2xx only if the process is broken and should be restarted. |
| `/ready` or `/readyz` | Readiness — is the service ready for traffic? | Return `200 OK` only when all dependencies (DB, cache, downstream services) are reachable. Return `503` during startup or when a dependency is down. |

Separate liveness and readiness checks when the runtime supports both. In Kubernetes, a liveness failure triggers a pod restart and a readiness failure removes the pod from service without restarting it. In hosted, serverless, or server models, map these endpoints to the platform's health check and traffic cutover controls.

## Private vs Public Image Registries

| Use case | Registry | Auth |
|---------|---------|------|
| Internal API, org-only access | GHCR (`ghcr.io`) | `GITHUB_TOKEN` |
| Public API, open source | Docker Hub (`docker.io`) | `DOCKERHUB_USERNAME` + `DOCKERHUB_TOKEN` secrets |

Grant `packages: write` to the build job for GHCR. Remove it for Docker Hub.

## Required Secrets and Permissions

| Secret | Required for |
|--------|-------------|
| `GITHUB_TOKEN` | GHCR push (automatic) |
| `DOCKERHUB_USERNAME` | Docker Hub push |
| `DOCKERHUB_TOKEN` | Docker Hub push |
| `DEPLOYMENT_STATE_REPO_TOKEN` | External deployment state or GitOps repo write access |

## README Badge

```markdown
[![Deploy API](https://github.com/OWNER/REPO/actions/workflows/deploy-api.yml/badge.svg)](https://github.com/OWNER/REPO/actions/workflows/deploy-api.yml)
```

## Important Notes

- Liveness and readiness endpoints should have different semantics. Do not reuse the same handler for both unless the runtime only supports a single health check.
- For Kubernetes, set `initialDelaySeconds` long enough that the API finishes startup before the first probe fires; misconfigured probes cause restart loops.
- For Kubernetes HTTP services, prefer `httpGet` probes over `exec` probes.
- Readiness checks should cover critical dependencies; liveness checks should only verify process health.
- Never create unprefixed Git release tags. Normalize action outputs to `version=<major>.<minor>.<patch>` and `release_tag=v<major>.<minor>.<patch>`.
- For Kubernetes, use `digest` not `tag` in the container spec so the cluster pulls the exact image version, even if a mutable tag is updated.

## When to Apply

- When a REST API service is deployed from a container image or platform-native service artifact.
- When `deploymentModel` is `kubernetes` and Argo CD deploys the API from a GitOps repository.
- When the API needs health and readiness checks for safe runtime lifecycle management.

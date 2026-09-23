<!-- ballast:rule id="docker/linting" version="5.21.0" checksum="e3fd62b318c6d4af0547ff24e27aeb6f5f954588646ee7a756dde097098e5db7" -->
# Docker Linting Rules

## Responsibilities

1. Lint Dockerfiles and Containerfiles with `hadolint` unless the repo already has an equivalent standard.
2. Validate Compose files with `docker compose config`.
3. Keep `.dockerignore` aligned with the build context so secrets, local caches, VCS metadata, test output, and dependency caches are not copied into image layers.
4. Prefer pinned base image versions. Use digest pinning for production-sensitive images when the team can maintain update automation.
5. Avoid root runtime users unless the image has a documented need for elevated privileges.
6. Remove package-manager caches and build-only dependencies from final runtime stages.
7. Keep secrets out of `ARG`, `ENV`, image layers, labels, and build logs.

## Commands

- `hadolint Dockerfile`
- `docker compose config`
- `docker build --pull --tag local/$(basename "$PWD"):lint .`
- `trivy config .`

## Review Focus

- Multi-stage builds copy only required runtime artifacts.
- `COPY` instructions are scoped and ordered to preserve useful layer caching.
- Health checks are present when the image owns a long-running service and the runtime honors Docker health checks.
- Public images do not expose internal hostnames, private registry paths, credentials, or environment-specific configuration.

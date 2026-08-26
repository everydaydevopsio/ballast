# `docker-registry-publish`

The `docker-registry-publish` skill sets up Docker image publishing to GHCR or Docker Hub, with public or private visibility.

## Install It

Examples:

```bash
pnpm exec ballast-typescript install --target codex --skill docker-registry-publish
ballast-go install --target claude --skill docker-registry-publish
ballast install --target opencode --skill docker-registry-publish --yes
```

Use it for Dockerfile or Containerfile repositories that need registry workflows, image tags, digest output, scoped credentials, and pull-request-safe publish behavior.

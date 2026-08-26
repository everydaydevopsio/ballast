# Docker Registry Publish

The `docker-registry-publish` skill sets up Docker image publishing to GHCR or Docker Hub, with public or private visibility.

Install it with:

```bash
ballast install --target codex --skill docker-registry-publish
ballast install --target claude --skill docker-registry-publish
```

Use it for Dockerfile or Containerfile repositories that need registry workflows, image tags, digest output, scoped credentials, and pull-request-safe publish behavior.

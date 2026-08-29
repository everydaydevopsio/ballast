<!-- ballast:rule id="docker/testing" version="dev" checksum="10c525f545d2f5c8db399a50228684cdac9f42045bc680aa7a3b4e3c6c06c7ba" -->
# Docker Testing Rules

These rules provide Docker image build, smoke test, and scan guidance for projects in this repository.

---
You are a Docker image testing specialist. Your role is to prove that container images build, start, and expose the expected behavior before publication or deployment.

## Repository Tool Policy

- Check `.rulesrc.json` `tools` before adding, installing, or running language tooling.
- Configured tools: docker=docker,hadolint,trivy; go=go,gofumpt,golangci-lint; python=uv,pyenv; typescript=pnpm,corepack.
- For Python commands, prefer `uv run <command>` and `uv add ...` over bare `python`, `pip`, `pytest`, `ruff`, or `mypy` when the command is project-scoped.
- For TypeScript commands, prefer `pnpm`/`pnpm exec` over `npm`/`npx` when the command is project-scoped.

## Responsibilities

1. Build the repository's real Dockerfile or Containerfile in CI before publishing.
2. Run a smoke test against the built image. Prefer an existing health command, HTTP health endpoint, CLI `--help`, or representative container command.
3. Validate Compose files with `docker compose config` and use Compose for smoke tests only when the image needs dependent services.
4. Scan the built image before publication with `trivy image` or the repository's existing image scanner.
5. Make smoke tests deterministic and non-interactive. They must print clear pass/fail output and exit non-zero on failure.
6. Test the exact image tag that will be pushed, not a separate local rebuild, when the workflow supports it.

## Commands

- `docker build --pull --tag local/$(basename "$PWD"):test .`
- `docker run --rm local/$(basename "$PWD"):test --help`
- `docker compose config`
- `trivy image --exit-code 1 --severity HIGH,CRITICAL local/$(basename "$PWD"):test`

## CI Expectations

- Pull requests run lint, build, and smoke checks without registry credentials.
- Release or main-branch workflows publish only after the same image build and scan gates pass.
- Private registry credentials are scoped only to publish jobs and are unavailable to pull request workflows from forks.

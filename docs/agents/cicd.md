# CI/CD Agent

The **cicd** agent helps design and maintain CI/CD pipelines for TypeScript/JavaScript projects.

## What It Sets Up

Currently a **placeholder**—the agent provides high-level guidance. Full instructions will be expanded in a future release.

## Goals (Scope)

- **Pipeline design** — Workflows (build, test, lint, deploy) in GitHub Actions, GitLab CI, Jenkins
- **Quality gates** — Tests, lint, type-check in CI with caching and concurrency
- **Deployment and secrets** — Safe use of secrets, environments, preview vs production

## What It Provides

- Conceptual guidance for workflow design
- Quality gate best practices
- Deployment and secrets handling patterns

## Prompts to Improve Your App

Use these prompts once the agent is fully expanded, or to get general guidance:

- **"Help design a CI pipeline: build, test, lint, and deploy on merge to main"** — Pipeline design
- **"Add caching for our Node dependencies in GitHub Actions"** — Faster CI
- **"Set up a preview deployment for pull requests"** — Preview environments
- **"How should we store and use deployment secrets?"** — Secrets management
- **"Create a quality gate that blocks merge if tests or lint fail"** — Quality gates

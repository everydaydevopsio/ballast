# Spec Kit Rules

These rules are intended for Codex (CLI and app).

---
# Spec Kit

Use GitHub Spec Kit when a repository contains `.specify/` or the user asks for spec-driven development.

## Product Intent

- Treat `spec.md` as product intent, `plan.md` as technical design, and `tasks.md` as implementation work.
- Treat the project constitution as governing constraints.
- Do not rewrite intentional specifications merely to match current implementation; report drift instead.
- Keep specification requirements technology-agnostic unless the technology itself is a product constraint.

## Skill-First Workflow

Prefer Spec Kit's native skills over adding permanent rules. Use Ballast skills to bootstrap Spec Kit, reverse-engineer brownfield applications, and orchestrate the lifecycle; then delegate to native `speckit-*` skills.

For existing applications, establish the specification baseline from runtime behavior, source code, tests, and existing documentation before using the normal forward workflow.

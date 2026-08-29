# Spec Kit Agent

The **spec-kit** agent adds common GitHub Spec Kit guidance for repositories that use `.specify/` and `specs/`.

## What It Sets Up

- Treats `spec.md` as product intent, `plan.md` as technical design, and `tasks.md` as implementation work.
- Keeps project constitution constraints separate from feature requirements.
- Instructs agents to report implementation/spec drift instead of rewriting intentional specs to match current code.
- Prefers Ballast's `speckit-*` skills and native GitHub Spec Kit skills over permanent repo rules for the full workflow.
- Scopes Cursor rule activation to `.specify/**` and `specs/**`.

## Install

```bash
ballast install --target codex --agent spec-kit
ballast install --target claude --agent spec-kit
ballast install --target cursor --agent spec-kit
```

It is also included by `--all`.

## When To Use It

Use `spec-kit` when a repository already contains `.specify/` or when the team is adopting GitHub Spec Kit for spec-driven development.

For brownfield applications, establish the baseline first with `speckit-bootstrap` and `speckit-reverse-engineer`. For new product changes after the baseline exists, use `speckit-delivery`.

The source of truth for this agent is `agents/common/spec-kit/content.md`.

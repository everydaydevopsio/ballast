# Spec Kit Reverse Engineer

Use `speckit-reverse-engineer` to create a GitHub Spec Kit baseline for an existing application before normal forward development begins.

## Install

```bash
ballast install --target codex --skill speckit-reverse-engineer
ballast install --target claude --skill speckit-reverse-engineer
```

It is also included by `--all-skills`.

## What It Covers

- Running `speckit-bootstrap` first when `.specify/` or native `speckit-*` skills are missing.
- Building a capability map from runtime behavior, source, tests, and existing documentation.
- Treating current implementation as evidence, not automatically desired product intent.
- Separating high-confidence behavior from assumptions, contradictions, and open questions.
- Creating feature specifications with native Spec Kit skills instead of reimplementing the Spec Kit workflow inside Ballast.
- Optionally creating `specs/BASELINE.md` as a non-normative map for larger products.

## Evidence Order

1. Existing intentional specifications and explicit product decisions.
2. Observable runtime behavior.
3. E2E and smoke tests.
4. Integration tests.
5. Unit tests.
6. Source code and configuration.
7. Structural inference.

The source of truth for this skill is `skills/common/speckit-reverse-engineer/SKILL.md`.

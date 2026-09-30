# Ballast Constitution

Cross-cutting constraints that hold for every feature. Feature requirements belong in
`specs/`, not here.

Each principle below is here because violating it has already caused a defect in this
repository. The referenced ADR or issue is the evidence.

## Core Principles

### I. Four backends or none (NON-NEGOTIABLE)

Install behaviour lives in four independent places: `packages/ballast-typescript/src/`,
`packages/ballast-go/cmd/ballast-go/main.go`, `packages/ballast-python/ballast/cli.py`, and
the wrapper `cli/ballast/main.go`. Any change to a destination path, emitted content, patch
semantics, manifest wording, or legacy-path cleanup lands in all four, or in none.

Which backend runs depends on the repository's languages. A TypeScript-only change passes
every TypeScript test while leaving Go, Python, Ansible, Terraform, Dart and Docker
repositories broken — so "the tests pass" is not evidence of parity.

Where a concern genuinely belongs to one backend, say so explicitly and show why.

### II. Generated output is never the source

Checked-in `.claude/` and `.codex/` output is generated. It is changed by editing
`agents/` or `skills/` and regenerating — never by hand. A hand edit becomes drift on the
next audit and is silently lost on the next refresh.

Repo-root content has packaged copies under `packages/*`. Editing one without syncing the
others leaves the published package correct and the repository's own artifacts stale, or
the reverse. Local development resolves to the repo root and hides the gap.

### III. What the tool reports must match what is on disk (ADR-002)

A report is built from what was done, not recomputed from what should have happened. An
index never points at a file that is not there. Where a failure cannot be reported
honestly mid-flight, the operation refuses to start rather than half-applying.

A check that nothing runs is not a check.

### IV. Always-on context is budgeted (ADR-003)

Rules load on every turn in every managed repository. No emitted rule exceeds 5 KB, and no
target exceeds its configured payload budget; both are enforced in CI, not by review.

Procedural and reference-heavy material belongs in skills, which load on demand. Content
that configuration has switched off is not emitted at all — rendering it behind a disabled
banner still costs its full body.

### V. Test-first for behaviour (NON-NEGOTIABLE)

Bug fixes, features, contract changes, and refactors with behavioural impact start from a
test that fails **for the expected reason**, and the failure is confirmed before
implementing. Failure paths — errors, edge cases, misuse — are covered, not only the happy
path.

A claim that something is fixed names the test that failed before and passes now. "This
should be fine now" is not evidence, and a confidently-worded finding is not a verified
one.

### VI. Never ship a reference consumers cannot resolve

Install writes rules, skills and support files. It does not write `docs/`. A rule pointing
at a path install does not create is a dead reference in every consuming repository
(#363), and a manifest indexing a pruned rule leaves agents loading nothing at all.

Anything a rule tells an agent to read must be something the agent receives.

## Compatibility and Safety

- Managed content is regenerated wholesale for skills and merged under `--patch` for rules,
  so user-authored rule sections survive an upgrade and stale skill text does not.
- Destructive operations require a recoverable starting point. `upgrade` refuses on a dirty
  Ballast-managed tree or outside git, with `--force` as the explicit opt-out.
- Support files carrying hand-written sections are never silently overwritten in
  non-interactive mode.
- Secrets never appear in rules, skills, generated output, logs, or committed config.

## Development Workflow

`docs/development-process.md` is the sequence; `docs/code_review.md` is the review bar.
Each artifact owns one kind of truth (ADR-004): product intent in `spec.md`, technical
design in `plan.md`, branch evidence in `tasks/todo.md`, durable follow-up in GitHub
Issues, and decisions that outlive their change in `adr/`.

The process is scaled to the change. A single-file fix does not need a spec. The two rules
that never relax are Principle V and documentation changing alongside behaviour.

## Governance

This constitution constrains every feature spec and plan. Where a spec conflicts with it,
the constitution wins until it is amended.

Amendments require: the change, the reason, and the evidence — normally an ADR or a defect
that the current wording permitted. Principles are added when something has gone wrong,
not pre-emptively; a constitution nobody can violate is decoration.

Compliance is checked at review. Complexity that a principle forbids must be justified in
writing or removed.

**Version**: 1.0.0 | **Ratified**: 2026-09-29 | **Last Amended**: 2026-09-29

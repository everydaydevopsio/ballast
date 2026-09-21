# Architecture Decision Records

Accepted decisions that shape how Ballast generates, scopes, and maintains agent rules.

Plans in [`plans/`](../plans/README.md) capture work in progress; an ADR records the decision once
that work lands. ADRs are never deleted — a reversed decision is marked superseded and a new ADR
takes its place.

| ADR                                                     | Status   | Decision                                                                                                                  |
| ------------------------------------------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------- |
| [001](001-scope-generated-rules-to-repository-shape.md) | Accepted | Generated state must be internally consistent, and rule scope follows the repository's shape rather than a fixed default. |

## Conventions

- Filenames are `NNN-<decision-title>.md` with sequential zero-padded numbering, one decision per
  ADR.
- Status is `Accepted`, `Deprecated`, or `Superseded`.
- Each ADR records Context, Decision, Alternatives Considered, Consequences (positive **and**
  negative), Implementation Notes, Verification, and Lessons Learned.
- Take the next number from this table when graduating a plan.

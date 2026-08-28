---
name: speckit-reverse-engineer
description: >
  Reverse-engineer an existing application into a high-level GitHub Spec Kit
  baseline using the running app, source code, tests, and existing docs. Use
  for brownfield adoption before normal spec-driven development begins.
---

<!-- Created by [Ballast](https://github.com/everydaydevopsio/ballast) v5.18.0. Do not edit this section. -->

# Spec Kit Reverse Engineer

Create the specification baseline that reasonably could have existed before the current implementation.

## Preconditions

- Run `speckit-bootstrap` first when `.specify/` or native `speckit-*` skills are missing.
- For web applications, use the Pilot MCP server when available to inspect the running application.
- Ensure source code and unit, integration, E2E, and smoke tests are accessible.

## Evidence Rules

Treat evidence in this order: existing intentional specifications and explicit product decisions; observable runtime behavior; E2E and smoke tests; integration tests; unit tests; source code and configuration; structural inference.

Current implementation is evidence, not automatically desired product intent.

## Phase 1: Build a Capability Map

Inspect repository structure, routes, domain modules, entities, authentication, authorization, integrations, jobs, tests, and existing docs. Then explore the running application breadth-first with Pilot. Identify primary actors, navigation, major workflows, terminology, permissions, lifecycle transitions, important business rules, and significant failure states.

Do not crawl every link or create one capability per route. Prefer capability boundaries such as Authentication, Organization Management, Customer Management, Billing, Reporting, and External Integrations rather than page or component boundaries.

## Phase 2: Corroborate with Tests and Source

Review tests in this order when available: E2E, smoke, integration, unit. Group test cases by underlying product intent. Use source to discover hidden rules, permissions, integrations, background behavior, and state transitions.

Classify findings internally as High confidence, Medium confidence, or Low confidence. Do not silently turn low-confidence behavior into requirements.

## Phase 3: Propose Feature Boundaries

Before writing specifications, present the product summary, primary actors, proposed capability map, proposed Spec Kit feature boundaries, confidence findings, assumptions, contradictions, and open product questions. For interactive work, get user approval before creating the baseline.

## Phase 4: Create the Baseline with Native Spec Kit Skills

Use native Spec Kit skills and the repository's installed templates. Do not reimplement `speckit-specify` inside this skill.

Create one feature specification per coherent product capability, not per page, route, endpoint, component, database table, or test.

A specification should focus on user stories and actors, acceptance scenarios, functional requirements, key domain entities when relevant, edge cases, assumptions, and technology-independent success criteria.

Do not generate `plan.md` or `tasks.md` for the whole existing application. Those belong to bounded future change cycles.

## Optional Baseline Index

For non-trivial products, create `specs/BASELINE.md` as a non-normative map containing the product summary, primary actors, capability map, links to feature specs, confidence and assumptions, cross-cutting product rules, open questions, and implementation/spec mismatches.

Keep feature requirements in their individual `spec.md` files.

## Evidence Traceability

When useful, place volatile implementation evidence in `evidence.md` beside a feature specification. Include observed routes/workflows, relevant source areas, relevant tests, confidence, and known mismatches. Do not place transient file paths inside normative functional requirements.

## Brownfield Completion

The baseline is complete when major user capabilities and business rules are represented and ambiguous or conflicting behavior is surfaced rather than guessed.

After baseline adoption, stop routinely reverse-engineering the application. Intentional specs become the product contract and future changes should use the normal Spec Kit lifecycle through `speckit-delivery`.

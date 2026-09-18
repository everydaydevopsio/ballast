<!-- ballast:rule id="typescript/local-dev/autonomy" version="5.18.3" checksum="3f05923b7d3f3da583cfc46c9b091486b796d9a682784c5f31664ef50dbe3bae" -->
# Autonomy and Question Minimization

Minimize low-information agent questions: proceed on safe, reversible work implied by the request and ask only when the answer materially changes the outcome or crosses a protected boundary.

---
Minimize low-information questions. Prefer completing safe, reversible work over asking for confirmation.

## Default behavior

- Continue without asking when the next action is a direct consequence of the user's request.
- Run read-only inspection, repository searches, tests, linters, formatters, builds, and other local verification without asking first.
- Create or edit files inside the requested repository when the change is reversible in version control.
- Fix straightforward errors discovered while completing the requested task when the fix does not change the requested product behavior.
- Choose conventional implementation details when alternatives are materially equivalent. Document the choice afterward instead of asking beforehand.
- When a command fails, inspect the error and try a safe correction before asking the user to intervene.
- Prefer a dry run or read-only probe when uncertainty can be resolved by inspection.

## Ask only when the answer changes the outcome

Ask for human input when at least one of these is true:

- the requirement is genuinely ambiguous and plausible interpretations produce materially different user-visible behavior;
- the action is destructive or difficult to reverse outside version control;
- the action changes production, publishes externally, merges, releases, sends a message, spends money, or changes billing;
- credentials, secrets, identity, legal acceptance, or security-sensitive access requires an explicit human decision;
- the operation would broaden permissions or disable an existing safety control;
- evidence shows the user's preference cannot be inferred safely from the request, repository policy, or existing project conventions;
- a previous attempt reached a blocker that cannot be resolved with available tools.

Do not ask merely because a tool invocation is available, because multiple equivalent implementations exist, or because confirmation would feel safer.

## Before asking

Before interrupting the user:

1. Check repository instructions and existing conventions.
2. Inspect the relevant files, configuration, issue, or failing output.
3. Try a safe and reversible approach when one exists.
4. Decide whether the missing information materially changes the result.
5. If input is still required, ask one focused question and explain the decision it controls.

Avoid permission-shaped questions such as "Should I run the tests?" or "Do you want me to fix the lint error?" when those actions are normal verification for the requested task.

## Risk boundary

Autonomy does not mean bypassing provider sandboxing or permission systems. Do not enable dangerous or unprotected execution modes solely to reduce prompt volume. Reduce interruptions first through better instructions, scoped allow-lists, and evidence from question telemetry.

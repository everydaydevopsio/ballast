TDD is required for bug fixes, new features, refactors with behavioral impact, and contract changes:

1. Start from acceptance criteria in `PRD.md`, the linked issue, or the current task.
2. Write a failing test first that proves the requirement is not yet met.
3. Confirm the test fails for the right reason before implementation.
4. Implement the minimum change needed to make the failing test pass.
5. Refactor only after the relevant tests are green.
6. Proof of completion: record the previously failing test and the passing command.
7. Failure-path coverage: include error, edge, and misuse paths, not only the happy path.
8. Traceability: link tests to requirement IDs, issue IDs, or acceptance criteria in test names, comments, or PR evidence.

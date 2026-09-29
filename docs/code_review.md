# Code Review

The review policy this repository follows, and that `AGENTS.md` points agent reviewers at.

Reviews exist to catch defects, not to relitigate style. Formatting is settled by the
formatter and linter; if a review comment could have been a lint rule, it should be one.

## What a review is looking for

In priority order:

1. **Correctness** — does it do what it claims, including on the failure paths?
2. **Security** — secrets, injection, permission and credential scope, dependency risk.
3. **Regressions** — behaviour other code or downstream repositories already rely on.
4. **Missing tests** — especially for the failure paths, not only the happy one.
5. **Maintainability** — will the next person be able to change this safely?

## Severity

| Level | Use when | Blocks merge |
| --- | --- | --- |
| P0 | Concrete failure mode with user or data impact | Yes |
| P1 | Credible risk, or a missing test on behaviour that matters | Yes |
| P2 | Real but bounded; fine to follow up under an issue | No |
| P3 / Nit | Polish and preference | No |

Rules for applying it:

- Flag P0/P1 only when there is a **concrete failure mode or credible risk**. "This could
  be cleaner" is not a risk.
- Treat missing tests as **P1** when the change touches behaviour, auth, billing,
  persistence, migrations, concurrency, permissions, or user-visible output.
- Treat documentation gaps as **P1** only when the change alters setup, public APIs,
  release or deploy steps, or user-visible behaviour.
- Never block merge on personal preference.

## Evidence, not assertion

A review finding should be reproducible by the author from the comment alone. State the
failing input or state, and what goes wrong — not just that something looks wrong. The
same standard applies to the author: a claim that something is fixed should name the test
that failed before and passes now.

This repository has been bitten specifically by *plausible but unverified* claims. See
[ADR-002](../adr/002-trust-ballasts-own-feedback-channels.md): a check that reports success
without running, or a log that names files it did not write, is the failure mode to watch
for. When a review or a fix rests on "this should be fine now", ask for the evidence.

## Gates before requesting review

- The smallest relevant test, lint, or type-check command has been run, and passes.
- Commit-time hooks ran — a change committed with `--no-verify` is not review-ready
  without saying so and stating what was run instead.
- Behaviour changes carry a test that failed before the change.
- Docs changed alongside behaviour, CLI, configuration, architecture, or workflow changes.
- For multi-backend changes, **all four install surfaces** were updated together — see the
  multi-backend note in `CLAUDE.md`. A TypeScript-only change passes every TypeScript test
  while leaving Go, Python and the wrapper broken.

## Agent reviewers

Agent-assisted review runs through the `github-pr-copilot-cycle` skill, which requests
review, triages the comments, applies the actionable ones, and repeats for up to three
cycles. See [skills/github-pr-copilot-cycle.md](skills/github-pr-copilot-cycle.md).

Agent review supplements human judgement. Treat its findings as claims to verify, with the
same evidence bar as any other reviewer — a confidently-worded finding is not a verified
one.

## Scope discipline

- Review the change that was proposed, not the change you would have made.
- Out-of-scope problems found during review become issues, not review blockers.
- A large diff is not automatically a problem, and a small diff is not automatically safe.

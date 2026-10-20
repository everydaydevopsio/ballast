# ADR-005: The code review agent is repository policy, resolved at run time

Status: Accepted
Date: 2026-10-20
Branch: `code-review-agent-selection`
PR: [#402](https://github.com/everydaydevopsio/ballast/pull/402)
Supersedes: none

## Context

Ballast shipped one PR-review skill, `github-pr-copilot-cycle`, and it hard-coded GitHub
Copilot. Every step assumed Copilot specifically: `gh pr edit --add-reviewer "@copilot"`,
matching on `copilot-pull-request-reviewer[bot]`, and a settle poll tuned to Copilot's
roughly ten-minute turnaround. A repository that wanted a different reviewer had no
supported path, and "never use Copilot" could not be expressed as repository policy.

Two reviewers needed to work: Copilot, and Bosun (`everydaydevopsio/bosun`), a self-hosted
reviewer authenticating as a GitHub App. Teams also needed to plug in a reviewer Ballast
has never heard of.

Two constraints shaped the design. First, `.rulesrc.json` is rebuilt key-by-key from a
known set in `writeRulesConfig`, so any key Ballast does not explicitly know is dropped on
the next install — a skill cannot simply read an unregistered key. Second,
`buildSkillDirectoryMarkdown` takes only a skill ID: skills are copied verbatim with no
variable or conditional-token substitution, unlike agent rules.

## Decision

Split the decision in two.

**Policy lives in `.rulesrc.json`** as a flat `codeReviewer` key, validated and plumbed
exactly like `taskSystem` and `deploymentModel`: a `--code-reviewer` flag, an interactive
prompt when the review skill is selected, `ballast doctor` reporting, and persistence
across reinstalls. Values are `copilot` (default), `bosun`, `none`, and `skill:<skill-name>`
for a reviewer the repository implements itself.

**Dispatch happens at run time inside the skill.** `github-pr-review-cycle` reads
`codeReviewer` when it runs and drives the matching backend. The cycle, the 0–3 scoring
rubric, the three-cycle limit, and the stop-on-score-3 rule are reviewer-independent; only
how a review is requested, how settling is detected, and how findings are read back vary.

`github-pr-copilot-cycle` is deprecated since 5.22.0 and removed in 6.0.0, with its content
left working.

## Alternatives Considered

**Template the reviewer into the skill at build time.** Rejected: it would require threading
variables into skill rendering across four backends and changing every call site, and it
would force a regenerate-and-commit cycle to change reviewer.

**A separate committed config file such as `.github/code-review.json`.** Rejected: no
backend changes, but it splits repository policy across two files and stays invisible to
`ballast doctor`.

**Rename `github-pr-copilot-cycle` in place.** Rejected: a published skill ID is part of the
install contract, so renaming fails validation for every repository still listing it.

**A nested `codeReview: { reviewer, ... }` object.** Rejected for now: every other policy
knob in `.rulesrc.json` is a flat scalar, and a flat string keeps validation and
four-backend plumbing simple. Revisit if per-reviewer options are needed.

## Consequences

**Positive.** A repository can avoid Copilot entirely. Default behavior is unchanged for
every existing repository. Switching reviewers takes effect immediately — no reinstall, no
regeneration — because resolution is at run time. A user-defined reviewer needs no Ballast
change. The reviewer choice is visible in `ballast doctor` alongside the other policy knobs.

**Negative.** The reviewer is no longer discoverable by reading the generated skill alone;
you must also read `.rulesrc.json`. Four backends must stay in parity on a new key, which is
a standing maintenance cost. The deprecated skill's content is duplicated until 6.0.0.

## Implementation Notes

Three defects reached a fully green test suite and were caught only by end-to-end installs:

- **Python's `install()` was called positionally.** Inserting `code_reviewer` before
  `refresh_task_rules` shifted the argument and silently disabled task-rule refresh. An
  existing test caught it; the call site now passes keywords.
- **The Go backend dropped the value at four points** — two `rulesConfig` construction
  paths, the `saveConfig` preservation branch, and the `loadConfig` conversion from
  `rawRulesConfig`.
- **The wrapper's own config handling runs only for true monorepos.** Single-language
  repositories return `nil, nil` from `resolveMonorepoPlan` (`cli/ballast/main.go:3066`) and
  forward args verbatim, so that path depends entirely on backend-side persistence.

## Verification

TypeScript 393 tests, Python 136, both Go suites; `pnpm lint`, `gofmt`, and `go vet` clean;
`repo-generated-artifacts` comparing checked-in `.claude/`/`.codex/` against freshly
generated output. Per-backend installs into scratch repositories confirmed fresh-install
persistence, preservation across a reinstall that omits the flag, rejection of an invalid
value with exit 1, and the rendered deprecation warning.

## Review

Three Bosun review cycles ran against the branch before it was opened, using this change's
own `codeReviewer: bosun` path. Seven findings, all real: five fixed in the PR, two filed as
issues ([#401](https://github.com/everydaydevopsio/ballast/issues/401), and
[bosun#53](https://github.com/everydaydevopsio/bosun/issues/53) for the review surface).

Two are worth recording because they are design errors rather than slips:

- **The skill defaulted to Copilot when it could not read the config.** The resolver swallowed
  every error and substituted `copilot`, so a missing parser or malformed file turned "this
  repository must not use Copilot" into a Copilot review request — defeating the feature's
  entire purpose at exactly the moment it mattered. A repository that cannot be read is not a
  repository that chose Copilot.
- **The Bosun poll discarded the evidence the next paragraph required.** The query returned
  only a comment's first line, while the instructions told the agent to compare the stamped
  commit against `HEAD_OID` — data the command had already thrown away. An instruction and
  the command meant to satisfy it drifted apart inside one document.

## Lessons Learned

A green four-backend test suite does not prove four-backend parity. Each backend owns its
own config read, write, preservation, and validation code, and unit tests tend to cover the
function under test rather than the path an install actually takes. The cheap check that
would have caught all three defects immediately: install into a scratch repository with each
backend and read back the resulting `.rulesrc.json`.

The corollary for the install contract: adding a parameter to a function called positionally
anywhere is a silent behavior change, not an additive one.

Two further lessons from the review cycles:

**A default is a policy decision, and it must not be reachable from a failure path.** Three of
the seven findings had the same shape — an error swallowed, a flag unrecognized, a value
unparsed — and in each the fallback was `copilot`. Defaults are safe when they mean "nothing
was specified" and dangerous when they also mean "we could not tell". Separate those cases
explicitly, and prefer stopping over guessing whenever the guess would override an expressed
intent.

**A skill is executable prose, and it drifts like code.** The polling query and the paragraph
telling the agent what to do with its output were correct in isolation and contradictory
together. Prose carries no type checker, so commands embedded in a skill need the same scrutiny
as the instructions around them — ideally by checking that each instruction's required inputs
are actually produced by the command above it.

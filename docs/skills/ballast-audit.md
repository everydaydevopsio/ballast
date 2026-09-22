# Ballast Audit Skill Guide

This skill audits what Ballast has actually installed into a repository, against two questions:

1. **Is it current?** Does the checked-in output match what the installed Ballast version generates?
2. **Is it needed?** Does every emitted rule and skill correspond to something this repository really does?

Rules load on every turn, so an out-of-date or irrelevant rule is a permanent tax on every session in every target.

`ballast-audit` is installed by default. Every install adds it, even when no skills are selected, because an installation that has drifted cannot be detected by the rules it emits.

## When to Use

- After upgrading Ballast across several versions, especially on a repository first installed by an older one.
- When a target's rules directory is noticeably larger than another target's built from the same config.
- When the agent feels slow, hits context limits, or follows guidance that does not match the repository.
- After changing `languages`, `paths`, `deploymentModel`, or `publishingProfiles` in `.rulesrc.json`.
- Before adding a language profile, to confirm the existing set is still correct.

## What It Checks

**Ownership.** Every file Ballast generates starts with an ownership marker:

```
<!-- ballast:rule id="typescript/cicd" version="5.19.1" checksum="0d00ce..." -->
```

Files without that marker are `unowned`: Ballast will neither refresh nor prune them, so they persist across every upgrade. The wrapper `ballast doctor` does not print rule-file status — the language backend does, and the skill runs it directly.

**Baseline drift.** The skill clean-installs the repository's own `.rulesrc.json` into a scratch copy and diffs. Files only in the repository are orphans the current version no longer emits; files that differ are stale or hand-edited. This is the only check that catches unowned drift.

Note that a refresh does not necessarily repair what the diff finds. `ballast install --refresh-config` skips files that already exist, and `--refresh-config --patch` can report installing every agent while leaving unowned files byte-for-byte unchanged. The remediation for unowned files is to delete the rules directory and reinstall:

```bash
rm -rf .codex/rules && ballast install --refresh-config --yes
```

**Fit.** Each emitted rule and skill is checked against evidence in the repository: `publishing-cli` against a `bin` entry or `.goreleaser.yaml`, `spec-kit` and the `speckit-*` skills against `.specify/`, the `aws-*` skills against AWS configuration, `docker-registry-publish` against a `Dockerfile`. Absence of evidence is the finding, reported with the rule's byte cost.

Two config-level checks matter most here: leaving `publishingProfiles` unset emits every publishing variant except `apt` and `brew`, and `deploymentModel: none` suppresses the `web` and `api` variants but not `publishing-apps`.

**Density.** Within what survives, the skill flags rules over 5120 bytes, placeholder rules with no actionable instruction, duplication within a single target's rules directory, and rule sets that differ between targets built from the same config.

## Output

A report giving always-on cost per target before and after, findings ordered by byte cost, and remediation commands in dependency order — reinstall, then narrow the config, then trim. It distinguishes *stale* (a refresh fixes it), *unowned* (only deletion and reinstall fixes it), and *irrelevant* (a config change fixes it).

## Remediation Boundary

Do not hand-edit generated rule or skill files to resolve a finding. Ballast-managed output is regenerated from `agents/` and `skills/` in the Ballast repository; a hand edit shows up as `drifted` on the next audit and is lost on the next refresh. Fix the config, reinstall, or raise the change upstream.

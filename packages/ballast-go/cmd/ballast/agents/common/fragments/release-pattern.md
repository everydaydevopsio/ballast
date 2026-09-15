Model release workflows on the Ballast `publish.yml` pattern:

1. Trigger on `workflow_dispatch` with a required `release_type` choice input of `patch`, `minor`, or `major` (and on release tags when the project publishes from `refs/tags/v*`).
2. Add a `bump_and_tag` job, gated to `if: github.event_name == 'workflow_dispatch'` so tag-triggered runs never re-bump, that reads the previous tag with `WyriHaximus/github-action-get-previous-tag@v2`, computes next versions with `WyriHaximus/github-action-next-semvers`, selects the version for the chosen `release_type`, updates version files, commits the bump, and creates and pushes the `v<version>` tag.
3. Expose the computed version as a job output; publish jobs must check out the release tag, never the branch head — `refs/tags/v<version>` from the bump job's output on `workflow_dispatch` runs, or `github.ref` (the pushed `v*` tag) on tag-triggered runs where the bump job is skipped.
4. Add a workflow-level `concurrency` block with `group: ${{ github.workflow }}-${{ github.ref }}` and `cancel-in-progress: false` so an in-flight publish is never cancelled mid-run.
5. Validate before publishing: check out the tagged ref, install dependencies, run build and tests.
6. Keep publish jobs separate per language or distribution target, each with only the permissions it needs.

Version and tag rules:

- Use semantic versioning with `v`-prefixed tags such as `v1.8.0`; never create unprefixed release tags.
- The published artifact version must equal the tag version without the `v` prefix.
- Create the tag first, then publish from that tag; publishing steps must be idempotent or fail safely on duplicate versions.
- Changelog or release notes must exist for the version, and build and tests must pass before publish.

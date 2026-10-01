<!-- ballast:rule id="typescript/publishing" version="5.21.3" checksum="674aa495013ae71cb2c28fda66eebf2c91b9f2ed17a7d019f5a3d89aab7647b2" -->
# Publishing Rules

Shared release pattern for every publishing variant in this repository. The `publishing-<variant>` rules add artifact-specific requirements on top of this pattern.

## Release Workflow Pattern

Model release workflows on the Ballast `publish.yml` pattern:

1. Trigger on `workflow_dispatch` with a required `release_type` choice input of `patch`, `minor`, or `major` (and on release tags when the project publishes from `refs/tags/v*`).
2. Add a `bump_and_tag` job, gated to `if: github.event_name == 'workflow_dispatch'` so tag-triggered runs never re-bump, that reads the previous tag with `WyriHaximus/github-action-get-previous-tag@v2`, computes next versions with `WyriHaximus/github-action-next-semvers`, selects the version for the chosen `release_type`, updates version files, commits the bump, and creates and pushes the `v<version>` tag.
3. Expose the computed version as a job output; publish jobs must check out the release tag, never the branch head — `refs/tags/v<version>` from the bump job's output on `workflow_dispatch` runs, or `github.ref` (the pushed `v*` tag) on tag-triggered runs where the bump job is skipped. Because the bump job can be skipped, dependent publish jobs need a condition such as `if: always() && !failure() && !cancelled()` (GitHub otherwise skips jobs whose required job was skipped), and the tag path must exclude the bot-generated tag push from the bump job so a dispatch run does not publish the same tag twice.
4. Add a workflow-level `concurrency` block with `group: ${{ github.workflow }}-${{ github.ref }}` and `cancel-in-progress: false` so an in-flight publish is never cancelled mid-run.
5. Validate before publishing: check out the tagged ref, install dependencies, run build and tests.
6. Keep publish jobs separate per language or distribution target, each with only the permissions it needs.

Version and tag rules:

- Use semantic versioning with `v`-prefixed tags such as `v1.8.0`; never create unprefixed release tags.
- The published artifact version must equal the tag version without the `v` prefix.
- Create the tag first, then publish from that tag; publishing steps must be idempotent or fail safely on duplicate versions.
- Changelog or release notes must exist for the version, and build and tests must pass before publish.

## Recovering A Release That Failed After The Tag

`bump_and_tag` is the point of no return: everything after it publishes outward. Classify the failure before retrying.

| Failure | Example | Recovery |
| --- | --- | --- |
| Before any outward write | signing or notarization gated ahead of upload | Re-run the failed job. |
| After a partial outward write | assets uploaded or an index updated, then a step failed | Roll forward with a new patch release. |
| After a complete write | registry rejects a duplicate version | Nothing to do. |

The middle case is the trap: re-running a job that already published fails on its own artifacts — GitHub rejects duplicate asset names with `422 already_exists`, and indexes already advertise checksums for what is being replaced. Default to rolling forward; a burned version number is cheaper than a mutated one. Deleting artifacts to retry the same version leaves the release missing assets while indexes still point at them; reserve it for a version consumers cannot move off.

Prevent it instead:

- Order every verification that can fail — signing, notarization, attestation, scanning — ahead of the first upload, so a failure aborts the release instead of publishing partial artifacts.
- Let a re-run overwrite its own artifacts where supported (GoReleaser: `release.mode: replace`).
- Workflows read config from the checked-out tag, so neither fix helps an already-tagged version.

## Registry Publishing

Per-registry publish guidance:

- **npmjs (TypeScript/Node)**: require `package.json` with `name`, `version`, `license`, `repository`, and correct `files`/`exports` (plus `bin` and `engines` for CLIs). Use npm trusted publishing via GitHub Actions OIDC (`id-token: write`), `actions/setup-node` with the registry URL, lockfile installs, build before tests when tests need compiled output, and `npm publish --access public --provenance`.
- **PyPI (Python)**: prefer PyPI trusted publishing via OIDC over long-lived tokens. Use `actions/setup-python` (and `astral-sh/setup-uv` when the project uses `uv`), build both wheel and sdist, run tests, and publish with `uv publish` or `pypa/gh-action-pypi-publish`. Grant `id-token: write` only to the publish job. Ensure `pyproject.toml` has complete metadata, supported Python versions, and classifiers.
- **Go (GitHub)**: publish by tagging the module and creating a matching GitHub release — no registry upload step. Check out the tag with full history, run `go test ./...`, verify the build, and create release notes for the tag. Preserve import-path stability and semantic import versioning for `v2+` modules.

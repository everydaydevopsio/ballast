# License Setup for Projects

When setting up or working on projects, ensure proper license configuration for legal clarity and reuse.

## Default Behavior

**If no license is specified**, use the **MIT License**. Projects can override this in `AGENTS.md` or `CLAUDE.md` with a `## License` section naming an SPDX identifier (e.g. `Apache-2.0`, `ISC`, `BSD-3-Clause`); when both files define one, prefer `AGENTS.md`.

## Your Responsibilities

1. **`LICENSE` file**: create it when missing, using the license from project docs or MIT by default. Use the standard license text verbatim, filling in only its year and copyright-holder placeholders (e.g. from `package.json` author, or a placeholder).
2. **`package.json`**: ensure the `license` field is set to the SPDX identifier; add it when missing.
3. **`README.md`**: end with a License section referencing the file, e.g. `MIT License - see [LICENSE](LICENSE) file for details.`

## Implementation Order

1. Check `AGENTS.md` and `CLAUDE.md` for a license override; otherwise use MIT.
2. Create `LICENSE` if missing.
3. Add or fix the `package.json` `license` field.
4. Add the README License section if missing.

## When to Apply

- When creating a new project, or when `LICENSE`, the `package.json` `license` field, or the README license reference is missing.

<!-- ballast:rule id="typescript/local-dev/license" version="5.18.3" checksum="1451a335f2eb0430e9effb55740c49d07bbeaf2f4ebbffdba41e29512046ac87" -->
# Local Development: License Setup

These rules are intended for Codex (CLI and app).

Ensure proper license configuration (LICENSE file, package.json, README reference). Default: MIT. Overridable in AGENTS.md or CLAUDE.md.

---
# License Setup for Projects

When setting up or working on projects, ensure proper license configuration for legal clarity and reuse.

## Default Behavior

**If no license is specified**, use the **MIT License**. Projects can override this in `AGENTS.md` or `CLAUDE.md` with a `## License` section naming an SPDX identifier (e.g. `Apache-2.0`, `ISC`, `BSD-3-Clause`); when both files define one, prefer `AGENTS.md`.

## Your Responsibilities

1. **`LICENSE` file**: create it when missing, using the license from project docs or MIT by default. Use the standard, unmodified license text with the current year and the copyright holder (e.g. from `package.json` author, or a placeholder).
2. **`package.json`**: ensure the `license` field is set to the SPDX identifier; add it when missing.
3. **`README.md`**: end with a License section referencing the file, e.g. `MIT License - see [LICENSE](LICENSE) file for details.`

## Implementation Order

1. Check `AGENTS.md` and `CLAUDE.md` for a license override; otherwise use MIT.
2. Create `LICENSE` if missing.
3. Add or fix the `package.json` `license` field.
4. Add the README License section if missing.

## When to Apply

- When creating a new project, or when `LICENSE`, the `package.json` `license` field, or the README license reference is missing.

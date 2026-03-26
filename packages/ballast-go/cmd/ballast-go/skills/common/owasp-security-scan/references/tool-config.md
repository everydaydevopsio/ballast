# Tool Configuration Examples

## Semgrep - `.semgrepignore`

```text
# .semgrepignore
node_modules/
vendor/
dist/
build/
.venv/
venv/
migrations/
*_test.go
**/*.test.ts
**/__tests__/
```

## Semgrep - `semgrep.yml` (project config)

```yaml
# .semgrep.yml - run with: semgrep --config .semgrep.yml
rules: []

# To reference built-in rulesets alongside:
# semgrep --config .semgrep.yml --config p/owasp-top-ten .
```

---

## Bandit - `.bandit`

```ini
[bandit]
exclude_dirs = tests,test,.venv,venv,migrations
skips = B101
```

Or as `pyproject.toml`:

```toml
[tool.bandit]
exclude_dirs = ["tests", ".venv", "migrations"]
skips = ["B101"]
```

---

## gosec - `.gosec.conf.json`

```json
{
  "global": {
    "nosec": "enabled",
    "audit": "enabled"
  },
  "G104": {
    "ErrorFunctions": {
      "fmt": ["Fprintln", "Fprintf", "Fprintf"]
    }
  }
}
```

Run with config:

```bash
gosec -conf .gosec.conf.json ./...
```

Common rules to explicitly enable:
- **G101** - Hardcoded credentials
- **G202** - SQL string concat
- **G304** - File path from variable (open/read)
- **G401/G501** - Weak hash (MD5)
- **G402** - TLS InsecureSkipVerify
- **G404** - weak random

---

## pip-audit - `pyproject.toml`

```toml
[tool.pip-audit]
require-hashes = false
ignore-vulns = ["GHSA-xxxx-xxxx-xxxx"]
```

---

## npm audit - `.npmrc`

```ini
audit-level=moderate
```

---

## gitleaks - `.gitleaks.toml`

```toml
[extend]
useDefault = true

[[rules]]
id = "custom-internal-token"
description = "Internal API token"
regex = '''INTERNAL_[A-Z0-9]{32}'''
tags = ["api", "internal"]

[allowlist]
description = "Global allowlist"
paths = [
  '''.gitleaks.toml''',
  '''tests/fixtures/.*'''
]
regexes = [
  '''EXAMPLE_KEY_[A-Z0-9]+'''
]
```

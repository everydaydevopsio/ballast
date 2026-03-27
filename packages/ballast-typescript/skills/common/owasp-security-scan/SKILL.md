---
name: owasp-security-scan
description: >
  Run OWASP-aligned security scans across Go, TypeScript, and Python codebases.
  Use this skill whenever the user asks to: scan for security vulnerabilities,
  run OWASP checks, audit dependencies, find CVEs, check for injection flaws,
  run SAST or SCA analysis, review code security, or harden their app against
  the OWASP Top 10. Also trigger for phrases like "security audit", "check my
  code for vulns", "are my dependencies safe", or any mention of gosec, bandit,
  semgrep, or npm audit in a security context. Covers Go, TypeScript/JavaScript,
  and Python with language-specific tools plus cross-language Semgrep rulesets.
---

# OWASP Security Scan Skill

Runs a layered, OWASP Top 10-aligned security audit across Go, TypeScript, and Python projects using best-in-class open-source tools. Produces a consolidated findings report with severity, OWASP category, and remediation guidance.

---

## Scan Architecture

| Layer   | Purpose                     | Tools                                                               |
| ------- | --------------------------- | ------------------------------------------------------------------- |
| SAST    | Static code analysis        | Semgrep (all langs), gosec (Go), Bandit (Python)                    |
| SCA     | Dependency CVEs             | `go mod` + govulncheck (Go), `npm audit` (TS), `pip-audit` (Python) |
| Secrets | Hardcoded credentials       | gitleaks                                                            |
| IaC     | Dockerfile/config misconfig | Semgrep IaC rules                                                   |

---

## Step 1 - Detect Project Languages

```bash
# Detect what's present
[ -f go.mod ] && echo "GO" || true
[ -f package.json ] && echo "TYPESCRIPT" || true
[ -f requirements.txt ] || [ -f pyproject.toml ] || [ -f Pipfile ] && echo "PYTHON" || true
find . -name "*.go" | head -1
find . -name "*.ts" -not -path "*/node_modules/*" | head -1
find . -name "*.py" | head -1
```

Run scans only for detected languages. Skip gracefully with a note if a language isn't present.

---

## Step 2 - Install Tools (check first, install only if missing)

```bash
# Semgrep (cross-language, required for all projects)
command -v semgrep || pip install semgrep --break-system-packages

# Go tools
command -v gosec     || go install github.com/securego/gosec/v2/cmd/gosec@latest
command -v govulncheck || go install golang.org/x/vuln/cmd/govulncheck@latest

# Python tools
command -v bandit    || pip install bandit --break-system-packages
command -v pip-audit || pip install pip-audit --break-system-packages

# Secrets scanning
command -v gitleaks  || (curl -sSfL https://raw.githubusercontent.com/gitleaks/gitleaks/main/scripts/install.sh | sh -s -- -b /usr/local/bin 2>/dev/null || brew install gitleaks 2>/dev/null || echo "gitleaks not installed - skipping secrets scan")
```

---

## Step 3 - Run Scans

### Go

```bash
# SAST - gosec maps to OWASP Top 10
gosec -fmt json -out gosec-results.json ./... 2>/dev/null || gosec -fmt json ./... > gosec-results.json 2>&1

# Dependency CVEs
govulncheck ./... 2>&1 | tee govulncheck-results.txt

# Semgrep - Go-specific OWASP rules
semgrep --config "p/golang" --config "p/owasp-top-ten" \
  --json --output semgrep-go.json \
  --exclude "vendor/" --exclude "*_test.go" \
  . 2>/dev/null
```

### TypeScript / JavaScript

```bash
# Dependency CVEs
npm audit --json > npm-audit.json 2>/dev/null || true

# Semgrep - TS/JS OWASP rules
semgrep --config "p/javascript" --config "p/typescript" \
  --config "p/owasp-top-ten" --config "p/nodejs" \
  --json --output semgrep-ts.json \
  --exclude "node_modules/" --exclude "dist/" --exclude "build/" \
  . 2>/dev/null
```

### Python

```bash
# SAST - Bandit maps findings to CWE/OWASP
bandit -r . -f json -o bandit-results.json \
  --exclude ".venv,venv,tests,test" 2>/dev/null || \
bandit -r . -f json --exclude ".venv,venv" > bandit-results.json 2>&1

# Dependency CVEs
pip-audit --format json --output pip-audit.json 2>/dev/null || \
pip-audit > pip-audit.txt 2>&1

# Semgrep - Python OWASP rules
semgrep --config "p/python" --config "p/owasp-top-ten" \
  --json --output semgrep-py.json \
  --exclude ".venv" --exclude "venv" --exclude "tests" \
  . 2>/dev/null
```

### Secrets (all languages)

```bash
gitleaks detect --source . --report-format json \
  --report-path gitleaks-results.json --no-git 2>/dev/null || \
gitleaks detect --source . --report-format json \
  --report-path gitleaks-results.json 2>/dev/null || true
```

---

## Step 4 - Parse & Consolidate Results

Read each output file and build a unified finding list. For each finding, extract:

- **Tool** (gosec / bandit / semgrep / npm audit / govulncheck / gitleaks)
- **Severity** (Critical / High / Medium / Low / Info)
- **OWASP Category** - map using `references/owasp-mapping.md`
- **File + Line**
- **Description**
- **Remediation** - use `references/remediation-guide.md` for standard fixes

Severity mapping from tool scores:

- gosec: HIGH->High, MEDIUM->Medium, LOW->Low
- bandit: HIGH->High, MEDIUM->Medium, LOW->Low (confidence also shown)
- semgrep: ERROR->High, WARNING->Medium, INFO->Low
- npm audit: critical->Critical, high->High, moderate->Medium, low->Low
- govulncheck: all findings -> High (known exploitable CVEs)
- gitleaks: all findings -> Critical

---

## Step 5 - Generate Report

Present findings in this structure:

```text
## Security Scan Report
**Date**: <date>
**Project**: <detected name>
**Languages Scanned**: Go | TypeScript | Python

---
### Summary
| Severity  | Count |
|-----------|-------|
| Critical  | X     |
| High      | X     |
| Medium    | X     |
| Low       | X     |

---
### Critical & High Findings

For each finding:
**[SEVERITY] OWASP Category - Short Description**
- File: `path/to/file.go:42`
- Tool: gosec (G304)
- Detail: <what the issue is>
- Fix: <concrete remediation - see references/remediation-guide.md>

---
### Dependency Vulnerabilities
<list CVEs with affected package, version, fix version>

---
### Secrets Detected
<file, type of secret, line - NEVER print the actual secret value>

---
### Medium / Low Findings
<condensed table: File | Issue | Tool | Fix Link>

---
### Recommended Next Steps
1. <prioritized action items>
```

---

## Step 6 - Post-Scan Guidance

After presenting the report:

1. **Offer to fix** Critical/High findings directly in code (one at a time, with confirmation)
2. **Offer to generate** a GitHub Actions workflow - see `references/ci-workflow.md`
3. **Note false positives**: Ask if any findings seem incorrect; suppression comments can be added (`// #nosec G304`, `# nosec`, `// eslint-disable-line`)

---

## Reference Files

Load these on-demand as needed:

| File                              | Load When                                            |
| --------------------------------- | ---------------------------------------------------- |
| `references/owasp-mapping.md`     | Mapping tool codes -> OWASP Top 10 categories        |
| `references/remediation-guide.md` | Standard fix patterns per vulnerability type         |
| `references/ci-workflow.md`       | GitHub Actions / CI pipeline templates               |
| `references/tool-config.md`       | `.semgrepignore`, `.bandit`, `gosec` config examples |

---

## Edge Cases

- **Monorepo**: Detect sub-directories per language; run scans from each root
- **No lock file**: Note that SCA scan may be incomplete; recommend `go mod tidy`, `npm install`, or `pip freeze`
- **Large codebase**: Add `--max-target-bytes 1000000` to semgrep; warn user scans may take time
- **Tool install fails**: Skip that tool, note it in report, suggest Docker alternative (see `references/ci-workflow.md`)
- **No findings**: Explicitly confirm "No issues found" - don't silently omit sections

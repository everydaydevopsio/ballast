# CI Pipeline Templates

## GitHub Actions - Full Multi-Language OWASP Scan

Save as `.github/workflows/security-scan.yml`:

```yaml
name: OWASP Security Scan

on:
  push:
    branches: [main, develop]
  pull_request:
  schedule:
    - cron: '0 6 * * 1'

jobs:
  security-scan:
    name: OWASP Security Scan
    runs-on: ubuntu-latest
    permissions:
      contents: read
      security-events: write

    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        if: hashFiles('go.mod') != ''
        uses: actions/setup-go@v5
        with:
          go-version: 'stable'

      - name: gosec (Go SAST)
        if: hashFiles('go.mod') != ''
        run: |
          go install github.com/securego/gosec/v2/cmd/gosec@latest
          gosec -fmt sarif -out gosec.sarif ./... || true

      - name: govulncheck (Go SCA)
        if: hashFiles('go.mod') != ''
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck ./... 2>&1 | tee govulncheck.txt || true

      - name: Upload gosec SARIF
        if: hashFiles('go.mod') != '' && hashFiles('gosec.sarif') != ''
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: gosec.sarif
          category: gosec

      - name: Set up Node
        if: hashFiles('package.json') != ''
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: npm install (needed for audit)
        if: hashFiles('package.json') != ''
        run: npm ci --prefer-offline || npm install

      - name: npm audit (TS/JS SCA)
        if: hashFiles('package.json') != ''
        run: npm audit --audit-level=moderate --json > npm-audit.json || true

      - name: Set up Python
        if: hashFiles('requirements.txt') != '' || hashFiles('pyproject.toml') != ''
        uses: actions/setup-python@v5
        with:
          python-version: '3.12'

      - name: bandit (Python SAST)
        if: hashFiles('requirements.txt') != '' || hashFiles('pyproject.toml') != ''
        run: |
          pip install bandit
          bandit -r . -f sarif -o bandit.sarif \
            --exclude ".venv,venv,tests,migrations" || true

      - name: pip-audit (Python SCA)
        if: hashFiles('requirements.txt') != '' || hashFiles('pyproject.toml') != ''
        run: |
          pip install pip-audit
          pip-audit --format json --output pip-audit.json || true

      - name: Upload bandit SARIF
        if: hashFiles('bandit.sarif') != ''
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: bandit.sarif
          category: bandit

      - name: Semgrep OWASP scan
        uses: semgrep/semgrep-action@v1
        with:
          config: >-
            p/owasp-top-ten
            p/security-audit
            p/golang
            p/python
            p/javascript
            p/typescript
        env:
          SEMGREP_APP_TOKEN: ${{ secrets.SEMGREP_APP_TOKEN }}

      - name: gitleaks
        uses: gitleaks/gitleaks-action@v2
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Upload scan results
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: security-scan-results
          path: |
            gosec.sarif
            bandit.sarif
            govulncheck.txt
            npm-audit.json
            pip-audit.json
          retention-days: 30
```

---

## Minimal PR Gate (fail on High+)

```yaml
name: Security Gate

on: [pull_request]

jobs:
  gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Semgrep
        run: |
          pip install semgrep
          semgrep --config "p/owasp-top-ten" --config "p/security-audit" \
            --severity ERROR --error \
            --exclude "node_modules,vendor,.venv" \
            .
```

---

## Docker-Based Scan (no tool install required)

Run locally without installing anything:

```bash
docker run --rm -v "$(pwd):/src" \
  returntocorp/semgrep semgrep \
  --config "p/owasp-top-ten" \
  --config "p/security-audit" \
  --json --output /src/semgrep-results.json \
  /src

docker run --rm -v "$(pwd):/app" \
  securego/gosec:latest /go/bin/gosec -fmt json /app/...

docker run --rm -v "$(pwd):/path" \
  zricethezav/gitleaks:latest detect --source /path --report-format json
```

# OWASP Top 10 (2021) Tool Mapping Reference

## A01 - Broken Access Control

**gosec**: G401, G402, G501, G502
**bandit**: B105, B106, B107 (hardcoded passwords used for access), B601, B602
**semgrep**: `insecure-direct-object-reference`, `missing-authorization`, `path-traversal`
**Keywords**: path traversal, IDOR, missing auth check, privilege escalation

## A02 - Cryptographic Failures

**gosec**: G401 (weak hash MD5/SHA1), G402 (TLS with InsecureSkipVerify), G403 (RSA < 2048), G404 (weak rand), G501 (MD5), G502 (SHA1)
**bandit**: B303 (MD5/SHA1), B304 (ciphers), B305 (ECB mode), B324 (MD5/SHA1 hashlib)
**semgrep**: `insecure-hash-algorithm`, `hardcoded-secret`, `weak-tls`
**Keywords**: MD5, SHA1, DES, ECB, InsecureSkipVerify, hardcoded key

## A03 - Injection

**gosec**: G202 (SQL string concat), G203 (template injection), G204 (exec with variable)
**bandit**: B608 (SQL injection), B601 (shell injection), B602 (subprocess shell=True), B605, B606
**semgrep**: `sql-injection`, `command-injection`, `xss`, `template-injection`, `nosql-injection`
**npm audit**: injection-related CVEs
**Keywords**: exec, eval, raw SQL, f-string in query, shell=True, dangerouslySetInnerHTML

## A04 - Insecure Design

**semgrep**: `mass-assignment`, `insecure-default`, `missing-rate-limiting`
**Keywords**: mass assignment, missing input validation, no rate limiting

## A05 - Security Misconfiguration

**gosec**: G108 (pprof exposed), G112 (ReadHeaderTimeout), G114 (HTTP without TLS)
**bandit**: B201 (Flask debug=True), B104 (bind all interfaces 0.0.0.0), B105/B106 (hardcoded passwords)
**semgrep**: `debug-enabled`, `cors-wildcard`, `permissive-cors`
**Keywords**: debug=True, 0.0.0.0, CORS \*, permissive headers, pprof endpoint

## A06 - Vulnerable and Outdated Components

**govulncheck**: all findings (known CVE in Go module)
**npm audit**: all findings
**pip-audit**: all findings
**Keywords**: CVE, known vulnerability, outdated dependency, GHSA

## A07 - Identification and Authentication Failures

**gosec**: G401, G402
**bandit**: B105, B106, B107 (password strings), B301 (pickle - can deserialize to RCE)
**semgrep**: `jwt-none-algorithm`, `weak-password-requirements`, `insecure-cookie`
**Keywords**: hardcoded password, JWT none alg, missing httpOnly, weak session

## A08 - Software and Data Integrity Failures

**bandit**: B301 (pickle), B302 (marshal), B403 (import pickle)
**semgrep**: `deserialization`, `unsafe-yaml-load`, `pickle-usage`
**Keywords**: pickle, marshal, yaml.load (not safe_load), deserialization

## A09 - Security Logging and Monitoring Failures

**semgrep**: `missing-error-logging`, `sensitive-data-in-logs`
**bandit**: B110 (try/except/pass - silences errors)
**Keywords**: bare except, logging passwords/tokens, missing audit log

## A10 - Server-Side Request Forgery (SSRF)

**gosec**: G107 (url from variable in HTTP request)
**bandit**: B310 (urllib with variable URL)
**semgrep**: `ssrf`, `unvalidated-redirect`
**Keywords**: http.Get(userInput), requests.get(url), unvalidated external URL

---

## Severity -> OWASP Risk Level

| Tool Severity  | OWASP Risk                        |
| -------------- | --------------------------------- |
| Critical (npm) | A06 CVEs with CVSS >= 9.0         |
| High           | Typically A01, A02, A03, A07, A10 |
| Medium         | Typically A04, A05, A08, A09      |
| Low / Info     | Configuration suggestions         |

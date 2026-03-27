# Remediation Guide

Standard fix patterns for the most common OWASP findings across Go, TypeScript, and Python.

---

## SQL Injection (A03)

### Go
```go
// Vulnerable
db.Query("SELECT * FROM users WHERE id = " + userID)

// Fixed - parameterized query
db.Query("SELECT * FROM users WHERE id = ?", userID)
```

### TypeScript (node-postgres / Prisma)
```ts
// Vulnerable
client.query(`SELECT * FROM users WHERE id = ${userId}`)

// Fixed
client.query('SELECT * FROM users WHERE id = $1', [userId])
```

### Python
```python
# Vulnerable
cursor.execute(f"SELECT * FROM users WHERE id = {user_id}")

# Fixed
cursor.execute("SELECT * FROM users WHERE id = %s", (user_id,))
```

---

## Command Injection (A03)

### Go
```go
// Vulnerable
exec.Command("sh", "-c", "ls " + userInput).Run()

// Fixed - avoid shell; pass args as slice
exec.Command("ls", userInput).Run()
```

### TypeScript
```ts
// Vulnerable
const { exec } = require('child_process')
exec(`ls ${userInput}`)

// Fixed
const { execFile } = require('child_process')
execFile('ls', [userInput])
```

### Python
```python
# Vulnerable
subprocess.run(f"ls {user_input}", shell=True)

# Fixed
subprocess.run(["ls", user_input], shell=False)
```

---

## Hardcoded Secrets (A02 / A07)

### All languages
```text
# Never hardcode credentials
API_KEY = "sk-prod-abc123..."

# Use environment variables
import os
API_KEY = os.environ["API_KEY"]
```

For Go:
```go
apiKey := os.Getenv("API_KEY")
if apiKey == "" {
    log.Fatal("API_KEY not set")
}
```

---

## Weak Cryptography (A02)

### Go
```go
// Vulnerable - MD5
h := md5.New()

// Fixed - SHA-256 or better
h := sha256.New()

// InsecureSkipVerify
tlsCfg := &tls.Config{InsecureSkipVerify: true}

// Fixed
tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
```

### Python
```python
# Vulnerable
import hashlib
hashlib.md5(data).hexdigest()

# Fixed
hashlib.sha256(data).hexdigest()

# For passwords - never use raw hash
# Use bcrypt
import bcrypt
hashed = bcrypt.hashpw(password.encode(), bcrypt.gensalt())
```

---

## Insecure Deserialization (A08)

### Python
```python
# Vulnerable
import pickle
obj = pickle.loads(user_data)

# yaml.load without Loader
import yaml
data = yaml.load(user_input)

# Fixed
import json
obj = json.loads(user_data)  # prefer JSON

# Safe yaml
data = yaml.safe_load(user_input)
```

---

## Path Traversal (A01)

### Go
```go
// Vulnerable
filePath := filepath.Join("/uploads", userInput)
os.Open(filePath)

// Fixed - validate path stays within base
base := "/uploads"
filePath := filepath.Join(base, userInput)
if !strings.HasPrefix(filepath.Clean(filePath), base) {
    return errors.New("invalid path")
}
```

### TypeScript
```ts
// Vulnerable
const file = path.join('/uploads', req.params.filename)
res.sendFile(file)

// Fixed
const base = path.resolve('/uploads')
const file = path.resolve(base, req.params.filename)
if (!file.startsWith(base + path.sep)) {
  return res.status(400).send('Invalid path')
}
res.sendFile(file)
```

---

## SSRF (A10)

### Go
```go
// Vulnerable
resp, _ := http.Get(userURL)

// Fixed - validate URL is not internal
u, err := url.Parse(userURL)
if err != nil || u.Hostname() == "localhost" || isPrivateIP(u.Hostname()) {
    return errors.New("disallowed URL")
}
```

### Python
```python
# Vulnerable
requests.get(user_url)

# Fixed - allowlist approach
ALLOWED_HOSTS = {"api.example.com", "cdn.example.com"}
parsed = urllib.parse.urlparse(user_url)
if parsed.hostname not in ALLOWED_HOSTS:
    raise ValueError("Disallowed host")
requests.get(user_url)
```

---

## Flask/Express Debug Mode (A05)

### Python (Flask)
```python
# Never in production
app.run(debug=True)

# Fixed
app.run(debug=os.environ.get("FLASK_DEBUG", "false").lower() == "true")
```

### TypeScript (Express)
```ts
// Leaks stack traces
app.use((err, req, res, next) => {
  res.json({ error: err.stack })
})

// Fixed
app.use((err: Error, req: Request, res: Response, next: NextFunction) => {
  console.error(err)
  res.status(500).json({ error: 'Internal server error' })
})
```

---

## Suppression Comments (for confirmed false positives)

```go
// Go - suppress specific gosec rule
result, err := os.Open(path) // #nosec G304

// Go - suppress in semgrep
x := md5.Sum(data) //nolint:gosec // non-security use (content hash for cache key)
```

```python
# Python - suppress bandit
data = subprocess.run(cmd, shell=True)  # nosec B602
```

```yaml
# .semgrepignore - exclude paths from semgrep
tests/
vendor/
migrations/
```

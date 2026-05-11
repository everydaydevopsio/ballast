# Google Workspace CLI (`gws`) Rules
name: gws-rules

This skill provides rules for safe and efficient usage of the Google Workspace CLI extension (`gws`).

---

## Guidelines

### 1. Schema Discovery
If you are unsure about the exact JSON payload structure for a method, ALWAYS run `gws schema` first:
```bash
gws schema <service>.<resource>.<method>
```

### 2. Context Window Protection
Workspace APIs (especially Drive and Gmail) return large JSON blobs. You MUST use field masks to avoid overwhelming the context window:
- Use the `--fields` flag or `--params '{"fields": "..."}'`.
- Example: `gws drive files list --fields "files(id,name,mimeType)"`

### 3. Dry-Run Safety
Always use the `--dry-run` flag for mutating operations (create, update, delete) to validate your JSON payload before actual execution.

### 4. Pagination and NDJSON
Use the `--page-all` flag for listing large collections. The output is Newline Delimited JSON (NDJSON), which is efficient for streaming and processing.

### 5. Sanitization
Use the `--sanitize <TEMPLATE>` flag to filter sensitive output through Google Cloud Model Armor if required by the project's security policy.

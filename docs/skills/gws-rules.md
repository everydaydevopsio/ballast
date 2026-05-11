# GWS Rules Skill Guide

This skill provides rules for safe and efficient usage of the Google Workspace CLI extension (`gws`) with AI agents.

## When to Use

- When using the `gws` CLI to interact with Drive, Gmail, Calendar, or Sheets.
- When you want to ensure the AI agent uses field masks to protect the context window.
- When you want to verify destructive operations with `--dry-run`.

## Guidelines

### 1. Schema Discovery
AI agents should always check the schema before making calls if they are unsure of the payload structure.
```bash
gws schema <service>.<resource>.<method>
```

### 2. Context Window Protection
ALWAYS use field masks.
```bash
# Bad: loads full file metadata
gws drive files list

# Good: loads only what is needed
gws drive files list --fields "files(id,name,mimeType)"
```

### 3. Dry-Run Safety
Use `--dry-run` for all create, update, and delete operations during the research or planning phase.

### 4. Efficient Processing
Use `--page-all` for streaming large results in NDJSON format.

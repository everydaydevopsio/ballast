# Logging Agent

The **logging** agent sets up structured, production-safe logging patterns for TypeScript, Python, Go, Ansible, and Terraform automation.

## What It Sets Up by Language

- **TypeScript/JavaScript**
  - Pino-based structured logging
  - Fluentd-oriented aggregation patterns
  - Browser and API logging guidance for web apps
- **Python**
  - Structured logging with `structlog` or JSON-capable standard logging
  - Environment-aware handlers and levels
- **Go**
  - Structured logging with `log/slog` (or `zerolog` if already adopted)
  - Standardized fields for request and error context
- **Ansible**
  - Callback and task-output guidance for readable execution logs
  - `no_log: true` handling for secrets and sensitive command output
  - Consistent task names so playbook runs are easy to audit
- **Terraform**
  - Reviewable `plan` and `apply` output conventions
  - Sensitive output and variable handling to keep secrets out of logs
  - Guidance for plan artifacts, `TF_LOG`, and environment-safe operator messaging

## What It Provides

- Structured logs that are easy to aggregate and query
- Guidance for environment-based log levels
- Patterns that reduce sensitive-data leakage in logs

## Monorepo Usage

For monorepos, apply a shared logging schema across all services (field names, trace/request IDs, severity mapping) while keeping language-native logging libraries.

Common baseline fields:

- `service`
- `env`
- `request_id`
- `trace_id`
- `user_id` (when available)
- `error_code`

## Prompts to Improve Your App

- **"Standardize logging fields across TypeScript, Python, Go, Ansible, and Terraform automation"** — Cross-language schema
- **"Set log levels so development is verbose and production is noise-controlled"** — Environment policy
- **"Add request/trace propagation to logs in this service"** — Correlation
- **"Review our logs for secrets and high-cardinality fields"** — Safety check
- **"Clean up our Ansible task output so failures are easy to audit"** — Automation visibility
- **"Make Terraform plan and apply logs safer and easier to review"** — Infra reviewability

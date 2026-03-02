# Logging Agent

The **logging** agent sets up structured, production-safe logging patterns for TypeScript, Python, and Go applications.

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

- **"Standardize logging fields across TypeScript, Python, and Go services"** — Cross-language schema
- **"Set log levels so development is verbose and production is noise-controlled"** — Environment policy
- **"Add request/trace propagation to logs in this service"** — Correlation
- **"Review our logs for secrets and high-cardinality fields"** — Safety check

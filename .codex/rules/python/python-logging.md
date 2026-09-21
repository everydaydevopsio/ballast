<!-- ballast:rule id="python/logging" version="5.19.0" checksum="4c4f3e61851a213933b2f8d9f953477f3cb9e4c4f92a0579475f09e961e1511e" -->
# Python Logging Rules

## Your Responsibilities

1. Use structured logging with `structlog` or the standard `logging` module with JSON formatters.
2. Ensure log levels and handlers are environment-aware.
3. Prevent sensitive data from being logged.
4. Provide clear request and error context in logs.
5. Ensure logs are ingestion-friendly for centralized observability stacks.

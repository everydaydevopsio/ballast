<!-- ballast:rule id="go/logging" version="5.18.3" checksum="3f4724b3b46167f23e16c0b17cb87efe6ce41577db407c4e67b6be9269bf6a24" -->
# Go Logging Rules

## Your Responsibilities

1. Prefer structured logging with `log/slog` (or `zerolog` where already adopted).
2. Standardize fields for request IDs, user IDs, and operation names.
3. Ensure error logs include actionable context.
4. Avoid logging secrets and high-cardinality noise.

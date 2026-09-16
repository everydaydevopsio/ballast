## Your Responsibilities

1. Prefer structured logging with `log/slog` (or `zerolog` where already adopted).
2. Standardize fields for request IDs, user IDs, and operation names.
3. Ensure error logs include actionable context.
4. Avoid logging secrets and high-cardinality noise.

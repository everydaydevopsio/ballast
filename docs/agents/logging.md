# Logging Agent

The **logging** agent configures centralized, structured logging for TypeScript/JavaScript applications using Pino and Fluentd.

## What It Sets Up

- **Server-side** — Pino to Fluentd for Node.js apps and Next.js API routes
- **Browser-side** — pino-browser with pino-transmit-http to send logs to `/api/logs`
- **CLI** — Pino for CLI tools with pretty output (dev) and JSON (CI)
- **Error capture** — `window.onerror` and `unhandledrejection` in the browser

## What It Provides

- Structured JSON logs for easier parsing and aggregation
- Centralized log ingestion via Fluentd (Elasticsearch, S3, etc.)
- Browser logs and errors forwarded to your backend
- Configurable log levels (DEBUG in dev, ERROR in prod)

## Prompts to Improve Your App

### Server-Side

- **"Help me set up Pino logging with Fluentd for our Node.js app"** — Full server setup
- **"Create a custom Fluentd transport for Next.js serverless (no piping)"** — Serverless
- **"Add structured logging to our API routes with request ID propagation"** — Request context

### Browser-Side

- **"Wire up pino-browser to post logs to /api/logs"** — Browser logging
- **"Capture window.onerror and unhandledrejection and send to our backend"** — Error capture
- **"Replace console.log with the browser logger in our React components"** — Migration

### Next.js

- **"Create the /api/logs endpoint for App Router"** — API route
- **"Create the /api/logs endpoint for Pages Router"** — API route
- **"Import initBrowserLogging in our root layout"** — Wire-up

### Configuration

- **"Add LOG*LEVEL and FLUENT*\* env vars to .env.example"** — Env vars
- **"Set up minimal Fluentd config to receive logs on port 24224"** — Fluentd
- **"Use DEBUG in development and ERROR in production"** — Log levels

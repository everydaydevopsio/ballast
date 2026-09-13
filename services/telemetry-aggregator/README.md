# Ballast Telemetry Aggregator

Collects question-friction telemetry from `bridgectl` and exposes aggregated data for Ballast feedback workflows.

## API

- `POST /v1/events` — ingest one schema v1 telemetry event
- `GET /v1/summary?days=30` — aggregate events by provider and question fingerprint
- `GET /healthz` — liveness/readiness probe

Set `BALLAST_TELEMETRY_TOKEN` to require `Authorization: Bearer <token>` for ingest and summary endpoints.

## Local Docker

```bash
docker build -t ballast-telemetry .
docker run --rm -p 8080:8080 \
  -e BALLAST_TELEMETRY_TOKEN=change-me \
  -v ballast-telemetry:/data \
  ballast-telemetry
```

The default backend is `local`, writing `/data/telemetry.jsonl`.

## S3 storage

```bash
docker run --rm -p 8080:8080 \
  -e BALLAST_TELEMETRY_STORAGE=s3 \
  -e BALLAST_TELEMETRY_S3_BUCKET=my-ballast-telemetry \
  -e BALLAST_TELEMETRY_S3_PREFIX=ballast/telemetry \
  -e AWS_REGION=us-west-2 \
  -e BALLAST_TELEMETRY_TOKEN=change-me \
  ballast-telemetry
```

The AWS SDK default credential chain is used. On Kubernetes, prefer workload identity / IRSA rather than static access keys.

Each S3 event is immutable and stored under:

```text
<prefix>/YYYY/MM/DD/HH/<event-id>.json
```

This allows multiple aggregator replicas to ingest concurrently without locking a shared file.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `BALLAST_TELEMETRY_LISTEN` | `:8080` | Listen address |
| `BALLAST_TELEMETRY_STORAGE` | `local` | `local` or `s3` |
| `BALLAST_TELEMETRY_LOCAL_PATH` | `/data/telemetry.jsonl` | Local JSONL file |
| `BALLAST_TELEMETRY_S3_BUCKET` | | S3 bucket; required for S3 mode |
| `BALLAST_TELEMETRY_S3_PREFIX` | `ballast/telemetry` | S3 object prefix |
| `BALLAST_TELEMETRY_TOKEN` | | Optional bearer token |

## Event example

```json
{
  "schema_version": 1,
  "timestamp": "2026-09-13T17:00:00Z",
  "provider": "codex",
  "project_id": "orchael/crew",
  "session_id": "session-123",
  "fingerprint": "run-tests-before-completion",
  "question_type": "confirmation",
  "outcome": "accepted",
  "latency_ms": 710,
  "question": "Run the test suite?"
}
```

`bridgectl` should redact sensitive content before sending events. The aggregator intentionally does not accept arbitrary transcript uploads.

## Scaling notes

Local storage is intended for one replica backed by a Docker volume or Kubernetes PVC. S3 storage is the distributed mode and supports multiple replicas safely because each event is written as its own immutable object.

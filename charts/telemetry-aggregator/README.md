# Ballast Telemetry Aggregator Helm Chart

## Local/PVC mode

Use one replica. The chart rejects multiple replicas with local storage because the JSONL file is not a distributed datastore.

```bash
helm upgrade --install ballast-telemetry ./charts/telemetry-aggregator \
  --set storage.backend=local \
  --set storage.local.size=1Gi
```

## Distributed S3 mode

Use S3 when more than one collector needs to ingest telemetry.

```bash
helm upgrade --install ballast-telemetry ./charts/telemetry-aggregator \
  --set replicaCount=3 \
  --set storage.backend=s3 \
  --set storage.s3.bucket=my-ballast-telemetry \
  --set storage.s3.region=us-west-2
```

Use workload identity or IRSA to provide AWS credentials to the pods. Do not place long-lived AWS secrets directly in Helm values.

For API authentication, create a Kubernetes Secret containing a `token` key and set `existingSecret` to its name.

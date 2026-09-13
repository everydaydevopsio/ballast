{{- define "ballast.telemetry.validate" -}}
{{- if and (eq .Values.storage.backend "local") (gt (int .Values.replicaCount) 1) -}}
{{- fail "storage.backend=local supports only replicaCount=1; use storage.backend=s3 for distributed replicas" -}}
{{- end -}}
{{- if and (eq .Values.storage.backend "s3") (not .Values.storage.s3.bucket) -}}
{{- fail "storage.s3.bucket is required when storage.backend=s3" -}}
{{- end -}}
{{- end -}}

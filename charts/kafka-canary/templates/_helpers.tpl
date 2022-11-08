{{/*
Expand the name of the chart.
*/}}
{{- define "kafka-canary.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "kafka-canary.fullname" -}}
kafka-canary
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "kafka-canary.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kafka-canary.labels" -}}
helm.sh/chart: {{ include "kafkarator.chart" . }}
{{ include "kafka-canary.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kafka-canary.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kafka-canary.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "kafka-canary.serviceAccountName" -}}
{{- include "kafka-canary.fullname" . }}
{{- end }}

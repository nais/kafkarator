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
kafka-canary-deployer
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
helm.sh/chart: {{ include "kafka-canary.chart" . }}
{{ include "kafka-canary.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
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
{{- if .Values.serviceAccount.create }}
{{- default (include "kafka-canary.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Shared pod template for the kafka canary deployer. Pass the root context with
a "command" key set (provision|deploy) via (set . "command" "deploy").
*/}}
{{- define "kafka-canary.job-template" -}}
metadata:
  labels:
    {{- include "kafka-canary.selectorLabels" . | nindent 4 }}
spec:
  restartPolicy: Never
  containers:
    - name: {{ include "kafka-canary.fullname" . }}
      image: "{{ .Values.deployer.image.repository }}:{{ .Values.deployer.image.tag }}"
      imagePullPolicy: {{ .Values.deployer.image.pullPolicy }}
      args:
        - {{ .command | quote }}
      env:
        - name: CANARY_IMAGE
          value: "{{ .Values.canary.image.repository }}:{{ .Values.canary.image.tag }}"
        - name: DEPLOY_CONFIGS_PATH
          value: /config/deploy_configs.json
        - name: GRPC_TARGET
          value: "{{ .Values.grpcTarget }}"
        - name: API_BASE_URL
          value: "{{ .Values.apiBaseUrl }}"
      securityContext:
        capabilities:
          drop:
            - ALL
        privileged: false
        readOnlyRootFilesystem: true
        runAsGroup: 1069
        runAsNonRoot: true
        runAsUser: 1069
        allowPrivilegeEscalation: false
        seccompProfile:
          type: RuntimeDefault
      volumeMounts:
        - mountPath: /tmp
          name: tmp
        - mountPath: /config
          name: config
  volumes:
    - name: tmp
      emptyDir:
        medium: Memory
    - name: config
      configMap:
        name: {{ include "kafka-canary.fullname" . }}
{{- end }}

{{/*
Expand the name of the chart.
*/}}
{{- define "kafka-canary.job-template" -}}
metadata:
  labels:
    {{- include "kafka-canary.selectorLabels" . | nindent 12 }}
spec:
  restartPolicy: Never
  containers:
    - name: {{ include "kafka-canary.fullname" . }}
      image: "{{ .Values.deployer.image.repository }}:{{ .Values.deployer.image.tag }}"
      imagePullPolicy: {{ .Values.deployer.image.pullPolicy }}
      envFrom:
        - secretRef:
            name: {{ include "kafka-canary.fullname" . }}
      env:
        - name: IMAGE
          value: "{{ .Values.canary.image.repository }}:{{ .Values.canary.image.tag }}"
        - name: TEAM
          value: "nais-verification"
        - name: TOPIC_BASE
          value: "kafka-canary"
        - name: CLUSTER_POOLS
          value: "{{ .Values.cluster_pools }}"
        - name: TENANT
          value: "{{ .Values.tenant }}"
        - name: ALERT_ENABLED
          value: "{{ .Values.alert_enabled }}"
        - name: DEPLOY_SERVER
          value: "hookd-grpc:9090"
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
  volumes:
    - name: tmp
      emptyDir:
        medium: Memory
{{- end }}

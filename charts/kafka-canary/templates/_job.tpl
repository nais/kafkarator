{{/*
Expand the name of the chart.
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
      env:
        - name: IMAGE
          value: "{{ .Values.canary.image.repository }}:{{ .Values.canary.image.tag }}"
        - name: TEAM
          value: "nais-verification"
        - name: DEPLOY_CONFIGS_PATH
          value: /config/deploy_configs.json

        # Passed directly to deploy-cli
        - name: DEPLOY_SERVER
          value: "{{ .Values.deploy_server }}"
        - name: GRPC_USE_TLS
          value: "{{ .Values.deploy_use_tls }}"
        - name: APIKEY
          valueFrom:
            secretKeyRef:
              key: DEPLOY_API_KEY
              name: {{ .Values.deploy_key_secret_name }}
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

{{/*
Returns a non-empty string when a CA certificate secret must be mounted for
the platform WebSocket connection.
*/}}
{{- define "platformTLS.caEnabled" -}}
{{- if .Values.platformTLS.caCertSecret.name -}}
true
{{- end -}}
{{- end }}

{{/*
Volume definition for the platform CA certificate.
*/}}
{{- define "platformTLS.caVolume" -}}
- name: platform-tls-ca
  secret:
    secretName: {{ .Values.platformTLS.caCertSecret.name }}
{{- end }}

{{/*
Volume mount for the platform CA certificate.
*/}}
{{- define "platformTLS.caVolumeMount" -}}
- name: platform-tls-ca
  mountPath: /etc/platform-tls
  readOnly: true
{{- end }}

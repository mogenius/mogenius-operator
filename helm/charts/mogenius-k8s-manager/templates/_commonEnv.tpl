{{- define "common.env" -}}
- name: STAGE
  value: {{ .Values.global.stage | quote }}
- name: MO_CLUSTER_MFA_ID
  value: "" # the secret is loaded lazily as a kubernetes secret
- name: api_key
  {{- if .Values.global.api_key }}
  value: {{ .Values.global.api_key | quote }}
  {{- else }}
  valueFrom:
    secretKeyRef:
      name: {{ .Values.global.apiKeySecret.secretName }}
      key: {{ .Values.global.apiKeySecret.secretKey }}
  {{- end }}
- name: cluster_name
  value: {{ .Values.global.cluster_name | quote }}
- name: OWN_NAMESPACE
  valueFrom:
    fieldRef:
      apiVersion: v1
      fieldPath: metadata.namespace
- name: OWN_NODE_NAME
  valueFrom:
    fieldRef:
      apiVersion: v1
      fieldPath: spec.nodeName
- name: OWN_POD_NAME
  valueFrom:
    fieldRef:
      apiVersion: v1
      fieldPath: metadata.name
- name: OWN_DEPLOYMENT_NAME
  value: {{ .Values.fullnameOverride | quote }}
{{- if .Values.valkey.enabled }}
- name: MO_VALKEY_ADDR
  value: "{{ .Values.fullnameOverride }}-valkey:{{ .Values.valkey.port }}"
- name: MO_VALKEY_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ .Values.fullnameOverride }}-valkey
      key: valkey-password
{{- end }}
- name: CLUSTER_DOMAIN
  value: {{ .Values.cluster.domain | quote }}
{{- end }}

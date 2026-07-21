{{- define "common.env" -}}
{{-  range $key, $value := .Values.envVars }}
- name: {{ $key }}
  value: {{ $value | quote }}
{{- end }}
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
{{- if and .Values.valkey.enabled .Values.externalKeyValueStore.enabled }}
{{- fail "valkey.enabled and externalKeyValueStore.enabled are mutually exclusive; enable only one" }}
{{- end }}
{{- if .Values.valkey.enabled }}
- name: MO_VALKEY_ADDR
  value: "{{ .Values.fullnameOverride }}-valkey:{{ .Values.valkey.port }}"
- name: MO_VALKEY_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ .Values.fullnameOverride }}-valkey
      key: valkey-password
{{- else if .Values.externalKeyValueStore.enabled }}
{{- if not .Values.externalKeyValueStore.host }}
{{- fail "externalKeyValueStore.enabled is true: you must set externalKeyValueStore.host" }}
{{- end }}
- name: MO_VALKEY_ADDR
  value: "{{ .Values.externalKeyValueStore.host }}:{{ .Values.externalKeyValueStore.port }}"
{{- if .Values.externalKeyValueStore.username }}
- name: MO_VALKEY_USERNAME
  value: {{ .Values.externalKeyValueStore.username | quote }}
{{- end }}
{{- if .Values.externalKeyValueStore.existingSecret.name }}
- name: MO_VALKEY_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ .Values.externalKeyValueStore.existingSecret.name }}
      key: {{ .Values.externalKeyValueStore.existingSecret.key }}
{{- end }}
{{- if .Values.externalKeyValueStore.tls.enabled }}
- name: MO_VALKEY_TLS_ENABLED
  value: "true"
{{- if .Values.externalKeyValueStore.tls.insecureSkipVerify }}
- name: MO_VALKEY_TLS_INSECURE_SKIP_VERIFY
  value: "true"
{{- end }}
{{- if .Values.externalKeyValueStore.tls.caCertSecret.name }}
- name: MO_VALKEY_TLS_CA_CERT_FILE
  value: "/etc/valkey-tls/{{ .Values.externalKeyValueStore.tls.caCertSecret.key }}"
{{- end }}
{{- end }}
{{- else }}
{{- fail "no key-value store configured: enable the bundled valkey (valkey.enabled) or an external store (externalKeyValueStore.enabled)" }}
{{- end }}
- name: CLUSTER_DOMAIN
  value: {{ .Values.cluster.domain | quote }}
- name: GOMEMLIMIT
  value: {{ .Values.goRuntime.memLimit | quote }}
- name: GOGC
  value: {{ .Values.goRuntime.gcPercent | quote }}
- name: GODEBUG
  value: "madvdontneed=1"
{{- end }}

{{/*
Returns a non-empty string when a CA certificate secret must be mounted for the
external key-value store TLS connection.
*/}}
{{- define "externalKvs.caEnabled" -}}
{{- if and .Values.externalKeyValueStore.enabled .Values.externalKeyValueStore.tls.enabled .Values.externalKeyValueStore.tls.caCertSecret.name -}}
true
{{- end -}}
{{- end }}

{{/*
Volume definition for the external key-value store CA certificate.
*/}}
{{- define "externalKvs.caVolume" -}}
- name: valkey-tls-ca
  secret:
    secretName: {{ .Values.externalKeyValueStore.tls.caCertSecret.name }}
{{- end }}

{{/*
Volume mount for the external key-value store CA certificate.
*/}}
{{- define "externalKvs.caVolumeMount" -}}
- name: valkey-tls-ca
  mountPath: /etc/valkey-tls
  readOnly: true
{{- end }}

{{- define "valkey.wait-for-connection" -}}
- name: wait-for-valkey
  image: {{ .Values.valkey.image.registry }}/{{ .Values.valkey.image.repository }}:{{ .Values.valkey.image.tag }}
  imagePullPolicy: {{ .Values.valkey.imagePullPolicy }}
  env:
    {{- if .Values.valkey.enabled }}
    - name: VALKEY_HOST
      value: {{ .Values.fullnameOverride }}-valkey
    - name: VALKEY_PORT
      value: {{ .Values.valkey.port | quote }}
    - name: VALKEY_PASSWORD
      valueFrom:
        secretKeyRef:
          name: {{ .Values.fullnameOverride }}-valkey
          key: valkey-password
    {{- else }}
    - name: VALKEY_HOST
      value: {{ .Values.externalKeyValueStore.host | quote }}
    - name: VALKEY_PORT
      value: {{ .Values.externalKeyValueStore.port | quote }}
    {{- with .Values.externalKeyValueStore.username }}
    - name: VALKEY_USERNAME
      value: {{ . | quote }}
    {{- end }}
    {{- if .Values.externalKeyValueStore.existingSecret.name }}
    - name: VALKEY_PASSWORD
      valueFrom:
        secretKeyRef:
          name: {{ .Values.externalKeyValueStore.existingSecret.name }}
          key: {{ .Values.externalKeyValueStore.existingSecret.key }}
    {{- end }}
    {{- if .Values.externalKeyValueStore.tls.enabled }}
    - name: VALKEY_TLS
      value: "true"
    {{- if .Values.externalKeyValueStore.tls.insecureSkipVerify }}
    - name: VALKEY_TLS_INSECURE
      value: "true"
    {{- end }}
    {{- if .Values.externalKeyValueStore.tls.caCertSecret.name }}
    - name: VALKEY_CA_CERT_FILE
      value: "/etc/valkey-tls/{{ .Values.externalKeyValueStore.tls.caCertSecret.key }}"
    {{- end }}
    {{- end }}
    {{- end }}
  command: ["/bin/sh", "-c"]
  args:
    - |
      echo "waiting for valkey at $VALKEY_HOST:$VALKEY_PORT..."
      TIMEOUT=60
      ELAPSED=0
      AUTH=""
      [ -n "$VALKEY_PASSWORD" ] && AUTH="-a $VALKEY_PASSWORD --no-auth-warning"
      [ -n "$VALKEY_USERNAME" ] && AUTH="$AUTH --user $VALKEY_USERNAME"
      TLS=""
      if [ "$VALKEY_TLS" = "true" ]; then
        TLS="--tls"
        [ -n "$VALKEY_CA_CERT_FILE" ] && TLS="$TLS --cacert $VALKEY_CA_CERT_FILE"
        [ "$VALKEY_TLS_INSECURE" = "true" ] && TLS="$TLS --insecure"
      fi
      until valkey-cli -h "$VALKEY_HOST" -p "$VALKEY_PORT" $AUTH $TLS ping 2>/dev/null | grep -q PONG; do
        if [ "$ELAPSED" -ge "$TIMEOUT" ]; then
          echo "valkey not reachable after ${TIMEOUT}s — starting anyway, main container will retry with backoff"
          exit 0
        fi
        echo "valkey not ready, retrying in 2s..."
        sleep 2
        ELAPSED=$((ELAPSED + 2))
      done
      echo "valkey is ready"
  {{- if eq (include "externalKvs.caEnabled" .) "true" }}
  volumeMounts:
    {{- include "externalKvs.caVolumeMount" . | nindent 4 }}
  {{- end }}
  {{- with .Values.valkey.containerSecurityContext }}
  securityContext:
    {{- toYaml . | nindent 4 }}
  {{- end }}
{{- end }}

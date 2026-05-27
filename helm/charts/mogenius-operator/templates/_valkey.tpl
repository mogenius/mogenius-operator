{{- define "valkey.wait-for-connection" -}}
- name: wait-for-valkey
  image: {{ .Values.valkey.image.registry }}/{{ .Values.valkey.image.repository }}:{{ .Values.valkey.image.tag }}
  imagePullPolicy: {{ .Values.valkey.imagePullPolicy }}
  env:
    - name: VALKEY_HOST
      value: {{ .Values.fullnameOverride }}-valkey
    - name: VALKEY_PORT
      value: {{ .Values.valkey.port | quote }}
    - name: VALKEY_PASSWORD
      valueFrom:
        secretKeyRef:
          name: {{ .Values.fullnameOverride }}-valkey
          key: valkey-password
  command: ["/bin/sh", "-c"]
  args:
    - |
      set -e
      echo "waiting for valkey at $VALKEY_HOST:$VALKEY_PORT..."
      TIMEOUT=120
      ELAPSED=0
      until valkey-cli -h "$VALKEY_HOST" -p "$VALKEY_PORT" -a "$VALKEY_PASSWORD" --no-auth-warning ping 2>/dev/null | grep -q PONG; do
        if [ "$ELAPSED" -ge "$TIMEOUT" ]; then
          echo "timed out after ${TIMEOUT}s waiting for valkey, failing init container"
          exit 1
        fi
        echo "valkey not ready, retrying in 2s..."
        sleep 2
        ELAPSED=$((ELAPSED + 2))
      done
      echo "valkey is ready"
  {{- with .Values.valkey.containerSecurityContext }}
  securityContext:
    {{- toYaml . | nindent 4 }}
  {{- end }}
{{- end }}

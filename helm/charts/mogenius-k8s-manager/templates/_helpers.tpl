{{/*
This function checks if a Kubernetes Secret exists for the given name.
If the Secret does not exist, it generates a new password and stores it in a global variable.
If the Secret exists, it retrieves the existing password from the Secret.
*/}}
{{- define "lookupSecret" -}}


{{- $secretName := printf "%s-valkey" .Values.fullnameOverride }}
{{- $lookupResult := lookup "v1" "Secret" .Release.Namespace $secretName }}

{{- if or (not $lookupResult) (not $lookupResult.data) }}

  {{- if not .global }}
  {{- $_ := set . "global" (dict) }}
  {{- end }}

  {{- if not .global.generatedPassword }}
  {{- $_ := set .global "generatedPassword" (randAlphaNum 32 | b64enc) }}
  {{- end }}

  {{- .global.generatedPassword }}

{{- else }}
{{- index $lookupResult.data "valkey-password" }}
{{- end }}
{{- end }}

{{/*
This function computes the SHA-256 hash of a base64-decoded password.
It is used to generate a checksum for the password, which can be used in annotations or other purposes.
*/}}
{{- define "getPasswordHash" -}}
{{- $password := . }}
{{- $hash := $password | b64dec | sha256sum }}
{{- $hash -}}
{{- end }}

{{/*
Fails unless exactly one engine is selected. Both would race for ownership of
the same PlatformConfig, neither leaves anything to sync the config with -- in
both cases the install is better refused than half-applied.
*/}}
{{- define "bootstrap.validateEngine" -}}
{{- if and .Values.argoCd.enabled .Values.flux.enabled -}}
{{- fail "argoCd.enabled and flux.enabled are mutually exclusive: pick one GitOps engine" -}}
{{- end -}}
{{- if and (not .Values.argoCd.enabled) (not .Values.flux.enabled) -}}
{{- fail "no GitOps engine selected: set either argoCd.enabled or flux.enabled" -}}
{{- end -}}
{{- if not .Values.repository.url -}}
{{- fail "repository.url is required: without it the engine has nothing to sync the PlatformConfig from" -}}
{{- end -}}
{{- end -}}

{{/*
Name of the repository credential Secret, or empty when the repository needs no
credentials. An existing Secret wins over a token: it is the more explicit of
the two, and it is what a user who manages their own credentials would set.
*/}}
{{- define "bootstrap.repositorySecretName" -}}
{{- if .Values.repository.existingSecret.name -}}
{{- .Values.repository.existingSecret.name -}}
{{- else if .Values.repository.token -}}
{{- printf "%s-repository" .Values.repository.name -}}
{{- end -}}
{{- end -}}

{{- define "bootstrap.labels" -}}
app.kubernetes.io/managed-by: mogenius-platform-bootstrap
app.kubernetes.io/component: platform-config
app.kubernetes.io/part-of: mogenius
{{- end -}}

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

{{/*
Name of the Secret the mogenius platform commits the PlatformConfig with.

Not a name of our choosing: the platform API looks the credential up by
`mogenius-gitops-write-<repository name>` in the operator's namespace, so the
prefix and the repository name together are a contract with it. Rendering a
different name produces a Secret nobody reads.
*/}}
{{- define "bootstrap.writeCredentialSecretName" -}}
{{- printf "mogenius-gitops-write-%s" .Values.repository.name -}}
{{- end -}}

{{/*
Labels the platform recognises its own write credentials by. Kept identical to
the ones the API writes when a token is rotated through the UI, so a Secret
created here and one created there are the same object.
*/}}
{{- define "bootstrap.writeCredentialLabels" -}}
managed-by: mogenius
mogenius.com/component: gitops-write
{{- end -}}

{{/*
How the platform is allowed to write. PULL_REQUEST is the API's own default and
the safer one -- a pull request is reviewable, a direct commit is already live.
Validated here because an unknown value is silently ignored by the reader,
which would leave the cluster on a mode nobody chose.
*/}}
{{- define "bootstrap.writeCredentialMode" -}}
{{- $mode := .Values.repository.writeCredential.mode | default "PULL_REQUEST" -}}
{{- if not (has $mode (list "DIRECT_COMMIT" "PULL_REQUEST")) -}}
{{- fail (printf "repository.writeCredential.mode must be PULL_REQUEST or DIRECT_COMMIT, got %q" $mode) -}}
{{- end -}}
{{- $mode -}}
{{- end -}}

{{/*
Git provider of repository.url, in the platform's own spelling.

Only the three providers whose API can read and write a file are accepted --
committing a PlatformConfig needs both, and the API reports anything else as
unsupported rather than attempting it. Host matching mirrors the API's list
(github., gitlab., gitea., codeberg.org).

An explicit repository.writeCredential.provider wins, for the self-hosted
installation whose host says nothing. Neither an explicit nor a derived
provider fails the render: a Secret with an empty provider is skipped by the
reader, so the alternative is an install that looks fine and a first commit
weeks later that cannot find a credential.
*/}}
{{- define "bootstrap.writeCredentialProvider" -}}
{{- $supported := list "GIT_HUB" "GIT_LAB" "GITEA" -}}
{{- $explicit := .Values.repository.writeCredential.provider | default "" -}}
{{- if $explicit -}}
{{- if not (has $explicit $supported) -}}
{{- fail (printf "repository.writeCredential.provider %q cannot write files: set it to one of GIT_HUB, GIT_LAB, GITEA" $explicit) -}}
{{- end -}}
{{- $explicit -}}
{{- else -}}
{{- $url := .Values.repository.url | lower -}}
{{- $derived := "" -}}
{{- if contains "github." $url -}}
{{- $derived = "GIT_HUB" -}}
{{- else if contains "gitlab." $url -}}
{{- $derived = "GIT_LAB" -}}
{{- else if or (contains "gitea." $url) (contains "codeberg.org" $url) -}}
{{- $derived = "GITEA" -}}
{{- end -}}
{{- if not $derived -}}
{{- fail (printf "cannot derive the git provider of %s: set repository.writeCredential.provider to GIT_HUB, GIT_LAB or GITEA, or set repository.writeCredential.enabled=false to install without a write credential" .Values.repository.url) -}}
{{- end -}}
{{- $derived -}}
{{- end -}}
{{- end -}}

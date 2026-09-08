# mogenius-platform-bootstrap

Installs the GitOps engine for a mogenius platform and the two credentials for
the repository holding your `PlatformConfig`: the read credential the engine
syncs it with, and the write credential the mogenius platform commits it with.

Installed alongside `mogenius-operator` during cluster onboarding. The mogenius
UI generates the exact command; this README explains what it does.

## Why the engine is not installed by the operator

`spec.gitOps.argocd.enabled` / `spec.gitOps.fluxcd.enabled` let the operator
install and own the engine, and that is a valid setup — but not for bootstrap.
The engine is what delivers the `PlatformConfig` into the cluster, so it has to
exist before there is a `PlatformConfig` to declare it in. For Flux there is no
choice at all: the `flux-operator` chart ships no controllers, those come from a
`FluxInstance`, and the operator's own component delivery needs a running
`helm-controller` before it can create anything.

So this chart owns the engine, and the `PlatformConfig` declares it as
`enabled: false`. The operator then reports it as `isUserManaged` and delivers
the platform components through it without trying to take it over.

**Do not also enable the engine in your `PlatformConfig`** — the operator would
install a second one next to this release.

## Why there are two credentials

`templates/repository-secret.yaml` is the engine's, and it only ever reads:
Argo CD or Flux pulls `platformconfig.yaml` out of the repository with it.

`templates/write-credential-secret.yaml` is the mogenius platform's, and it
writes: editing the platform configuration in the UI means committing that file
back through the git provider's API, which the engine's Secret cannot do (for
Flux it may even be an SSH key no API accepts).

The write credential is created **per cluster, in the cluster**, from the token
given to this install. The platform holds none of it centrally, so a token
reaches exactly one cluster's repository — and a cluster that is restored from a
backup or re-registered keeps working, because the credential and the write mode
travel with it.

Both are driven by the same `repository.token`. Set
`repository.writeCredential.enabled=false` to let the engine read with that
token without letting the platform write with it; the `PlatformConfig` then has
to be edited in git by hand.

### The write credential lands in the operator's namespace

Everything else this chart creates lands in the release namespace — the
engine's, `flux-system` or `argocd`. The write credential does not: the platform
API reads it from the namespace the mogenius operator runs in, by the name
`mogenius-gitops-write-<repository.name>`. That is `operatorNamespace`
(`mogenius` by default), which has to match where `mogenius-operator` was
installed and has to exist before this release is installed.

Rotating the token later through the mogenius UI updates the same Secret in
place. A `helm upgrade` of this release that still passes the old
`repository.token` would put the old token back, so pass the new one — or drop
it and use `repository.writeCredential.existingSecret.name`.

## Why the sync objects are not in this chart

The Argo CD `Application`, and Flux's `FluxInstance` / `GitRepository` /
`Kustomization`, are created by the mogenius platform once the operator connects
and reports the engine as running — not by this chart.

Helm resolves every manifest in a release through the API server's RESTMapper
*before* it creates anything. A custom resource shipped in the same release as
the CRD that defines it therefore fails the install outright ("resource mapping
not found for kind ...") on any cluster that does not already have that CRD —
which is every cluster being onboarded. Helm hooks do not fix it either: a
persistent object cannot be a hook, because hooks are re-applied on every
upgrade and would have to delete and recreate it.

Letting the platform create them also means the same code path handles a cluster
that already runs its own Argo CD or Flux, where this chart is not installed at
all.

## Values

| Key | Default | Description |
| --- | --- | --- |
| `flux.enabled` | `true` | Install Flux via `flux-operator`. The default engine. Mutually exclusive with `argoCd.enabled`. Upstream chart values pass through under this key. |
| `argoCd.enabled` | `false` | Install Argo CD instead. Mutually exclusive with `flux.enabled`. Upstream `argo-cd` chart values pass through under this key. |
| `repository.name` | `platform` | Name given to the credential Secret and, later, to the sync objects. |
| `repository.url` | *required* | Clone URL of the repository holding `platformconfig.yaml`. |
| `repository.token` | `""` | Token for a private repository. Written into a Secret in the engine's own format. Leave empty for a public repository. |
| `repository.existingSecret.name` | `""` | Use a credential Secret you created yourself instead of passing a token. It must already be in the engine's format: labelled `argocd.argoproj.io/secret-type=repository` for Argo CD, a basic-auth Secret for Flux. Wins over `token`. |
| `repository.writeCredential.enabled` | `true` | Create the Secret the mogenius platform commits `platformconfig.yaml` with. Rendered only when `repository.token` is set. Turn it off to let the engine read with the token without letting the platform write with it. |
| `repository.writeCredential.mode` | `PULL_REQUEST` | How the platform writes: `PULL_REQUEST` opens a branch and a PR against the tracked branch, `DIRECT_COMMIT` commits straight to it. The `PlatformConfig`'s own `spec.gitOps.repositories[].write.mode` wins over it once the config is synced. |
| `repository.writeCredential.provider` | `""` | Git provider in the platform's spelling: `GIT_HUB`, `GIT_LAB` or `GITEA`. Derived from `repository.url` when the host says which (`github.`, `gitlab.`, `gitea.`, `codeberg.org`); required for a self-hosted host that does not, otherwise the install fails rather than writing a credential the platform would skip. |
| `repository.writeCredential.username` | `mogenius` | Username stored next to the token. Only some provider APIs use it. |
| `repository.writeCredential.existingSecret.name` | `""` | Use a write credential Secret you created yourself; nothing is rendered when set. It has to be in `operatorNamespace` with `token`/`username`/`provider` keys, and unless it is named `mogenius-gitops-write-<repository.name>` the `PlatformConfig` has to point `spec.gitOps.repositories[].write.secretRef` at it. |
| `operatorNamespace` | `mogenius` | Namespace the mogenius operator runs in. The write credential is created there, not in the release namespace, because that is where the platform API reads it. |

Install into the namespace the engine should run in — `flux-system` or `argocd`
by convention. Every object this chart creates lands in the release namespace,
except the write credential, which goes to `operatorNamespace`.

Because the engine charts are aliased (`argoCd`, `flux`) so that one value both
selects the engine and carries its settings, subchart resources render with a
`helm.sh/chart: argoCd-<version>` label rather than `argo-cd-<version>`. Only
that label is affected; the engine's own `app.kubernetes.io/*` labels, which is
what the operator detects it by, are unchanged.

## Engines

Chart versions track [`platform-defaults`](https://github.com/mogenius/platform-defaults)
(`argocd.yaml`, `flux-operator.yaml`), so a cluster onboarded through this chart
runs the same engine version the operator would install itself.

# mogenius-platform-bootstrap

Installs the GitOps engine for a mogenius platform and the credential it needs
to read the repository holding your `PlatformConfig`.

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

Install into the namespace the engine should run in — `flux-system` or `argocd`
by convention. Every object this chart creates lands in the release namespace.

Because the engine charts are aliased (`argoCd`, `flux`) so that one value both
selects the engine and carries its settings, subchart resources render with a
`helm.sh/chart: argoCd-<version>` label rather than `argo-cd-<version>`. Only
that label is affected; the engine's own `app.kubernetes.io/*` labels, which is
what the operator detects it by, are unchanged.

## Engines

Chart versions track [`platform-defaults`](https://github.com/mogenius/platform-defaults)
(`argocd.yaml`, `flux-operator.yaml`), so a cluster onboarded through this chart
runs the same engine version the operator would install itself.

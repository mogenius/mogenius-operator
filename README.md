<p align="center">
  <img src="https://imagedelivery.net/T7YEW5IAgZJ0dY4-LDTpyQ/3ae4fcf0-289c-48d2-3323-d2c5bc932300/detail" alt="mogenius" width="140"/>
</p>
<h1 align="center">mogenius-operator</h1>
<p align="center">Kubernetes cluster manager & runtime control-plane components for the <a href="https://mogenius.com" target="_blank">mogenius</a> platform.</p>

---

[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/mogenius)](https://artifacthub.io/packages/helm/mogenius/mogenius-operator)
[![Go Version](https://img.shields.io/github/go-mod/go-version/mogenius/mogenius-operator)](go.mod)
[![Release](https://img.shields.io/github/v/release/mogenius/mogenius-operator)](https://github.com/mogenius/mogenius-operator/releases)
[![License](https://img.shields.io/github/license/mogenius/mogenius-operator)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/mogenius/mogenius-operator/main.yml?label=CI)](https://github.com/mogenius/mogenius-operator/actions)

Go (≥1.25) operator that manages CRDs, Helm deployments, metrics collection, WebSocket communication, and IaC for the mogenius platform.

---

## Architecture

Modular packages under `src/`:

- `cmd/` – CLI entry points (cluster, nodemetrics, system, config).
- `core/` – lifecycle, reconcilers, socket API, node metrics collector.
- `kubernetes/` – resource CRUD, backups, issuers, cronjobs, etc.
- `crds/` – Custom Resource Definitions (Workspace, User, Grant).
- `k8sclient/` – Kubernetes client provider & kubeconfig.
- `valkeyclient/` – Valkey/Redis caching & time-series helpers.
- `websocket/` – WebSocket multiplexing with auto-reconnect.
- `xterm/` – Terminal/shell access over WebSocket.
- `helm/` – Helm SDK integration & chart management.
- `gitmanager/` – Git operations orchestration.
- `iacmanager/` – Infrastructure-as-Code orchestration.
- `networkmonitor/` – Network traffic collection (eBPF via snoopy, or procdev).
- `containerenumerator/` – Container PID discovery via cgroup inspection.
- `cpumonitor/`, `podstatscollector/`, `rammonitor/` – CPU, pod & RAM telemetry.
- `config/` – Immutable config with validation & change callbacks.
- `logging/` – Structured slog with custom handlers & secret masking.
- `secrets/`, `store/`, `watcher/`, `shutdown/`, `services/`, `utils/`, `assert/`, `version/` – supporting packages.

Generated artifacts: `generated/spec.yaml` (pattern spec) and `generated/client.ts` (TypeScript bindings).

---

## Local Development

Prerequisites: Go 1.25+, [`just`](https://github.com/casey/just), access to a Kubernetes cluster with the `mogenius` namespace.

```sh
# 1. Create .env (see Configuration below)
# 2. Optionally scale down in-cluster deployment to avoid conflicts
just scale-down

# 3. Build & run
just build
just run

# Restore in-cluster deployment afterward
just scale-up
```

Key tasks:

```sh
just build            # compile + regenerate spec.yaml & client.ts
just run              # run operator locally
just run-node-metrics # run node metrics DaemonSet mode locally
just check            # generate + lint + unit tests
just test-unit
just test-integration
just golangci-lint
just generate         # run go generate (CRD deepcopy)
just scale-down / scale-up
```

---

## Configuration

Create a `.env` in the repo root:

```sh
MO_API_KEY=<api-key>       # From operator secret (mogenius/mogenius)
MO_CLUSTER_NAME=<name>     # Cluster identifier
MO_CLUSTER_MFA_ID=<id>     # MFA/instance id
MO_API_SERVER=<url>        # Platform API WebSocket URL
MO_EVENT_SERVER=<url>      # Platform Event WebSocket URL
MO_VALKEY_ADDR=<host:port> # Valkey/Redis address
```

Load (bash/zsh):

```sh
if [[ -f .env ]]; then export $(grep -v '^#' .env | xargs); fi
```

### All Environment Variables

| Variable | Default | Description |
|---|---|---|
| `MO_API_KEY` | — | API key to access the mogenius platform (**required**, secret) |
| `MO_CLUSTER_NAME` | — | Name of the Kubernetes cluster (**required**) |
| `MO_CLUSTER_MFA_ID` | — | NanoId of the cluster for MFA purpose (**required**, secret) |
| `MO_API_SERVER` | — | URL of the platform API WebSocket server (**required**) |
| `MO_API_SERVER_CLIENTS` | `1` | Number of parallel WebSocket connections to the API server |
| `MO_EVENT_SERVER` | — | URL of the platform event WebSocket server (**required**) |
| `MO_SKIP_TLS_VERIFICATION` | `false` | Skip TLS verification for API and Event Server |
| `MO_VALKEY_ADDR` | — | Address (`host:port`) of the Valkey/Redis server (**required**) |
| `MO_VALKEY_PASSWORD` | — | Password for the Valkey/Redis server |
| `MO_HTTP_ADDR` | `:1337` | Listen address for the operator HTTP API |
| `MO_OWN_NAMESPACE` | `mogenius` | Namespace the mogenius platform is installed in |
| `OWN_NODE_NAME` | — | Node name the application is running on (set by DaemonSet) |
| `OWN_DEPLOYMENT_NAME` | `mogenius-operator` | Deployment name the application is running in |
| `CLUSTER_DOMAIN` | `cluster.local` | Internal cluster domain |
| `MO_HELM_DATA_PATH` | `<workdir>/helm-data` | Path to Helm data directory |
| `MO_GIT_USER_NAME` | `mogenius git-user` | Git username for IaC operations |
| `MO_GIT_USER_EMAIL` | `git@mogenius.com` | Git email for IaC operations |
| `MO_AUDIT_LOG_LIMIT` | `1000` | Maximum number of audit log entries to persist |
| `MO_ENABLE_POD_STATS_COLLECTOR` | `true` | Enable collection of pod CPU/memory stats |
| `MO_ENABLE_TRAFFIC_COLLECTOR` | `false` | Enable collection of network traffic stats |
| `MO_SNOOPY_IMPLEMENTATION` | `auto` | Network traffic backend: `auto`, `snoopy` (eBPF), or `procdev` |
| `MO_HOST_PROC_PATH` | `/proc` | Mount path of the host `/proc` filesystem (DaemonSet uses `/hostproc`) |
| `MO_LOG_LEVEL` | `info` | Log level: `mo`, `debug`, `info`, `warn`, or `error` |
| `MO_LOG_FILTER` | — | Comma-separated list of components to enable logs for (empty = all) |
| `MO_ALLOW_COUNTRY_CHECK` | `true` | Allow the operator to determine its location country via IP lookup |
| `KUBERNETES_DEBUG` | `false` | Enable Kubernetes SDK debug output |

List all config options at runtime: `go run -trimpath src/main.go config`

---

## Docker (local image)

```sh
docker build -t localk8smanager \
  --build-arg GOOS=linux \
  --build-arg GOARCH=arm64 \
  --build-arg BUILD_TIMESTAMP="$(date -Iseconds)" \
  --build-arg COMMIT_HASH="$(git rev-parse --short HEAD || echo XXX)" \
  --build-arg GIT_BRANCH=local-development \
  --build-arg VERSION="dev-local" \
  -f Dockerfile .
```

To use the local image, patch the deployment to `image: localk8smanager:latest` with `imagePullPolicy: Never`, then restart.

---

## Helm

Install via OCI:

```sh
helm -n mogenius upgrade --install mogenius-platform \
  oci://ghcr.io/mogenius/helm-charts/mogenius-operator \
  --create-namespace \
  --set global.cluster_name="<cluster>" \
  --set global.api_key="<api-key>"
```

Or via Helm repo:

```sh
helm repo add mo-public https://helm.mogenius.com/public
helm repo update
helm upgrade --install mogenius-platform mo-public/mogenius-operator \
  --namespace mogenius --create-namespace \
  --set global.cluster_name="<cluster>" \
  --set global.api_key="<api-key>"
```

Upgrade: `helm repo update && helm upgrade mogenius-platform mo-public/mogenius-operator`

Uninstall: `helm uninstall mogenius-platform`

---

## Troubleshooting

- Scale down in-cluster deployment before running locally: `just scale-down`.
- Regenerate patterns after structural changes: `just build`.
- Auth issues: verify `.env` secrets match the `mogenius/mogenius` operator secret.
- Stale dependencies: `go clean -modcache && go mod tidy`.

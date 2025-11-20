<p align="center">
  <img src="https://imagedelivery.net/T7YEW5IAgZJ0dY4-LDTpyQ/3ae4fcf0-289c-48d2-3323-d2c5bc932300/detail" alt="mogenius" width="140"/>
</p>
<h1 align="center">mogenius-operator</h1>
<p align="center">Kubernetes cluster manager & runtime control-plane components for the <a href="https://mogenius.com" target="_blank">mogenius</a> platform.</p>

---
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/mogenius)](https://artifacthub.io/packages/helm/mogenius/mogenius-operator)

## Table of Contents
1. Overview
2. Features
3. Architecture (High-Level)
4. Quick Start (Local Development)
5. Configuration (.env)
6. Build & Code Generation
7. Running & Tasks (Justfile)
8. Testing & Linting
9. Docker & Images
10. Helm (Install / Upgrade / Uninstall)
11. eBPF Development Helpers
12. Troubleshooting
13. Contributing
14. License & Attribution

---

## 1. Overview
`mogenius-operator` is a Go (>=1.25) service that coordinates cluster resources, patterns, secrets, metrics collection, and auxiliary runtime capabilities (websockets, git/helm/iac integration, valkey caching, etc.) for the mogenius platform.

Major subsystems include:
- Kubernetes controllers & reconcilers
- Pattern (spec) generation (YAML + TypeScript client in `generated/`)
- Git, Helm & IaC managers
- Metrics & node monitoring (CPU, pod stats, Prometheus integration)
- Websocket multiplexing & terminal/xterm services
- Valkey (Redis compatible) caching layer
- eBPF based system/network insights (optional)

---

## 2. Features
- Declarative pattern & client generation (`just build` auto-updates `generated/spec.yaml` & `generated/client.ts`).
- Multi-environment configuration via `.env` or environment variables.
- Pluggable secret & external config handling.
- Node & workload metrics collection.
- Rich CLI powered by `kong` (see `go run -trimpath src/main.go --help`).
- Built-in task automation with `just`.
- Helm deployment artifacts & local override workflows.
- Optional eBPF utilities for advanced networking/CPU visibility.

---

## 3. Architecture (High-Level)
Monolithic binary with modular packages under `src/`:
- `core/` – lifecycle, reconcilers, socket APIs.
- `kubernetes/` – resource CRUD, backups, issuers, cronjobs, etc.
- `valkeyclient/` – caching & time-series helpers.
- `gitmanager/`, `helm/`, `iacmanager/` – integration layers.
- `xterm/`, `websocket/` – interactive & streaming comms.
- `cpumonitor/`, `podstatscollector/`, `nodemetricscollector.go` – telemetry.
- `dtos/` – transport/data contracts.

Generated artifacts:
- `generated/spec.yaml` – pattern specification (YAML)
- `generated/client.ts` – TypeScript client bindings.

---

## 4. Quick Start (Local Development)
Prerequisites:
- A running mogenius platform (see official docs) or at least the helm chart installed.
- Go 1.25+
- `just` task runner (https://github.com/casey/just)
- Access to a Kubernetes cluster with the operator namespace (`mogenius`).

Steps:
1. Create `.env` (see section 5).
2. Optionally scale down in-cluster deployment to avoid conflicts:
   ```sh
   just scale-down
   ```
3. Build & generate artifacts:
   ```sh
   just build
   ```
4. Run locally:
   ```sh
   just run
   ```

Restore cluster components afterward with:
```sh
just scale-up
```

---

## 5. Configuration (.env)
Create a `.env` file in repo root. Minimal keys:
```sh
MO_API_KEY=                       # From operator secret (mogenius/mogenius)
MO_CLUSTER_NAME=                  # Cluster identifier
MO_CLUSTER_MFA_ID=                # MFA/instance id
MO_STAGE=dev                      # prod | pre-prod | dev | local | (empty for manual URLs)
# Optional advanced overrides:
# MO_API_SERVER=...
# MO_EVENT_SERVER=...
```
Load (bash/zsh):
```sh
if [[ -f .env ]]; then export $(grep -v '^#' .env | xargs); fi
```
List available runtime config options:
```sh
go run -trimpath src/main.go config
```

---

## 6. Build & Code Generation
The build step embeds version metadata (commit, branch, timestamp) and regenerates patterns + TypeScript client.
```sh
just build
```
Artifacts:
- `dist/native/mogenius-operator`
- `generated/spec.yaml`
- `generated/client.ts`

Cross compilation & images:
```sh
just build-all                # All target architectures
just build-docker-linux-amd64 # Docker image (amd64)
just build-docker-linux-arm64 # Docker image (arm64)
```

---

## 7. Running & Tasks (Justfile)
Discover tasks:
```sh
just --list --unsorted
```
Key tasks:
- `just run` – Start cluster manager (local dev)
- `just run-node-metrics` – Run only node metrics mode
- `just scale-down` / `scale-up` – Toggle in-cluster instances
- `just generate` – Run `go generate`
- `just check` – Lint + unit tests
- `just test-unit` / `test-integration`
- `just golangci-lint`

---

## 8. Testing & Linting
```sh
just check            # generate + lint + unit tests
just test-unit        # unit tests only
just test-integration # integration suite
just golangci-lint    # lint only
```

Upgrade dependencies:
```sh
go get -u ./...
go mod tidy
```

---

## 9. Docker & Images
Local development image example:
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
Swap image in deployment:
```yaml
# from
image: ghcr.io/mogenius/mogenius-operator:latest
imagePullPolicy: Always
# to
image: localk8smanager:latest
imagePullPolicy: Never
```
Then restart the deployment.

---

## 10. Helm
Add & install:
```sh
helm repo add mo-public helm.mogenius.com/public
helm repo update
helm search repo mo-public
helm upgrade --install mogenius-platform mo-public/mogenius-operator \
  --namespace mogenius --create-namespace \
  --set global.cluster_name="<cluster>" \
  --set global.api_key="<api-key>"
```
Upgrade:
```sh
helm repo update
helm upgrade mogenius-platform mo-public/mogenius-operator
```
Uninstall:
```sh
helm uninstall mogenius-platform
```
Clean local helm cache (if needed):
```sh
rm -rf ~/.helm/cache/archive/* ~/.helm/repository/cache/*
helm repo update
```

---

## 11. eBPF Development Helpers
Run example program:
```sh
go generate ./ebpf
sudo go run ./cmd/main.go
# or
just ebpf
```
Generate synthetic network load:
```sh
ping -i 0.002 127.0.0.1
```
Docker dev environment examples:
```sh
# test inside ephemeral container
docker build -t my-go-ebpf-app -f Dockerfile-Dev-Environment . \
  && docker run --rm -it --privileged --pid=host --net=host my-go-ebpf-app \
  sh -c "cd /app && just ebpf"

# interactive shell
docker build -t my-go-ebpf-app -f Dockerfile-Dev-Environment . \
  && docker run --rm -it --privileged --pid=host --net=host my-go-ebpf-app sh

# with local kubeconfig + .env
docker build -t my-go-ebpf-app -f Dockerfile-Dev-Environment . \
  && docker run --rm -it \
     -v "$KUBECONFIG:/root/.kube/config:ro" \
     -v "$(pwd)/.env:/app/.env:ro" \
     --privileged --pid=host --net=host my-go-ebpf-app sh
```
Access valkey from container:
```sh
kubectl -n mogenius port-forward svc/mogenius-operator-valkey 6379:6379 &
```

---

## 12. Troubleshooting
- Ensure in-cluster manager scaled down when running locally: `just scale-down`.
- Regenerate patterns after structural changes: `just build` or `just generate`.
- Connection / auth issues: verify `.env` secrets still match operator secret in namespace `mogenius`.
- Helm drift: run `helm repo update` before upgrade.
- Stale dependencies: run `go clean -modcache` then `go mod tidy`.

---

## 13. Contributing
1. Fork & create a feature branch.
2. Keep PRs small & focused.
3. Run `just check` before pushing.
4. Add/update tests where behavior changes.

Issues & PRs welcome.

---

## 14. License & Attribution
Copyright (c) mogenius. All rights reserved.

Built with ❤️ by the <a href="https://mogenius.com" target="_blank">mogenius</a> team.

---

References:
- Task runner: https://github.com/casey/just
- Go module: `mogenius-operator`


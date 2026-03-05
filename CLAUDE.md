# CLAUDE.md - Kubernetes Operator Development Guidelines

## Project Overview

This is `mogenius-k8s-manager`, the Go-based Kubernetes operator for the mogenius platform. It runs inside customer clusters, manages workspaces/users/grants via CRDs, handles Helm deployments, and communicates with the platform API via WebSocket.

## Tech Stack

- **Language:** Go 1.25
- **Kubernetes:** client-go, controller-runtime v0.23
- **Helm:** helm.sh/helm/v4 SDK
- **CLI:** alecthomas/kong
- **Caching:** Valkey (Redis-compatible) via valkey-io/valkey-go
- **WebSocket:** gorilla/websocket (full-duplex with auto-reconnect)
- **Git:** go-git/go-git/v5
- **Logging:** log/slog with custom handlers
- **Validation:** go-playground/validator/v10
- **AI:** anthropic-sdk-go, openai-go
- **Task runner:** Just (Justfile)
- **Testing:** testify/assert

## Key Commands

```bash
just build               # Compile + generate patterns + TypeScript client
just run                 # Run operator locally
just check               # generate + lint + unit tests
just test-unit           # Unit tests only
just test-integration    # Integration tests
just golangci-lint       # Linting
just scale-down          # Scale in-cluster replica to 0 (for local dev)
just scale-up            # Scale back to 1
just generate            # go generate (controller-gen for CRD deepcopy)
```

## Project Structure

```
mogenius-k8s-manager/
├── src/
│   ├── cmd/                # CLI commands (cluster, nodemetrics, system, config)
│   ├── core/               # Core lifecycle, reconcilers, APIs, monitoring
│   ├── kubernetes/         # K8s resource CRUD, reconciliation, custom resources
│   ├── crds/               # Custom Resource Definitions (Workspace, User, Grant)
│   ├── k8sclient/          # Kubernetes client provider & kubeconfig
│   ├── valkeyclient/       # Redis-compatible caching layer
│   ├── websocket/          # WebSocket multiplexing with auto-reconnect
│   ├── xterm/              # Terminal/shell access over WebSocket
│   ├── helm/               # Helm SDK integration, chart management
│   ├── gitmanager/         # Git operations orchestration
│   ├── iacmanager/         # Infrastructure-as-Code orchestration
│   ├── config/             # Immutable config with validation & change callbacks
│   ├── logging/            # Structured slog with custom handlers & secret masking
│   ├── dtos/               # Data transfer objects
│   ├── services/           # Application services (AI, ArgoCD)
│   ├── utils/              # Helpers, validation, K8s provider detection
│   ├── assert/             # Fail-fast assertion helpers
│   ├── secrets/            # Secret masking & management
│   ├── store/              # Valkey-backed storage patterns
│   ├── watcher/            # Kubernetes resource watcher
│   ├── shutdown/           # Graceful shutdown coordination
│   └── version/            # Build metadata (commit, branch, timestamp)
├── helm/                   # Helm charts for deploying this operator
├── test/                   # Integration tests
├── generated/              # Auto-generated (spec.yaml, client.ts)
├── Justfile                # Task runner
└── Dockerfile              # Multi-stage build (Go + eBPF tooling)
```

## Architecture Rules

### Configuration

- **Immutable pattern:** Config uses `ConfigDeclaration` with validator functions.
- **Change callbacks:** `OnChanged` for reactive config updates.
- **Secret marking:** `IsSecret: true` for PII masking in logs.
- **Env loading:** `.env` file loaded via godotenv in main.go.

```go
configModule.Declare(config.ConfigDeclaration{
  Key:      "MO_API_KEY",
  IsSecret: true,
  Validate: urlValidator,
})
```

### Kubernetes Client

- **K8sClientProvider** interface provides typed clients: `K8sClientSet()`, `MetricsClientSet()`, `DynamicClient()`, `MogeniusClientSet()`.
- Detects in-cluster vs local execution automatically.
- Supports impersonation for local development.

### Reconciliation

- **CRDs:** Workspace, User, Grant (in `src/crds/v1alpha1/`).
- **Thread safety:** RWMutex per resource type (`workspacesLock`, `grantsLock`, `usersLock`).
- **Idempotency:** Reconcile must produce same result when called repeatedly.
- **Leader election:** Multi-replica coordination built-in.
- **Generated code:** `zz_generated.deepcopy.go` via `controller-gen` (run `just generate`).

### WebSocket

- **Full-duplex multiplexing** with auto-reconnect and exponential backoff.
- **Channel-based API** for thread-safe communication.
- **Outbound connections:** `jobConnectionClient` → API server, `eventConnectionClient` → event server.
- **Headers:** `x-authorization`, `x-cluster-mfa-id`, `x-cluster-name`.

### Helm SDK

- Direct helm.sh/helm/v4 SDK usage (not CLI wrapper).
- Caching layer: 2-hour TTL with 30-min cleanup.
- Environment-based config: `HELM_CACHE_HOME`, etc.

### Service Initialization

**Link pattern:** Services wired in phases via `InitializeSystems()`:
1. Low-level: Valkey, K8s client, logging
2. Mid-level: API module, Socket API
3. Link phase: `.Link()` methods wire dependencies

### Error Handling

- **assert/** for fail-fast initialization: `Assert(condition, messages...)` → exits on failure.
- **Return errors** in production paths — never `panic()`.
- **Wrap errors:** `fmt.Errorf("context: %w", err)` (not `fmt.Sprintf`).
- **slog for logging:** Custom handlers with secret masking and log filtering.

### Valkey (Redis) Patterns

- Time-series via sorted lists (timestamp-indexed).
- Key patterns: `logs:*`, `pod-stats:*`, `traffic-stats:*`.
- TTL management: 7-day max retention.
- Pattern-based deletion: `DeleteMultiple(patterns...)`.

## Naming Conventions

| Item | Convention | Example |
|------|-----------|---------|
| Packages | lowercase, short | `kubernetes`, `helm`, `config` |
| Interfaces | at point of use (consumer side) | `K8sClientProvider` |
| Constructors | `New` + PascalCase | `NewK8sClientProvider()` |
| Test files | `_test.go` suffix | `config_test.go` |
| Exported types | Doc comment required | `// Config manages...` |
| Errors | Wrap with context | `fmt.Errorf("create workspace: %w", err)` |

## Testing

- **Unit tests:** Colocated `_test.go` files with testify/assert.
- **Integration tests:** In `test/` directory (kubernetes, helm, gitmanager).
- **Table-driven tests:** Standard pattern with `[]struct{ name, input, expected }`.
- **K8s fakes:** `k8s.io/client-go/kubernetes/fake` for mocking the API server.

## Key Config Variables

| Variable | Purpose | Validation |
|----------|---------|-----------|
| `MO_API_KEY` | Platform API auth (secret) | Required |
| `MO_CLUSTER_NAME` | Cluster identity | Required |
| `MO_API_SERVER` | Platform API URL | URL format |
| `MO_EVENT_SERVER` | Event WebSocket URL | URL format |
| `MO_VALKEY_ADDR` | Valkey/Redis address | Required |
| `MO_HTTP_ADDR` | HTTP listen address (default: `:1337`) | Host:port |
| `MO_OWN_NAMESPACE` | Operator namespace (default: `mogenius`) | String |
| `MO_LOG_LEVEL` | Log level: mo, debug, info, warn, error | Enum |

## Don'ts

- Do NOT use `panic()` in production code — return errors.
- Do NOT use `fmt.Println` for logging — use `slog.Logger`.
- Do NOT mutate shared state without holding the appropriate mutex.
- Do NOT ignore `context.Context` cancellation in long-running operations.
- Do NOT hardcode secrets — use config with `IsSecret: true`.
- Do NOT fire-and-forget goroutines — always define lifetime and shutdown path.
- Do NOT modify generated files (`zz_generated.deepcopy.go`) — run `just generate`.

## Agent Behavior

- When debugging operator issues, check slog output and reconciler state first.
- When adding CRD fields, run `just generate` to regenerate deepcopy.
- When modifying Helm operations, test with `just test-integration`.
- Ask me if you're unsure about the Link pattern or service wiring.

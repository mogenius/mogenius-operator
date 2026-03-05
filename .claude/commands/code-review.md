Review the Go operator code at $ARGUMENTS for idiomatic Go, Kubernetes patterns, and safety.

## Phase 1: Idiomatic Go

- Error wrapping: fmt.Errorf("context: %w", err), not fmt.Sprintf
- Exported types have doc comments
- No panic() in production paths — return errors
- Interfaces defined at point of use (consumer side)
- Struct fields grouped logically

## Phase 2: Kubernetes / controller-runtime Patterns

- Reconciler is idempotent — repeated reconcile = same result
- context.Context propagated through all k8s API calls
- Finalizers added before external resource creation, removed in deletion path
- Status conditions updated via status subresource
- Requeue with exponential backoff on transient errors

## Phase 3: Concurrency Safety

- Mutexes protect shared state (check *Lock fields)
- sync.RWMutex used correctly (RLock for reads, Lock for writes)
- Context cancellation respected in goroutines
- Goroutines have defined lifetimes (no fire-and-forget leaks)
- Channel operations don't block indefinitely

## Phase 4: Error Handling

- All errors handled — no blank identifier (_) on errors
- Errors wrapped with context before returning
- slog.Logger used (not fmt.Println)
- No sensitive data in log messages (check config IsSecret)

## Phase 5: Helm & WebSocket

- Helm v4 SDK used correctly
- Release names are valid k8s resource names
- WebSocket reconnection handles auth token refresh
- Message serialization matches platform API expectations

## Report

Rate each finding as Critical / High / Medium / Low with file:line references.

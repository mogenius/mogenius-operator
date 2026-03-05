Debug the Go operator issue: $ARGUMENTS

## Phase 1: Common Operator Failure Patterns

- **Reconciler stuck**: Check finalizer removal logic, status update errors, infinite requeue
- **Object not found (404)**: Watch cache not synced, use eventual consistency patterns
- **Conflict (409)**: Object updated by another process, re-fetch before update
- **Helm install fails**: Namespace missing, RBAC insufficient, values schema mismatch
- **Leader election lost**: Pod restart, check lease object TTL
- **WebSocket disconnect**: Auth token expired, network partition, reconnect backoff

## Phase 2: Investigation

1. Check slog output for the error chain — trace from first error
2. Verify context is not cancelled before k8s API call
3. Check watcher event routing in watcher/
4. Trace through the reconciler reconcile path for the affected resource
5. Check Valkey connectivity if caching related
6. Review config validation output on startup

## Phase 3: Root Cause

Present findings as:
- **Root Cause**: What exactly is wrong and why
- **Fix**: Minimal code change to resolve
- **Prevention**: How to prevent this class of bug

## Phase 4: Verify

Run `go build ./...` and `just test-unit` to confirm the fix compiles and tests pass.

Deep review of the reconciler at $ARGUMENTS for correctness, idempotency, and safety.

## Phase 1: Idempotency

- Can reconcile() be called 10x on the same object without side effects?
- Does it check current state before mutating (no blind create/update)?
- Are external resources only created when actually missing?

## Phase 2: Finalizer Lifecycle

- Finalizer added in creation path before any external resource is created
- Finalizer removed only after external resources are fully cleaned up
- DeletionTimestamp check present at the start of reconcile
- No external resources leaked on object deletion

## Phase 3: Status Management

- Status conditions updated after every significant state change
- Status subresource patched separately from spec (not overwriting spec)
- Status reflects actual observed state, not desired state

## Phase 4: Error Handling

- Transient errors trigger requeue: return reconcile.Result{RequeueAfter: ...}
- Permanent errors logged + status updated (not infinite requeue loop)
- context.Context checked for cancellation in long loops
- Errors wrapped with context for debugging

## Phase 5: Concurrency

- workspacesLock / grantsLock / usersLock used correctly
- No stale cache reads after mutations
- Leader election considered for operations that must run on a single replica

## Report

List specific issues with file:line references and severity (Critical / High / Medium / Low).

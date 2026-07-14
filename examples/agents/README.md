# AI Agents as CRDs

Agents are regular Kubernetes custom resources (`agents.mogenius.com/v1alpha1`) —
you can manage them entirely with `kubectl` or GitOps, no UI required. The CRD
is installed automatically by the operator on startup.

```bash
kubectl apply -f agent-minimal.yaml
kubectl get agents -n mogenius        # shortname: aiagent, category: mogenius
```

```
NAME             ENABLED   READY   REASON   CRON        AGE
cost-optimizer   true      True    Valid    0 6 * * 1   1m
```

## Rules

- **Namespace:** agents are only processed in the operator's own namespace
  (`MO_OWN_NAMESPACE`, default `mogenius`). Agents elsewhere get
  `READY=False, REASON=IgnoredNamespace`.
- **Scope is required:** `spec.scope` must reference a workspace and/or list at
  least one namespace (enforced by the API server via CEL). The single entry
  `"*"` scopes the agent to all namespaces. Agents are always **read-only**;
  they create task proposals that a user approves or rejects in the UI.
- **`enabled` is explicit:** state it in every manifest. Disabled agents are
  valid but never run.
- **Validation feedback:** the operator writes a `Ready` condition
  (`kubectl get agents` shows READY/REASON; details via
  `kubectl describe agent <name> -n mogenius`). Schema errors (empty scope,
  invalid namespace names, event filters without `contains`) are rejected at
  apply time; everything else (e.g. an invalid cron expression, a missing
  workspace) surfaces as `Ready=False` with a reason.
- **GitOps note:** if agents are managed by Flux/Argo, toggling them in the UI
  will be reverted by the next GitOps sync — treat Git as the source of truth.

## Default agents

On first leadership the operator seeds five disabled default agents
(cluster-cleanup, resource-optimizer, workload-doctor, security-auditor,
best-practices-advisor). Seeding happens once, guarded by the
`mogenius-ai-default-agents-seeded` ConfigMap — deleting a default agent is
permanent unless you also delete that marker.

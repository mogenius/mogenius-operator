# AI Models as CRDs

AI model configurations are regular Kubernetes custom resources
(`aimodels.mogenius.com/v1alpha1`) — manageable entirely with `kubectl` or
GitOps, no UI required. The CRD is installed automatically by the operator on
startup. There is **no global AI configuration**: providers, credentials and
budgets all live on the model (and optionally per agent).

```bash
kubectl apply -f aimodel-anthropic.yaml
kubectl get aimodels -n mogenius      # shortname: aimodel, category: mogenius
```

```
NAME                SDK         MODEL             DEFAULT   LIMIT    READY   REASON   AGE
anthropic-default   anthropic   claude-opus-4-8   true      300000   True    Valid    1m
ollama-local        ollama      qwen3-coder:30b             0        True    Valid    1m
```

## Rules

- **Namespace:** models are only processed in the operator's own namespace
  (`MO_OWN_NAMESPACE`, default `mogenius`).
- **API keys are Secret references** (`spec.apiKeySecretRef`), never inline —
  use SealedSecrets or the External Secrets Operator in GitOps repos. Ollama
  needs no key but requires `spec.apiUrl`.
- **Exactly one default:** the model with `spec.default: true` serves chat and
  every agent without an explicit `modelRef`. A second default created via
  GitOps drift is flagged `Ready=False, REASON=DuplicateDefault` on the
  election loser (oldest wins) until resolved.
- **Budgets:**
  - `maxToolCalls` (unset → 50) and `maxTokensPerRun` (unset → 30000, `0` =
    unlimited) cap a single run; agents may override both in their spec
    (see `agent-with-model.yaml`).
  - `dailyTokenLimit` (unset → 300000, `0` = unlimited) caps everything that
    runs against this model per day — runs and chat combined. Budgets are
    independent per model: an exhausted local model never blocks a hosted one.
- **Validation feedback:** the operator writes a `Ready` condition
  (`kubectl get aimodels` shows READY/REASON): unknown SDK, missing apiUrl
  (ollama), missing Secret or key all surface there before a run fails.

## Token usage & reset

Usage is tracked per model by the operator (source of truth: its Valkey
store; also exported as the Prometheus counter
`mogenius_operator_ai_tokens_used_total{model="<cr-name>"}`).

Reset a model's usage for today by bumping the reset annotation — the
Flux-style, idempotent way (the same value is acted on exactly once,
recorded in `status.lastUsageResetAt`):

```bash
kubectl annotate aimodel ollama-local -n mogenius \
  mogenius.com/reset-usage-at=$(date -u +%FT%TZ) --overwrite
```

The operator confirms with a Kubernetes Event on the model:

```bash
kubectl describe aimodel ollama-local -n mogenius   # Events: UsageReset ...
```

The annotation may live in a GitOps repo; syncs re-applying the same value
never trigger a second reset.

# Helm Chart: mogenius-operator

![Version: 1.7.9](https://img.shields.io/badge/Version-1.7.9-informational?style=flat-square) ![AppVersion: 1.0](https://img.shields.io/badge/AppVersion-1.0-informational?style=flat-square)

**Homepage:** <https://mogenius.com>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| the mogenius tech team | <info@mogenius.com> |  |

## Source Code

* <https://github.com/mogenius/mogenius-operator>
* <https://helm.mogenius.com/public>
* <oci://ghcr.io/mogenius/helm-charts/mogenius-operator>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| cluster | object | `{"domain":"cluster.local"}` | settings for the internal cluster communication |
| cluster.domain | string | `"cluster.local"` | the cluster domain, default for kubernetes is "cluster.local" |
| containerSecurityContext.capabilities.drop[0] | string | `"ALL"` |  |
| containerSecurityContext.privileged | bool | `false` |  |
| containerSecurityContext.readOnlyRootFilesystem | bool | `true` |  |
| envVars | object | `{"MO_API_SERVER":"wss://k8s-ws.mogenius.com/ws","MO_EVENT_SERVER":"wss://k8s-dispatcher.mogenius.com/ws"}` | environment variables to be set in the mogenius-operator deployment |
| externalKeyValueStore | object | `{"enabled":false,"existingSecret":{"key":"valkey-password","name":""},"host":"","port":6379,"tls":{"caCertSecret":{"key":"ca.crt","name":""},"enabled":false,"insecureSkipVerify":false},"username":""}` | When enabled, set valkey.enabled to false. |
| externalKeyValueStore.enabled | bool | `false` | enable using an external key-value store (mutually exclusive with valkey.enabled) |
| externalKeyValueStore.existingSecret | object | `{"key":"valkey-password","name":""}` | reference to an existing secret holding the password. Leave name empty if the store requires no auth. |
| externalKeyValueStore.existingSecret.key | string | `"valkey-password"` | key within the existing secret that holds the password |
| externalKeyValueStore.existingSecret.name | string | `""` | name of the existing secret holding the password |
| externalKeyValueStore.host | string | `""` | hostname of the external key-value store (e.g. "my-redis.example.com") |
| externalKeyValueStore.port | int | `6379` | port of the external key-value store (default: the standard Redis/Valkey port) |
| externalKeyValueStore.tls | object | `{"caCertSecret":{"key":"ca.crt","name":""},"enabled":false,"insecureSkipVerify":false}` | TLS settings for the connection to the external key-value store |
| externalKeyValueStore.tls.caCertSecret | object | `{"key":"ca.crt","name":""}` | Leave name empty to use the system trust store (e.g. for publicly-trusted managed services). |
| externalKeyValueStore.tls.caCertSecret.key | string | `"ca.crt"` | key within the existing secret that holds the CA certificate |
| externalKeyValueStore.tls.caCertSecret.name | string | `""` | name of the existing secret holding the CA certificate |
| externalKeyValueStore.tls.enabled | bool | `false` | enable TLS for the connection |
| externalKeyValueStore.tls.insecureSkipVerify | bool | `false` | skip TLS certificate verification (insecure; only for self-signed certs in trusted networks) |
| externalKeyValueStore.username | string | `""` | optional ACL username for the external key-value store (leave empty for the default user) |
| features | object | `{"autoUpgrade":{"enabled":true},"debugTools":{"enabled":false,"image":{"pullPolicy":"IfNotPresent","registry":"docker.io","repository":"nicolaka/netshoot","tag":"v0.16"}},"nodeMetricsDashboard":{"enabled":false,"httproute":{"annotations":{},"enabled":false,"hostnames":[],"labels":{},"parentRefs":[]},"ingress":{"annotations":{},"enabled":false,"host":null,"ingressClassName":null,"labels":{},"tls":[]}}}` | feature toggles for the mogenius-operator |
| features.debugTools.enabled | bool | `false` | enable the debug tools container in the mogenius-operator pod |
| features.nodeMetricsDashboard | object | `{"enabled":false,"httproute":{"annotations":{},"enabled":false,"hostnames":[],"labels":{},"parentRefs":[]},"ingress":{"annotations":{},"enabled":false,"host":null,"ingressClassName":null,"labels":{},"tls":[]}}` | configure the integrated node metrics dashboard |
| features.nodeMetricsDashboard.enabled | bool | `false` | enable the node metrics dashboard |
| features.nodeMetricsDashboard.httproute | object | `{"annotations":{},"enabled":false,"hostnames":[],"labels":{},"parentRefs":[]}` | HTTPRoute settings for the node metrics dashboard |
| features.nodeMetricsDashboard.httproute.hostnames | list | `[]` | hostnames to place on the HTTPRoute |
| features.nodeMetricsDashboard.httproute.labels | object | `{}` | labels to place on the HTTPRoute |
| features.nodeMetricsDashboard.httproute.parentRefs | list | `[]` | parentRefs to place on the HTTPRoute |
| features.nodeMetricsDashboard.ingress | object | `{"annotations":{},"enabled":false,"host":null,"ingressClassName":null,"labels":{},"tls":[]}` | ingress settings for the node metrics dashboard |
| fullnameOverride | string | `"mogenius-operator"` |  |
| global.apiKeySecret | object | `{"secretKey":"API_KEY","secretName":"mogenius-operator-api-secret"}` | secret reference for the api-key (will be used if global.api_key is not set) |
| global.api_key | string | `nil` | the api key provided for your cluster by the mogenius platform (alternativly you can leave this empty and use global.apiKeySecret) |
| global.cluster_name | string | `nil` | the name you gave your cluster on the mogenius platform |
| goRuntime | object | `{"gcPercent":"50","memLimit":"180MiB"}` | Go runtime memory tuning |
| goRuntime.gcPercent | string | `"50"` | GC target percentage (GOGC). Lower = more frequent GC, less memory |
| goRuntime.memLimit | string | `"180MiB"` | Soft memory limit for the Go runtime (GOMEMLIMIT) |
| image | object | `{"pullPolicy":"IfNotPresent","registry":"ghcr.io","repository":"mogenius/mogenius-operator","tag":null}` | image settings for the mogenius-operator and nodemetrics container |
| image.tag | string | `nil` | tag of the image, if not set, the chart appVersion is used |
| metrics.enabled | bool | `false` | enable the Prometheus /metrics endpoint scraping (creates a Service for the ServiceMonitor) |
| metrics.serviceMonitor | object | `{"enabled":false,"interval":"30s","labels":{},"relabelings":[],"scrapeTimeout":"10s"}` | ServiceMonitor for prometheus-operator; requires metrics.enabled: true |
| metrics.serviceMonitor.labels | object | `{}` | labels added to the ServiceMonitor (use to match your Prometheus selector) |
| metrics.serviceMonitor.relabelings | list | `[]` | additional relabelings appended after the default instance relabeling |
| nodeMetrics.affinity | object | `{}` |  |
| nodeMetrics.containerSecurityContext.capabilities.add[0] | string | `"NET_ADMIN"` |  |
| nodeMetrics.containerSecurityContext.capabilities.add[1] | string | `"SYS_ADMIN"` |  |
| nodeMetrics.containerSecurityContext.capabilities.add[2] | string | `"BPF"` |  |
| nodeMetrics.containerSecurityContext.capabilities.add[3] | string | `"SYS_PTRACE"` |  |
| nodeMetrics.containerSecurityContext.readOnlyRootFilesystem | bool | `true` |  |
| nodeMetrics.enabled | bool | `true` | enable the node metrics daemonset |
| nodeMetrics.nodeSelector | object | `{}` |  |
| nodeMetrics.podLabels | object | `{}` |  |
| nodeMetrics.podSecurityContext | object | `{}` |  |
| nodeMetrics.resources | object | `{}` |  |
| nodeMetrics.tolerations[0].effect | string | `"NoSchedule"` |  |
| nodeMetrics.tolerations[0].key | string | `"node-role.kubernetes.io/control-plane"` |  |
| nodeMetrics.tolerations[0].operator | string | `"Exists"` |  |
| nodeSelector | object | `{}` |  |
| podLabels | object | `{}` | extra labels to be added to the pod |
| podSecurityContext | object | `{}` |  |
| probes.enabled | bool | `true` |  |
| probes.livenessProbe.enabled | bool | `true` |  |
| probes.livenessProbe.path | string | `"/healthz"` |  |
| probes.readinessProbe.enabled | bool | `true` |  |
| probes.readinessProbe.path | string | `"/healthz"` |  |
| probes.startupProbe.enabled | bool | `true` |  |
| probes.startupProbe.path | string | `"/healthz"` |  |
| rbac | object | `{"create":true,"rules":[]}` | permissions granted to the operator's ServiceAccount. The operator manages a cluster end to end, so the defaults are admin-equivalent. Both settings below let you cut that down; doing so is unsupported (see the chart README). |
| rbac.create | bool | `true` | create the operator's ClusterRole and ClusterRoleBinding. Set to false to grant the ServiceAccount permissions yourself, out of band. |
| rbac.rules | list | `[]` | replace the default blanket rule with your own ClusterRole rules. Whatever you leave out, the operator will hit as a Forbidden error at runtime. |
| resources | object | `{}` |  |
| revisionHistoryLimit | int | `10` |  |
| serviceAccount.name | string | `nil` | name of the ServiceAccount used by the operator (default: mogenius-operator-service-account-app) |
| tolerations | list | `[]` |  |
| valkey.affinity | object | `{}` |  |
| valkey.auth.password | string | `""` |  |
| valkey.containerSecurityContext.allowPrivilegeEscalation | bool | `false` |  |
| valkey.containerSecurityContext.capabilities.drop[0] | string | `"ALL"` |  |
| valkey.containerSecurityContext.readOnlyRootFilesystem | bool | `true` |  |
| valkey.containerSecurityContext.runAsGroup | int | `999` |  |
| valkey.containerSecurityContext.runAsNonRoot | bool | `true` |  |
| valkey.containerSecurityContext.runAsUser | int | `999` |  |
| valkey.customValkeyConf | string | `""` | optional additional Valkey configuration appended verbatim (e.g. "maxmemory 256mb\nmaxmemory-policy allkeys-lru") |
| valkey.enabled | bool | `true` | deploy the bundled Valkey instance. Set to false to use an external key-value store (see externalKeyValueStore) |
| valkey.image.registry | string | `"docker.io"` |  |
| valkey.image.repository | string | `"valkey/valkey"` |  |
| valkey.image.tag | string | `"9.1.1"` |  |
| valkey.imagePullPolicy | string | `"IfNotPresent"` |  |
| valkey.metrics.enabled | bool | `false` | enable the Prometheus metrics exporter sidecar (oliver006/redis_exporter) |
| valkey.metrics.exporter.containerSecurityContext.allowPrivilegeEscalation | bool | `false` |  |
| valkey.metrics.exporter.containerSecurityContext.capabilities.drop[0] | string | `"ALL"` |  |
| valkey.metrics.exporter.containerSecurityContext.readOnlyRootFilesystem | bool | `true` |  |
| valkey.metrics.exporter.containerSecurityContext.runAsNonRoot | bool | `true` |  |
| valkey.metrics.exporter.image.registry | string | `"docker.io"` |  |
| valkey.metrics.exporter.image.repository | string | `"oliver006/redis_exporter"` |  |
| valkey.metrics.exporter.image.tag | string | `"v1.88.0"` |  |
| valkey.metrics.exporter.resources | object | `{}` |  |
| valkey.metrics.serviceMonitor | object | `{"enabled":false,"interval":"30s","labels":{},"relabelings":[],"scrapeTimeout":"10s"}` | ServiceMonitor for prometheus-operator; requires metrics.enabled: true |
| valkey.metrics.serviceMonitor.labels | object | `{}` | labels added to the ServiceMonitor (use to match your Prometheus selector) |
| valkey.metrics.serviceMonitor.relabelings | list | `[]` | additional relabelings appended after the default instance relabeling |
| valkey.nodeSelector | object | `{}` |  |
| valkey.persistence.accessModes[0] | string | `"ReadWriteOnce"` |  |
| valkey.persistence.size | string | `"5Gi"` |  |
| valkey.persistence.storageClass | string | `""` | storage class to be used, default is empty which will use the class defined as standard |
| valkey.podLabels | object | `{}` |  |
| valkey.podSecurityContext.fsGroup | int | `999` |  |
| valkey.podSecurityContext.runAsGroup | int | `999` |  |
| valkey.podSecurityContext.runAsUser | int | `999` |  |
| valkey.podSecurityContext.seccompProfile.type | string | `"RuntimeDefault"` |  |
| valkey.port | int | `6379` |  |
| valkey.resources | object | `{}` |  |
| valkey.tolerations | list | `[]` |  |
| valkey.updateStrategy.type | string | `"Recreate"` |  |

## Add Mogenius Helm Repository, Update, and Deploy the Helm Chart

### Add the Helm Repository:

To add the Mogenius Helm repository and update it, run the following commands:

```
helm repo add mogenius https://github.com/mogenius/mogenius-operator
helm repo update mogenius
```

### Select the Correct Kubernetes Cluster Context

Ensure that the correct Kubernetes cluster context is selected before proceeding:

```
kubectl config current-context
# If the correct context is not selected:
kubectl config use-context <yourCluster>
```

### Install the Helm Chart

To install the `mogenius-operator` Helm chart, use the following command:

```
helm upgrade -i mogenius-operator mogenius/mogenius-operator -n mogenius --create-namespace --wait
```

Once installed, the following deployments will be created:
- `mogenius-operator`
- `mogenius-operator-valkey`

### Override Default Values

If you need to override default values, create a `values.yaml` file and use the following command:

```
helm upgrade -i mogenius-operator mogenius/mogenius-operator -n mogenius --create-namespace --wait -f values.yaml
```

#### Example `values.yaml`:

```yaml
global:
  cluster_name: your-cluster
  api_key: your-api-key
```

### Using an External Key-Value Store

By default the chart deploys a bundled Valkey instance. To use an existing external
Redis/Valkey-compatible key-value store instead, disable the bundled Valkey and enable
`externalKeyValueStore`:

```yaml
valkey:
  enabled: false

externalKeyValueStore:
  enabled: true
  host: my-redis.example.com
  # port defaults to 6379 (the standard Redis/Valkey port)
  port: 6379
  # optional ACL username (leave empty for the default user)
  username: appuser
  # the password is always read from an existing secret (no plaintext value).
  # Leave name empty if your store requires no authentication.
  existingSecret:
    name: my-redis-secret
    key: valkey-password
```

`valkey.enabled` and `externalKeyValueStore.enabled` are mutually exclusive — enable
exactly one. The chart rendering fails fast if both are enabled, if neither is enabled,
or if `externalKeyValueStore.enabled` is set without a `host`.

#### TLS

Enable TLS for the connection to the external store via `externalKeyValueStore.tls`:

```yaml
externalKeyValueStore:
  enabled: true
  host: my-redis.example.com
  port: 6380
  existingSecret:
    name: my-redis-secret
    key: valkey-password
  tls:
    enabled: true
    # For publicly-trusted managed services (e.g. AWS ElastiCache), the system
    # trust store is used and no further config is needed.
    #
    # For a private/self-signed CA, provide the CA certificate from an existing
    # secret. It is mounted into the operator (and node-metrics) pods and used to
    # verify the server certificate:
    caCertSecret:
      name: my-ca-secret
      key: ca.crt
    # Alternatively, skip verification entirely (insecure, only for trusted networks):
    # insecureSkipVerify: true
```

### Restricting the Operator's Permissions (Unsupported)

By default the chart grants the operator a ClusterRole with every verb on every
resource. That is deliberate: the product manages a cluster from A to Z, and the
permissions it needs are whatever its current feature set touches — deploying
workloads into any namespace, creating the namespaces themselves, installing Helm
charts and their CRDs and RBAC, projecting users and grants into RoleBindings,
provisioning storage, and mirroring every resource type in the cluster into the
platform UI. That set grows with the product.

If your security posture does not allow that, you can narrow it — but you own the
result. **A restricted operator is not a supported configuration.** New features, and
existing features you have not exercised yet, will fail with `Forbidden` errors from
the API server, and the fix is always to widen your own rules. Nothing in the chart
tracks which permissions the current version needs.

Two ways to do it. Replace the rules, keeping the chart's ClusterRole and binding:

```yaml
rbac:
  rules:
    # Cluster-wide reads: the operator runs an informer for every resource type the
    # API server serves, so the UI can display arbitrary objects. Narrowing this
    # empties parts of the UI instead of failing loudly.
    - apiGroups: ["*"]
      resources: ["*"]
      verbs: ["get", "list", "watch"]
    - apiGroups: [""]
      resources: ["pods/log"]
      verbs: ["get"]
    - apiGroups: [""]
      resources: ["pods/exec", "pods/attach", "pods/portforward"]
      verbs: ["create"]
    # ... plus every write your usage of the platform requires
```

Or take RBAC out of the chart's hands entirely and wire it up yourself (with your own
policy tooling, GitOps repo, or a pre-provisioned role):

```yaml
rbac:
  create: false
```

The ServiceAccount is still created either way, so you only need to bind permissions to
it. To see what the chart would grant before applying anything:

```
helm template mogenius-operator mogenius/mogenius-operator \
  -f values.yaml -s templates/rbac.yaml
```

### Upgrade the Helm Chart

To upgrade the `mogenius-operator` Helm chart from the repository, run:

```
helm repo update mogenius
helm install -i mogenius-operator mogenius/mogenius-operator -n mogenius --create-namespace --wait
```

#### Upgrade with Custom Values:

If you have custom values in a `values.yaml` file, use the following command:

```
helm upgrade -i mogenius-operator mogenius/mogenius-operator -n mogenius --create-namespace --wait -f values.yaml
```

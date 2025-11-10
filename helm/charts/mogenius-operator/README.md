# Helm Chart: mogenius-operator

![Version: 1.7.9](https://img.shields.io/badge/Version-1.7.9-informational?style=flat-square) ![AppVersion: 1.0](https://img.shields.io/badge/AppVersion-1.0-informational?style=flat-square)

**Homepage:** <https://mogenius.com>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Rüdiger Küpper @mogenius | <ruediger@mogenius.com> | <https://mogenius.com> |
| Lukas Hankeln @mogenius | <lukas@mogenius.com> | <https://mogenius.com> |
| Benedikt Iltisberger @mogenius | <bene@mogenius.com> | <https://mogenius.com> |
| Behrang Alavi @mogenius | <behrang@mogenius.com> | <https://mogenius.com> |

## Source Code

* <https://helm.mogenius.com/public>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| cluster | object | `{"domain":"cluster.local","readonly":{"enabled":false}}` | settings for the internal cluster communication |
| cluster.domain | string | `"cluster.local"` | the cluster domain, default for kubernetes is "cluster.local" |
| cluster.readonly.enabled | bool | `false` | readonly operator permissions true/false (default: false) |
| containerSecurityContext.capabilities.drop[0] | string | `"ALL"` |  |
| containerSecurityContext.privileged | bool | `true` |  |
| containerSecurityContext.readOnlyRootFilesystem | bool | `true` |  |
| envVars.MO_API_SERVER | string | `""` |  |
| envVars.MO_EVENT_SERVER | string | `""` |  |
| fullnameOverride | string | `"mogenius-operator"` |  |
| global.apiKeySecret | object | `{"secretKey":"API_KEY","secretName":"mogenius-operator-api-secret"}` | secret reference for the api-key (will be used if global.api_key is not set) |
| global.api_key | string | `nil` | the api key provided for your cluster by the mogenius platform (alternativly you can leave this empty and use global.apiKeySecret) |
| global.cluster_name | string | `nil` | the name you gave your cluster on the mogenius platform |
| global.stage | string | `"prod"` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.registry | string | `"ghcr.io"` |  |
| image.repository | string | `"mogenius/mogenius-operator"` |  |
| image.tag | string | `"v1.0.72"` |  |
| nodeMetrics.affinity | object | `{}` |  |
| nodeMetrics.containerSecurityContext.capabilities.add[0] | string | `"NET_ADMIN"` |  |
| nodeMetrics.containerSecurityContext.capabilities.add[1] | string | `"SYS_ADMIN"` |  |
| nodeMetrics.containerSecurityContext.privileged | bool | `true` |  |
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
| resources | object | `{}` |  |
| revisionHistoryLimit | int | `10` |  |
| tolerations | list | `[]` |  |
| valkey.affinity | object | `{}` |  |
| valkey.auth.password | string | `""` |  |
| valkey.containerSecurityContext.allowPrivilegeEscalation | bool | `false` |  |
| valkey.containerSecurityContext.capabilities.drop[0] | string | `"ALL"` |  |
| valkey.containerSecurityContext.readOnlyRootFilesystem | bool | `true` |  |
| valkey.containerSecurityContext.runAsGroup | int | `999` |  |
| valkey.containerSecurityContext.runAsNonRoot | bool | `true` |  |
| valkey.containerSecurityContext.runAsUser | int | `999` |  |
| valkey.enabled | bool | `true` |  |
| valkey.image.registry | string | `"docker.io"` |  |
| valkey.image.repository | string | `"valkey/valkey"` |  |
| valkey.image.tag | float | `8.1` |  |
| valkey.imagePullPolicy | string | `"IfNotPresent"` |  |
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
| valkey.updateStrategy.type | string | `"RollingUpdate"` |  |

## Add Mogenius Helm Repository, Update, and Deploy the Helm Chart

### Add the Helm Repository:

To add the Mogenius Helm repository and update it, run the following commands:

```
helm repo add mogenius https://helm.mogenius.com/public
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

package utils

import (
	_ "embed"
	"time"
)

const HELM_INDEX string = "https://helm.mogenius.com/public/index.yaml"
const NFS_POD_PREFIX string = "nfs-server-pod"

var ResourceResyncTime time.Duration = time.Minute * 30

// This object will initially created in secrets when the software is installed into the cluster for the first time (resource: secret -> mogenius/mogenius)
type ClusterSecret struct {
	ApiKey                string
	ClusterMfaId          string
	ClusterName           string
	RedisDataModelVersion string
}

type ResourceDescriptor struct {
	Kind       string `json:"kind"`
	Plural     string `json:"plural"`
	ApiVersion string `json:"apiVersion"`
	Namespaced bool   `json:"namespaced"`
}

type WorkloadSingleRequest struct {
	ResourceDescriptor
	Namespace    string `json:"namespace"`
	ResourceName string `json:"resourceName"`
}

type WorkloadChangeRequest struct {
	ResourceDescriptor
	Namespace string `json:"namespace"`
	YamlData  string `json:"yamlData"`
}

var DeploymentResource = ResourceDescriptor{
	Kind:       "Deployment",
	Plural:     "deployments",
	ApiVersion: "apps/v1",
	Namespaced: true,
}
var StatefulSetResource = ResourceDescriptor{
	Kind:       "StatefulSet",
	Plural:     "statefulsets",
	ApiVersion: "apps/v1",
	Namespaced: true,
}
var DaemonSetResource = ResourceDescriptor{
	Kind:       "DaemonSet",
	Plural:     "daemonsets",
	ApiVersion: "apps/v1",
	Namespaced: true,
}

var JobResource = ResourceDescriptor{
	Kind:       "Job",
	Plural:     "jobs",
	ApiVersion: "batch/v1",
	Namespaced: true,
}

var CronJobResource = ResourceDescriptor{
	Kind:       "CronJob",
	Plural:     "cronjobs",
	ApiVersion: "batch/v1",
	Namespaced: true,
}

var ReplicaSetResource = ResourceDescriptor{
	Kind:       "ReplicaSet",
	Plural:     "replicasets",
	ApiVersion: "apps/v1",
	Namespaced: true,
}

var NetworkPolicyResource = ResourceDescriptor{
	Kind:       "NetworkPolicy",
	Plural:     "networkpolicies",
	ApiVersion: "networking.k8s.io/v1",
	Namespaced: true,
}

var PodResource = ResourceDescriptor{
	Kind:       "Pod",
	Plural:     "pods",
	ApiVersion: "v1",
	Namespaced: true,
}

var IngressClassResource = ResourceDescriptor{
	Kind:       "IngressClass",
	Plural:     "ingressclasses",
	ApiVersion: "networking.k8s.io/v1",
	Namespaced: false,
}

var SecretResource = ResourceDescriptor{
	Kind:       "Secret",
	Plural:     "secrets",
	ApiVersion: "v1",
	Namespaced: true,
}

var NamespaceResource = ResourceDescriptor{
	Kind:       "Namespace",
	Plural:     "namespaces",
	ApiVersion: "v1",
	Namespaced: false,
}

var EventResource = ResourceDescriptor{
	Kind:       "Event",
	Plural:     "events",
	ApiVersion: "v1",
	Namespaced: true,
}

var NodeResource = ResourceDescriptor{
	Kind:       "Node",
	Plural:     "nodes",
	ApiVersion: "v1",
	Namespaced: false,
}

var WorkspaceResource = ResourceDescriptor{
	Kind:       "Workspace",
	Plural:     "workspaces",
	ApiVersion: "mogenius.com/v1alpha1",
	Namespaced: true,
}

var ConfigMapResource = ResourceDescriptor{
	Kind:       "ConfigMap",
	Plural:     "configmaps",
	ApiVersion: "v1",
	Namespaced: true,
}

var ServiceResource = ResourceDescriptor{
	Kind:       "Service",
	Plural:     "services",
	ApiVersion: "v1",
	Namespaced: true,
}

const STAGE_DEV = "dev"
const STAGE_PROD = "prod"

var ClusterProviderCached KubernetesProvider = UNKNOWN

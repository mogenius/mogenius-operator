package utils

import (
	_ "embed"
)

const HELM_INDEX string = "https://helm.mogenius.com/public/index.yaml"
const NFS_POD_PREFIX string = "nfs-server-pod"

// This object will initially created in secrets when the software is installed into the cluster for the first time (resource: secret -> mogenius/mogenius)
type ClusterSecret struct {
	ApiKey       string
	ClusterMfaId string
	ClusterName  string
}

type ResourceEntry struct {
	Kind      string  `json:"kind"`
	Name      string  `json:"name"`
	Group     string  `json:"group"`
	Version   string  `json:"version"`
	Namespace *string `json:"namespace"`
}

type ResourceData struct {
	Kind      string  `json:"kind"`
	Name      string  `json:"name"`
	Group     string  `json:"group"`
	Version   string  `json:"version"`
	Namespace *string `json:"namespace"`
	YamlData  string  `json:"yamlData"`
}

type ResourceItem struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Group        string `json:"group"`
	Version      string `json:"version"`
	ResourceName string `json:"resourceName"`
	Namespace    string `json:"namespace"`
}

var DeploymentResource = ResourceEntry{
	Kind:  "Deployment",
	Name:  "deployments",
	Group: "apps/v1",
}
var StatefulSetResource = ResourceEntry{
	Kind:  "StatefulSet",
	Name:  "statefulsets",
	Group: "apps/v1",
}
var DaemonSetResource = ResourceEntry{
	Kind:  "DaemonSet",
	Name:  "daemonsets",
	Group: "apps/v1",
}

var JobResource = ResourceEntry{
	Kind:  "Job",
	Name:  "jobs",
	Group: "batch/v1",
}

var CronJobResource = ResourceEntry{
	Kind:  "CronJob",
	Name:  "cronjobs",
	Group: "batch/v1",
}

var ReplicaSetResource = ResourceEntry{
	Kind:  "ReplicaSet",
	Name:  "replicasets",
	Group: "apps/v1",
}

var NetworkPolicyResource = ResourceEntry{
	Kind:  "NetworkPolicy",
	Name:  "networkpolicies",
	Group: "networking.k8s.io/v1",
}

var PodResource = ResourceEntry{
	Kind:  "Pod",
	Name:  "pods",
	Group: "v1",
}

var NamespaceResource = ResourceEntry{
	Kind:  "Namespace",
	Name:  "namespaces",
	Group: "v1",
}

var EventResource = ResourceEntry{
	Kind:  "Event",
	Name:  "events",
	Group: "v1",
}

var NodeResource = ResourceEntry{
	Kind:  "Node",
	Name:  "nodes",
	Group: "v1",
}

const STAGE_DEV = "dev"
const STAGE_PROD = "prod"
const STAGE_LOCAL = "local"

var ClusterProviderCached KubernetesProvider = UNKNOWN

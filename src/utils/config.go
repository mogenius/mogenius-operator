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

type SyncResourceEntry struct {
	Kind      string  `json:"kind"`
	Name      string  `json:"name"`
	Group     string  `json:"group"`
	Version   string  `json:"version"`
	Namespace *string `json:"namespace"`
}

type SyncResourceData struct {
	Kind      string  `json:"kind"`
	Name      string  `json:"name"`
	Group     string  `json:"group"`
	Version   string  `json:"version"`
	Namespace *string `json:"namespace"`
	YamlData  string  `json:"yamlData"`
}

type SyncResourceItem struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Group        string `json:"group"`
	Version      string `json:"version"`
	ResourceName string `json:"resourceName"`
	Namespace    string `json:"namespace"`
}

var DeploymentResource = SyncResourceEntry{
	Kind:  "Deployment",
	Name:  "deployments",
	Group: "apps/v1",
}
var StatefulSetResource = SyncResourceEntry{
	Kind:  "StatefulSet",
	Name:  "statefulsets",
	Group: "apps/v1",
}
var DaemonSetResource = SyncResourceEntry{
	Kind:  "DaemonSet",
	Name:  "daemonsets",
	Group: "apps/v1",
}

var JobResource = SyncResourceEntry{
	Kind:  "Job",
	Name:  "jobs",
	Group: "batch/v1",
}

var CronJobResource = SyncResourceEntry{
	Kind:  "CronJob",
	Name:  "cronjobs",
	Group: "batch/v1",
}

var ReplicaSetResource = SyncResourceEntry{
	Kind:  "ReplicaSet",
	Name:  "replicasets",
	Group: "apps/v1",
}

var NetworkPolicyResource = SyncResourceEntry{
	Kind:  "NetworkPolicy",
	Name:  "networkpolicies",
	Group: "networking.k8s.io/v1",
}

var PodResource = SyncResourceEntry{
	Kind:  "Pod",
	Name:  "pods",
	Group: "v1",
}

var NamespaceResource = SyncResourceEntry{
	Kind:  "Namespace",
	Name:  "namespaces",
	Group: "v1",
}

var EventResource = SyncResourceEntry{
	Kind:  "Event",
	Name:  "events",
	Group: "v1",
}

const STAGE_DEV = "dev"
const STAGE_PROD = "prod"
const STAGE_LOCAL = "local"

var ClusterProviderCached KubernetesProvider = UNKNOWN

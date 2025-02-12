package structs

type JobStateEnum string

const (
	JobStateFailed    JobStateEnum = "FAILED"
	JobStateSucceeded JobStateEnum = "SUCCEEDED"
	JobStateStarted   JobStateEnum = "STARTED"
	JobStatePending   JobStateEnum = "PENDING"
	JobStateCanceled  JobStateEnum = "CANCELED"
	JobStateTimeout   JobStateEnum = "TIMEOUT"
)

type HelmGetEnum string

const (
	HelmGetAll      HelmGetEnum = "all"
	HelmGetHooks    HelmGetEnum = "hooks"
	HelmGetManifest HelmGetEnum = "manifest"
	HelmGetNotes    HelmGetEnum = "notes"
	HelmGetValues   HelmGetEnum = "values"
)

const (
	PAT_INSTALL_METRICS_SERVER               string = "install-metrics-server"
	PAT_UNINSTALL_METRICS_SERVER             string = "uninstall-metrics-server"
	PAT_INSTALL_CERT_MANAGER                 string = "install-cert-manager"
	PAT_UNINSTALL_CERT_MANAGER               string = "uninstall-cert-manager"
	PAT_INSTALL_INGRESS_CONTROLLER_TREAFIK   string = "install-ingress-controller-traefik"
	PAT_UNINSTALL_INGRESS_CONTROLLER_TREAFIK string = "uninstall-ingress-controller-traefik"
	PAT_INSTALL_CLUSTER_ISSUER               string = "install-cluster-issuer"
	PAT_UNINSTALL_CLUSTER_ISSUER             string = "uninstall-cluster-issuer"
	PAT_INSTALL_TRAFFIC_COLLECTOR            string = "install-traffic-collector"
	PAT_UNINSTALL_TRAFFIC_COLLECTOR          string = "uninstall-traffic-collector"
	PAT_UPGRADE_TRAFFIC_COLLECTOR            string = "upgrade-traffic-collector"
	PAT_INSTALL_POD_STATS_COLLECTOR          string = "install-pod-stats-collector"
	PAT_UNINSTALL_POD_STATS_COLLECTOR        string = "uninstall-pod-stats-collector"
	PAT_UPGRADE_PODSTATS_COLLECTOR           string = "upgrade-pod-stats-collector"
	PAT_INSTALL_CONTAINER_REGISTRY           string = "install-container-registry"
	PAT_UNINSTALL_CONTAINER_REGISTRY         string = "uninstall-container-registry"
	PAT_INSTALL_EXTERNAL_SECRETS             string = "install-external-secrets"
	PAT_UNINSTALL_EXTERNAL_SECRETS           string = "uninstall-external-secrets"
	PAT_INSTALL_METALLB                      string = "install-metallb"
	PAT_UNINSTALL_METALLB                    string = "uninstall-metallb"

	PAT_FILES_UPLOAD string = "files/upload"

	PAT_LIVE_STREAM_NODES_TRAFFIC_REQUEST string = "live-stream/nodes-traffic"
	PAT_LIVE_STREAM_NODES_CPU_REQUEST     string = "live-stream/nodes-cpu"
	PAT_LIVE_STREAM_NODES_MEMORY_REQUEST  string = "live-stream/nodes-memory"
)

var BINARY_REQUEST_UPLOAD = []string{
	PAT_FILES_UPLOAD,
}

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
	PAT_CLUSTER_HELM_REPO_ADD              string = "cluster/helm-repo-add" // e.g. helm repo add mogenius https://helm.mogenius.com/public
	PAT_CLUSTER_HELM_REPO_PATCH            string = "cluster/helm-repo-patch"
	PAT_CLUSTER_HELM_REPO_UPDATE           string = "cluster/helm-repo-update"       // e.g. helm repo update
	PAT_CLUSTER_HELM_REPO_LIST             string = "cluster/helm-repo-list"         // e.g. helm repo list
	PAT_CLUSTER_HELM_REPO_REMOVE           string = "cluster/helm-chart-remove"      // e.g. helm repo remove mogenius
	PAT_CLUSTER_HELM_CHART_SEARCH          string = "cluster/helm-chart-search"      // e.g. helm search repo <name>
	PAT_CLUSTER_HELM_CHART_INSTALL         string = "cluster/helm-chart-install"     // e.g. helm install mogenius-traffic-collector mogenius/mogenius-traffic-collector -n mogenius
	PAT_CLUSTER_HELM_CHART_SHOW            string = "cluster/helm-chart-show"        // e.g. helm show all mogenius/mogenius-traffic-collector
	PAT_CLUSTER_HELM_CHART_VERSIONS        string = "cluster/helm-chart-versions"    // e.g. helm search repo mogenius/mogenius-traffic-collector --versions
	PAT_CLUSTER_HELM_RELEASE_UPGRADE       string = "cluster/helm-release-upgrade"   // e.g. helm upgrade mogenius-traffic-collector mogenius/mogenius-traffic-collector -n mogenius
	PAT_CLUSTER_HELM_RELEASE_UNINSTALL     string = "cluster/helm-release-uninstall" // e.g. helm uninstall mogenius-traffic-collector -n mogenius
	PAT_CLUSTER_HELM_RELEASE_LIST          string = "cluster/helm-release-list"      // e.g. helm list -n mogenius
	PAT_CLUSTER_HELM_RELEASE_STATUS        string = "cluster/helm-release-status"    // e.g. helm status mogenius-traffic-collector -n mogenius
	PAT_CLUSTER_HELM_RELEASE_HISTORY       string = "cluster/helm-release-history"   // e.g. helm history mogenius-traffic-collector -n mogenius
	PAT_CLUSTER_HELM_RELEASE_ROLLBACK      string = "cluster/helm-release-rollback"  // e.g. helm rollback mogenius-traffic-collector 1 -n mogenius
	PAT_CLUSTER_HELM_RELEASE_GET           string = "cluster/helm-release-get"       // e.g. helm get values mogenius-traffic-collector -n mogenius
	PAT_CLUSTER_HELM_RELEASE_GET_WORKLOADS string = "cluster/helm-release-get-workloads"

	// ##########################################################################################################################

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

var COMMAND_REQUESTS = []string{
	// REPO
	PAT_CLUSTER_HELM_REPO_ADD,
	PAT_CLUSTER_HELM_REPO_PATCH,
	PAT_CLUSTER_HELM_REPO_UPDATE,
	PAT_CLUSTER_HELM_REPO_LIST,
	PAT_CLUSTER_HELM_REPO_REMOVE,
	// CHART
	PAT_CLUSTER_HELM_CHART_SEARCH,
	PAT_CLUSTER_HELM_CHART_INSTALL,
	PAT_CLUSTER_HELM_CHART_SHOW,
	PAT_CLUSTER_HELM_CHART_VERSIONS,
	// RELEASE
	PAT_CLUSTER_HELM_RELEASE_UPGRADE,
	PAT_CLUSTER_HELM_RELEASE_UNINSTALL,
	PAT_CLUSTER_HELM_RELEASE_LIST,
	PAT_CLUSTER_HELM_RELEASE_STATUS,
	PAT_CLUSTER_HELM_RELEASE_HISTORY,
	PAT_CLUSTER_HELM_RELEASE_ROLLBACK,
	PAT_CLUSTER_HELM_RELEASE_GET,
	PAT_CLUSTER_HELM_RELEASE_GET_WORKLOADS,
}

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
	PAT_K8SNOTIFICATION             string = "K8sNotification"
	PAT_CLUSTERSTATUS               string = "ClusterStatus"
	PAT_CLUSTERRESOURCEINFO         string = "ClusterResourceInfo"
	PAT_KUBERNETESEVENT             string = "KubernetesEvent"
	PAT_UPGRADEK8SMANAGER           string = "UpgradeK8sManager"
	PAT_SERVICE_POD_EXISTS          string = "SERVICE_POD_EXISTS"
	PAT_SERVICE_PODS                string = "SERVICE_PODS"
	PAT_CLUSTER_FORCE_RECONNECT     string = "ClusterForceReconnect"
	PAT_CLUSTER_FORCE_DISCONNECT    string = "ClusterForceDisconnect"
	PAT_SYSTEM_CHECK                string = "SYSTEM_CHECK"
	PAT_SYSTEM_PRINT_CURRENT_CONFIG string = "print-current-config"

	PAT_IAC_FORCE_SYNC       string = "iac/force-sync"
	PAT_IAC_GET_STATUS       string = "iac/get-status"
	PAT_IAC_RESET_LOCAL_REPO string = "iac/reset-local-repo"
	PAT_IAC_RESET_FILE       string = "iac/reset-file"

	PAT_INSTALL_TRAFFIC_COLLECTOR            string = "install-traffic-collector"
	PAT_INSTALL_POD_STATS_COLLECTOR          string = "install-pod-stats-collector"
	PAT_INSTALL_METRICS_SERVER               string = "install-metrics-server"
	PAT_INSTALL_INGRESS_CONTROLLER_TREAFIK   string = "install-ingress-controller-traefik"
	PAT_INSTALL_CERT_MANAGER                 string = "install-cert-manager"
	PAT_INSTALL_CLUSTER_ISSUER               string = "install-cluster-issuer"
	PAT_INSTALL_CONTAINER_REGISTRY           string = "install-container-registry"
	PAT_INSTALL_EXTERNAL_SECRETS             string = "install-external-secrets"
	PAT_INSTALL_METALLB                      string = "install-metallb"
	PAT_INSTALL_KEPLER                       string = "install-kepler"
	PAT_UNINSTALL_TRAFFIC_COLLECTOR          string = "uninstall-traffic-collector"
	PAT_UNINSTALL_POD_STATS_COLLECTOR        string = "uninstall-pod-stats-collector"
	PAT_UNINSTALL_METRICS_SERVER             string = "uninstall-metrics-server"
	PAT_UNINSTALL_INGRESS_CONTROLLER_TREAFIK string = "uninstall-ingress-controller-traefik"
	PAT_UNINSTALL_CERT_MANAGER               string = "uninstall-cert-manager"
	PAT_UNINSTALL_CLUSTER_ISSUER             string = "uninstall-cluster-issuer"
	PAT_UNINSTALL_CONTAINER_REGISTRY         string = "uninstall-container-registry"
	PAT_UNINSTALL_EXTERNAL_SECRETS           string = "uninstall-external-secrets"
	PAT_UNINSTALL_METALLB                    string = "uninstall-metallb"
	PAT_UNINSTALL_KEPLER                     string = "uninstall-kepler"
	PAT_UPGRADE_TRAFFIC_COLLECTOR            string = "upgrade-traffic-collector"
	PAT_UPGRADE_PODSTATS_COLLECTOR           string = "upgrade-pod-stats-collector"
	PAT_UPGRADE_METRICS_SERVER               string = "upgrade-metrics-server"
	PAT_UPGRADE_INGRESS_CONTROLLER_TREAFIK   string = "upgrade-ingress-controller-traefik"
	PAT_UPGRADE_CERT_MANAGER                 string = "upgrade-cert-manager"
	PAT_UPGRADE_CONTAINER_REGISTRY           string = "upgrade-container-registry"
	PAT_UPGRADE_METALLB                      string = "upgrade-metallb"
	PAT_UPGRADE_KEPLER                       string = "upgrade-kepler"

	PAT_FILES_LIST          string = "files/list"
	PAT_FILES_DOWNLOAD      string = "files/download"
	PAT_FILES_CREATE_FOLDER string = "files/create-folder"
	PAT_FILES_RENAME        string = "files/rename"
	PAT_FILES_CHOWN         string = "files/chown"
	PAT_FILES_CHMOD         string = "files/chmod"
	PAT_FILES_DELETE        string = "files/delete"
	PAT_FILES_INFO          string = "files/info"

	PAT_CLUSTER_EXECUTE_HELM_CHART_TASK                 string = "cluster/execute-helm-chart-task"
	PAT_CLUSTER_UNINSTALL_HELM_CHART                    string = "cluster/uninstall-helm-chart"
	PAT_CLUSTER_TCP_UDP_CONFIGURATION                   string = "cluster/tcp-udp-configuration"
	PAT_CLUSTER_BACKUP                                  string = "cluster/backup"
	PAT_CLUSTER_RESTART                                 string = "cluster/restart"
	PAT_ENERGY_CONSUMPTION                              string = "cluster/energy-consumption"
	PAT_CLUSTER_SYNC_INFO                               string = "cluster/sync-info"
	PAT_CLUSTER_SYNC_UPDATE                             string = "cluster/sync-update"
	PAT_CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST string = "cluster/component-log-stream-connection-request"

	PAT_CLUSTER_WRITE_CONFIGMAP               string = "cluster/write-configmap"
	PAT_CLUSTER_READ_CONFIGMAP                string = "cluster/read-configmap"
	PAT_CLUSTER_LIST_CONFIGMAPS               string = "cluster/list-configmaps"
	PAT_CLUSTER_WRITE_DEPLOYMENT              string = "cluster/write-deployment"
	PAT_CLUSTER_READ_DEPLOYMENT               string = "cluster/read-deployment"
	PAT_CLUSTER_LIST_DEPLOYMENTS              string = "cluster/list-deployments"
	PAT_CLUSTER_WRITE_PERSISTENT_VOLUME_CLAIM string = "cluster/write-persistent-volume-claim"
	PAT_CLUSTER_READ_PERSISTENT_VOLUME_CLAIM  string = "cluster/read-persistent-volume-claim"
	PAT_CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS string = "cluster/list-persistent-volume-claims"

	PAT_CLUSTER_UPDATE_LOCAL_TLS_SECRET string = "cluster/update-local-tls-secret"

	PAT_STATS_TRAFFIC_FOR_POD_ALL                       string = "stats/traffic/all-for-pod"
	PAT_STATS_TRAFFIC_FOR_POD_SUM                       string = "stats/traffic/sum-for-pod"
	PAT_STATS_TRAFFIC_FOR_POD_LAST                      string = "stats/traffic/last-for-pod" // legacy-support TODO: REMOVE
	PAT_STATS_TRAFFIC_FOR_CONTROLLER_ALL                string = "stats/traffic/all-for-controller"
	PAT_STATS_TRAFFIC_FOR_CONTROLLER_SUM                string = "stats/traffic/sum-for-controller"
	PAT_STATS_TRAFFIC_FOR_CONTROLLER_LAST               string = "stats/traffic/last-for-controller" // legacy-support TODO: REMOVE
	PAT_STATS_TRAFFIC_FOR_NAMESPACE_ALL                 string = "stats/traffic/all-for-namespace"
	PAT_STATS_TRAFFIC_FOR_NAMESPACE_SUM                 string = "stats/traffic/sum-for-namespace"
	PAT_STATS_TRAFFIC_FOR_NAMESPACE_LAST                string = "stats/traffic/last-for-namespace" // legacy-support TODO: REMOVE
	PAT_STATS_TRAFFIC_FOR_CONTROLLER_SOCKET_CONNECTIONS string = "stats/traffic/for-controller-socket-connections"
	PAT_STATS_PODSTAT_FOR_POD_ALL                       string = "stats/podstat/all-for-pod"
	PAT_STATS_PODSTAT_FOR_POD_LAST                      string = "stats/podstat/last-for-pod"
	PAT_STATS_PODSTAT_FOR_CONTROLLER_ALL                string = "stats/podstat/all-for-controller"
	PAT_STATS_PODSTAT_FOR_CONTROLLER_LAST               string = "stats/podstat/last-for-controller"
	PAT_STATS_PODSTAT_FOR_NAMESPACE_ALL                 string = "stats/podstat/all-for-namespace"
	PAT_STATS_PODSTAT_FOR_NAMESPACE_LAST                string = "stats/podstat/last-for-namespace"

	PAT_METRICS_DEPLOYMENT_AVG_UTILIZATION string = "metrics/deployment/average-utilization"

	PAT_NAMESPACE_CREATE                string = "namespace/create"
	PAT_NAMESPACE_DELETE                string = "namespace/delete"
	PAT_NAMESPACE_SHUTDOWN              string = "namespace/shutdown"
	PAT_NAMESPACE_POD_IDS               string = "namespace/pod-ids"
	PAT_NAMESPACE_VALIDATE_CLUSTER_PODS string = "namespace/validate-cluster-pods"
	PAT_NAMESPACE_VALIDATE_PORTS        string = "namespace/validate-ports"
	PAT_NAMESPACE_LIST_ALL              string = "namespace/list-all"
	PAT_NAMESPACE_GATHER_ALL_RESOURCES  string = "namespace/gather-all-resources"
	PAT_NAMESPACE_BACKUP                string = "namespace/backup"
	PAT_NAMESPACE_RESTORE               string = "namespace/restore"
	PAT_NAMESPACE_RESOURCE_YAML         string = "namespace/resource-yaml"

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

	PAT_SERVICE_CREATE  string = "service/create"
	PAT_SERVICE_DELETE  string = "service/delete"
	PAT_SERVICE_POD_IDS string = "service/pod-ids"
	// PAT_SERVICE_SET_IMAGE       string = "service/set-image"
	PAT_SERVICE_LOG             string = "service/log"
	PAT_SERVICE_LOG_ERROR       string = "service/log-error"
	PAT_SERVICE_RESOURCE_STATUS string = "service/resource-status"
	PAT_SERVICE_RESTART         string = "service/restart"
	PAT_SERVICE_STOP            string = "service/stop"
	PAT_SERVICE_START           string = "service/start"
	PAT_SERVICE_UPDATE_SERVICE  string = "service/update-service"
	PAT_SERVICE_TRIGGER_JOB     string = "service/trigger-job"
	PAT_SERVICE_STATUS          string = "service/status"

	PAT_SERVICE_LOG_STREAM                               string = "service/log-stream"
	PAT_SERVICE_EXEC_SH_CONNECTION_REQUEST               string = "service/exec-sh-connection-request"
	PAT_SERVICE_LOG_STREAM_CONNECTION_REQUEST            string = "service/log-stream-connection-request"
	PAT_SERVICE_BUILD_LOG_STREAM_CONNECTION_REQUEST      string = "service/build-log-stream-connection-request"
	PAT_SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST      string = "service/pod-event-stream-connection-request"
	PAT_SERVICE_SCAN_IMAGE_LOG_STREAM_CONNECTION_REQUEST string = "service/scan-image-log-stream-connection-request"
	PAT_SERVICE_CLUSTER_TOOL_STREAM_CONNECTION_REQUEST   string = "service/cluster-tool-stream-connection-request"

	PAT_LIST_ALL_WORKLOADS          string = "list/all-workloads"
	PAT_GET_WORKLOAD_LIST           string = "get/workload-list"
	PAT_GET_NAMESPACE_WORKLOAD_LIST string = "get/namespace-workload-list"
	PAT_GET_LABELED_WORKLOAD_LIST   string = "get/labeled-workload-list"
	PAT_CREATE_NEW_WORKLOAD         string = "create/new-workload"
	PAT_DESCRIBE_WORKLOAD           string = "describe/workload"
	PAT_GET_WORKLOAD                string = "get/workload"
	PAT_GET_WORKLOAD_EXAMPLE        string = "get/workload-example"
	PAT_UPDATE_WORKLOAD             string = "update/workload"
	PAT_DELETE_WORKLOAD             string = "delete/workload"

	PAT_GET_WORKSPACES   = "get/workspaces"
	PAT_CREATE_WORKSPACE = "create/workspace"
	PAT_GET_WORKSPACE    = "get/workspace"
	PAT_UPDATE_WORKSPACE = "update/workspace"
	PAT_DELETE_WORKSPACE = "delete/workspace"

	PAT_STORAGE_CREATE_VOLUME   string = "storage/create-volume"
	PAT_STORAGE_DELETE_VOLUME   string = "storage/delete-volume"
	PAT_STORAGE_BACKUP_VOLUME   string = "storage/backup-volume"
	PAT_STORAGE_RESTORE_VOLUME  string = "storage/restore-volume"
	PAT_STORAGE_STATS           string = "storage/stats"
	PAT_STORAGE_NAMESPACE_STATS string = "storage/namespace/stats"
	PAT_STORAGE_STATUS          string = "storage/status"

	PAT_BUILDER_STATUS        string = "build/builder-status"
	PAT_BUILD_INFOS           string = "build/info"
	PAT_BUILD_LAST_INFOS      string = "build/last-infos"
	PAT_BUILD_LIST_ALL        string = "build/list-all"
	PAT_BUILD_LIST_BY_PROJECT string = "build/list-by-project"
	PAT_BUILD_ADD             string = "build/add"
	// PAT_BUILD_SCAN                     string = "build/scan"
	PAT_BUILD_CANCEL                string = "build/cancel"
	PAT_BUILD_DELETE                string = "build/delete"
	PAT_BUILD_LAST_JOB_OF_SERVICES  string = "build/last-job-of-services"
	PAT_BUILD_JOB_LIST_OF_SERVICE   string = "build/job-list-of-service"
	PAT_BUILD_DELETE_ALL_OF_SERVICE string = "build/delete-of-service"
	// PAT_BUILD_LAST_JOB_INFO_OF_SERVICE string = "build/last-job-info-of-service"

	PAT_LOG_LIST_ALL string = "log/list-all"

	PAT_EXEC_SHELL string = "exec/shell"

	PAT_FILES_UPLOAD string = "files/upload"

	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// External Secrets
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	PAT_EXTERNAL_SECRET_STORE_CREATE           string = "external-secret-store/create"
	PAT_EXTERNAL_SECRET_STORE_LIST             string = "external-secret-store/list"
	PAT_EXTERNAL_SECRET_STORE_DELETE           string = "external-secret-store/delete"
	PAT_EXTERNAL_SECRET_LIST_AVAILABLE_SECRETS string = "external-secret/list-available-secrets"

	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// Labeled Network Policies
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	PAT_ATTACH_LABELED_NETWORK_POLICY        string = "attach/labeled_network_policy"
	PAT_DETACH_LABELED_NETWORK_POLICY        string = "detach/labeled_network_policy"
	PAT_LIST_LABELED_NETWORK_POLICY_PORTS    string = "list/labeled_network_policy_ports"
	PAT_LIST_CONFLICTING_NETWORK_POLICIES    string = "list/conflicting_network_policies"
	PAT_LIST_CONTROLLER_NETWORK_POLICIES     string = "list/controller_network_policies"
	PAT_REMOVE_CONFLICTING_NETWORK_POLICIES  string = "remove/conflicting_network_policies"
	PAT_UPDATE_NETWORK_POLICIES_TEMPLATE     string = "update/network_policies_template"
	PAT_LIST_ALL_NETWORK_POLICIES            string = "list/all_network_policies"
	PAT_LIST_NAMESPACE_NETWORK_POLICIES      string = "list/namespace_network_policies"
	PAT_ENFORCE_NETWORK_POLICY_MANAGER       string = "enforce/network_policy_manager"
	PAT_DISABLE_NETWORK_POLICY_MANAGER       string = "disable/network_policy_manager"
	PAT_REMOVE_UNMANAGED_NETWORK_POLICIES    string = "remove/unmanaged_network_policies"
	PAT_LIST_ONLY_NAMESPACE_NETWORK_POLICIES string = "list/only_namespace_network_policies"
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// Cronjobs
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	PAT_LIST_CRONJOB_JOBS string = "list/cronjob-jobs"

	PAT_LIVE_STREAM_NODES_TRAFFIC_REQUEST string = "live-stream/nodes-traffic"
	PAT_LIVE_STREAM_NODES_CPU_REQUEST     string = "live-stream/nodes-cpu"
	PAT_LIVE_STREAM_NODES_MEMORY_REQUEST  string = "live-stream/nodes-memory"
)

var BINARY_REQUEST_UPLOAD = []string{
	PAT_FILES_UPLOAD,
}

var COMMAND_REQUESTS = []string{
	PAT_K8SNOTIFICATION,
	PAT_CLUSTERSTATUS,
	PAT_CLUSTERRESOURCEINFO,
	PAT_KUBERNETESEVENT,
	PAT_UPGRADEK8SMANAGER,
	PAT_SERVICE_POD_EXISTS,
	PAT_SERVICE_PODS,
	PAT_CLUSTER_FORCE_RECONNECT,
	PAT_CLUSTER_FORCE_DISCONNECT,
	PAT_SYSTEM_CHECK,
	PAT_SYSTEM_PRINT_CURRENT_CONFIG,

	PAT_IAC_FORCE_SYNC,
	PAT_IAC_GET_STATUS,
	PAT_IAC_RESET_LOCAL_REPO,
	PAT_IAC_RESET_FILE,

	PAT_INSTALL_TRAFFIC_COLLECTOR,
	PAT_INSTALL_POD_STATS_COLLECTOR,
	PAT_INSTALL_METRICS_SERVER,
	PAT_INSTALL_INGRESS_CONTROLLER_TREAFIK,
	PAT_INSTALL_CERT_MANAGER,
	PAT_INSTALL_CLUSTER_ISSUER,
	PAT_INSTALL_CONTAINER_REGISTRY,
	PAT_INSTALL_EXTERNAL_SECRETS,
	PAT_INSTALL_METALLB,
	PAT_INSTALL_KEPLER,
	PAT_UNINSTALL_TRAFFIC_COLLECTOR,
	PAT_UNINSTALL_POD_STATS_COLLECTOR,
	PAT_UNINSTALL_METRICS_SERVER,
	PAT_UNINSTALL_INGRESS_CONTROLLER_TREAFIK,
	PAT_UNINSTALL_CERT_MANAGER,
	PAT_UNINSTALL_CLUSTER_ISSUER,
	PAT_UNINSTALL_CONTAINER_REGISTRY,
	PAT_UNINSTALL_EXTERNAL_SECRETS,
	PAT_UNINSTALL_METALLB,
	PAT_UNINSTALL_KEPLER,
	PAT_UPGRADE_TRAFFIC_COLLECTOR,
	PAT_UPGRADE_PODSTATS_COLLECTOR,
	PAT_UPGRADE_METRICS_SERVER,
	PAT_UPGRADE_INGRESS_CONTROLLER_TREAFIK,
	PAT_UPGRADE_CERT_MANAGER,
	PAT_UPGRADE_CONTAINER_REGISTRY,
	PAT_UPGRADE_METALLB,
	PAT_UPGRADE_KEPLER,

	PAT_FILES_LIST,
	PAT_FILES_DOWNLOAD,
	PAT_FILES_CREATE_FOLDER,
	PAT_FILES_RENAME,
	PAT_FILES_CHOWN,
	PAT_FILES_CHMOD,
	PAT_FILES_DELETE,
	PAT_FILES_INFO,

	PAT_CLUSTER_EXECUTE_HELM_CHART_TASK,
	PAT_CLUSTER_UNINSTALL_HELM_CHART,
	PAT_CLUSTER_TCP_UDP_CONFIGURATION,
	PAT_CLUSTER_BACKUP,
	PAT_CLUSTER_RESTART,
	PAT_ENERGY_CONSUMPTION,
	PAT_CLUSTER_SYNC_INFO,
	PAT_CLUSTER_SYNC_UPDATE,
	PAT_CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST,
	PAT_CLUSTER_WRITE_CONFIGMAP,
	PAT_CLUSTER_READ_CONFIGMAP,
	PAT_CLUSTER_LIST_CONFIGMAPS,
	PAT_CLUSTER_WRITE_DEPLOYMENT,
	PAT_CLUSTER_READ_DEPLOYMENT,
	PAT_CLUSTER_LIST_DEPLOYMENTS,
	PAT_CLUSTER_WRITE_PERSISTENT_VOLUME_CLAIM,
	PAT_CLUSTER_READ_PERSISTENT_VOLUME_CLAIM,
	PAT_CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS,

	PAT_CLUSTER_UPDATE_LOCAL_TLS_SECRET,

	PAT_STATS_TRAFFIC_FOR_POD_ALL,
	PAT_STATS_TRAFFIC_FOR_POD_SUM,
	PAT_STATS_TRAFFIC_FOR_CONTROLLER_ALL,
	PAT_STATS_TRAFFIC_FOR_CONTROLLER_SUM,
	PAT_STATS_PODSTAT_FOR_POD_ALL,
	PAT_STATS_PODSTAT_FOR_POD_LAST,
	PAT_STATS_PODSTAT_FOR_CONTROLLER_ALL,
	PAT_STATS_PODSTAT_FOR_CONTROLLER_LAST,
	PAT_STATS_TRAFFIC_FOR_NAMESPACE_ALL,
	PAT_STATS_TRAFFIC_FOR_NAMESPACE_SUM,
	PAT_STATS_TRAFFIC_FOR_CONTROLLER_SOCKET_CONNECTIONS,
	PAT_STATS_PODSTAT_FOR_NAMESPACE_ALL,
	PAT_STATS_PODSTAT_FOR_NAMESPACE_LAST,
	PAT_STATS_TRAFFIC_FOR_POD_LAST,        // legacy-support TODO: REMOVE
	PAT_STATS_TRAFFIC_FOR_CONTROLLER_LAST, // legacy-support TODO: REMOVE
	PAT_STATS_TRAFFIC_FOR_NAMESPACE_LAST,  // legacy-support TODO: REMOVE

	PAT_METRICS_DEPLOYMENT_AVG_UTILIZATION,

	PAT_NAMESPACE_CREATE,
	PAT_NAMESPACE_DELETE,
	PAT_NAMESPACE_SHUTDOWN,
	PAT_NAMESPACE_POD_IDS,
	PAT_NAMESPACE_VALIDATE_CLUSTER_PODS,
	PAT_NAMESPACE_VALIDATE_PORTS,
	PAT_NAMESPACE_LIST_ALL,
	PAT_NAMESPACE_GATHER_ALL_RESOURCES,
	PAT_NAMESPACE_BACKUP,
	PAT_NAMESPACE_RESTORE,
	PAT_NAMESPACE_RESOURCE_YAML,

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

	PAT_SERVICE_CREATE,
	PAT_SERVICE_DELETE,
	PAT_SERVICE_POD_IDS,
	// PAT_SERVICE_SET_IMAGE,
	PAT_SERVICE_LOG,
	PAT_SERVICE_LOG_ERROR,
	PAT_SERVICE_RESOURCE_STATUS,
	PAT_SERVICE_RESTART,
	PAT_SERVICE_STOP,
	PAT_SERVICE_START,
	PAT_SERVICE_UPDATE_SERVICE,
	PAT_SERVICE_TRIGGER_JOB,
	PAT_SERVICE_STATUS,

	PAT_SERVICE_LOG_STREAM,
	PAT_SERVICE_EXEC_SH_CONNECTION_REQUEST,
	PAT_SERVICE_LOG_STREAM_CONNECTION_REQUEST,
	PAT_SERVICE_BUILD_LOG_STREAM_CONNECTION_REQUEST,
	PAT_SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST,
	PAT_SERVICE_SCAN_IMAGE_LOG_STREAM_CONNECTION_REQUEST,
	PAT_SERVICE_CLUSTER_TOOL_STREAM_CONNECTION_REQUEST,

	PAT_LIST_ALL_WORKLOADS,
	PAT_GET_WORKLOAD_LIST,
	PAT_GET_NAMESPACE_WORKLOAD_LIST,
	PAT_GET_LABELED_WORKLOAD_LIST,
	PAT_CREATE_NEW_WORKLOAD,
	PAT_UPDATE_WORKLOAD,
	PAT_GET_WORKLOAD,
	PAT_GET_WORKLOAD_EXAMPLE,
	PAT_DELETE_WORKLOAD,
	PAT_DESCRIBE_WORKLOAD,

	PAT_GET_WORKSPACES,
	PAT_CREATE_WORKSPACE,
	PAT_GET_WORKSPACE,
	PAT_UPDATE_WORKSPACE,
	PAT_DELETE_WORKSPACE,

	PAT_STORAGE_CREATE_VOLUME,
	PAT_STORAGE_DELETE_VOLUME,
	PAT_STORAGE_BACKUP_VOLUME,
	PAT_STORAGE_RESTORE_VOLUME,
	PAT_STORAGE_STATS,
	PAT_STORAGE_NAMESPACE_STATS,
	PAT_STORAGE_STATUS,

	PAT_BUILDER_STATUS,
	PAT_BUILD_INFOS,
	PAT_BUILD_LAST_INFOS,
	PAT_BUILD_LIST_ALL,
	PAT_BUILD_LIST_BY_PROJECT,
	PAT_BUILD_ADD,
	PAT_BUILD_CANCEL,
	PAT_BUILD_DELETE,
	PAT_BUILD_LAST_JOB_OF_SERVICES,
	PAT_BUILD_JOB_LIST_OF_SERVICE,
	PAT_BUILD_DELETE_ALL_OF_SERVICE,

	PAT_EXEC_SHELL,

	PAT_LOG_LIST_ALL,

	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// External Secrets
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	PAT_EXTERNAL_SECRET_STORE_CREATE,
	PAT_EXTERNAL_SECRET_STORE_LIST,
	PAT_EXTERNAL_SECRET_STORE_DELETE,
	PAT_EXTERNAL_SECRET_LIST_AVAILABLE_SECRETS,

	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// Labeled Network Policies
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	PAT_ATTACH_LABELED_NETWORK_POLICY,
	PAT_DETACH_LABELED_NETWORK_POLICY,
	PAT_LIST_LABELED_NETWORK_POLICY_PORTS,
	PAT_LIST_CONFLICTING_NETWORK_POLICIES,
	PAT_LIST_CONTROLLER_NETWORK_POLICIES,
	PAT_REMOVE_CONFLICTING_NETWORK_POLICIES,
	PAT_UPDATE_NETWORK_POLICIES_TEMPLATE,
	PAT_LIST_ALL_NETWORK_POLICIES,
	PAT_LIST_NAMESPACE_NETWORK_POLICIES,
	PAT_ENFORCE_NETWORK_POLICY_MANAGER,
	PAT_DISABLE_NETWORK_POLICY_MANAGER,
	PAT_REMOVE_UNMANAGED_NETWORK_POLICIES,
	PAT_LIST_ONLY_NAMESPACE_NETWORK_POLICIES,

	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// Cronjobs
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	PAT_LIST_CRONJOB_JOBS,

	PAT_LIVE_STREAM_NODES_TRAFFIC_REQUEST,
	PAT_LIVE_STREAM_NODES_CPU_REQUEST,
	PAT_LIVE_STREAM_NODES_MEMORY_REQUEST,
}

var SUPPRESSED_OUTPUT_PATTERN = []string{
	PAT_CLUSTERRESOURCEINFO,
	PAT_SERVICE_LOG_STREAM_CONNECTION_REQUEST,
	PAT_SERVICE_STATUS,
	PAT_STORAGE_STATS,
	PAT_STORAGE_STATUS,
	PAT_STORAGE_NAMESPACE_STATS,
	PAT_CLUSTER_UPDATE_LOCAL_TLS_SECRET,
	// PAT_BUILD_LAST_JOB_OF_SERVICES,
	// PAT_BUILD_SCAN,
	PAT_SYSTEM_CHECK,
	PAT_LIST_CRONJOB_JOBS,
}

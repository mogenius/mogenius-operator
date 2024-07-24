package structs

type SystemCheckStatus string

const (
	UNKNOWN_STATUS SystemCheckStatus = "UNKNOWN_STATUS"
	INSTALLING     SystemCheckStatus = "INSTALLING"
	UNINSTALLING   SystemCheckStatus = "UNINSTALLING"
	NOT_INSTALLED  SystemCheckStatus = "NOT_INSTALLED"
	INSTALLED      SystemCheckStatus = "INSTALLED"
)

type HelmTaskEnum string

const (
	HelmInstall   HelmTaskEnum = "install"
	HelmUpgrade   HelmTaskEnum = "upgrade"
	HelmUninstall HelmTaskEnum = "uninstall"
)

type ComponentEnum string

const (
	ComponentAll        ComponentEnum = "all"
	ComponentIacManager ComponentEnum = "iac"
	ComponentDb         ComponentEnum = "db"
	ComponentDbStats    ComponentEnum = "db-stats"
	ComponentCrds       ComponentEnum = "crds"
	ComponentKubernetes ComponentEnum = "kubernetes"
	ComponentServices   ComponentEnum = "services"
)

type JobStateEnum string

const (
	JobStateFailed    JobStateEnum = "FAILED"
	JobStateSucceeded JobStateEnum = "SUCCEEDED"
	JobStateStarted   JobStateEnum = "STARTED"
	JobStatePending   JobStateEnum = "PENDING"
	JobStateCanceled  JobStateEnum = "CANCELED"
	JobStateTimeout   JobStateEnum = "TIMEOUT"
)

const (
	PAT_K8SNOTIFICATION         string = "K8sNotification"
	PAT_CLUSTERSTATUS           string = "ClusterStatus"
	PAT_CLUSTERRESOURCEINFO     string = "ClusterResourceInfo"
	PAT_KUBERNETESEVENT         string = "KubernetesEvent"
	PAT_UPGRADEK8SMANAGER       string = "UpgradeK8sManager"
	PAT_SERVICE_POD_EXISTS      string = "SERVICE_POD_EXISTS"
	PAT_SERVICE_PODS            string = "SERVICE_PODS"
	PAT_CLUSTER_FORCE_RECONNECT string = "ClusterForceReconnect"
	PAT_SYSTEM_CHECK            string = "SYSTEM_CHECK"

	PAT_INSTALL_LOCAL_DEV_COMPONENTS         string = "install-local-dev-components"
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

	PAT_STATS_TRAFFIC_FOR_POD_ALL         string = "stats/traffic/all-for-pod"
	PAT_STATS_TRAFFIC_FOR_POD_SUM         string = "stats/traffic/sum-for-pod"
	PAT_STATS_TRAFFIC_FOR_POD_LAST        string = "stats/traffic/last-for-pod" // legacy-support TODO: REMOVE
	PAT_STATS_TRAFFIC_FOR_CONTROLLER_ALL  string = "stats/traffic/all-for-controller"
	PAT_STATS_TRAFFIC_FOR_CONTROLLER_SUM  string = "stats/traffic/sum-for-controller"
	PAT_STATS_TRAFFIC_FOR_CONTROLLER_LAST string = "stats/traffic/last-for-controller" // legacy-support TODO: REMOVE
	PAT_STATS_TRAFFIC_FOR_NAMESPACE_ALL   string = "stats/traffic/all-for-namespace"
	PAT_STATS_TRAFFIC_FOR_NAMESPACE_SUM   string = "stats/traffic/sum-for-namespace"
	PAT_STATS_TRAFFIC_FOR_NAMESPACE_LAST  string = "stats/traffic/last-for-namespace" // legacy-support TODO: REMOVE
	PAT_STATS_PODSTAT_FOR_POD_ALL         string = "stats/podstat/all-for-pod"
	PAT_STATS_PODSTAT_FOR_POD_LAST        string = "stats/podstat/last-for-pod"
	PAT_STATS_PODSTAT_FOR_CONTROLLER_ALL  string = "stats/podstat/all-for-controller"
	PAT_STATS_PODSTAT_FOR_CONTROLLER_LAST string = "stats/podstat/last-for-controller"
	PAT_STATS_PODSTAT_FOR_NAMESPACE_ALL   string = "stats/podstat/all-for-namespace"
	PAT_STATS_PODSTAT_FOR_NAMESPACE_LAST  string = "stats/podstat/last-for-namespace"
	PAT_STATS_CHART_FOR_POD               string = "stats/chart/for-pod"

	PAT_PROJECT_CREATE string = "project/create"
	PAT_PROJECT_UPDATE string = "project/update"
	PAT_PROJECT_DELETE string = "project/delete"
	PAT_PROJECT_LIST   string = "project/list"
	PAT_PROJECT_COUNT  string = "project/count"

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
	PAT_SERVICE_OPERATOR_LOG_STREAM_CONNECTION_REQUEST   string = "service/operator-log-stream-connection-request"
	PAT_SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST      string = "service/pod-event-stream-connection-request"
	PAT_SERVICE_SCAN_IMAGE_LOG_STREAM_CONNECTION_REQUEST string = "service/scan-image-log-stream-connection-request"
	PAT_SERVICE_CLUSTER_TOOL_STREAM_CONNECTION_REQUEST   string = "service/cluster-tool-stream-connection-request"

	PAT_LIST_CREATE_TEMPLATES string = "list/create-templates"

	PAT_LIST_NAMESPACES                  string = "list/namespaces"
	PAT_LIST_DEPLOYMENTS                 string = "list/deployments"
	PAT_LIST_SERVICES                    string = "list/services"
	PAT_LIST_PODS                        string = "list/pods"
	PAT_LIST_INGRESSES                   string = "list/ingresses"
	PAT_LIST_CONFIGMAPS                  string = "list/configmaps"
	PAT_LIST_SECRETS                     string = "list/secrets"
	PAT_LIST_NODES                       string = "list/nodes"
	PAT_LIST_DAEMONSETS                  string = "list/daemonsets"
	PAT_LIST_STATEFULSETS                string = "list/statefulsets"
	PAT_LIST_JOBS                        string = "list/jobs"
	PAT_LIST_CRONJOBS                    string = "list/cronjobs"
	PAT_LIST_REPLICASETS                 string = "list/replicasets"
	PAT_LIST_PERSISTENT_VOLUMES          string = "list/persistent_volumes"
	PAT_LIST_PERSISTENT_VOLUME_CLAIMS    string = "list/persistent_volume_claims"
	PAT_LIST_HORIZONTAL_POD_AUTOSCALERS  string = "list/horizontal_pod_autoscalers"
	PAT_LIST_EVENTS                      string = "list/events"
	PAT_LIST_CERTIFICATES                string = "list/certificates"
	PAT_LIST_CERTIFICATEREQUESTS         string = "list/certificaterequests"
	PAT_LIST_ORDERS                      string = "list/orders"
	PAT_LIST_ISSUERS                     string = "list/issuers"
	PAT_LIST_CLUSTERISSUERS              string = "list/clusterissuers"
	PAT_LIST_SERVICE_ACCOUNT             string = "list/service_account"
	PAT_LIST_ROLE                        string = "list/role"
	PAT_LIST_ROLE_BINDING                string = "list/role_binding"
	PAT_LIST_CLUSTER_ROLE                string = "list/cluster_role"
	PAT_LIST_CLUSTER_ROLE_BINDING        string = "list/cluster_role_binding"
	PAT_LIST_VOLUME_ATTACHMENT           string = "list/volume_attachment"
	PAT_LIST_NETWORK_POLICY              string = "list/network_policy"
	PAT_LIST_STORAGE_CLASS               string = "list/storage_class"
	PAT_LIST_CUSTOM_RESOURCE_DEFINITIONS string = "list/custom_resource_definitions"
	PAT_LIST_ENDPOINTS                   string = "list/endpoints"
	PAT_LIST_LEASES                      string = "list/leases"
	PAT_LIST_PRIORITYCLASSES             string = "list/priorityclasses"
	PAT_LIST_VOLUMESNAPSHOTS             string = "list/volumesnapshots"
	PAT_LIST_RESOURCEQUOTAS              string = "list/resourcequotas"

	// PAT_CREATE_NAMESPACE  string = "create/namespace"
	// PAT_CREATE_DEPLOYMENT string = "create/deployment"
	// PAT_CREATE_SERVICE                     string = "create/service"
	// PAT_CREATE_POD                         string = "create/pod"
	// PAT_CREATE_INGRESS                     string = "create/ingress"
	// PAT_CREATE_CONFIGMAP string = "create/configmap"
	// PAT_CREATE_SECRET                      string = "create/secret"
	// PAT_CREATE_DAEMONSET                   string = "create/daemonset"
	// PAT_CREATE_STATEFULSET                 string = "create/statefulset"
	// PAT_CREATE_JOB                         string = "create/job"
	// PAT_CREATE_CRONJOB                     string = "create/cronjob"
	// PAT_CREATE_REPLICASET                  string = "create/replicaset"
	// PAT_CREATE_PERSISTENT_VOLUME           string = "create/persistent_volume"
	// PAT_CREATE_PERSISTENT_VOLUME_CLAIM     string = "create/persistent_volume_claim"
	// PAT_CREATE_HORIZONTAL_POD_AUTOSCALER   string = "create/horizontal_pod_autoscaler"
	// PAT_CREATE_CERTIFICATE                 string = "create/certificate"
	// PAT_CREATE_CERTIFICATEREQUEST          string = "create/certificaterequest"
	// PAT_CREATE_ORDER                       string = "create/order"
	// PAT_CREATE_ISSUER                      string = "create/issuer"
	// PAT_CREATE_CLUSTERISSUER               string = "create/clusterissuer"
	// PAT_CREATE_SERVICE_ACCOUNT             string = "create/service_account"
	// PAT_CREATE_ROLE                        string = "create/role"
	// PAT_CREATE_ROLE_BINDING                string = "create/role_binding"
	// PAT_CREATE_CLUSTER_ROLE                string = "create/cluster_role"
	// PAT_CREATE_CLUSTER_ROLE_BINDING        string = "create/cluster_role_binding"
	// PAT_CREATE_VOLUME_ATTACHMENT           string = "create/volume_attachment"
	// PAT_CREATE_NETWORK_POLICY              string = "create/network_policy"
	// PAT_CREATE_STORAGE_CLASS               string = "create/storage_class"
	// PAT_CREATE_CUSTOM_RESOURCE_DEFINITIONS string = "create/custom_resource_definitions"
	// PAT_CREATE_ENDPOINTS                   string = "create/endpoints"
	// PAT_CREATE_LEASES                      string = "create/leases"
	// PAT_CREATE_PRIORITYCLASSES             string = "create/priorityclasses"
	// PAT_CREATE_VOLUMESNAPSHOTS             string = "create/volumesnapshots"
	// PAT_CREATE_RESOURCEQUOTAS              string = "create/resourcequotas"

	PAT_DESCRIBE_NAMESPACE                   string = "describe/namespace"
	PAT_DESCRIBE_DEPLOYMENT                  string = "describe/deployment"
	PAT_DESCRIBE_SERVICE                     string = "describe/service"
	PAT_DESCRIBE_POD                         string = "describe/pod"
	PAT_DESCRIBE_INGRESS                     string = "describe/ingress"
	PAT_DESCRIBE_CONFIGMAP                   string = "describe/configmap"
	PAT_DESCRIBE_SECRET                      string = "describe/secret"
	PAT_DESCRIBE_NODE                        string = "describe/node"
	PAT_DESCRIBE_DAEMONSET                   string = "describe/daemonset"
	PAT_DESCRIBE_STATEFULSET                 string = "describe/statefulset"
	PAT_DESCRIBE_JOB                         string = "describe/job"
	PAT_DESCRIBE_CRONJOB                     string = "describe/cronjob"
	PAT_DESCRIBE_REPLICASET                  string = "describe/replicaset"
	PAT_DESCRIBE_PERSISTENT_VOLUME           string = "describe/persistent_volume"
	PAT_DESCRIBE_PERSISTENT_VOLUME_CLAIM     string = "describe/persistent_volume_claim"
	PAT_DESCRIBE_HORIZONTAL_POD_AUTOSCALER   string = "describe/horizontal_pod_autoscaler"
	PAT_DESCRIBE_EVENT                       string = "describe/event"
	PAT_DESCRIBE_CERTIFICATE                 string = "describe/certificate"
	PAT_DESCRIBE_CERTIFICATEREQUEST          string = "describe/certificaterequest"
	PAT_DESCRIBE_ORDER                       string = "describe/order"
	PAT_DESCRIBE_ISSUER                      string = "describe/issuer"
	PAT_DESCRIBE_CLUSTERISSUER               string = "describe/clusterissuer"
	PAT_DESCRIBE_SERVICE_ACCOUNT             string = "describe/service_account"
	PAT_DESCRIBE_ROLE                        string = "describe/role"
	PAT_DESCRIBE_ROLE_BINDING                string = "describe/role_binding"
	PAT_DESCRIBE_CLUSTER_ROLE                string = "describe/cluster_role"
	PAT_DESCRIBE_CLUSTER_ROLE_BINDING        string = "describe/cluster_role_binding"
	PAT_DESCRIBE_VOLUME_ATTACHMENT           string = "describe/volume_attachment"
	PAT_DESCRIBE_NETWORK_POLICY              string = "describe/network_policy"
	PAT_DESCRIBE_STORAGE_CLASS               string = "describe/storage_class"
	PAT_DESCRIBE_CUSTOM_RESOURCE_DEFINITIONS string = "describe/custom_resource_definitions"
	PAT_DESCRIBE_ENDPOINTS                   string = "describe/endpoints"
	PAT_DESCRIBE_LEASES                      string = "describe/leases"
	PAT_DESCRIBE_PRIORITYCLASSES             string = "describe/priorityclasses"
	PAT_DESCRIBE_VOLUMESNAPSHOTS             string = "describe/volumesnapshots"
	PAT_DESCRIBE_RESOURCEQUOTAS              string = "describe/resourcequotas"

	PAT_UPDATE_NAMESPACE                   string = "update/namespace"
	PAT_UPDATE_DEPLOYMENT                  string = "update/deployment"
	PAT_UPDATE_SERVICE                     string = "update/service"
	PAT_UPDATE_POD                         string = "update/pod"
	PAT_UPDATE_INGRESS                     string = "update/ingress"
	PAT_UPDATE_CONFIGMAP                   string = "update/configmap"
	PAT_UPDATE_SECRET                      string = "update/secret"
	PAT_UPDATE_DAEMONSET                   string = "update/daemonset"
	PAT_UPDATE_STATEFULSET                 string = "update/statefulset"
	PAT_UPDATE_JOB                         string = "update/job"
	PAT_UPDATE_CRONJOB                     string = "update/cronjob"
	PAT_UPDATE_REPLICASET                  string = "update/replicaset"
	PAT_UPDATE_PERSISTENT_VOLUME           string = "update/persistent_volume"
	PAT_UPDATE_PERSISTENT_VOLUME_CLAIM     string = "update/persistent_volume_claim"
	PAT_UPDATE_HORIZONTAL_POD_AUTOSCALERS  string = "update/horizontal_pod_autoscalers"
	PAT_UPDATE_CERTIFICATES                string = "update/certificates"
	PAT_UPDATE_CERTIFICATEREQUESTS         string = "update/certificaterequests"
	PAT_UPDATE_ORDERS                      string = "update/orders"
	PAT_UPDATE_ISSUERS                     string = "update/issuers"
	PAT_UPDATE_CLUSTERISSUERS              string = "update/clusterissuers"
	PAT_UPDATE_SERVICE_ACCOUNT             string = "update/service_account"
	PAT_UPDATE_ROLE                        string = "update/role"
	PAT_UPDATE_ROLE_BINDING                string = "update/role_binding"
	PAT_UPDATE_CLUSTER_ROLE                string = "update/cluster_role"
	PAT_UPDATE_CLUSTER_ROLE_BINDING        string = "update/cluster_role_binding"
	PAT_UPDATE_VOLUME_ATTACHMENT           string = "update/volume_attachment"
	PAT_UPDATE_NETWORK_POLICY              string = "update/network_policy"
	PAT_UPDATE_STORAGE_CLASS               string = "update/storage_class"
	PAT_UPDATE_CUSTOM_RESOURCE_DEFINITIONS string = "update/custom_resource_definitions"
	PAT_UPDATE_ENDPOINTS                   string = "update/endpoints"
	PAT_UPDATE_LEASES                      string = "update/leases"
	PAT_UPDATE_PRIORITYCLASSES             string = "update/priorityclasses"
	PAT_UPDATE_VOLUMESNAPSHOTS             string = "update/volumesnapshots"
	PAT_UPDATE_RESOURCEQUOTAS              string = "update/resourcequotas"

	PAT_DELETE_NAMESPACE                   string = "delete/namespace"
	PAT_DELETE_DEPLOYMENT                  string = "delete/deployment"
	PAT_DELETE_SERVICE                     string = "delete/service"
	PAT_DELETE_POD                         string = "delete/pod"
	PAT_DELETE_INGRESS                     string = "delete/ingress"
	PAT_DELETE_CONFIGMAP                   string = "delete/configmap"
	PAT_DELETE_SECRET                      string = "delete/secret"
	PAT_DELETE_DAEMONSET                   string = "delete/daemonset"
	PAT_DELETE_STATEFULSET                 string = "delete/statefulset"
	PAT_DELETE_JOB                         string = "delete/job"
	PAT_DELETE_CRONJOB                     string = "delete/cronjob"
	PAT_DELETE_REPLICASET                  string = "delete/replicaset"
	PAT_DELETE_PERSISTENT_VOLUME           string = "delete/persistent_volume"
	PAT_DELETE_PERSISTENT_VOLUME_CLAIM     string = "delete/persistent_volume_claim"
	PAT_DELETE_HORIZONTAL_POD_AUTOSCALERS  string = "delete/horizontal_pod_autoscalers"
	PAT_DELETE_CERTIFICATES                string = "delete/certificates"
	PAT_DELETE_CERTIFICATEREQUESTS         string = "delete/certificaterequests"
	PAT_DELETE_ORDERS                      string = "delete/orders"
	PAT_DELETE_ISSUERS                     string = "delete/issuers"
	PAT_DELETE_CLUSTERISSUERS              string = "delete/clusterissuers"
	PAT_DELETE_SERVICE_ACCOUNT             string = "delete/service_account"
	PAT_DELETE_ROLE                        string = "delete/role"
	PAT_DELETE_ROLE_BINDING                string = "delete/role_binding"
	PAT_DELETE_CLUSTER_ROLE                string = "delete/cluster_role"
	PAT_DELETE_CLUSTER_ROLE_BINDING        string = "delete/cluster_role_binding"
	PAT_DELETE_VOLUME_ATTACHMENT           string = "delete/volume_attachment"
	PAT_DELETE_NETWORK_POLICY              string = "delete/network_policy"
	PAT_DELETE_STORAGE_CLASS               string = "delete/storage_class"
	PAT_DELETE_CUSTOM_RESOURCE_DEFINITIONS string = "delete/custom_resource_definitions"
	PAT_DELETE_ENDPOINTS                   string = "delete/endpoints"
	PAT_DELETE_LEASES                      string = "delete/leases"
	PAT_DELETE_PRIORITYCLASSES             string = "delete/priorityclasses"
	PAT_DELETE_VOLUMESNAPSHOTS             string = "delete/volumesnapshots"
	PAT_DELETE_RESOURCEQUOTAS              string = "delete/resourcequotas"

	PAT_GET_NAMESPACE                 string = "get/namespace"
	PAT_GET_DEPLOYMENT                string = "get/deployment"
	PAT_GET_SERVICE                   string = "get/service"
	PAT_GET_POD                       string = "get/pod"
	PAT_GET_INGRESS                   string = "get/ingress"
	PAT_GET_CONFIGMAP                 string = "get/configmap"
	PAT_GET_SECRET                    string = "get/secret"
	PAT_GET_NODE                      string = "get/node"
	PAT_GET_DAEMONSET                 string = "get/daemonset"
	PAT_GET_STATEFULSET               string = "get/statefulset"
	PAT_GET_JOB                       string = "get/job"
	PAT_GET_CRONJOB                   string = "get/cronjob"
	PAT_GET_REPLICASET                string = "get/replicaset"
	PAT_GET_PERSISTENT_VOLUME         string = "get/persistent_volume"
	PAT_GET_PERSISTENT_VOLUME_CLAIM   string = "get/persistent_volume_claim"
	PAT_GET_HORIZONTAL_POD_AUTOSCALER string = "get/horizontal_pod_autoscaler"
	PAT_GET_EVENT                     string = "get/event"
	PAT_GET_CERTIFICATE               string = "get/certificate"
	PAT_GET_CERTIFICATEREQUEST        string = "get/certificaterequest"
	PAT_GET_ORDER                     string = "get/order"
	PAT_GET_ISSUER                    string = "get/issuer"
	PAT_GET_CLUSTERISSUER             string = "get/clusterissuer"
	PAT_GET_SERVICE_ACCOUNT           string = "get/service_account"
	PAT_GET_ROLE                      string = "get/role"
	PAT_GET_ROLE_BINDING              string = "get/role_binding"
	PAT_GET_CLUSTER_ROLE              string = "get/cluster_role"
	PAT_GET_CLUSTER_ROLE_BINDING      string = "get/cluster_role_binding"
	PAT_GET_VOLUME_ATTACHMENT         string = "get/volume_attachment"
	PAT_GET_NETWORK_POLICY            string = "get/network_policy"
	PAT_GET_STORAGE_CLASS             string = "get/storage_class"
	PAT_GET_ENDPOINTS                 string = "get/endpoints"
	PAT_GET_LEASES                    string = "get/leases"
	PAT_GET_PRIORITYCLASSES           string = "get/priorityclasses"
	PAT_GET_VOLUMESNAPSHOTS           string = "get/volumesnapshots"
	PAT_GET_RESOURCEQUOTAS            string = "get/resourcequotas"

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

	PAT_POPEYE_CONSOLE string = "popeye_console"

	PAT_FILES_UPLOAD string = "files/upload"

	PAT_EXTERNAL_SECRET_STORE_CREATE                 string = "external-secret-store/create"
	PAT_EXTERNAL_SECRET_STORE_LIST                   string = "external-secret-store/list"
	PAT_EXTERNAL_SECRET_STORE_LIST_AVAILABLE_SECRETS string = "external-secret-store/list-available-secrets"
	PAT_EXTERNAL_SECRET_STORE_DELETE                 string = "external-secret/delete"
	PAT_EXTERNAL_SECRET_CREATE                       string = "external-secret/create"
	PAT_EXTERNAL_SECRET_DELETE                       string = "external-secret/delete"

	PAT_LIST_CRONJOB_JOBS string = "list/cronjob-jobs"
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
	PAT_SYSTEM_CHECK,

	PAT_INSTALL_LOCAL_DEV_COMPONENTS,
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
	PAT_STATS_PODSTAT_FOR_NAMESPACE_ALL,
	PAT_STATS_PODSTAT_FOR_NAMESPACE_LAST,
	PAT_STATS_TRAFFIC_FOR_POD_LAST,        // legacy-support TODO: REMOVE
	PAT_STATS_TRAFFIC_FOR_CONTROLLER_LAST, // legacy-support TODO: REMOVE
	PAT_STATS_TRAFFIC_FOR_NAMESPACE_LAST,  // legacy-support TODO: REMOVE
	PAT_STATS_CHART_FOR_POD,

	PAT_PROJECT_CREATE,
	PAT_PROJECT_UPDATE,
	PAT_PROJECT_DELETE,
	PAT_PROJECT_LIST,
	PAT_PROJECT_COUNT,

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
	PAT_SERVICE_OPERATOR_LOG_STREAM_CONNECTION_REQUEST,
	PAT_SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST,
	PAT_SERVICE_SCAN_IMAGE_LOG_STREAM_CONNECTION_REQUEST,
	PAT_SERVICE_CLUSTER_TOOL_STREAM_CONNECTION_REQUEST,

	PAT_LIST_CREATE_TEMPLATES,

	PAT_LIST_NAMESPACES,
	PAT_LIST_DEPLOYMENTS,
	PAT_LIST_SERVICES,
	PAT_LIST_PODS,
	PAT_LIST_INGRESSES,
	PAT_LIST_CONFIGMAPS,
	PAT_LIST_SECRETS,
	PAT_LIST_NODES,
	PAT_LIST_DAEMONSETS,
	PAT_LIST_STATEFULSETS,
	PAT_LIST_JOBS,
	PAT_LIST_CRONJOBS,
	PAT_LIST_REPLICASETS,
	PAT_LIST_PERSISTENT_VOLUMES,
	PAT_LIST_PERSISTENT_VOLUME_CLAIMS,
	PAT_LIST_HORIZONTAL_POD_AUTOSCALERS,
	PAT_LIST_EVENTS,
	PAT_LIST_CERTIFICATES,
	PAT_LIST_CERTIFICATEREQUESTS,
	PAT_LIST_ORDERS,
	PAT_LIST_ISSUERS,
	PAT_LIST_CLUSTERISSUERS,
	PAT_LIST_SERVICE_ACCOUNT,
	PAT_LIST_ROLE,
	PAT_LIST_ROLE_BINDING,
	PAT_LIST_CLUSTER_ROLE,
	PAT_LIST_CLUSTER_ROLE_BINDING,
	PAT_LIST_VOLUME_ATTACHMENT,
	PAT_LIST_NETWORK_POLICY,
	PAT_LIST_STORAGE_CLASS,
	PAT_LIST_CUSTOM_RESOURCE_DEFINITIONS,
	PAT_LIST_ENDPOINTS,
	PAT_LIST_LEASES,
	PAT_LIST_PRIORITYCLASSES,
	PAT_LIST_VOLUMESNAPSHOTS,
	PAT_LIST_RESOURCEQUOTAS,

	// PAT_CREATE_NAMESPACE,
	// PAT_CREATE_DEPLOYMENT,
	// PAT_CREATE_SERVICE,
	// PAT_CREATE_POD,
	// PAT_CREATE_INGRESS,
	// PAT_CREATE_CONFIGMAP,
	// PAT_CREATE_SECRET,
	// PAT_CREATE_DAEMONSET,
	// PAT_CREATE_STATEFULSET,
	// PAT_CREATE_JOB,
	// PAT_CREATE_CRONJOB,
	// PAT_CREATE_REPLICASET,
	// PAT_CREATE_PERSISTENT_VOLUME,
	// PAT_CREATE_PERSISTENT_VOLUME_CLAIM,
	// PAT_CREATE_HORIZONTAL_POD_AUTOSCALER,
	// PAT_CREATE_CERTIFICATE,
	// PAT_CREATE_CERTIFICATEREQUEST,
	// PAT_CREATE_ORDER,
	// PAT_CREATE_ISSUER,
	// PAT_CREATE_CLUSTERISSUER,
	// PAT_CREATE_SERVICE_ACCOUNT,
	// PAT_CREATE_ROLE,
	// PAT_CREATE_ROLE_BINDING,
	// PAT_CREATE_CLUSTER_ROLE,
	// PAT_CREATE_CLUSTER_ROLE_BINDING,
	// PAT_CREATE_VOLUME_ATTACHMENT,
	// PAT_CREATE_NETWORK_POLICY,
	// PAT_CREATE_STORAGE_CLASS,
	// PAT_CREATE_CUSTOM_RESOURCE_DEFINITIONS,
	// PAT_CREATE_ENDPOINTS,
	// PAT_CREATE_LEASES,
	// PAT_CREATE_PRIORITYCLASSES,
	// PAT_CREATE_VOLUMESNAPSHOTS,
	// PAT_CREATE_RESOURCEQUOTAS,

	PAT_DESCRIBE_NAMESPACE,
	PAT_DESCRIBE_DEPLOYMENT,
	PAT_DESCRIBE_SERVICE,
	PAT_DESCRIBE_POD,
	PAT_DESCRIBE_INGRESS,
	PAT_DESCRIBE_CONFIGMAP,
	PAT_DESCRIBE_SECRET,
	PAT_DESCRIBE_NODE,
	PAT_DESCRIBE_DAEMONSET,
	PAT_DESCRIBE_STATEFULSET,
	PAT_DESCRIBE_JOB,
	PAT_DESCRIBE_CRONJOB,
	PAT_DESCRIBE_REPLICASET,
	PAT_DESCRIBE_PERSISTENT_VOLUME,
	PAT_DESCRIBE_PERSISTENT_VOLUME_CLAIM,
	PAT_DESCRIBE_HORIZONTAL_POD_AUTOSCALER,
	PAT_DESCRIBE_EVENT,
	PAT_DESCRIBE_CERTIFICATE,
	PAT_DESCRIBE_CERTIFICATEREQUEST,
	PAT_DESCRIBE_ORDER,
	PAT_DESCRIBE_ISSUER,
	PAT_DESCRIBE_CLUSTERISSUER,
	PAT_DESCRIBE_SERVICE_ACCOUNT,
	PAT_DESCRIBE_ROLE,
	PAT_DESCRIBE_ROLE_BINDING,
	PAT_DESCRIBE_CLUSTER_ROLE,
	PAT_DESCRIBE_CLUSTER_ROLE_BINDING,
	PAT_DESCRIBE_VOLUME_ATTACHMENT,
	PAT_DESCRIBE_NETWORK_POLICY,
	PAT_DESCRIBE_STORAGE_CLASS,
	PAT_DESCRIBE_CUSTOM_RESOURCE_DEFINITIONS,
	PAT_DESCRIBE_ENDPOINTS,
	PAT_DESCRIBE_LEASES,
	PAT_DESCRIBE_PRIORITYCLASSES,
	PAT_DESCRIBE_VOLUMESNAPSHOTS,
	PAT_DESCRIBE_RESOURCEQUOTAS,

	PAT_UPDATE_NAMESPACE,
	PAT_UPDATE_DEPLOYMENT,
	PAT_UPDATE_SERVICE,
	PAT_UPDATE_POD,
	PAT_UPDATE_INGRESS,
	PAT_UPDATE_CONFIGMAP,
	PAT_UPDATE_SECRET,
	PAT_UPDATE_DAEMONSET,
	PAT_UPDATE_STATEFULSET,
	PAT_UPDATE_JOB,
	PAT_UPDATE_CRONJOB,
	PAT_UPDATE_REPLICASET,
	PAT_UPDATE_PERSISTENT_VOLUME,
	PAT_UPDATE_PERSISTENT_VOLUME_CLAIM,
	PAT_UPDATE_HORIZONTAL_POD_AUTOSCALERS,
	PAT_UPDATE_CERTIFICATES,
	PAT_UPDATE_CERTIFICATEREQUESTS,
	PAT_UPDATE_ORDERS,
	PAT_UPDATE_ISSUERS,
	PAT_UPDATE_CLUSTERISSUERS,
	PAT_UPDATE_SERVICE_ACCOUNT,
	PAT_UPDATE_ROLE,
	PAT_UPDATE_ROLE_BINDING,
	PAT_UPDATE_CLUSTER_ROLE,
	PAT_UPDATE_CLUSTER_ROLE_BINDING,
	PAT_UPDATE_VOLUME_ATTACHMENT,
	PAT_UPDATE_NETWORK_POLICY,
	PAT_UPDATE_STORAGE_CLASS,
	PAT_UPDATE_CUSTOM_RESOURCE_DEFINITIONS,
	PAT_UPDATE_ENDPOINTS,
	PAT_UPDATE_LEASES,
	PAT_UPDATE_PRIORITYCLASSES,
	PAT_UPDATE_VOLUMESNAPSHOTS,
	PAT_UPDATE_RESOURCEQUOTAS,

	PAT_DELETE_NAMESPACE,
	PAT_DELETE_DEPLOYMENT,
	PAT_DELETE_SERVICE,
	PAT_DELETE_POD,
	PAT_DELETE_INGRESS,
	PAT_DELETE_CONFIGMAP,
	PAT_DELETE_SECRET,
	PAT_DELETE_DAEMONSET,
	PAT_DELETE_STATEFULSET,
	PAT_DELETE_JOB,
	PAT_DELETE_CRONJOB,
	PAT_DELETE_REPLICASET,
	PAT_DELETE_PERSISTENT_VOLUME,
	PAT_DELETE_PERSISTENT_VOLUME_CLAIM,
	PAT_DELETE_HORIZONTAL_POD_AUTOSCALERS,
	PAT_DELETE_CERTIFICATES,
	PAT_DELETE_CERTIFICATEREQUESTS,
	PAT_DELETE_ORDERS,
	PAT_DELETE_ISSUERS,
	PAT_DELETE_CLUSTERISSUERS,
	PAT_DELETE_SERVICE_ACCOUNT,
	PAT_DELETE_ROLE,
	PAT_DELETE_ROLE_BINDING,
	PAT_DELETE_CLUSTER_ROLE,
	PAT_DELETE_CLUSTER_ROLE_BINDING,
	PAT_DELETE_VOLUME_ATTACHMENT,
	PAT_DELETE_NETWORK_POLICY,
	PAT_DELETE_STORAGE_CLASS,
	PAT_DELETE_CUSTOM_RESOURCE_DEFINITIONS,
	PAT_DELETE_ENDPOINTS,
	PAT_DELETE_LEASES,
	PAT_DELETE_PRIORITYCLASSES,
	PAT_DELETE_VOLUMESNAPSHOTS,
	PAT_DELETE_RESOURCEQUOTAS,

	PAT_GET_NAMESPACE,
	PAT_GET_DEPLOYMENT,
	PAT_GET_SERVICE,
	PAT_GET_POD,
	PAT_GET_INGRESS,
	PAT_GET_CONFIGMAP,
	PAT_GET_SECRET,
	PAT_GET_NODE,
	PAT_GET_DAEMONSET,
	PAT_GET_STATEFULSET,
	PAT_GET_JOB,
	PAT_GET_CRONJOB,
	PAT_GET_REPLICASET,
	PAT_GET_PERSISTENT_VOLUME,
	PAT_GET_PERSISTENT_VOLUME_CLAIM,
	PAT_GET_HORIZONTAL_POD_AUTOSCALER,
	PAT_GET_EVENT,
	PAT_GET_CERTIFICATE,
	PAT_GET_CERTIFICATEREQUEST,
	PAT_GET_ORDER,
	PAT_GET_ISSUER,
	PAT_GET_CLUSTERISSUER,
	PAT_GET_SERVICE_ACCOUNT,
	PAT_GET_ROLE,
	PAT_GET_ROLE_BINDING,
	PAT_GET_CLUSTER_ROLE,
	PAT_GET_CLUSTER_ROLE_BINDING,
	PAT_GET_VOLUME_ATTACHMENT,
	PAT_GET_NETWORK_POLICY,
	PAT_GET_STORAGE_CLASS,
	PAT_GET_ENDPOINTS,
	PAT_GET_LEASES,
	PAT_GET_PRIORITYCLASSES,
	PAT_GET_VOLUMESNAPSHOTS,
	PAT_GET_RESOURCEQUOTAS,

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
	// PAT_BUILD_SCAN,
	PAT_BUILD_CANCEL,
	PAT_BUILD_DELETE,
	PAT_BUILD_LAST_JOB_OF_SERVICES,
	PAT_BUILD_JOB_LIST_OF_SERVICE,
	PAT_BUILD_DELETE_ALL_OF_SERVICE,
	// PAT_BUILD_LAST_JOB_INFO_OF_SERVICE,

	PAT_EXEC_SHELL,

	PAT_POPEYE_CONSOLE,

	PAT_LOG_LIST_ALL,

	PAT_EXTERNAL_SECRET_STORE_CREATE,
	PAT_EXTERNAL_SECRET_STORE_LIST,
	PAT_EXTERNAL_SECRET_STORE_LIST_AVAILABLE_SECRETS,
	PAT_EXTERNAL_SECRET_STORE_DELETE,
	PAT_EXTERNAL_SECRET_CREATE,
	PAT_EXTERNAL_SECRET_DELETE,

	PAT_LIST_CRONJOB_JOBS,
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
	PAT_LIST_CRONJOB_JOBS,
}

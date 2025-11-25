//===============================================================
//===================== Pattern Enumeration =====================
//===============================================================

export enum Pattern {
  AUDIT_LOG_LIST = "audit-log/list",
  CLUSTER_ARGO_CD_APPLICATION_REFRESH = "cluster/argo-cd-application-refresh",
  CLUSTER_ARGO_CD_CREATE_API_TOKEN = "cluster/argo-cd-create-api-token",
  CLUSTER_CLEAR_VALKEY_CACHE = "cluster/clear-valkey-cache",
  CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST = "cluster/component-log-stream-connection-request",
  CLUSTER_FORCE_DISCONNECT = "cluster/force-disconnect",
  CLUSTER_FORCE_RECONNECT = "cluster/force-reconnect",
  CLUSTER_HELM_CHART_INSTALL = "cluster/helm-chart-install",
  CLUSTER_HELM_CHART_INSTALL_OCI = "cluster/helm-chart-install-oci",
  CLUSTER_HELM_CHART_REMOVE = "cluster/helm-chart-remove",
  CLUSTER_HELM_CHART_SEARCH = "cluster/helm-chart-search",
  CLUSTER_HELM_CHART_SHOW = "cluster/helm-chart-show",
  CLUSTER_HELM_CHART_VERSIONS = "cluster/helm-chart-versions",
  CLUSTER_HELM_RELEASE_GET = "cluster/helm-release-get",
  CLUSTER_HELM_RELEASE_GET_WORKLOADS = "cluster/helm-release-get-workloads",
  CLUSTER_HELM_RELEASE_HISTORY = "cluster/helm-release-history",
  CLUSTER_HELM_RELEASE_LINK = "cluster/helm-release-link",
  CLUSTER_HELM_RELEASE_LIST = "cluster/helm-release-list",
  CLUSTER_HELM_RELEASE_ROLLBACK = "cluster/helm-release-rollback",
  CLUSTER_HELM_RELEASE_STATUS = "cluster/helm-release-status",
  CLUSTER_HELM_RELEASE_UNINSTALL = "cluster/helm-release-uninstall",
  CLUSTER_HELM_RELEASE_UPGRADE = "cluster/helm-release-upgrade",
  CLUSTER_HELM_REPO_ADD = "cluster/helm-repo-add",
  CLUSTER_HELM_REPO_LIST = "cluster/helm-repo-list",
  CLUSTER_HELM_REPO_PATCH = "cluster/helm-repo-patch",
  CLUSTER_HELM_REPO_UPDATE = "cluster/helm-repo-update",
  CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS = "cluster/list-persistent-volume-claims",
  CLUSTER_MACHINE_STATS = "cluster/machine-stats",
  CLUSTER_RESOURCE_INFO = "cluster/resource-info",
  CREATE_GRANT = "create/grant",
  CREATE_NEW_WORKLOAD = "create/new-workload",
  CREATE_USER = "create/user",
  CREATE_WORKSPACE = "create/workspace",
  DELETE_GRANT = "delete/grant",
  DELETE_USER = "delete/user",
  DELETE_WORKLOAD = "delete/workload",
  DELETE_WORKSPACE = "delete/workspace",
  DESCRIBE = "describe",
  DESCRIBE_WORKLOAD = "describe/workload",
  FILES_CHMOD = "files/chmod",
  FILES_CHOWN = "files/chown",
  FILES_CREATE_FOLDER = "files/create-folder",
  FILES_DELETE = "files/delete",
  FILES_DOWNLOAD = "files/download",
  FILES_INFO = "files/info",
  FILES_LIST = "files/list",
  FILES_RENAME = "files/rename",
  GET_GRANT = "get/grant",
  GET_GRANTS = "get/grants",
  GET_LABELED_WORKLOAD_LIST = "get/labeled-workload-list",
  GET_NAMESPACE_WORKLOAD_LIST = "get/namespace-workload-list",
  GET_NODES_METRICS = "get/nodes-metrics",
  GET_USER = "get/user",
  GET_USERS = "get/users",
  GET_WORKLOAD = "get/workload",
  GET_WORKLOAD_EXAMPLE = "get/workload-example",
  GET_WORKLOAD_LIST = "get/workload-list",
  GET_WORKLOAD_STATUS = "get/workload-status",
  GET_WORKSPACE = "get/workspace",
  GET_WORKSPACES = "get/workspaces",
  GET_WORKSPACE_WORKLOADS = "get/workspace-workloads",
  INSTALL_CERT_MANAGER = "install-cert-manager",
  INSTALL_CLUSTER_ISSUER = "install-cluster-issuer",
  INSTALL_INGRESS_CONTROLLER_TRAEFIK = "install-ingress-controller-traefik",
  INSTALL_KEPLER = "install-kepler",
  INSTALL_METALLB = "install-metallb",
  INSTALL_METRICS_SERVER = "install-metrics-server",
  LIST_ALL_RESOURCE_DESCRIPTORS = "list/all-resource-descriptors",
  LIVE_STREAM_NODES_CPU = "live-stream/nodes-cpu",
  LIVE_STREAM_NODES_MEMORY = "live-stream/nodes-memory",
  LIVE_STREAM_NODES_TRAFFIC = "live-stream/nodes-traffic",
  LIVE_STREAM_POD_CPU = "live-stream/pod-cpu",
  LIVE_STREAM_POD_MEMORY = "live-stream/pod-memory",
  LIVE_STREAM_POD_TRAFFIC = "live-stream/pod-traffic",
  LIVE_STREAM_WORKSPACE_CPU = "live-stream/workspace-cpu",
  LIVE_STREAM_WORKSPACE_MEMORY = "live-stream/workspace-memory",
  LIVE_STREAM_WORKSPACE_TRAFFIC = "live-stream/workspace-traffic",
  PROMETHEUS_CHARTS_ADD = "prometheus/charts/add",
  PROMETHEUS_CHARTS_GET = "prometheus/charts/get",
  PROMETHEUS_CHARTS_LIST = "prometheus/charts/list",
  PROMETHEUS_CHARTS_REMOVE = "prometheus/charts/remove",
  PROMETHEUS_IS_REACHABLE = "prometheus/is-reachable",
  PROMETHEUS_QUERY = "prometheus/query",
  PROMETHEUS_VALUES = "prometheus/values",
  SEALED_SECRET_CREATE_FROM_EXISTING = "sealed-secret/create-from-existing",
  SEALED_SECRET_GET_CERTIFICATE = "sealed-secret/get-certificate",
  SERVICE_EXEC_SH_CONNECTION_REQUEST = "service/exec-sh-connection-request",
  SERVICE_LOG_STREAM_CONNECTION_REQUEST = "service/log-stream-connection-request",
  SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST = "service/pod-event-stream-connection-request",
  STATS_POD_ALL_FOR_CONTROLLER = "stats/pod/all-for-controller",
  STATS_TRAFFIC_ALL_FOR_CONTROLLER = "stats/traffic/all-for-controller",
  STATS_WORKSPACE_CPU_UTILIZATION = "stats/workspace-cpu-utilization",
  STATS_WORKSPACE_MEMORY_UTILIZATION = "stats/workspace-memory-utilization",
  STATS_WORKSPACE_TRAFFIC_UTILIZATION = "stats/workspace-traffic-utilization",
  STORAGE_CREATE_VOLUME = "storage/create-volume",
  STORAGE_DELETE_VOLUME = "storage/delete-volume",
  STORAGE_STATS = "storage/stats",
  STORAGE_STATUS = "storage/status",
  SYSTEM_CHECK = "system/check",
  TRIGGER_WORKLOAD = "trigger/workload",
  UNINSTALL_CERT_MANAGER = "uninstall-cert-manager",
  UNINSTALL_CLUSTER_ISSUER = "uninstall-cluster-issuer",
  UNINSTALL_INGRESS_CONTROLLER_TRAEFIK = "uninstall-ingress-controller-traefik",
  UNINSTALL_KEPLER = "uninstall-kepler",
  UNINSTALL_METALLB = "uninstall-metallb",
  UNINSTALL_METRICS_SERVER = "uninstall-metrics-server",
  UPDATE_GRANT = "update/grant",
  UPDATE_USER = "update/user",
  UPDATE_WORKLOAD = "update/workload",
  UPDATE_WORKSPACE = "update/workspace",
  UPGRADEK8SMANAGER = "UpgradeK8sManager",
  UPGRADE_CERT_MANAGER = "upgrade-cert-manager",
  UPGRADE_INGRESS_CONTROLLER_TRAEFIK = "upgrade-ingress-controller-traefik",
  UPGRADE_KEPLER = "upgrade-kepler",
  UPGRADE_METALLB = "upgrade-metallb",
  UPGRADE_METRICS_SERVER = "upgrade-metrics-server",
  WORKSPACE_CLEAN_UP = "workspace/clean-up",
}

//===============================================================
//====================== Pattern Mappings =======================
//===============================================================

export const StringToPattern = {
  "audit-log/list": Pattern.AUDIT_LOG_LIST,
  "cluster/argo-cd-application-refresh": Pattern.CLUSTER_ARGO_CD_APPLICATION_REFRESH,
  "cluster/argo-cd-create-api-token": Pattern.CLUSTER_ARGO_CD_CREATE_API_TOKEN,
  "cluster/clear-valkey-cache": Pattern.CLUSTER_CLEAR_VALKEY_CACHE,
  "cluster/component-log-stream-connection-request": Pattern.CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST,
  "cluster/force-disconnect": Pattern.CLUSTER_FORCE_DISCONNECT,
  "cluster/force-reconnect": Pattern.CLUSTER_FORCE_RECONNECT,
  "cluster/helm-chart-install": Pattern.CLUSTER_HELM_CHART_INSTALL,
  "cluster/helm-chart-install-oci": Pattern.CLUSTER_HELM_CHART_INSTALL_OCI,
  "cluster/helm-chart-remove": Pattern.CLUSTER_HELM_CHART_REMOVE,
  "cluster/helm-chart-search": Pattern.CLUSTER_HELM_CHART_SEARCH,
  "cluster/helm-chart-show": Pattern.CLUSTER_HELM_CHART_SHOW,
  "cluster/helm-chart-versions": Pattern.CLUSTER_HELM_CHART_VERSIONS,
  "cluster/helm-release-get": Pattern.CLUSTER_HELM_RELEASE_GET,
  "cluster/helm-release-get-workloads": Pattern.CLUSTER_HELM_RELEASE_GET_WORKLOADS,
  "cluster/helm-release-history": Pattern.CLUSTER_HELM_RELEASE_HISTORY,
  "cluster/helm-release-link": Pattern.CLUSTER_HELM_RELEASE_LINK,
  "cluster/helm-release-list": Pattern.CLUSTER_HELM_RELEASE_LIST,
  "cluster/helm-release-rollback": Pattern.CLUSTER_HELM_RELEASE_ROLLBACK,
  "cluster/helm-release-status": Pattern.CLUSTER_HELM_RELEASE_STATUS,
  "cluster/helm-release-uninstall": Pattern.CLUSTER_HELM_RELEASE_UNINSTALL,
  "cluster/helm-release-upgrade": Pattern.CLUSTER_HELM_RELEASE_UPGRADE,
  "cluster/helm-repo-add": Pattern.CLUSTER_HELM_REPO_ADD,
  "cluster/helm-repo-list": Pattern.CLUSTER_HELM_REPO_LIST,
  "cluster/helm-repo-patch": Pattern.CLUSTER_HELM_REPO_PATCH,
  "cluster/helm-repo-update": Pattern.CLUSTER_HELM_REPO_UPDATE,
  "cluster/list-persistent-volume-claims": Pattern.CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS,
  "cluster/machine-stats": Pattern.CLUSTER_MACHINE_STATS,
  "cluster/resource-info": Pattern.CLUSTER_RESOURCE_INFO,
  "create/grant": Pattern.CREATE_GRANT,
  "create/new-workload": Pattern.CREATE_NEW_WORKLOAD,
  "create/user": Pattern.CREATE_USER,
  "create/workspace": Pattern.CREATE_WORKSPACE,
  "delete/grant": Pattern.DELETE_GRANT,
  "delete/user": Pattern.DELETE_USER,
  "delete/workload": Pattern.DELETE_WORKLOAD,
  "delete/workspace": Pattern.DELETE_WORKSPACE,
  "describe": Pattern.DESCRIBE,
  "describe/workload": Pattern.DESCRIBE_WORKLOAD,
  "files/chmod": Pattern.FILES_CHMOD,
  "files/chown": Pattern.FILES_CHOWN,
  "files/create-folder": Pattern.FILES_CREATE_FOLDER,
  "files/delete": Pattern.FILES_DELETE,
  "files/download": Pattern.FILES_DOWNLOAD,
  "files/info": Pattern.FILES_INFO,
  "files/list": Pattern.FILES_LIST,
  "files/rename": Pattern.FILES_RENAME,
  "get/grant": Pattern.GET_GRANT,
  "get/grants": Pattern.GET_GRANTS,
  "get/labeled-workload-list": Pattern.GET_LABELED_WORKLOAD_LIST,
  "get/namespace-workload-list": Pattern.GET_NAMESPACE_WORKLOAD_LIST,
  "get/nodes-metrics": Pattern.GET_NODES_METRICS,
  "get/user": Pattern.GET_USER,
  "get/users": Pattern.GET_USERS,
  "get/workload": Pattern.GET_WORKLOAD,
  "get/workload-example": Pattern.GET_WORKLOAD_EXAMPLE,
  "get/workload-list": Pattern.GET_WORKLOAD_LIST,
  "get/workload-status": Pattern.GET_WORKLOAD_STATUS,
  "get/workspace": Pattern.GET_WORKSPACE,
  "get/workspaces": Pattern.GET_WORKSPACES,
  "get/workspace-workloads": Pattern.GET_WORKSPACE_WORKLOADS,
  "install-cert-manager": Pattern.INSTALL_CERT_MANAGER,
  "install-cluster-issuer": Pattern.INSTALL_CLUSTER_ISSUER,
  "install-ingress-controller-traefik": Pattern.INSTALL_INGRESS_CONTROLLER_TRAEFIK,
  "install-kepler": Pattern.INSTALL_KEPLER,
  "install-metallb": Pattern.INSTALL_METALLB,
  "install-metrics-server": Pattern.INSTALL_METRICS_SERVER,
  "list/all-resource-descriptors": Pattern.LIST_ALL_RESOURCE_DESCRIPTORS,
  "live-stream/nodes-cpu": Pattern.LIVE_STREAM_NODES_CPU,
  "live-stream/nodes-memory": Pattern.LIVE_STREAM_NODES_MEMORY,
  "live-stream/nodes-traffic": Pattern.LIVE_STREAM_NODES_TRAFFIC,
  "live-stream/pod-cpu": Pattern.LIVE_STREAM_POD_CPU,
  "live-stream/pod-memory": Pattern.LIVE_STREAM_POD_MEMORY,
  "live-stream/pod-traffic": Pattern.LIVE_STREAM_POD_TRAFFIC,
  "live-stream/workspace-cpu": Pattern.LIVE_STREAM_WORKSPACE_CPU,
  "live-stream/workspace-memory": Pattern.LIVE_STREAM_WORKSPACE_MEMORY,
  "live-stream/workspace-traffic": Pattern.LIVE_STREAM_WORKSPACE_TRAFFIC,
  "prometheus/charts/add": Pattern.PROMETHEUS_CHARTS_ADD,
  "prometheus/charts/get": Pattern.PROMETHEUS_CHARTS_GET,
  "prometheus/charts/list": Pattern.PROMETHEUS_CHARTS_LIST,
  "prometheus/charts/remove": Pattern.PROMETHEUS_CHARTS_REMOVE,
  "prometheus/is-reachable": Pattern.PROMETHEUS_IS_REACHABLE,
  "prometheus/query": Pattern.PROMETHEUS_QUERY,
  "prometheus/values": Pattern.PROMETHEUS_VALUES,
  "sealed-secret/create-from-existing": Pattern.SEALED_SECRET_CREATE_FROM_EXISTING,
  "sealed-secret/get-certificate": Pattern.SEALED_SECRET_GET_CERTIFICATE,
  "service/exec-sh-connection-request": Pattern.SERVICE_EXEC_SH_CONNECTION_REQUEST,
  "service/log-stream-connection-request": Pattern.SERVICE_LOG_STREAM_CONNECTION_REQUEST,
  "service/pod-event-stream-connection-request": Pattern.SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST,
  "stats/pod/all-for-controller": Pattern.STATS_POD_ALL_FOR_CONTROLLER,
  "stats/traffic/all-for-controller": Pattern.STATS_TRAFFIC_ALL_FOR_CONTROLLER,
  "stats/workspace-cpu-utilization": Pattern.STATS_WORKSPACE_CPU_UTILIZATION,
  "stats/workspace-memory-utilization": Pattern.STATS_WORKSPACE_MEMORY_UTILIZATION,
  "stats/workspace-traffic-utilization": Pattern.STATS_WORKSPACE_TRAFFIC_UTILIZATION,
  "storage/create-volume": Pattern.STORAGE_CREATE_VOLUME,
  "storage/delete-volume": Pattern.STORAGE_DELETE_VOLUME,
  "storage/stats": Pattern.STORAGE_STATS,
  "storage/status": Pattern.STORAGE_STATUS,
  "system/check": Pattern.SYSTEM_CHECK,
  "trigger/workload": Pattern.TRIGGER_WORKLOAD,
  "uninstall-cert-manager": Pattern.UNINSTALL_CERT_MANAGER,
  "uninstall-cluster-issuer": Pattern.UNINSTALL_CLUSTER_ISSUER,
  "uninstall-ingress-controller-traefik": Pattern.UNINSTALL_INGRESS_CONTROLLER_TRAEFIK,
  "uninstall-kepler": Pattern.UNINSTALL_KEPLER,
  "uninstall-metallb": Pattern.UNINSTALL_METALLB,
  "uninstall-metrics-server": Pattern.UNINSTALL_METRICS_SERVER,
  "update/grant": Pattern.UPDATE_GRANT,
  "update/user": Pattern.UPDATE_USER,
  "update/workload": Pattern.UPDATE_WORKLOAD,
  "update/workspace": Pattern.UPDATE_WORKSPACE,
  "UpgradeK8sManager": Pattern.UPGRADEK8SMANAGER,
  "upgrade-cert-manager": Pattern.UPGRADE_CERT_MANAGER,
  "upgrade-ingress-controller-traefik": Pattern.UPGRADE_INGRESS_CONTROLLER_TRAEFIK,
  "upgrade-kepler": Pattern.UPGRADE_KEPLER,
  "upgrade-metallb": Pattern.UPGRADE_METALLB,
  "upgrade-metrics-server": Pattern.UPGRADE_METRICS_SERVER,
  "workspace/clean-up": Pattern.WORKSPACE_CLEAN_UP,
};

export const PatternToString = {
  [Pattern.AUDIT_LOG_LIST]: "audit-log/list",
  [Pattern.CLUSTER_ARGO_CD_APPLICATION_REFRESH]: "cluster/argo-cd-application-refresh",
  [Pattern.CLUSTER_ARGO_CD_CREATE_API_TOKEN]: "cluster/argo-cd-create-api-token",
  [Pattern.CLUSTER_CLEAR_VALKEY_CACHE]: "cluster/clear-valkey-cache",
  [Pattern.CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST]: "cluster/component-log-stream-connection-request",
  [Pattern.CLUSTER_FORCE_DISCONNECT]: "cluster/force-disconnect",
  [Pattern.CLUSTER_FORCE_RECONNECT]: "cluster/force-reconnect",
  [Pattern.CLUSTER_HELM_CHART_INSTALL]: "cluster/helm-chart-install",
  [Pattern.CLUSTER_HELM_CHART_INSTALL_OCI]: "cluster/helm-chart-install-oci",
  [Pattern.CLUSTER_HELM_CHART_REMOVE]: "cluster/helm-chart-remove",
  [Pattern.CLUSTER_HELM_CHART_SEARCH]: "cluster/helm-chart-search",
  [Pattern.CLUSTER_HELM_CHART_SHOW]: "cluster/helm-chart-show",
  [Pattern.CLUSTER_HELM_CHART_VERSIONS]: "cluster/helm-chart-versions",
  [Pattern.CLUSTER_HELM_RELEASE_GET]: "cluster/helm-release-get",
  [Pattern.CLUSTER_HELM_RELEASE_GET_WORKLOADS]: "cluster/helm-release-get-workloads",
  [Pattern.CLUSTER_HELM_RELEASE_HISTORY]: "cluster/helm-release-history",
  [Pattern.CLUSTER_HELM_RELEASE_LINK]: "cluster/helm-release-link",
  [Pattern.CLUSTER_HELM_RELEASE_LIST]: "cluster/helm-release-list",
  [Pattern.CLUSTER_HELM_RELEASE_ROLLBACK]: "cluster/helm-release-rollback",
  [Pattern.CLUSTER_HELM_RELEASE_STATUS]: "cluster/helm-release-status",
  [Pattern.CLUSTER_HELM_RELEASE_UNINSTALL]: "cluster/helm-release-uninstall",
  [Pattern.CLUSTER_HELM_RELEASE_UPGRADE]: "cluster/helm-release-upgrade",
  [Pattern.CLUSTER_HELM_REPO_ADD]: "cluster/helm-repo-add",
  [Pattern.CLUSTER_HELM_REPO_LIST]: "cluster/helm-repo-list",
  [Pattern.CLUSTER_HELM_REPO_PATCH]: "cluster/helm-repo-patch",
  [Pattern.CLUSTER_HELM_REPO_UPDATE]: "cluster/helm-repo-update",
  [Pattern.CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS]: "cluster/list-persistent-volume-claims",
  [Pattern.CLUSTER_MACHINE_STATS]: "cluster/machine-stats",
  [Pattern.CLUSTER_RESOURCE_INFO]: "cluster/resource-info",
  [Pattern.CREATE_GRANT]: "create/grant",
  [Pattern.CREATE_NEW_WORKLOAD]: "create/new-workload",
  [Pattern.CREATE_USER]: "create/user",
  [Pattern.CREATE_WORKSPACE]: "create/workspace",
  [Pattern.DELETE_GRANT]: "delete/grant",
  [Pattern.DELETE_USER]: "delete/user",
  [Pattern.DELETE_WORKLOAD]: "delete/workload",
  [Pattern.DELETE_WORKSPACE]: "delete/workspace",
  [Pattern.DESCRIBE]: "describe",
  [Pattern.DESCRIBE_WORKLOAD]: "describe/workload",
  [Pattern.FILES_CHMOD]: "files/chmod",
  [Pattern.FILES_CHOWN]: "files/chown",
  [Pattern.FILES_CREATE_FOLDER]: "files/create-folder",
  [Pattern.FILES_DELETE]: "files/delete",
  [Pattern.FILES_DOWNLOAD]: "files/download",
  [Pattern.FILES_INFO]: "files/info",
  [Pattern.FILES_LIST]: "files/list",
  [Pattern.FILES_RENAME]: "files/rename",
  [Pattern.GET_GRANT]: "get/grant",
  [Pattern.GET_GRANTS]: "get/grants",
  [Pattern.GET_LABELED_WORKLOAD_LIST]: "get/labeled-workload-list",
  [Pattern.GET_NAMESPACE_WORKLOAD_LIST]: "get/namespace-workload-list",
  [Pattern.GET_NODES_METRICS]: "get/nodes-metrics",
  [Pattern.GET_USER]: "get/user",
  [Pattern.GET_USERS]: "get/users",
  [Pattern.GET_WORKLOAD]: "get/workload",
  [Pattern.GET_WORKLOAD_EXAMPLE]: "get/workload-example",
  [Pattern.GET_WORKLOAD_LIST]: "get/workload-list",
  [Pattern.GET_WORKLOAD_STATUS]: "get/workload-status",
  [Pattern.GET_WORKSPACE]: "get/workspace",
  [Pattern.GET_WORKSPACES]: "get/workspaces",
  [Pattern.GET_WORKSPACE_WORKLOADS]: "get/workspace-workloads",
  [Pattern.INSTALL_CERT_MANAGER]: "install-cert-manager",
  [Pattern.INSTALL_CLUSTER_ISSUER]: "install-cluster-issuer",
  [Pattern.INSTALL_INGRESS_CONTROLLER_TRAEFIK]: "install-ingress-controller-traefik",
  [Pattern.INSTALL_KEPLER]: "install-kepler",
  [Pattern.INSTALL_METALLB]: "install-metallb",
  [Pattern.INSTALL_METRICS_SERVER]: "install-metrics-server",
  [Pattern.LIST_ALL_RESOURCE_DESCRIPTORS]: "list/all-resource-descriptors",
  [Pattern.LIVE_STREAM_NODES_CPU]: "live-stream/nodes-cpu",
  [Pattern.LIVE_STREAM_NODES_MEMORY]: "live-stream/nodes-memory",
  [Pattern.LIVE_STREAM_NODES_TRAFFIC]: "live-stream/nodes-traffic",
  [Pattern.LIVE_STREAM_POD_CPU]: "live-stream/pod-cpu",
  [Pattern.LIVE_STREAM_POD_MEMORY]: "live-stream/pod-memory",
  [Pattern.LIVE_STREAM_POD_TRAFFIC]: "live-stream/pod-traffic",
  [Pattern.LIVE_STREAM_WORKSPACE_CPU]: "live-stream/workspace-cpu",
  [Pattern.LIVE_STREAM_WORKSPACE_MEMORY]: "live-stream/workspace-memory",
  [Pattern.LIVE_STREAM_WORKSPACE_TRAFFIC]: "live-stream/workspace-traffic",
  [Pattern.PROMETHEUS_CHARTS_ADD]: "prometheus/charts/add",
  [Pattern.PROMETHEUS_CHARTS_GET]: "prometheus/charts/get",
  [Pattern.PROMETHEUS_CHARTS_LIST]: "prometheus/charts/list",
  [Pattern.PROMETHEUS_CHARTS_REMOVE]: "prometheus/charts/remove",
  [Pattern.PROMETHEUS_IS_REACHABLE]: "prometheus/is-reachable",
  [Pattern.PROMETHEUS_QUERY]: "prometheus/query",
  [Pattern.PROMETHEUS_VALUES]: "prometheus/values",
  [Pattern.SEALED_SECRET_CREATE_FROM_EXISTING]: "sealed-secret/create-from-existing",
  [Pattern.SEALED_SECRET_GET_CERTIFICATE]: "sealed-secret/get-certificate",
  [Pattern.SERVICE_EXEC_SH_CONNECTION_REQUEST]: "service/exec-sh-connection-request",
  [Pattern.SERVICE_LOG_STREAM_CONNECTION_REQUEST]: "service/log-stream-connection-request",
  [Pattern.SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST]: "service/pod-event-stream-connection-request",
  [Pattern.STATS_POD_ALL_FOR_CONTROLLER]: "stats/pod/all-for-controller",
  [Pattern.STATS_TRAFFIC_ALL_FOR_CONTROLLER]: "stats/traffic/all-for-controller",
  [Pattern.STATS_WORKSPACE_CPU_UTILIZATION]: "stats/workspace-cpu-utilization",
  [Pattern.STATS_WORKSPACE_MEMORY_UTILIZATION]: "stats/workspace-memory-utilization",
  [Pattern.STATS_WORKSPACE_TRAFFIC_UTILIZATION]: "stats/workspace-traffic-utilization",
  [Pattern.STORAGE_CREATE_VOLUME]: "storage/create-volume",
  [Pattern.STORAGE_DELETE_VOLUME]: "storage/delete-volume",
  [Pattern.STORAGE_STATS]: "storage/stats",
  [Pattern.STORAGE_STATUS]: "storage/status",
  [Pattern.SYSTEM_CHECK]: "system/check",
  [Pattern.TRIGGER_WORKLOAD]: "trigger/workload",
  [Pattern.UNINSTALL_CERT_MANAGER]: "uninstall-cert-manager",
  [Pattern.UNINSTALL_CLUSTER_ISSUER]: "uninstall-cluster-issuer",
  [Pattern.UNINSTALL_INGRESS_CONTROLLER_TRAEFIK]: "uninstall-ingress-controller-traefik",
  [Pattern.UNINSTALL_KEPLER]: "uninstall-kepler",
  [Pattern.UNINSTALL_METALLB]: "uninstall-metallb",
  [Pattern.UNINSTALL_METRICS_SERVER]: "uninstall-metrics-server",
  [Pattern.UPDATE_GRANT]: "update/grant",
  [Pattern.UPDATE_USER]: "update/user",
  [Pattern.UPDATE_WORKLOAD]: "update/workload",
  [Pattern.UPDATE_WORKSPACE]: "update/workspace",
  [Pattern.UPGRADEK8SMANAGER]: "UpgradeK8sManager",
  [Pattern.UPGRADE_CERT_MANAGER]: "upgrade-cert-manager",
  [Pattern.UPGRADE_INGRESS_CONTROLLER_TRAEFIK]: "upgrade-ingress-controller-traefik",
  [Pattern.UPGRADE_KEPLER]: "upgrade-kepler",
  [Pattern.UPGRADE_METALLB]: "upgrade-metallb",
  [Pattern.UPGRADE_METRICS_SERVER]: "upgrade-metrics-server",
  [Pattern.WORKSPACE_CLEAN_UP]: "workspace/clean-up",
};

//===============================================================
//================= Request and Response Types ==================
//===============================================================

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             limit:
 *                 type: int
 *             offset:
 *                 type: int
 *             workspaceName:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type AUDIT_LOG_LIST_REQUEST = AUDIT_LOG_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Response:
 *         name: mogenius-operator/src/core.Response
 *         properties:
 *             data:
 *                 structRef: mogenius-operator/src/store.AuditLogResponse
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·34,mogenius-operator/src/core.Response·35]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·34,mogenius-operator/src/core.Response·35]
 *         properties:
 *             data:
 *                 structRef: mogenius-operator/src/core.Response
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/store.AuditLogEntry:
 *         name: mogenius-operator/src/store.AuditLogEntry
 *         properties:
 *             createdAt:
 *                 structRef: time.Time
 *                 type: struct
 *             diff:
 *                 type: string
 *             error:
 *                 type: string
 *             pattern:
 *                 type: string
 *             payload:
 *                 pointer: true
 *                 type: any
 *             result:
 *                 pointer: true
 *                 type: any
 *             user:
 *                 structRef: mogenius-operator/src/structs.User
 *                 type: struct
 *             workspace:
 *                 type: string
 *     mogenius-operator/src/store.AuditLogResponse:
 *         name: mogenius-operator/src/store.AuditLogResponse
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/store.AuditLogEntry
 *                     type: struct
 *                 type: array
 *             totalCount:
 *                 type: int
 *     mogenius-operator/src/structs.User:
 *         name: mogenius-operator/src/structs.User
 *         properties:
 *             email:
 *                 type: string
 *             firstName:
 *                 type: string
 *             lastName:
 *                 type: string
 *             source:
 *                 type: string
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·34,mogenius-operator/src/core.Response·35]
 *     type: struct
 * ```
 *
 */
export type AUDIT_LOG_LIST_RESPONSE = AUDIT_LOG_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST34_MOGENIUS_OPERATOR_SRC_CORE_RESPONSE35;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/argocd.ArgoCdApplicationRefreshRequest:
 *         name: mogenius-operator/src/argocd.ArgoCdApplicationRefreshRequest
 *         properties:
 *             applicationName:
 *                 type: string
 *             username:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/argocd.ArgoCdApplicationRefreshRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_ARGO_CD_APPLICATION_REFRESH_REQUEST = CLUSTER_ARGO_CD_APPLICATION_REFRESH_REQUEST__MOGENIUS_OPERATOR_SRC_ARGOCD_ARGOCDAPPLICATIONREFRESHREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/argocd.ArgoCdApplicationRefreshRequest,bool]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/argocd.ArgoCdApplicationRefreshRequest,bool]
 *         properties:
 *             data:
 *                 type: bool
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/argocd.ArgoCdApplicationRefreshRequest,bool]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_ARGO_CD_APPLICATION_REFRESH_RESPONSE = CLUSTER_ARGO_CD_APPLICATION_REFRESH_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_ARGOCD_ARGOCDAPPLICATIONREFRESHREQUEST_BOOL;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/argocd.ArgoCdCreateApiTokenRequest:
 *         name: mogenius-operator/src/argocd.ArgoCdCreateApiTokenRequest
 *         properties:
 *             username:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/argocd.ArgoCdCreateApiTokenRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_ARGO_CD_CREATE_API_TOKEN_REQUEST = CLUSTER_ARGO_CD_CREATE_API_TOKEN_REQUEST__MOGENIUS_OPERATOR_SRC_ARGOCD_ARGOCDCREATEAPITOKENREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/argocd.ArgoCdCreateApiTokenRequest,bool]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/argocd.ArgoCdCreateApiTokenRequest,bool]
 *         properties:
 *             data:
 *                 type: bool
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/argocd.ArgoCdCreateApiTokenRequest,bool]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_ARGO_CD_CREATE_API_TOKEN_RESPONSE = CLUSTER_ARGO_CD_CREATE_API_TOKEN_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_ARGOCD_ARGOCDCREATEAPITOKENREQUEST_BOOL;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             includeNodeStats:
 *                 type: bool
 *             includePodStats:
 *                 type: bool
 *             includeTraffic:
 *                 type: bool
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type CLUSTER_CLEAR_VALKEY_CACHE_REQUEST = CLUSTER_CLEAR_VALKEY_CACHE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·5,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·5,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·5,string]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_CLEAR_VALKEY_CACHE_RESPONSE = CLUSTER_CLEAR_VALKEY_CACHE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST5_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/xterm.ComponentLogConnectionRequest:
 *         name: mogenius-operator/src/xterm.ComponentLogConnectionRequest
 *         properties:
 *             component:
 *                 type: string
 *             controller:
 *                 pointer: true
 *                 type: string
 *             namespace:
 *                 pointer: true
 *                 type: string
 *             release:
 *                 pointer: true
 *                 type: string
 *             wsConnectionRequest:
 *                 structRef: mogenius-operator/src/xterm.WsConnectionRequest
 *                 type: struct
 *     mogenius-operator/src/xterm.WsConnectionRequest:
 *         name: mogenius-operator/src/xterm.WsConnectionRequest
 *         properties:
 *             channelId:
 *                 type: string
 *             cmdType:
 *                 type: string
 *             nodeName:
 *                 type: string
 *             podName:
 *                 type: string
 *             websocketHost:
 *                 type: string
 *             websocketScheme:
 *                 type: string
 *             workspace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/xterm.ComponentLogConnectionRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST_REQUEST = CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_COMPONENTLOGCONNECTIONREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_1:
 *         properties: {}
 *     mogenius-operator/src/core.Result[mogenius-operator/src/xterm.ComponentLogConnectionRequest,mogenius-operator/src/core.Void]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.ComponentLogConnectionRequest,mogenius-operator/src/core.Void]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: ANON_STRUCT_1
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.ComponentLogConnectionRequest,mogenius-operator/src/core.Void]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST_RESPONSE = CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_COMPONENTLOGCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type CLUSTER_FORCE_DISCONNECT_REQUEST = CLUSTER_FORCE_DISCONNECT_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,bool]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,bool]
 *         properties:
 *             data:
 *                 type: bool
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,bool]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_FORCE_DISCONNECT_RESPONSE = CLUSTER_FORCE_DISCONNECT_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_BOOL;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type CLUSTER_FORCE_RECONNECT_REQUEST = CLUSTER_FORCE_RECONNECT_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,bool]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,bool]
 *         properties:
 *             data:
 *                 type: bool
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,bool]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_FORCE_RECONNECT_RESPONSE = CLUSTER_FORCE_RECONNECT_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_BOOL;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmChartInstallUpgradeRequest:
 *         name: mogenius-operator/src/helm.HelmChartInstallUpgradeRequest
 *         properties:
 *             chart:
 *                 type: string
 *             dryRun:
 *                 type: bool
 *             namespace:
 *                 type: string
 *             release:
 *                 type: string
 *             values:
 *                 type: string
 *             version:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmChartInstallUpgradeRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_CHART_INSTALL_REQUEST = CLUSTER_HELM_CHART_INSTALL_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINSTALLUPGRADEREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartInstallUpgradeRequest,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartInstallUpgradeRequest,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartInstallUpgradeRequest,string]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_CHART_INSTALL_RESPONSE = CLUSTER_HELM_CHART_INSTALL_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINSTALLUPGRADEREQUEST_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmChartOciInstallUpgradeRequest:
 *         name: mogenius-operator/src/helm.HelmChartOciInstallUpgradeRequest
 *         properties:
 *             chart:
 *                 type: string
 *             dryRun:
 *                 type: bool
 *             namespace:
 *                 type: string
 *             password:
 *                 type: string
 *             registryUrl:
 *                 type: string
 *             release:
 *                 type: string
 *             username:
 *                 type: string
 *             values:
 *                 type: string
 *             version:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmChartOciInstallUpgradeRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_CHART_INSTALL_OCI_REQUEST = CLUSTER_HELM_CHART_INSTALL_OCI_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTOCIINSTALLUPGRADEREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartOciInstallUpgradeRequest,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartOciInstallUpgradeRequest,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartOciInstallUpgradeRequest,string]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_CHART_INSTALL_OCI_RESPONSE = CLUSTER_HELM_CHART_INSTALL_OCI_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMCHARTOCIINSTALLUPGRADEREQUEST_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmRepoRemoveRequest:
 *         name: mogenius-operator/src/helm.HelmRepoRemoveRequest
 *         properties:
 *             name:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmRepoRemoveRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_CHART_REMOVE_REQUEST = CLUSTER_HELM_CHART_REMOVE_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMREPOREMOVEREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmRepoRemoveRequest,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmRepoRemoveRequest,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmRepoRemoveRequest,string]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_CHART_REMOVE_RESPONSE = CLUSTER_HELM_CHART_REMOVE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMREPOREMOVEREQUEST_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmChartSearchRequest:
 *         name: mogenius-operator/src/helm.HelmChartSearchRequest
 *         properties:
 *             name:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmChartSearchRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_CHART_SEARCH_REQUEST = CLUSTER_HELM_CHART_SEARCH_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTSEARCHREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartSearchRequest,[]mogenius-operator/src/helm.HelmChartInfo]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartSearchRequest,[]mogenius-operator/src/helm.HelmChartInfo]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/helm.HelmChartInfo
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/helm.HelmChartInfo:
 *         name: mogenius-operator/src/helm.HelmChartInfo
 *         properties:
 *             app_version:
 *                 type: string
 *             description:
 *                 type: string
 *             name:
 *                 type: string
 *             version:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartSearchRequest,[]mogenius-operator/src/helm.HelmChartInfo]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_CHART_SEARCH_RESPONSE = CLUSTER_HELM_CHART_SEARCH_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMCHARTSEARCHREQUEST_MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINFO;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmChartShowRequest:
 *         name: mogenius-operator/src/helm.HelmChartShowRequest
 *         properties:
 *             chart:
 *                 type: string
 *             format:
 *                 type: string
 *             version:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmChartShowRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_CHART_SHOW_REQUEST = CLUSTER_HELM_CHART_SHOW_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTSHOWREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartShowRequest,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartShowRequest,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartShowRequest,string]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_CHART_SHOW_RESPONSE = CLUSTER_HELM_CHART_SHOW_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMCHARTSHOWREQUEST_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmChartVersionRequest:
 *         name: mogenius-operator/src/helm.HelmChartVersionRequest
 *         properties:
 *             chart:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmChartVersionRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_CHART_VERSIONS_REQUEST = CLUSTER_HELM_CHART_VERSIONS_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTVERSIONREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartVersionRequest,[]mogenius-operator/src/helm.HelmChartInfo]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartVersionRequest,[]mogenius-operator/src/helm.HelmChartInfo]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/helm.HelmChartInfo
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/helm.HelmChartInfo:
 *         name: mogenius-operator/src/helm.HelmChartInfo
 *         properties:
 *             app_version:
 *                 type: string
 *             description:
 *                 type: string
 *             name:
 *                 type: string
 *             version:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartVersionRequest,[]mogenius-operator/src/helm.HelmChartInfo]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_CHART_VERSIONS_RESPONSE = CLUSTER_HELM_CHART_VERSIONS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMCHARTVERSIONREQUEST_MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINFO;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmReleaseGetRequest:
 *         name: mogenius-operator/src/helm.HelmReleaseGetRequest
 *         properties:
 *             getFormat:
 *                 type: string
 *             namespace:
 *                 type: string
 *             release:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmReleaseGetRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_GET_REQUEST = CLUSTER_HELM_RELEASE_GET_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEGETREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseGetRequest,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseGetRequest,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseGetRequest,string]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_GET_RESPONSE = CLUSTER_HELM_RELEASE_GET_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEGETREQUEST_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmReleaseGetWorkloadsRequest:
 *         name: mogenius-operator/src/helm.HelmReleaseGetWorkloadsRequest
 *         properties:
 *             namespace:
 *                 type: string
 *             release:
 *                 type: string
 *             whitelist:
 *                 elementType:
 *                     pointer: true
 *                     structRef: mogenius-operator/src/utils.ResourceDescriptor
 *                     type: struct
 *                 type: array
 *     mogenius-operator/src/utils.ResourceDescriptor:
 *         name: mogenius-operator/src/utils.ResourceDescriptor
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *             namespaced:
 *                 type: bool
 *             plural:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmReleaseGetWorkloadsRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_GET_WORKLOADS_REQUEST = CLUSTER_HELM_RELEASE_GET_WORKLOADS_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEGETWORKLOADSREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *         properties:
 *             Object:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseGetWorkloadsRequest,[]k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseGetWorkloadsRequest,[]k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseGetWorkloadsRequest,[]k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_GET_WORKLOADS_RESPONSE = CLUSTER_HELM_RELEASE_GET_WORKLOADS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEGETWORKLOADSREQUEST_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmReleaseHistoryRequest:
 *         name: mogenius-operator/src/helm.HelmReleaseHistoryRequest
 *         properties:
 *             namespace:
 *                 type: string
 *             release:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmReleaseHistoryRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_HISTORY_REQUEST = CLUSTER_HELM_RELEASE_HISTORY_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEHISTORYREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     helm.sh/helm/v4/pkg/chart/common.File:
 *         name: helm.sh/helm/v4/pkg/chart/common.File
 *         properties:
 *             data:
 *                 elementType:
 *                     type: uint
 *                 type: array
 *             modtime:
 *                 structRef: time.Time
 *                 type: struct
 *             name:
 *                 type: string
 *     helm.sh/helm/v4/pkg/chart/v2.Chart:
 *         name: helm.sh/helm/v4/pkg/chart/v2.Chart
 *         properties:
 *             files:
 *                 elementType:
 *                     pointer: true
 *                     structRef: helm.sh/helm/v4/pkg/chart/common.File
 *                     type: struct
 *                 type: array
 *             lock:
 *                 pointer: true
 *                 structRef: helm.sh/helm/v4/pkg/chart/v2.Lock
 *                 type: struct
 *             metadata:
 *                 pointer: true
 *                 structRef: helm.sh/helm/v4/pkg/chart/v2.Metadata
 *                 type: struct
 *             modtime:
 *                 structRef: time.Time
 *                 type: struct
 *             schema:
 *                 elementType:
 *                     type: uint
 *                 type: array
 *             schemamodtime:
 *                 structRef: time.Time
 *                 type: struct
 *             templates:
 *                 elementType:
 *                     pointer: true
 *                     structRef: helm.sh/helm/v4/pkg/chart/common.File
 *                     type: struct
 *                 type: array
 *             values:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *     helm.sh/helm/v4/pkg/chart/v2.Dependency:
 *         name: helm.sh/helm/v4/pkg/chart/v2.Dependency
 *         properties:
 *             alias:
 *                 type: string
 *             condition:
 *                 type: string
 *             enabled:
 *                 type: bool
 *             import-values:
 *                 elementType:
 *                     pointer: true
 *                     type: any
 *                 type: array
 *             name:
 *                 type: string
 *             repository:
 *                 type: string
 *             tags:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             version:
 *                 type: string
 *     helm.sh/helm/v4/pkg/chart/v2.Lock:
 *         name: helm.sh/helm/v4/pkg/chart/v2.Lock
 *         properties:
 *             dependencies:
 *                 elementType:
 *                     pointer: true
 *                     structRef: helm.sh/helm/v4/pkg/chart/v2.Dependency
 *                     type: struct
 *                 type: array
 *             digest:
 *                 type: string
 *             generated:
 *                 structRef: time.Time
 *                 type: struct
 *     helm.sh/helm/v4/pkg/chart/v2.Maintainer:
 *         name: helm.sh/helm/v4/pkg/chart/v2.Maintainer
 *         properties:
 *             email:
 *                 type: string
 *             name:
 *                 type: string
 *             url:
 *                 type: string
 *     helm.sh/helm/v4/pkg/chart/v2.Metadata:
 *         name: helm.sh/helm/v4/pkg/chart/v2.Metadata
 *         properties:
 *             annotations:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             apiVersion:
 *                 type: string
 *             appVersion:
 *                 type: string
 *             condition:
 *                 type: string
 *             dependencies:
 *                 elementType:
 *                     pointer: true
 *                     structRef: helm.sh/helm/v4/pkg/chart/v2.Dependency
 *                     type: struct
 *                 type: array
 *             deprecated:
 *                 type: bool
 *             description:
 *                 type: string
 *             home:
 *                 type: string
 *             icon:
 *                 type: string
 *             keywords:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             kubeVersion:
 *                 type: string
 *             maintainers:
 *                 elementType:
 *                     pointer: true
 *                     structRef: helm.sh/helm/v4/pkg/chart/v2.Maintainer
 *                     type: struct
 *                 type: array
 *             name:
 *                 type: string
 *             sources:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             tags:
 *                 type: string
 *             type:
 *                 type: string
 *             version:
 *                 type: string
 *     helm.sh/helm/v4/pkg/release/v1.Hook:
 *         name: helm.sh/helm/v4/pkg/release/v1.Hook
 *         properties:
 *             delete_policies:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             events:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             kind:
 *                 type: string
 *             last_run:
 *                 structRef: helm.sh/helm/v4/pkg/release/v1.HookExecution
 *                 type: struct
 *             manifest:
 *                 type: string
 *             name:
 *                 type: string
 *             output_log_policies:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             path:
 *                 type: string
 *             weight:
 *                 type: int
 *     helm.sh/helm/v4/pkg/release/v1.HookExecution:
 *         name: helm.sh/helm/v4/pkg/release/v1.HookExecution
 *         properties:
 *             completed_at:
 *                 structRef: time.Time
 *                 type: struct
 *             phase:
 *                 type: string
 *             started_at:
 *                 structRef: time.Time
 *                 type: struct
 *     helm.sh/helm/v4/pkg/release/v1.Info:
 *         name: helm.sh/helm/v4/pkg/release/v1.Info
 *         properties:
 *             deleted:
 *                 structRef: time.Time
 *                 type: struct
 *             description:
 *                 type: string
 *             first_deployed:
 *                 structRef: time.Time
 *                 type: struct
 *             last_deployed:
 *                 structRef: time.Time
 *                 type: struct
 *             notes:
 *                 type: string
 *             resources:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     elementType:
 *                         pointer: true
 *                         type: any
 *                     type: array
 *             status:
 *                 type: string
 *     helm.sh/helm/v4/pkg/release/v1.Release:
 *         name: helm.sh/helm/v4/pkg/release/v1.Release
 *         properties:
 *             apply_method:
 *                 type: string
 *             chart:
 *                 pointer: true
 *                 structRef: helm.sh/helm/v4/pkg/chart/v2.Chart
 *                 type: struct
 *             config:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *             hooks:
 *                 elementType:
 *                     pointer: true
 *                     structRef: helm.sh/helm/v4/pkg/release/v1.Hook
 *                     type: struct
 *                 type: array
 *             info:
 *                 pointer: true
 *                 structRef: helm.sh/helm/v4/pkg/release/v1.Info
 *                 type: struct
 *             manifest:
 *                 type: string
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             version:
 *                 type: int
 *     mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseHistoryRequest,[]helm.sh/helm/v4/pkg/release/v1.Release]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseHistoryRequest,[]helm.sh/helm/v4/pkg/release/v1.Release]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: helm.sh/helm/v4/pkg/release/v1.Release
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseHistoryRequest,[]helm.sh/helm/v4/pkg/release/v1.Release]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_HISTORY_RESPONSE = CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEHISTORYREQUEST_HELM_SH_HELM_V4_PKG_RELEASE_V1_RELEASE;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmReleaseLinkRequest:
 *         name: mogenius-operator/src/helm.HelmReleaseLinkRequest
 *         properties:
 *             namespace:
 *                 type: string
 *             releaseName:
 *                 type: string
 *             repoName:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmReleaseLinkRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_LINK_REQUEST = CLUSTER_HELM_RELEASE_LINK_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASELINKREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseLinkRequest,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseLinkRequest,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseLinkRequest,string]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_LINK_RESPONSE = CLUSTER_HELM_RELEASE_LINK_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASELINKREQUEST_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmReleaseListRequest:
 *         name: mogenius-operator/src/helm.HelmReleaseListRequest
 *         properties:
 *             namespace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmReleaseListRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_LIST_REQUEST = CLUSTER_HELM_RELEASE_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASELISTREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     helm.sh/helm/v4/pkg/chart/common.File:
 *         name: helm.sh/helm/v4/pkg/chart/common.File
 *         properties:
 *             data:
 *                 elementType:
 *                     type: uint
 *                 type: array
 *             modtime:
 *                 structRef: time.Time
 *                 type: struct
 *             name:
 *                 type: string
 *     helm.sh/helm/v4/pkg/chart/v2.Chart:
 *         name: helm.sh/helm/v4/pkg/chart/v2.Chart
 *         properties:
 *             files:
 *                 elementType:
 *                     pointer: true
 *                     structRef: helm.sh/helm/v4/pkg/chart/common.File
 *                     type: struct
 *                 type: array
 *             lock:
 *                 pointer: true
 *                 structRef: helm.sh/helm/v4/pkg/chart/v2.Lock
 *                 type: struct
 *             metadata:
 *                 pointer: true
 *                 structRef: helm.sh/helm/v4/pkg/chart/v2.Metadata
 *                 type: struct
 *             modtime:
 *                 structRef: time.Time
 *                 type: struct
 *             schema:
 *                 elementType:
 *                     type: uint
 *                 type: array
 *             schemamodtime:
 *                 structRef: time.Time
 *                 type: struct
 *             templates:
 *                 elementType:
 *                     pointer: true
 *                     structRef: helm.sh/helm/v4/pkg/chart/common.File
 *                     type: struct
 *                 type: array
 *             values:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *     helm.sh/helm/v4/pkg/chart/v2.Dependency:
 *         name: helm.sh/helm/v4/pkg/chart/v2.Dependency
 *         properties:
 *             alias:
 *                 type: string
 *             condition:
 *                 type: string
 *             enabled:
 *                 type: bool
 *             import-values:
 *                 elementType:
 *                     pointer: true
 *                     type: any
 *                 type: array
 *             name:
 *                 type: string
 *             repository:
 *                 type: string
 *             tags:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             version:
 *                 type: string
 *     helm.sh/helm/v4/pkg/chart/v2.Lock:
 *         name: helm.sh/helm/v4/pkg/chart/v2.Lock
 *         properties:
 *             dependencies:
 *                 elementType:
 *                     pointer: true
 *                     structRef: helm.sh/helm/v4/pkg/chart/v2.Dependency
 *                     type: struct
 *                 type: array
 *             digest:
 *                 type: string
 *             generated:
 *                 structRef: time.Time
 *                 type: struct
 *     helm.sh/helm/v4/pkg/chart/v2.Maintainer:
 *         name: helm.sh/helm/v4/pkg/chart/v2.Maintainer
 *         properties:
 *             email:
 *                 type: string
 *             name:
 *                 type: string
 *             url:
 *                 type: string
 *     helm.sh/helm/v4/pkg/chart/v2.Metadata:
 *         name: helm.sh/helm/v4/pkg/chart/v2.Metadata
 *         properties:
 *             annotations:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             apiVersion:
 *                 type: string
 *             appVersion:
 *                 type: string
 *             condition:
 *                 type: string
 *             dependencies:
 *                 elementType:
 *                     pointer: true
 *                     structRef: helm.sh/helm/v4/pkg/chart/v2.Dependency
 *                     type: struct
 *                 type: array
 *             deprecated:
 *                 type: bool
 *             description:
 *                 type: string
 *             home:
 *                 type: string
 *             icon:
 *                 type: string
 *             keywords:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             kubeVersion:
 *                 type: string
 *             maintainers:
 *                 elementType:
 *                     pointer: true
 *                     structRef: helm.sh/helm/v4/pkg/chart/v2.Maintainer
 *                     type: struct
 *                 type: array
 *             name:
 *                 type: string
 *             sources:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             tags:
 *                 type: string
 *             type:
 *                 type: string
 *             version:
 *                 type: string
 *     helm.sh/helm/v4/pkg/release/v1.Hook:
 *         name: helm.sh/helm/v4/pkg/release/v1.Hook
 *         properties:
 *             delete_policies:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             events:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             kind:
 *                 type: string
 *             last_run:
 *                 structRef: helm.sh/helm/v4/pkg/release/v1.HookExecution
 *                 type: struct
 *             manifest:
 *                 type: string
 *             name:
 *                 type: string
 *             output_log_policies:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             path:
 *                 type: string
 *             weight:
 *                 type: int
 *     helm.sh/helm/v4/pkg/release/v1.HookExecution:
 *         name: helm.sh/helm/v4/pkg/release/v1.HookExecution
 *         properties:
 *             completed_at:
 *                 structRef: time.Time
 *                 type: struct
 *             phase:
 *                 type: string
 *             started_at:
 *                 structRef: time.Time
 *                 type: struct
 *     helm.sh/helm/v4/pkg/release/v1.Info:
 *         name: helm.sh/helm/v4/pkg/release/v1.Info
 *         properties:
 *             deleted:
 *                 structRef: time.Time
 *                 type: struct
 *             description:
 *                 type: string
 *             first_deployed:
 *                 structRef: time.Time
 *                 type: struct
 *             last_deployed:
 *                 structRef: time.Time
 *                 type: struct
 *             notes:
 *                 type: string
 *             resources:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     elementType:
 *                         pointer: true
 *                         type: any
 *                     type: array
 *             status:
 *                 type: string
 *     mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseListRequest,[]*mogenius-operator/src/helm.HelmRelease]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseListRequest,[]*mogenius-operator/src/helm.HelmRelease]
 *         properties:
 *             data:
 *                 elementType:
 *                     pointer: true
 *                     structRef: mogenius-operator/src/helm.HelmRelease
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/helm.HelmRelease:
 *         name: mogenius-operator/src/helm.HelmRelease
 *         properties:
 *             chart:
 *                 pointer: true
 *                 structRef: helm.sh/helm/v4/pkg/chart/v2.Chart
 *                 type: struct
 *             config:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *             hooks:
 *                 elementType:
 *                     pointer: true
 *                     structRef: helm.sh/helm/v4/pkg/release/v1.Hook
 *                     type: struct
 *                 type: array
 *             info:
 *                 pointer: true
 *                 structRef: helm.sh/helm/v4/pkg/release/v1.Info
 *                 type: struct
 *             manifest:
 *                 type: string
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             repoName:
 *                 type: string
 *             version:
 *                 type: int
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseListRequest,[]*mogenius-operator/src/helm.HelmRelease]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_LIST_RESPONSE = CLUSTER_HELM_RELEASE_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASELISTREQUEST_MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASE;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmReleaseRollbackRequest:
 *         name: mogenius-operator/src/helm.HelmReleaseRollbackRequest
 *         properties:
 *             namespace:
 *                 type: string
 *             release:
 *                 type: string
 *             revision:
 *                 type: int
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmReleaseRollbackRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_ROLLBACK_REQUEST = CLUSTER_HELM_RELEASE_ROLLBACK_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEROLLBACKREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseRollbackRequest,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseRollbackRequest,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseRollbackRequest,string]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_ROLLBACK_RESPONSE = CLUSTER_HELM_RELEASE_ROLLBACK_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEROLLBACKREQUEST_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmReleaseStatusRequest:
 *         name: mogenius-operator/src/helm.HelmReleaseStatusRequest
 *         properties:
 *             namespace:
 *                 type: string
 *             release:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmReleaseStatusRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_STATUS_REQUEST = CLUSTER_HELM_RELEASE_STATUS_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASESTATUSREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseStatusRequest,*mogenius-operator/src/helm.HelmReleaseStatusInfo]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseStatusRequest,*mogenius-operator/src/helm.HelmReleaseStatusInfo]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/helm.HelmReleaseStatusInfo
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/helm.HelmReleaseStatusInfo:
 *         name: mogenius-operator/src/helm.HelmReleaseStatusInfo
 *         properties:
 *             chart:
 *                 type: string
 *             lastDeployed:
 *                 structRef: time.Time
 *                 type: struct
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             status:
 *                 type: string
 *             version:
 *                 type: int
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseStatusRequest,*mogenius-operator/src/helm.HelmReleaseStatusInfo]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_STATUS_RESPONSE = CLUSTER_HELM_RELEASE_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASESTATUSREQUEST_MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASESTATUSINFO;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmReleaseUninstallRequest:
 *         name: mogenius-operator/src/helm.HelmReleaseUninstallRequest
 *         properties:
 *             dryRun:
 *                 type: bool
 *             namespace:
 *                 type: string
 *             release:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmReleaseUninstallRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_UNINSTALL_REQUEST = CLUSTER_HELM_RELEASE_UNINSTALL_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEUNINSTALLREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseUninstallRequest,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseUninstallRequest,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmReleaseUninstallRequest,string]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_UNINSTALL_RESPONSE = CLUSTER_HELM_RELEASE_UNINSTALL_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEUNINSTALLREQUEST_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmChartInstallUpgradeRequest:
 *         name: mogenius-operator/src/helm.HelmChartInstallUpgradeRequest
 *         properties:
 *             chart:
 *                 type: string
 *             dryRun:
 *                 type: bool
 *             namespace:
 *                 type: string
 *             release:
 *                 type: string
 *             values:
 *                 type: string
 *             version:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmChartInstallUpgradeRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_UPGRADE_REQUEST = CLUSTER_HELM_RELEASE_UPGRADE_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINSTALLUPGRADEREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartInstallUpgradeRequest,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartInstallUpgradeRequest,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmChartInstallUpgradeRequest,string]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_RELEASE_UPGRADE_RESPONSE = CLUSTER_HELM_RELEASE_UPGRADE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINSTALLUPGRADEREQUEST_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmRepoAddRequest:
 *         name: mogenius-operator/src/helm.HelmRepoAddRequest
 *         properties:
 *             insecureSkipTLSverify:
 *                 type: bool
 *             name:
 *                 type: string
 *             passCredentialsAll:
 *                 type: bool
 *             password:
 *                 type: string
 *             url:
 *                 type: string
 *             username:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmRepoAddRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_REPO_ADD_REQUEST = CLUSTER_HELM_REPO_ADD_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMREPOADDREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmRepoAddRequest,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmRepoAddRequest,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmRepoAddRequest,string]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_REPO_ADD_RESPONSE = CLUSTER_HELM_REPO_ADD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMREPOADDREQUEST_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_REPO_LIST_REQUEST = CLUSTER_HELM_REPO_LIST_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,[]*mogenius-operator/src/helm.HelmEntryWithoutPassword]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,[]*mogenius-operator/src/helm.HelmEntryWithoutPassword]
 *         properties:
 *             data:
 *                 elementType:
 *                     pointer: true
 *                     structRef: mogenius-operator/src/helm.HelmEntryWithoutPassword
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/helm.HelmEntryWithoutPassword:
 *         name: mogenius-operator/src/helm.HelmEntryWithoutPassword
 *         properties:
 *             insecure_skip_tls_verify:
 *                 type: bool
 *             name:
 *                 type: string
 *             pass_credentials_all:
 *                 type: bool
 *             url:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,[]*mogenius-operator/src/helm.HelmEntryWithoutPassword]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_REPO_LIST_RESPONSE = CLUSTER_HELM_REPO_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_HELM_HELMENTRYWITHOUTPASSWORD;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/helm.HelmRepoPatchRequest:
 *         name: mogenius-operator/src/helm.HelmRepoPatchRequest
 *         properties:
 *             insecureSkipTLSverify:
 *                 type: bool
 *             name:
 *                 type: string
 *             newName:
 *                 type: string
 *             passCredentialsAll:
 *                 type: bool
 *             password:
 *                 type: string
 *             url:
 *                 type: string
 *             username:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/helm.HelmRepoPatchRequest
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_REPO_PATCH_REQUEST = CLUSTER_HELM_REPO_PATCH_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMREPOPATCHREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmRepoPatchRequest,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmRepoPatchRequest,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/helm.HelmRepoPatchRequest,string]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_REPO_PATCH_RESPONSE = CLUSTER_HELM_REPO_PATCH_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMREPOPATCHREQUEST_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_REPO_UPDATE_REQUEST = CLUSTER_HELM_REPO_UPDATE_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,[]mogenius-operator/src/helm.HelmEntryStatus]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,[]mogenius-operator/src/helm.HelmEntryStatus]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/helm.HelmEntryStatus
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/helm.HelmEntryStatus:
 *         name: mogenius-operator/src/helm.HelmEntryStatus
 *         properties:
 *             entry:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/helm.HelmEntryWithoutPassword
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/helm.HelmEntryWithoutPassword:
 *         name: mogenius-operator/src/helm.HelmEntryWithoutPassword
 *         properties:
 *             insecure_skip_tls_verify:
 *                 type: bool
 *             name:
 *                 type: string
 *             pass_credentials_all:
 *                 type: bool
 *             url:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,[]mogenius-operator/src/helm.HelmEntryStatus]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_HELM_REPO_UPDATE_RESPONSE = CLUSTER_HELM_REPO_UPDATE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_HELM_HELMENTRYSTATUS;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/services.ClusterListWorkloads:
 *         name: mogenius-operator/src/services.ClusterListWorkloads
 *         properties:
 *             labelSelector:
 *                 type: string
 *             namespace:
 *                 type: string
 *             prefix:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/services.ClusterListWorkloads
 *     type: struct
 * ```
 *
 */
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_REQUEST = CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_REQUEST__MOGENIUS_OPERATOR_SRC_SERVICES_CLUSTERLISTWORKLOADS;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/api/core/v1.ModifyVolumeStatus:
 *         name: k8s.io/api/core/v1.ModifyVolumeStatus
 *         properties:
 *             status:
 *                 type: string
 *             targetVolumeAttributesClassName:
 *                 type: string
 *     k8s.io/api/core/v1.PersistentVolumeClaim:
 *         name: k8s.io/api/core/v1.PersistentVolumeClaim
 *         properties:
 *             TypeMeta:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta
 *                 type: struct
 *             metadata:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
 *                 type: struct
 *             spec:
 *                 structRef: k8s.io/api/core/v1.PersistentVolumeClaimSpec
 *                 type: struct
 *             status:
 *                 structRef: k8s.io/api/core/v1.PersistentVolumeClaimStatus
 *                 type: struct
 *     k8s.io/api/core/v1.PersistentVolumeClaimCondition:
 *         name: k8s.io/api/core/v1.PersistentVolumeClaimCondition
 *         properties:
 *             lastProbeTime:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             lastTransitionTime:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             message:
 *                 type: string
 *             reason:
 *                 type: string
 *             status:
 *                 type: string
 *             type:
 *                 type: string
 *     k8s.io/api/core/v1.PersistentVolumeClaimSpec:
 *         name: k8s.io/api/core/v1.PersistentVolumeClaimSpec
 *         properties:
 *             accessModes:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             dataSource:
 *                 pointer: true
 *                 structRef: k8s.io/api/core/v1.TypedLocalObjectReference
 *                 type: struct
 *             dataSourceRef:
 *                 pointer: true
 *                 structRef: k8s.io/api/core/v1.TypedObjectReference
 *                 type: struct
 *             resources:
 *                 structRef: k8s.io/api/core/v1.VolumeResourceRequirements
 *                 type: struct
 *             selector:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelector
 *                 type: struct
 *             storageClassName:
 *                 pointer: true
 *                 type: string
 *             volumeAttributesClassName:
 *                 pointer: true
 *                 type: string
 *             volumeMode:
 *                 pointer: true
 *                 type: string
 *             volumeName:
 *                 type: string
 *     k8s.io/api/core/v1.PersistentVolumeClaimStatus:
 *         name: k8s.io/api/core/v1.PersistentVolumeClaimStatus
 *         properties:
 *             accessModes:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             allocatedResourceStatuses:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             allocatedResources:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     structRef: k8s.io/apimachinery/pkg/api/resource.Quantity
 *                     type: struct
 *             capacity:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     structRef: k8s.io/apimachinery/pkg/api/resource.Quantity
 *                     type: struct
 *             conditions:
 *                 elementType:
 *                     structRef: k8s.io/api/core/v1.PersistentVolumeClaimCondition
 *                     type: struct
 *                 type: array
 *             currentVolumeAttributesClassName:
 *                 pointer: true
 *                 type: string
 *             modifyVolumeStatus:
 *                 pointer: true
 *                 structRef: k8s.io/api/core/v1.ModifyVolumeStatus
 *                 type: struct
 *             phase:
 *                 type: string
 *     k8s.io/api/core/v1.TypedLocalObjectReference:
 *         name: k8s.io/api/core/v1.TypedLocalObjectReference
 *         properties:
 *             apiGroup:
 *                 pointer: true
 *                 type: string
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *     k8s.io/api/core/v1.TypedObjectReference:
 *         name: k8s.io/api/core/v1.TypedObjectReference
 *         properties:
 *             apiGroup:
 *                 pointer: true
 *                 type: string
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             namespace:
 *                 pointer: true
 *                 type: string
 *     k8s.io/api/core/v1.VolumeResourceRequirements:
 *         name: k8s.io/api/core/v1.VolumeResourceRequirements
 *         properties:
 *             limits:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     structRef: k8s.io/apimachinery/pkg/api/resource.Quantity
 *                     type: struct
 *             requests:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     structRef: k8s.io/apimachinery/pkg/api/resource.Quantity
 *                     type: struct
 *     k8s.io/apimachinery/pkg/api/resource.Quantity:
 *         name: k8s.io/apimachinery/pkg/api/resource.Quantity
 *         properties:
 *             Format:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1
 *         properties: {}
 *     k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelector:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelector
 *         properties:
 *             matchExpressions:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelectorRequirement
 *                     type: struct
 *                 type: array
 *             matchLabels:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelectorRequirement:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelectorRequirement
 *         properties:
 *             key:
 *                 type: string
 *             operator:
 *                 type: string
 *             values:
 *                 elementType:
 *                     type: string
 *                 type: array
 *     k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry
 *         properties:
 *             apiVersion:
 *                 type: string
 *             fieldsType:
 *                 type: string
 *             fieldsV1:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1
 *                 type: struct
 *             manager:
 *                 type: string
 *             operation:
 *                 type: string
 *             subresource:
 *                 type: string
 *             time:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *     k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
 *         properties:
 *             annotations:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             creationTimestamp:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             deletionGracePeriodSeconds:
 *                 pointer: true
 *                 type: int
 *             deletionTimestamp:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             finalizers:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             generateName:
 *                 type: string
 *             generation:
 *                 type: int
 *             labels:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             managedFields:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry
 *                     type: struct
 *                 type: array
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             ownerReferences:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference
 *                     type: struct
 *                 type: array
 *             resourceVersion:
 *                 type: string
 *             selfLink:
 *                 type: string
 *             uid:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference
 *         properties:
 *             apiVersion:
 *                 type: string
 *             blockOwnerDeletion:
 *                 pointer: true
 *                 type: bool
 *             controller:
 *                 pointer: true
 *                 type: bool
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             uid:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.Time:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *         properties:
 *             Time:
 *                 structRef: time.Time
 *                 type: struct
 *     k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/services.ClusterListWorkloads,[]k8s.io/api/core/v1.PersistentVolumeClaim]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/services.ClusterListWorkloads,[]k8s.io/api/core/v1.PersistentVolumeClaim]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: k8s.io/api/core/v1.PersistentVolumeClaim
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/services.ClusterListWorkloads,[]k8s.io/api/core/v1.PersistentVolumeClaim]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE = CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_SERVICES_CLUSTERLISTWORKLOADS_K8S_IO_API_CORE_V1_PERSISTENTVOLUMECLAIM;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             nodes:
 *                 elementType:
 *                     type: string
 *                 type: array
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type CLUSTER_MACHINE_STATS_REQUEST = CLUSTER_MACHINE_STATS_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·16,[]mogenius-operator/src/structs.MachineStats]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·16,[]mogenius-operator/src/structs.MachineStats]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/structs.MachineStats
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/structs.MachineStats:
 *         name: mogenius-operator/src/structs.MachineStats
 *         properties:
 *             btfSupport:
 *                 type: bool
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·16,[]mogenius-operator/src/structs.MachineStats]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_MACHINE_STATS_RESPONSE = CLUSTER_MACHINE_STATS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST16_MOGENIUS_OPERATOR_SRC_STRUCTS_MACHINESTATS;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type CLUSTER_RESOURCE_INFO_REQUEST = CLUSTER_RESOURCE_INFO_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.ClusterResourceInfo:
 *         name: mogenius-operator/src/core.ClusterResourceInfo
 *         properties:
 *             cniConfig:
 *                 elementType:
 *                     structRef: mogenius-operator/src/structs.CniData
 *                     type: struct
 *                 type: array
 *             country:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/utils.CountryDetails
 *                 type: struct
 *             error:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             loadBalancerExternalIps:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             nodeStats:
 *                 elementType:
 *                     structRef: mogenius-operator/src/dtos.NodeStat
 *                     type: struct
 *                 type: array
 *             provider:
 *                 type: string
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,mogenius-operator/src/core.ClusterResourceInfo·3]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,mogenius-operator/src/core.ClusterResourceInfo·3]
 *         properties:
 *             data:
 *                 structRef: mogenius-operator/src/core.ClusterResourceInfo
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/dtos.NodeStat:
 *         name: mogenius-operator/src/dtos.NodeStat
 *         properties:
 *             architecture:
 *                 type: string
 *             cpuInCores:
 *                 type: int
 *             cpuInCoresLimited:
 *                 type: float
 *             cpuInCoresRequested:
 *                 type: float
 *             cpuInCoresUtilized:
 *                 type: float
 *             ephemeralInBytes:
 *                 type: int
 *             kubletVersion:
 *                 type: string
 *             machineStats:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/structs.MachineStats
 *                 type: struct
 *             maschineId:
 *                 type: string
 *             maxPods:
 *                 type: int
 *             memoryInBytes:
 *                 type: int
 *             memoryInBytesLimited:
 *                 type: int
 *             memoryInBytesRequested:
 *                 type: int
 *             memoryInBytesUtilized:
 *                 type: int
 *             name:
 *                 type: string
 *             osImage:
 *                 type: string
 *             osKernelVersion:
 *                 type: string
 *             osType:
 *                 type: string
 *             totalPods:
 *                 type: int
 *     mogenius-operator/src/structs.CniCapabilities:
 *         name: mogenius-operator/src/structs.CniCapabilities
 *         properties:
 *             bandwidth:
 *                 type: bool
 *             portMappings:
 *                 type: bool
 *     mogenius-operator/src/structs.CniData:
 *         name: mogenius-operator/src/structs.CniData
 *         properties:
 *             cniVersion:
 *                 type: string
 *             name:
 *                 type: string
 *             node:
 *                 type: string
 *             plugins:
 *                 elementType:
 *                     structRef: mogenius-operator/src/structs.Plugin
 *                     type: struct
 *                 type: array
 *     mogenius-operator/src/structs.CniIPAM:
 *         name: mogenius-operator/src/structs.CniIPAM
 *         properties:
 *             type:
 *                 type: string
 *     mogenius-operator/src/structs.CniPolicy:
 *         name: mogenius-operator/src/structs.CniPolicy
 *         properties:
 *             type:
 *                 type: string
 *     mogenius-operator/src/structs.MachineStats:
 *         name: mogenius-operator/src/structs.MachineStats
 *         properties:
 *             btfSupport:
 *                 type: bool
 *     mogenius-operator/src/structs.Plugin:
 *         name: mogenius-operator/src/structs.Plugin
 *         properties:
 *             capabilities:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/structs.CniCapabilities
 *                 type: struct
 *             datastore_type:
 *                 type: string
 *             ipam:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/structs.CniIPAM
 *                 type: struct
 *             log_file_path:
 *                 type: string
 *             log_level:
 *                 type: string
 *             mtu:
 *                 type: int
 *             nodename:
 *                 type: string
 *             policy:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/structs.CniPolicy
 *                 type: struct
 *             snat:
 *                 pointer: true
 *                 type: bool
 *             type:
 *                 type: string
 *     mogenius-operator/src/utils.CountryDetails:
 *         name: mogenius-operator/src/utils.CountryDetails
 *         properties:
 *             capitalCity:
 *                 type: string
 *             capitalCityLat:
 *                 type: float
 *             capitalCityLng:
 *                 type: float
 *             code:
 *                 type: string
 *             code3:
 *                 type: string
 *             continent:
 *                 type: string
 *             currency:
 *                 type: string
 *             currencyName:
 *                 type: string
 *             domainTld:
 *                 type: string
 *             isActive:
 *                 type: bool
 *             isEuMember:
 *                 type: bool
 *             isoId:
 *                 type: int
 *             languages:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             name:
 *                 type: string
 *             phoneNumberPrefix:
 *                 type: string
 *             taxPercent:
 *                 type: float
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,mogenius-operator/src/core.ClusterResourceInfo·3]
 *     type: struct
 * ```
 *
 */
export type CLUSTER_RESOURCE_INFO_RESPONSE = CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_CORE_CLUSTERRESOURCEINFO3;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             grantee:
 *                 type: string
 *             name:
 *                 type: string
 *             role:
 *                 type: string
 *             targetName:
 *                 type: string
 *             targetType:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type CREATE_GRANT_REQUEST = CREATE_GRANT_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·29,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·29,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·29,string]
 *     type: struct
 * ```
 *
 */
export type CREATE_GRANT_RESPONSE = CREATE_GRANT_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST29_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/utils.ResourceDescriptor:
 *         name: mogenius-operator/src/utils.ResourceDescriptor
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *             namespaced:
 *                 type: bool
 *             plural:
 *                 type: string
 *     mogenius-operator/src/utils.WorkloadChangeRequest:
 *         name: mogenius-operator/src/utils.WorkloadChangeRequest
 *         properties:
 *             ResourceDescriptor:
 *                 structRef: mogenius-operator/src/utils.ResourceDescriptor
 *                 type: struct
 *             namespace:
 *                 type: string
 *             yamlData:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/utils.WorkloadChangeRequest
 *     type: struct
 * ```
 *
 */
export type CREATE_NEW_WORKLOAD_REQUEST = CREATE_NEW_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_WORKLOADCHANGEREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *         properties:
 *             Object:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadChangeRequest,*k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadChangeRequest,*k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadChangeRequest,*k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     type: struct
 * ```
 *
 */
export type CREATE_NEW_WORKLOAD_RESPONSE = CREATE_NEW_WORKLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_UTILS_WORKLOADCHANGEREQUEST_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/api/rbac/v1.Subject:
 *         name: k8s.io/api/rbac/v1.Subject
 *         properties:
 *             apiGroup:
 *                 type: string
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             email:
 *                 type: string
 *             firstName:
 *                 type: string
 *             lastName:
 *                 type: string
 *             name:
 *                 type: string
 *             subject:
 *                 pointer: true
 *                 structRef: k8s.io/api/rbac/v1.Subject
 *                 type: struct
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type CREATE_USER_REQUEST = CREATE_USER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·24,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·24,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·24,string]
 *     type: struct
 * ```
 *
 */
export type CREATE_USER_RESPONSE = CREATE_USER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST24_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             displayName:
 *                 type: string
 *             name:
 *                 type: string
 *             resources:
 *                 elementType:
 *                     structRef: mogenius-operator/src/crds/v1alpha1.WorkspaceResourceIdentifier
 *                     type: struct
 *                 type: array
 *     mogenius-operator/src/crds/v1alpha1.WorkspaceResourceIdentifier:
 *         name: mogenius-operator/src/crds/v1alpha1.WorkspaceResourceIdentifier
 *         properties:
 *             id:
 *                 type: string
 *             namespace:
 *                 type: string
 *             type:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type CREATE_WORKSPACE_REQUEST = CREATE_WORKSPACE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·18,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·18,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·18,string]
 *     type: struct
 * ```
 *
 */
export type CREATE_WORKSPACE_RESPONSE = CREATE_WORKSPACE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST18_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             name:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type DELETE_GRANT_REQUEST = DELETE_GRANT_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·32,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·32,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·32,string]
 *     type: struct
 * ```
 *
 */
export type DELETE_GRANT_RESPONSE = DELETE_GRANT_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST32_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             name:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type DELETE_USER_REQUEST = DELETE_USER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·27,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·27,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·27,string]
 *     type: struct
 * ```
 *
 */
export type DELETE_USER_RESPONSE = DELETE_USER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST27_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/utils.ResourceDescriptor:
 *         name: mogenius-operator/src/utils.ResourceDescriptor
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *             namespaced:
 *                 type: bool
 *             plural:
 *                 type: string
 *     mogenius-operator/src/utils.WorkloadSingleRequest:
 *         name: mogenius-operator/src/utils.WorkloadSingleRequest
 *         properties:
 *             ResourceDescriptor:
 *                 structRef: mogenius-operator/src/utils.ResourceDescriptor
 *                 type: struct
 *             namespace:
 *                 type: string
 *             resourceName:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/utils.WorkloadSingleRequest
 *     type: struct
 * ```
 *
 */
export type DELETE_WORKLOAD_REQUEST = DELETE_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_1:
 *         properties: {}
 *     mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadSingleRequest,mogenius-operator/src/core.Void]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadSingleRequest,mogenius-operator/src/core.Void]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: ANON_STRUCT_1
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadSingleRequest,mogenius-operator/src/core.Void]
 *     type: struct
 * ```
 *
 */
export type DELETE_WORKLOAD_RESPONSE = DELETE_WORKLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             name:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type DELETE_WORKSPACE_REQUEST = DELETE_WORKSPACE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·22,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·22,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·22,string]
 *     type: struct
 * ```
 *
 */
export type DELETE_WORKSPACE_RESPONSE = DELETE_WORKSPACE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST22_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type DESCRIBE_REQUEST = DESCRIBE_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_2:
 *         properties:
 *             buildType:
 *                 type: string
 *             version:
 *                 structRef: mogenius-operator/src/version.Version
 *                 type: struct
 *     ANON_STRUCT_4:
 *         properties: {}
 *     mogenius-operator/src/core.PatternConfig:
 *         name: mogenius-operator/src/core.PatternConfig
 *         properties:
 *             deprecated:
 *                 type: bool
 *             deprecatedMessage:
 *                 type: string
 *             legacyResponseLayout:
 *                 type: bool
 *             needsUser:
 *                 type: bool
 *             requestSchema:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/schema.Schema
 *                 type: struct
 *             responseSchema:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/schema.Schema
 *                 type: struct
 *     mogenius-operator/src/core.Response:
 *         name: mogenius-operator/src/core.Response
 *         properties:
 *             buildInfo:
 *                 structRef: ANON_STRUCT_2
 *                 type: struct
 *             features:
 *                 structRef: ANON_STRUCT_4
 *                 type: struct
 *             patterns:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     structRef: mogenius-operator/src/core.PatternConfig
 *                     type: struct
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,mogenius-operator/src/core.Response·2]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,mogenius-operator/src/core.Response·2]
 *         properties:
 *             data:
 *                 structRef: mogenius-operator/src/core.Response
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/schema.Schema:
 *         name: mogenius-operator/src/schema.Schema
 *         properties:
 *             structs:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     structRef: mogenius-operator/src/schema.StructLayout
 *                     type: struct
 *             typeInfo:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/schema.TypeInfo
 *                 type: struct
 *     mogenius-operator/src/schema.StructLayout:
 *         name: mogenius-operator/src/schema.StructLayout
 *         properties:
 *             name:
 *                 type: string
 *             properties:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     structRef: mogenius-operator/src/schema.TypeInfo
 *                     type: struct
 *     mogenius-operator/src/schema.TypeInfo:
 *         name: mogenius-operator/src/schema.TypeInfo
 *         properties:
 *             elementType:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/schema.TypeInfo
 *                 type: struct
 *             keyType:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/schema.TypeInfo
 *                 type: struct
 *             pointer:
 *                 type: bool
 *             structRef:
 *                 type: string
 *             type:
 *                 type: string
 *             valueType:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/schema.TypeInfo
 *                 type: struct
 *     mogenius-operator/src/version.Version:
 *         name: mogenius-operator/src/version.Version
 *         properties:
 *             arch:
 *                 type: string
 *             branch:
 *                 type: string
 *             buildTimestamp:
 *                 type: string
 *             gitCommitHash:
 *                 type: string
 *             os:
 *                 type: string
 *             version:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,mogenius-operator/src/core.Response·2]
 *     type: struct
 * ```
 *
 */
export type DESCRIBE_RESPONSE = DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_CORE_RESPONSE2;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/utils.ResourceDescriptor:
 *         name: mogenius-operator/src/utils.ResourceDescriptor
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *             namespaced:
 *                 type: bool
 *             plural:
 *                 type: string
 *     mogenius-operator/src/utils.WorkloadSingleRequest:
 *         name: mogenius-operator/src/utils.WorkloadSingleRequest
 *         properties:
 *             ResourceDescriptor:
 *                 structRef: mogenius-operator/src/utils.ResourceDescriptor
 *                 type: struct
 *             namespace:
 *                 type: string
 *             resourceName:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/utils.WorkloadSingleRequest
 *     type: struct
 * ```
 *
 */
export type DESCRIBE_WORKLOAD_REQUEST = DESCRIBE_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadSingleRequest,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadSingleRequest,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadSingleRequest,string]
 *     type: struct
 * ```
 *
 */
export type DESCRIBE_WORKLOAD_RESPONSE = DESCRIBE_WORKLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             file:
 *                 structRef: mogenius-operator/src/dtos.PersistentFileRequestDto
 *                 type: struct
 *             mode:
 *                 type: string
 *     mogenius-operator/src/dtos.PersistentFileRequestDto:
 *         name: mogenius-operator/src/dtos.PersistentFileRequestDto
 *         properties:
 *             path:
 *                 type: string
 *             volumeName:
 *                 type: string
 *             volumeNamespace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type FILES_CHMOD_REQUEST = FILES_CHMOD_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·13,bool]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·13,bool]
 *         properties:
 *             data:
 *                 type: bool
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·13,bool]
 *     type: struct
 * ```
 *
 */
export type FILES_CHMOD_RESPONSE = FILES_CHMOD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST13_BOOL;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             file:
 *                 structRef: mogenius-operator/src/dtos.PersistentFileRequestDto
 *                 type: struct
 *             gid:
 *                 type: string
 *             uid:
 *                 type: string
 *     mogenius-operator/src/dtos.PersistentFileRequestDto:
 *         name: mogenius-operator/src/dtos.PersistentFileRequestDto
 *         properties:
 *             path:
 *                 type: string
 *             volumeName:
 *                 type: string
 *             volumeNamespace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type FILES_CHOWN_REQUEST = FILES_CHOWN_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·12,bool]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·12,bool]
 *         properties:
 *             data:
 *                 type: bool
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·12,bool]
 *     type: struct
 * ```
 *
 */
export type FILES_CHOWN_RESPONSE = FILES_CHOWN_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST12_BOOL;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             folder:
 *                 structRef: mogenius-operator/src/dtos.PersistentFileRequestDto
 *                 type: struct
 *     mogenius-operator/src/dtos.PersistentFileRequestDto:
 *         name: mogenius-operator/src/dtos.PersistentFileRequestDto
 *         properties:
 *             path:
 *                 type: string
 *             volumeName:
 *                 type: string
 *             volumeNamespace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type FILES_CREATE_FOLDER_REQUEST = FILES_CREATE_FOLDER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·10,bool]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·10,bool]
 *         properties:
 *             data:
 *                 type: bool
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·10,bool]
 *     type: struct
 * ```
 *
 */
export type FILES_CREATE_FOLDER_RESPONSE = FILES_CREATE_FOLDER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST10_BOOL;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             file:
 *                 structRef: mogenius-operator/src/dtos.PersistentFileRequestDto
 *                 type: struct
 *     mogenius-operator/src/dtos.PersistentFileRequestDto:
 *         name: mogenius-operator/src/dtos.PersistentFileRequestDto
 *         properties:
 *             path:
 *                 type: string
 *             volumeName:
 *                 type: string
 *             volumeNamespace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type FILES_DELETE_REQUEST = FILES_DELETE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·14,bool]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·14,bool]
 *         properties:
 *             data:
 *                 type: bool
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·14,bool]
 *     type: struct
 * ```
 *
 */
export type FILES_DELETE_RESPONSE = FILES_DELETE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST14_BOOL;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             file:
 *                 structRef: mogenius-operator/src/dtos.PersistentFileRequestDto
 *                 type: struct
 *             postTo:
 *                 type: string
 *     mogenius-operator/src/dtos.PersistentFileRequestDto:
 *         name: mogenius-operator/src/dtos.PersistentFileRequestDto
 *         properties:
 *             path:
 *                 type: string
 *             volumeName:
 *                 type: string
 *             volumeNamespace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type FILES_DOWNLOAD_REQUEST = FILES_DOWNLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·15,mogenius-operator/src/services.FilesDownloadResponse]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·15,mogenius-operator/src/services.FilesDownloadResponse]
 *         properties:
 *             data:
 *                 structRef: mogenius-operator/src/services.FilesDownloadResponse
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/services.FilesDownloadResponse:
 *         name: mogenius-operator/src/services.FilesDownloadResponse
 *         properties:
 *             error:
 *                 type: string
 *             sizeInBytes:
 *                 type: int
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·15,mogenius-operator/src/services.FilesDownloadResponse]
 *     type: struct
 * ```
 *
 */
export type FILES_DOWNLOAD_RESPONSE = FILES_DOWNLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST15_MOGENIUS_OPERATOR_SRC_SERVICES_FILESDOWNLOADRESPONSE;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/dtos.PersistentFileRequestDto:
 *         name: mogenius-operator/src/dtos.PersistentFileRequestDto
 *         properties:
 *             path:
 *                 type: string
 *             volumeName:
 *                 type: string
 *             volumeNamespace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/dtos.PersistentFileRequestDto
 *     type: struct
 * ```
 *
 */
export type FILES_INFO_REQUEST = FILES_INFO_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/dtos.PersistentFileRequestDto,mogenius-operator/src/dtos.PersistentFileDto]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/dtos.PersistentFileRequestDto,mogenius-operator/src/dtos.PersistentFileDto]
 *         properties:
 *             data:
 *                 structRef: mogenius-operator/src/dtos.PersistentFileDto
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/dtos.PersistentFileDto:
 *         name: mogenius-operator/src/dtos.PersistentFileDto
 *         properties:
 *             contentType:
 *                 type: string
 *             createdAt:
 *                 type: string
 *             extension:
 *                 type: string
 *             hash:
 *                 type: string
 *             mimeType:
 *                 type: string
 *             mode:
 *                 type: string
 *             modifiedAt:
 *                 type: string
 *             name:
 *                 type: string
 *             relativePath:
 *                 type: string
 *             size:
 *                 type: string
 *             sizeInBytes:
 *                 type: int
 *             type:
 *                 type: string
 *             uid_gid:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/dtos.PersistentFileRequestDto,mogenius-operator/src/dtos.PersistentFileDto]
 *     type: struct
 * ```
 *
 */
export type FILES_INFO_RESPONSE = FILES_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO_MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEDTO;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             folder:
 *                 structRef: mogenius-operator/src/dtos.PersistentFileRequestDto
 *                 type: struct
 *     mogenius-operator/src/dtos.PersistentFileRequestDto:
 *         name: mogenius-operator/src/dtos.PersistentFileRequestDto
 *         properties:
 *             path:
 *                 type: string
 *             volumeName:
 *                 type: string
 *             volumeNamespace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type FILES_LIST_REQUEST = FILES_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·9,[]mogenius-operator/src/dtos.PersistentFileDto]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·9,[]mogenius-operator/src/dtos.PersistentFileDto]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/dtos.PersistentFileDto
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/dtos.PersistentFileDto:
 *         name: mogenius-operator/src/dtos.PersistentFileDto
 *         properties:
 *             contentType:
 *                 type: string
 *             createdAt:
 *                 type: string
 *             extension:
 *                 type: string
 *             hash:
 *                 type: string
 *             mimeType:
 *                 type: string
 *             mode:
 *                 type: string
 *             modifiedAt:
 *                 type: string
 *             name:
 *                 type: string
 *             relativePath:
 *                 type: string
 *             size:
 *                 type: string
 *             sizeInBytes:
 *                 type: int
 *             type:
 *                 type: string
 *             uid_gid:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·9,[]mogenius-operator/src/dtos.PersistentFileDto]
 *     type: struct
 * ```
 *
 */
export type FILES_LIST_RESPONSE = FILES_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST9_MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEDTO;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             file:
 *                 structRef: mogenius-operator/src/dtos.PersistentFileRequestDto
 *                 type: struct
 *             newName:
 *                 type: string
 *     mogenius-operator/src/dtos.PersistentFileRequestDto:
 *         name: mogenius-operator/src/dtos.PersistentFileRequestDto
 *         properties:
 *             path:
 *                 type: string
 *             volumeName:
 *                 type: string
 *             volumeNamespace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type FILES_RENAME_REQUEST = FILES_RENAME_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·11,bool]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·11,bool]
 *         properties:
 *             data:
 *                 type: bool
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·11,bool]
 *     type: struct
 * ```
 *
 */
export type FILES_RENAME_RESPONSE = FILES_RENAME_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST11_BOOL;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             name:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type GET_GRANT_REQUEST = GET_GRANT_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1
 *         properties: {}
 *     k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry
 *         properties:
 *             apiVersion:
 *                 type: string
 *             fieldsType:
 *                 type: string
 *             fieldsV1:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1
 *                 type: struct
 *             manager:
 *                 type: string
 *             operation:
 *                 type: string
 *             subresource:
 *                 type: string
 *             time:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *     k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
 *         properties:
 *             annotations:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             creationTimestamp:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             deletionGracePeriodSeconds:
 *                 pointer: true
 *                 type: int
 *             deletionTimestamp:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             finalizers:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             generateName:
 *                 type: string
 *             generation:
 *                 type: int
 *             labels:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             managedFields:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry
 *                     type: struct
 *                 type: array
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             ownerReferences:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference
 *                     type: struct
 *                 type: array
 *             resourceVersion:
 *                 type: string
 *             selfLink:
 *                 type: string
 *             uid:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference
 *         properties:
 *             apiVersion:
 *                 type: string
 *             blockOwnerDeletion:
 *                 pointer: true
 *                 type: bool
 *             controller:
 *                 pointer: true
 *                 type: bool
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             uid:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.Time:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *         properties:
 *             Time:
 *                 structRef: time.Time
 *                 type: struct
 *     k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·30,*mogenius-operator/src/crds/v1alpha1.Grant]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·30,*mogenius-operator/src/crds/v1alpha1.Grant]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/crds/v1alpha1.Grant
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/crds/v1alpha1.Grant:
 *         name: mogenius-operator/src/crds/v1alpha1.Grant
 *         properties:
 *             TypeMeta:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta
 *                 type: struct
 *             metadata:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
 *                 type: struct
 *             spec:
 *                 structRef: mogenius-operator/src/crds/v1alpha1.GrantSpec
 *                 type: struct
 *             status:
 *                 structRef: mogenius-operator/src/crds/v1alpha1.GrantStatus
 *                 type: struct
 *     mogenius-operator/src/crds/v1alpha1.GrantSpec:
 *         name: mogenius-operator/src/crds/v1alpha1.GrantSpec
 *         properties:
 *             grantee:
 *                 type: string
 *             role:
 *                 type: string
 *             targetName:
 *                 type: string
 *             targetType:
 *                 type: string
 *     mogenius-operator/src/crds/v1alpha1.GrantStatus:
 *         name: mogenius-operator/src/crds/v1alpha1.GrantStatus
 *         properties: {}
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·30,*mogenius-operator/src/crds/v1alpha1.Grant]
 *     type: struct
 * ```
 *
 */
export type GET_GRANT_RESPONSE = GET_GRANT_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST30_MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANT;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             targetName:
 *                 pointer: true
 *                 type: string
 *             targetType:
 *                 pointer: true
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type GET_GRANTS_REQUEST = GET_GRANTS_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1
 *         properties: {}
 *     k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry
 *         properties:
 *             apiVersion:
 *                 type: string
 *             fieldsType:
 *                 type: string
 *             fieldsV1:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1
 *                 type: struct
 *             manager:
 *                 type: string
 *             operation:
 *                 type: string
 *             subresource:
 *                 type: string
 *             time:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *     k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
 *         properties:
 *             annotations:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             creationTimestamp:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             deletionGracePeriodSeconds:
 *                 pointer: true
 *                 type: int
 *             deletionTimestamp:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             finalizers:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             generateName:
 *                 type: string
 *             generation:
 *                 type: int
 *             labels:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             managedFields:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry
 *                     type: struct
 *                 type: array
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             ownerReferences:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference
 *                     type: struct
 *                 type: array
 *             resourceVersion:
 *                 type: string
 *             selfLink:
 *                 type: string
 *             uid:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference
 *         properties:
 *             apiVersion:
 *                 type: string
 *             blockOwnerDeletion:
 *                 pointer: true
 *                 type: bool
 *             controller:
 *                 pointer: true
 *                 type: bool
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             uid:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.Time:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *         properties:
 *             Time:
 *                 structRef: time.Time
 *                 type: struct
 *     k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·28,[]mogenius-operator/src/crds/v1alpha1.Grant]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·28,[]mogenius-operator/src/crds/v1alpha1.Grant]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/crds/v1alpha1.Grant
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/crds/v1alpha1.Grant:
 *         name: mogenius-operator/src/crds/v1alpha1.Grant
 *         properties:
 *             TypeMeta:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta
 *                 type: struct
 *             metadata:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
 *                 type: struct
 *             spec:
 *                 structRef: mogenius-operator/src/crds/v1alpha1.GrantSpec
 *                 type: struct
 *             status:
 *                 structRef: mogenius-operator/src/crds/v1alpha1.GrantStatus
 *                 type: struct
 *     mogenius-operator/src/crds/v1alpha1.GrantSpec:
 *         name: mogenius-operator/src/crds/v1alpha1.GrantSpec
 *         properties:
 *             grantee:
 *                 type: string
 *             role:
 *                 type: string
 *             targetName:
 *                 type: string
 *             targetType:
 *                 type: string
 *     mogenius-operator/src/crds/v1alpha1.GrantStatus:
 *         name: mogenius-operator/src/crds/v1alpha1.GrantStatus
 *         properties: {}
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·28,[]mogenius-operator/src/crds/v1alpha1.Grant]
 *     type: struct
 * ```
 *
 */
export type GET_GRANTS_RESPONSE = GET_GRANTS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST28_MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANT;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/kubernetes.GetUnstructuredLabeledResourceListRequest:
 *         name: mogenius-operator/src/kubernetes.GetUnstructuredLabeledResourceListRequest
 *         properties:
 *             blacklist:
 *                 elementType:
 *                     pointer: true
 *                     structRef: mogenius-operator/src/utils.ResourceDescriptor
 *                     type: struct
 *                 type: array
 *             label:
 *                 type: string
 *             whitelist:
 *                 elementType:
 *                     pointer: true
 *                     structRef: mogenius-operator/src/utils.ResourceDescriptor
 *                     type: struct
 *                 type: array
 *     mogenius-operator/src/utils.ResourceDescriptor:
 *         name: mogenius-operator/src/utils.ResourceDescriptor
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *             namespaced:
 *                 type: bool
 *             plural:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/kubernetes.GetUnstructuredLabeledResourceListRequest
 *     type: struct
 * ```
 *
 */
export type GET_LABELED_WORKLOAD_LIST_REQUEST = GET_LABELED_WORKLOAD_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_KUBERNETES_GETUNSTRUCTUREDLABELEDRESOURCELISTREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *         properties:
 *             Object:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *     k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.UnstructuredList:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.UnstructuredList
 *         properties:
 *             Object:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *             items:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *                     type: struct
 *                 type: array
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/kubernetes.GetUnstructuredLabeledResourceListRequest,k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.UnstructuredList]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/kubernetes.GetUnstructuredLabeledResourceListRequest,k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.UnstructuredList]
 *         properties:
 *             data:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.UnstructuredList
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/kubernetes.GetUnstructuredLabeledResourceListRequest,k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.UnstructuredList]
 *     type: struct
 * ```
 *
 */
export type GET_LABELED_WORKLOAD_LIST_RESPONSE = GET_LABELED_WORKLOAD_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_KUBERNETES_GETUNSTRUCTUREDLABELEDRESOURCELISTREQUEST_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTUREDLIST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/kubernetes.GetUnstructuredNamespaceResourceListRequest:
 *         name: mogenius-operator/src/kubernetes.GetUnstructuredNamespaceResourceListRequest
 *         properties:
 *             blacklist:
 *                 elementType:
 *                     pointer: true
 *                     structRef: mogenius-operator/src/utils.ResourceDescriptor
 *                     type: struct
 *                 type: array
 *             namespace:
 *                 type: string
 *             whitelist:
 *                 elementType:
 *                     pointer: true
 *                     structRef: mogenius-operator/src/utils.ResourceDescriptor
 *                     type: struct
 *                 type: array
 *     mogenius-operator/src/utils.ResourceDescriptor:
 *         name: mogenius-operator/src/utils.ResourceDescriptor
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *             namespaced:
 *                 type: bool
 *             plural:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/kubernetes.GetUnstructuredNamespaceResourceListRequest
 *     type: struct
 * ```
 *
 */
export type GET_NAMESPACE_WORKLOAD_LIST_REQUEST = GET_NAMESPACE_WORKLOAD_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_KUBERNETES_GETUNSTRUCTUREDNAMESPACERESOURCELISTREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *         properties:
 *             Object:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/kubernetes.GetUnstructuredNamespaceResourceListRequest,[]k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/kubernetes.GetUnstructuredNamespaceResourceListRequest,[]k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/kubernetes.GetUnstructuredNamespaceResourceListRequest,[]k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     type: struct
 * ```
 *
 */
export type GET_NAMESPACE_WORKLOAD_LIST_RESPONSE = GET_NAMESPACE_WORKLOAD_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_KUBERNETES_GETUNSTRUCTUREDNAMESPACERESOURCELISTREQUEST_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type GET_NODES_METRICS_REQUEST = GET_NODES_METRICS_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.NodeMetrics:
 *         name: mogenius-operator/src/core.NodeMetrics
 *         properties:
 *             cpu:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *             memory:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *             nodeName:
 *                 type: string
 *             traffic:
 *                 elementType:
 *                     structRef: mogenius-operator/src/networkmonitor.PodNetworkStats
 *                     type: struct
 *                 type: array
 *     mogenius-operator/src/core.Response:
 *         name: mogenius-operator/src/core.Response
 *         properties:
 *             nodes:
 *                 elementType:
 *                     structRef: mogenius-operator/src/core.NodeMetrics
 *                     type: struct
 *                 type: array
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,mogenius-operator/src/core.Response·38]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,mogenius-operator/src/core.Response·38]
 *         properties:
 *             data:
 *                 structRef: mogenius-operator/src/core.Response
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/networkmonitor.PodNetworkStats:
 *         name: mogenius-operator/src/networkmonitor.PodNetworkStats
 *         properties:
 *             createdAt:
 *                 structRef: time.Time
 *                 type: struct
 *             namespace:
 *                 type: string
 *             pod:
 *                 type: string
 *             receivedBytes:
 *                 type: uint
 *             receivedPackets:
 *                 type: uint
 *             receivedStartBytes:
 *                 type: uint
 *             transmitBytes:
 *                 type: uint
 *             transmitPackets:
 *                 type: uint
 *             transmitStartBytes:
 *                 type: uint
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,mogenius-operator/src/core.Response·38]
 *     type: struct
 * ```
 *
 */
export type GET_NODES_METRICS_RESPONSE = GET_NODES_METRICS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_CORE_RESPONSE38;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             name:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type GET_USER_REQUEST = GET_USER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/api/rbac/v1.Subject:
 *         name: k8s.io/api/rbac/v1.Subject
 *         properties:
 *             apiGroup:
 *                 type: string
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1
 *         properties: {}
 *     k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry
 *         properties:
 *             apiVersion:
 *                 type: string
 *             fieldsType:
 *                 type: string
 *             fieldsV1:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1
 *                 type: struct
 *             manager:
 *                 type: string
 *             operation:
 *                 type: string
 *             subresource:
 *                 type: string
 *             time:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *     k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
 *         properties:
 *             annotations:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             creationTimestamp:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             deletionGracePeriodSeconds:
 *                 pointer: true
 *                 type: int
 *             deletionTimestamp:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             finalizers:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             generateName:
 *                 type: string
 *             generation:
 *                 type: int
 *             labels:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             managedFields:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry
 *                     type: struct
 *                 type: array
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             ownerReferences:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference
 *                     type: struct
 *                 type: array
 *             resourceVersion:
 *                 type: string
 *             selfLink:
 *                 type: string
 *             uid:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference
 *         properties:
 *             apiVersion:
 *                 type: string
 *             blockOwnerDeletion:
 *                 pointer: true
 *                 type: bool
 *             controller:
 *                 pointer: true
 *                 type: bool
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             uid:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.Time:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *         properties:
 *             Time:
 *                 structRef: time.Time
 *                 type: struct
 *     k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·25,*mogenius-operator/src/crds/v1alpha1.User]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·25,*mogenius-operator/src/crds/v1alpha1.User]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/crds/v1alpha1.User
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/crds/v1alpha1.User:
 *         name: mogenius-operator/src/crds/v1alpha1.User
 *         properties:
 *             TypeMeta:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta
 *                 type: struct
 *             metadata:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
 *                 type: struct
 *             spec:
 *                 structRef: mogenius-operator/src/crds/v1alpha1.UserSpec
 *                 type: struct
 *             status:
 *                 structRef: mogenius-operator/src/crds/v1alpha1.UserStatus
 *                 type: struct
 *     mogenius-operator/src/crds/v1alpha1.UserSpec:
 *         name: mogenius-operator/src/crds/v1alpha1.UserSpec
 *         properties:
 *             email:
 *                 type: string
 *             firstName:
 *                 type: string
 *             lastName:
 *                 type: string
 *             subject:
 *                 pointer: true
 *                 structRef: k8s.io/api/rbac/v1.Subject
 *                 type: struct
 *     mogenius-operator/src/crds/v1alpha1.UserStatus:
 *         name: mogenius-operator/src/crds/v1alpha1.UserStatus
 *         properties: {}
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·25,*mogenius-operator/src/crds/v1alpha1.User]
 *     type: struct
 * ```
 *
 */
export type GET_USER_RESPONSE = GET_USER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST25_MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USER;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             email:
 *                 pointer: true
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type GET_USERS_REQUEST = GET_USERS_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/api/rbac/v1.Subject:
 *         name: k8s.io/api/rbac/v1.Subject
 *         properties:
 *             apiGroup:
 *                 type: string
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1
 *         properties: {}
 *     k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry
 *         properties:
 *             apiVersion:
 *                 type: string
 *             fieldsType:
 *                 type: string
 *             fieldsV1:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1
 *                 type: struct
 *             manager:
 *                 type: string
 *             operation:
 *                 type: string
 *             subresource:
 *                 type: string
 *             time:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *     k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
 *         properties:
 *             annotations:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             creationTimestamp:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             deletionGracePeriodSeconds:
 *                 pointer: true
 *                 type: int
 *             deletionTimestamp:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             finalizers:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             generateName:
 *                 type: string
 *             generation:
 *                 type: int
 *             labels:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             managedFields:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry
 *                     type: struct
 *                 type: array
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             ownerReferences:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference
 *                     type: struct
 *                 type: array
 *             resourceVersion:
 *                 type: string
 *             selfLink:
 *                 type: string
 *             uid:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference
 *         properties:
 *             apiVersion:
 *                 type: string
 *             blockOwnerDeletion:
 *                 pointer: true
 *                 type: bool
 *             controller:
 *                 pointer: true
 *                 type: bool
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             uid:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.Time:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *         properties:
 *             Time:
 *                 structRef: time.Time
 *                 type: struct
 *     k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·23,[]mogenius-operator/src/crds/v1alpha1.User]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·23,[]mogenius-operator/src/crds/v1alpha1.User]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/crds/v1alpha1.User
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/crds/v1alpha1.User:
 *         name: mogenius-operator/src/crds/v1alpha1.User
 *         properties:
 *             TypeMeta:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta
 *                 type: struct
 *             metadata:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
 *                 type: struct
 *             spec:
 *                 structRef: mogenius-operator/src/crds/v1alpha1.UserSpec
 *                 type: struct
 *             status:
 *                 structRef: mogenius-operator/src/crds/v1alpha1.UserStatus
 *                 type: struct
 *     mogenius-operator/src/crds/v1alpha1.UserSpec:
 *         name: mogenius-operator/src/crds/v1alpha1.UserSpec
 *         properties:
 *             email:
 *                 type: string
 *             firstName:
 *                 type: string
 *             lastName:
 *                 type: string
 *             subject:
 *                 pointer: true
 *                 structRef: k8s.io/api/rbac/v1.Subject
 *                 type: struct
 *     mogenius-operator/src/crds/v1alpha1.UserStatus:
 *         name: mogenius-operator/src/crds/v1alpha1.UserStatus
 *         properties: {}
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·23,[]mogenius-operator/src/crds/v1alpha1.User]
 *     type: struct
 * ```
 *
 */
export type GET_USERS_RESPONSE = GET_USERS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST23_MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USER;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/utils.ResourceDescriptor:
 *         name: mogenius-operator/src/utils.ResourceDescriptor
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *             namespaced:
 *                 type: bool
 *             plural:
 *                 type: string
 *     mogenius-operator/src/utils.WorkloadSingleRequest:
 *         name: mogenius-operator/src/utils.WorkloadSingleRequest
 *         properties:
 *             ResourceDescriptor:
 *                 structRef: mogenius-operator/src/utils.ResourceDescriptor
 *                 type: struct
 *             namespace:
 *                 type: string
 *             resourceName:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/utils.WorkloadSingleRequest
 *     type: struct
 * ```
 *
 */
export type GET_WORKLOAD_REQUEST = GET_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *         properties:
 *             Object:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadSingleRequest,*k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadSingleRequest,*k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadSingleRequest,*k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     type: struct
 * ```
 *
 */
export type GET_WORKLOAD_RESPONSE = GET_WORKLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/utils.ResourceDescriptor:
 *         name: mogenius-operator/src/utils.ResourceDescriptor
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *             namespaced:
 *                 type: bool
 *             plural:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/utils.ResourceDescriptor
 *     type: struct
 * ```
 *
 */
export type GET_WORKLOAD_EXAMPLE_REQUEST = GET_WORKLOAD_EXAMPLE_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/utils.ResourceDescriptor,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/utils.ResourceDescriptor,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/utils.ResourceDescriptor,string]
 *     type: struct
 * ```
 *
 */
export type GET_WORKLOAD_EXAMPLE_RESPONSE = GET_WORKLOAD_EXAMPLE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *             namespace:
 *                 pointer: true
 *                 type: string
 *             plural:
 *                 type: string
 *             withData:
 *                 pointer: true
 *                 type: bool
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type GET_WORKLOAD_LIST_REQUEST = GET_WORKLOAD_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *         properties:
 *             Object:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *     k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.UnstructuredList:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.UnstructuredList
 *         properties:
 *             Object:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *             items:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *                     type: struct
 *                 type: array
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·17,k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.UnstructuredList]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·17,k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.UnstructuredList]
 *         properties:
 *             data:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.UnstructuredList
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·17,k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.UnstructuredList]
 *     type: struct
 * ```
 *
 */
export type GET_WORKLOAD_LIST_RESPONSE = GET_WORKLOAD_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST17_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTUREDLIST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/kubernetes.GetWorkloadStatusHelmReleaseNameRequest:
 *         name: mogenius-operator/src/kubernetes.GetWorkloadStatusHelmReleaseNameRequest
 *         properties:
 *             namespace:
 *                 type: string
 *             release:
 *                 type: string
 *     mogenius-operator/src/kubernetes.GetWorkloadStatusRequest:
 *         name: mogenius-operator/src/kubernetes.GetWorkloadStatusRequest
 *         properties:
 *             helmReleases:
 *                 elementType:
 *                     structRef: mogenius-operator/src/kubernetes.GetWorkloadStatusHelmReleaseNameRequest
 *                     type: struct
 *                 pointer: true
 *                 type: array
 *             ignoreDependentResources:
 *                 pointer: true
 *                 type: bool
 *             namespaces:
 *                 elementType:
 *                     type: string
 *                 pointer: true
 *                 type: array
 *             resourceDescriptor:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/utils.ResourceDescriptor
 *                 type: struct
 *             resourceNames:
 *                 elementType:
 *                     type: string
 *                 pointer: true
 *                 type: array
 *     mogenius-operator/src/utils.ResourceDescriptor:
 *         name: mogenius-operator/src/utils.ResourceDescriptor
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *             namespaced:
 *                 type: bool
 *             plural:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/kubernetes.GetWorkloadStatusRequest
 *     type: struct
 * ```
 *
 */
export type GET_WORKLOAD_STATUS_REQUEST = GET_WORKLOAD_STATUS_REQUEST__MOGENIUS_OPERATOR_SRC_KUBERNETES_GETWORKLOADSTATUSREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/api/core/v1.Event:
 *         name: k8s.io/api/core/v1.Event
 *         properties:
 *             TypeMeta:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta
 *                 type: struct
 *             action:
 *                 type: string
 *             count:
 *                 type: int
 *             eventTime:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.MicroTime
 *                 type: struct
 *             firstTimestamp:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             involvedObject:
 *                 structRef: k8s.io/api/core/v1.ObjectReference
 *                 type: struct
 *             lastTimestamp:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             message:
 *                 type: string
 *             metadata:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
 *                 type: struct
 *             reason:
 *                 type: string
 *             related:
 *                 pointer: true
 *                 structRef: k8s.io/api/core/v1.ObjectReference
 *                 type: struct
 *             reportingComponent:
 *                 type: string
 *             reportingInstance:
 *                 type: string
 *             series:
 *                 pointer: true
 *                 structRef: k8s.io/api/core/v1.EventSeries
 *                 type: struct
 *             source:
 *                 structRef: k8s.io/api/core/v1.EventSource
 *                 type: struct
 *             type:
 *                 type: string
 *     k8s.io/api/core/v1.EventSeries:
 *         name: k8s.io/api/core/v1.EventSeries
 *         properties:
 *             count:
 *                 type: int
 *             lastObservedTime:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.MicroTime
 *                 type: struct
 *     k8s.io/api/core/v1.EventSource:
 *         name: k8s.io/api/core/v1.EventSource
 *         properties:
 *             component:
 *                 type: string
 *             host:
 *                 type: string
 *     k8s.io/api/core/v1.ObjectReference:
 *         name: k8s.io/api/core/v1.ObjectReference
 *         properties:
 *             apiVersion:
 *                 type: string
 *             fieldPath:
 *                 type: string
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             resourceVersion:
 *                 type: string
 *             uid:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1
 *         properties: {}
 *     k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry
 *         properties:
 *             apiVersion:
 *                 type: string
 *             fieldsType:
 *                 type: string
 *             fieldsV1:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1
 *                 type: struct
 *             manager:
 *                 type: string
 *             operation:
 *                 type: string
 *             subresource:
 *                 type: string
 *             time:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *     k8s.io/apimachinery/pkg/apis/meta/v1.MicroTime:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.MicroTime
 *         properties:
 *             Time:
 *                 structRef: time.Time
 *                 type: struct
 *     k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
 *         properties:
 *             annotations:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             creationTimestamp:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             deletionGracePeriodSeconds:
 *                 pointer: true
 *                 type: int
 *             deletionTimestamp:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             finalizers:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             generateName:
 *                 type: string
 *             generation:
 *                 type: int
 *             labels:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             managedFields:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry
 *                     type: struct
 *                 type: array
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             ownerReferences:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference
 *                     type: struct
 *                 type: array
 *             resourceVersion:
 *                 type: string
 *             selfLink:
 *                 type: string
 *             uid:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference
 *         properties:
 *             apiVersion:
 *                 type: string
 *             blockOwnerDeletion:
 *                 pointer: true
 *                 type: bool
 *             controller:
 *                 pointer: true
 *                 type: bool
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             uid:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.Time:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *         properties:
 *             Time:
 *                 structRef: time.Time
 *                 type: struct
 *     k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/kubernetes.GetWorkloadStatusRequest,[]mogenius-operator/src/kubernetes.WorkloadStatusDto]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/kubernetes.GetWorkloadStatusRequest,[]mogenius-operator/src/kubernetes.WorkloadStatusDto]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/kubernetes.WorkloadStatusDto
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/kubernetes.WorkloadStatusDto:
 *         name: mogenius-operator/src/kubernetes.WorkloadStatusDto
 *         properties:
 *             items:
 *                 elementType:
 *                     structRef: mogenius-operator/src/kubernetes.WorkloadStatusItemDto
 *                     type: struct
 *                 type: array
 *     mogenius-operator/src/kubernetes.WorkloadStatusItemDto:
 *         name: mogenius-operator/src/kubernetes.WorkloadStatusItemDto
 *         properties:
 *             apiVersion:
 *                 type: string
 *             creationTimestamp:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             endpoints:
 *                 pointer: true
 *                 type: any
 *             events:
 *                 elementType:
 *                     structRef: k8s.io/api/core/v1.Event
 *                     type: struct
 *                 type: array
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             ownerReferences:
 *                 pointer: true
 *                 type: any
 *             replicas:
 *                 pointer: true
 *                 type: int
 *             specClusterIP:
 *                 type: string
 *             specType:
 *                 type: string
 *             status:
 *                 pointer: true
 *                 type: any
 *             uid:
 *                 type: string
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/kubernetes.GetWorkloadStatusRequest,[]mogenius-operator/src/kubernetes.WorkloadStatusDto]
 *     type: struct
 * ```
 *
 */
export type GET_WORKLOAD_STATUS_RESPONSE = GET_WORKLOAD_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_KUBERNETES_GETWORKLOADSTATUSREQUEST_MOGENIUS_OPERATOR_SRC_KUBERNETES_WORKLOADSTATUSDTO;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type GET_WORKSPACE_REQUEST = GET_WORKSPACE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/apimachinery/pkg/apis/meta/v1.Time:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *         properties:
 *             Time:
 *                 structRef: time.Time
 *                 type: struct
 *     mogenius-operator/src/core.GetWorkspaceResult:
 *         name: mogenius-operator/src/core.GetWorkspaceResult
 *         properties:
 *             creationTimestamp:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             name:
 *                 type: string
 *             resources:
 *                 elementType:
 *                     structRef: mogenius-operator/src/crds/v1alpha1.WorkspaceResourceIdentifier
 *                     type: struct
 *                 type: array
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·19,*mogenius-operator/src/core.GetWorkspaceResult]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·19,*mogenius-operator/src/core.GetWorkspaceResult]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/core.GetWorkspaceResult
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/crds/v1alpha1.WorkspaceResourceIdentifier:
 *         name: mogenius-operator/src/crds/v1alpha1.WorkspaceResourceIdentifier
 *         properties:
 *             id:
 *                 type: string
 *             namespace:
 *                 type: string
 *             type:
 *                 type: string
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·19,*mogenius-operator/src/core.GetWorkspaceResult]
 *     type: struct
 * ```
 *
 */
export type GET_WORKSPACE_RESPONSE = GET_WORKSPACE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST19_MOGENIUS_OPERATOR_SRC_CORE_GETWORKSPACERESULT;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type GET_WORKSPACES_REQUEST = GET_WORKSPACES_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/apimachinery/pkg/apis/meta/v1.Time:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *         properties:
 *             Time:
 *                 structRef: time.Time
 *                 type: struct
 *     mogenius-operator/src/core.GetWorkspaceResult:
 *         name: mogenius-operator/src/core.GetWorkspaceResult
 *         properties:
 *             creationTimestamp:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             name:
 *                 type: string
 *             resources:
 *                 elementType:
 *                     structRef: mogenius-operator/src/crds/v1alpha1.WorkspaceResourceIdentifier
 *                     type: struct
 *                 type: array
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,[]mogenius-operator/src/core.GetWorkspaceResult]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,[]mogenius-operator/src/core.GetWorkspaceResult]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/core.GetWorkspaceResult
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/crds/v1alpha1.WorkspaceResourceIdentifier:
 *         name: mogenius-operator/src/crds/v1alpha1.WorkspaceResourceIdentifier
 *         properties:
 *             id:
 *                 type: string
 *             namespace:
 *                 type: string
 *             type:
 *                 type: string
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,[]mogenius-operator/src/core.GetWorkspaceResult]
 *     type: struct
 * ```
 *
 */
export type GET_WORKSPACES_RESPONSE = GET_WORKSPACES_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_CORE_GETWORKSPACERESULT;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             blacklist:
 *                 elementType:
 *                     pointer: true
 *                     structRef: mogenius-operator/src/utils.ResourceDescriptor
 *                     type: struct
 *                 type: array
 *             namespaceWhitelist:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             whitelist:
 *                 elementType:
 *                     pointer: true
 *                     structRef: mogenius-operator/src/utils.ResourceDescriptor
 *                     type: struct
 *                 type: array
 *             workspaceName:
 *                 type: string
 *     mogenius-operator/src/utils.ResourceDescriptor:
 *         name: mogenius-operator/src/utils.ResourceDescriptor
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *             namespaced:
 *                 type: bool
 *             plural:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type GET_WORKSPACE_WORKLOADS_REQUEST = GET_WORKSPACE_WORKLOADS_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *         properties:
 *             Object:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·33,[]k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·33,[]k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·33,[]k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     type: struct
 * ```
 *
 */
export type GET_WORKSPACE_WORKLOADS_RESPONSE = GET_WORKSPACE_WORKLOADS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST33_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type INSTALL_CERT_MANAGER_REQUEST = INSTALL_CERT_MANAGER_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type INSTALL_CERT_MANAGER_RESPONSE = INSTALL_CERT_MANAGER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             email:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type INSTALL_CLUSTER_ISSUER_REQUEST = INSTALL_CLUSTER_ISSUER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·6,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·6,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·6,string]
 *     type: struct
 * ```
 *
 */
export type INSTALL_CLUSTER_ISSUER_RESPONSE = INSTALL_CLUSTER_ISSUER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST6_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type INSTALL_INGRESS_CONTROLLER_TRAEFIK_REQUEST = INSTALL_INGRESS_CONTROLLER_TRAEFIK_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type INSTALL_INGRESS_CONTROLLER_TRAEFIK_RESPONSE = INSTALL_INGRESS_CONTROLLER_TRAEFIK_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type INSTALL_KEPLER_REQUEST = INSTALL_KEPLER_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type INSTALL_KEPLER_RESPONSE = INSTALL_KEPLER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type INSTALL_METALLB_REQUEST = INSTALL_METALLB_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type INSTALL_METALLB_RESPONSE = INSTALL_METALLB_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type INSTALL_METRICS_SERVER_REQUEST = INSTALL_METRICS_SERVER_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type INSTALL_METRICS_SERVER_RESPONSE = INSTALL_METRICS_SERVER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type LIST_ALL_RESOURCE_DESCRIPTORS_REQUEST = LIST_ALL_RESOURCE_DESCRIPTORS_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,[]mogenius-operator/src/utils.ResourceDescriptor]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,[]mogenius-operator/src/utils.ResourceDescriptor]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/utils.ResourceDescriptor
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/utils.ResourceDescriptor:
 *         name: mogenius-operator/src/utils.ResourceDescriptor
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *             namespaced:
 *                 type: bool
 *             plural:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,[]mogenius-operator/src/utils.ResourceDescriptor]
 *     type: struct
 * ```
 *
 */
export type LIST_ALL_RESOURCE_DESCRIPTORS_RESPONSE = LIST_ALL_RESOURCE_DESCRIPTORS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/xterm.WsConnectionRequest:
 *         name: mogenius-operator/src/xterm.WsConnectionRequest
 *         properties:
 *             channelId:
 *                 type: string
 *             cmdType:
 *                 type: string
 *             nodeName:
 *                 type: string
 *             podName:
 *                 type: string
 *             websocketHost:
 *                 type: string
 *             websocketScheme:
 *                 type: string
 *             workspace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/xterm.WsConnectionRequest
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_NODES_CPU_REQUEST = LIVE_STREAM_NODES_CPU_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_1:
 *         properties: {}
 *     mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: ANON_STRUCT_1
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_NODES_CPU_RESPONSE = LIVE_STREAM_NODES_CPU_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/xterm.WsConnectionRequest:
 *         name: mogenius-operator/src/xterm.WsConnectionRequest
 *         properties:
 *             channelId:
 *                 type: string
 *             cmdType:
 *                 type: string
 *             nodeName:
 *                 type: string
 *             podName:
 *                 type: string
 *             websocketHost:
 *                 type: string
 *             websocketScheme:
 *                 type: string
 *             workspace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/xterm.WsConnectionRequest
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_NODES_MEMORY_REQUEST = LIVE_STREAM_NODES_MEMORY_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_1:
 *         properties: {}
 *     mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: ANON_STRUCT_1
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_NODES_MEMORY_RESPONSE = LIVE_STREAM_NODES_MEMORY_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/xterm.WsConnectionRequest:
 *         name: mogenius-operator/src/xterm.WsConnectionRequest
 *         properties:
 *             channelId:
 *                 type: string
 *             cmdType:
 *                 type: string
 *             nodeName:
 *                 type: string
 *             podName:
 *                 type: string
 *             websocketHost:
 *                 type: string
 *             websocketScheme:
 *                 type: string
 *             workspace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/xterm.WsConnectionRequest
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_NODES_TRAFFIC_REQUEST = LIVE_STREAM_NODES_TRAFFIC_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_1:
 *         properties: {}
 *     mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: ANON_STRUCT_1
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_NODES_TRAFFIC_RESPONSE = LIVE_STREAM_NODES_TRAFFIC_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/xterm.WsConnectionRequest:
 *         name: mogenius-operator/src/xterm.WsConnectionRequest
 *         properties:
 *             channelId:
 *                 type: string
 *             cmdType:
 *                 type: string
 *             nodeName:
 *                 type: string
 *             podName:
 *                 type: string
 *             websocketHost:
 *                 type: string
 *             websocketScheme:
 *                 type: string
 *             workspace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/xterm.WsConnectionRequest
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_POD_CPU_REQUEST = LIVE_STREAM_POD_CPU_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_1:
 *         properties: {}
 *     mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: ANON_STRUCT_1
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_POD_CPU_RESPONSE = LIVE_STREAM_POD_CPU_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/xterm.WsConnectionRequest:
 *         name: mogenius-operator/src/xterm.WsConnectionRequest
 *         properties:
 *             channelId:
 *                 type: string
 *             cmdType:
 *                 type: string
 *             nodeName:
 *                 type: string
 *             podName:
 *                 type: string
 *             websocketHost:
 *                 type: string
 *             websocketScheme:
 *                 type: string
 *             workspace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/xterm.WsConnectionRequest
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_POD_MEMORY_REQUEST = LIVE_STREAM_POD_MEMORY_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_1:
 *         properties: {}
 *     mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: ANON_STRUCT_1
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_POD_MEMORY_RESPONSE = LIVE_STREAM_POD_MEMORY_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/xterm.WsConnectionRequest:
 *         name: mogenius-operator/src/xterm.WsConnectionRequest
 *         properties:
 *             channelId:
 *                 type: string
 *             cmdType:
 *                 type: string
 *             nodeName:
 *                 type: string
 *             podName:
 *                 type: string
 *             websocketHost:
 *                 type: string
 *             websocketScheme:
 *                 type: string
 *             workspace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/xterm.WsConnectionRequest
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_POD_TRAFFIC_REQUEST = LIVE_STREAM_POD_TRAFFIC_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_1:
 *         properties: {}
 *     mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: ANON_STRUCT_1
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,mogenius-operator/src/core.Void]
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_POD_TRAFFIC_RESPONSE = LIVE_STREAM_POD_TRAFFIC_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/xterm.WsConnectionRequest:
 *         name: mogenius-operator/src/xterm.WsConnectionRequest
 *         properties:
 *             channelId:
 *                 type: string
 *             cmdType:
 *                 type: string
 *             nodeName:
 *                 type: string
 *             podName:
 *                 type: string
 *             websocketHost:
 *                 type: string
 *             websocketScheme:
 *                 type: string
 *             workspace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/xterm.WsConnectionRequest
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_WORKSPACE_CPU_REQUEST = LIVE_STREAM_WORKSPACE_CPU_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,interface {}]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,interface {}]
 *         properties:
 *             data:
 *                 pointer: true
 *                 type: any
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,interface {}]
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_WORKSPACE_CPU_RESPONSE = LIVE_STREAM_WORKSPACE_CPU_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_INTERFACE {};

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/xterm.WsConnectionRequest:
 *         name: mogenius-operator/src/xterm.WsConnectionRequest
 *         properties:
 *             channelId:
 *                 type: string
 *             cmdType:
 *                 type: string
 *             nodeName:
 *                 type: string
 *             podName:
 *                 type: string
 *             websocketHost:
 *                 type: string
 *             websocketScheme:
 *                 type: string
 *             workspace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/xterm.WsConnectionRequest
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_WORKSPACE_MEMORY_REQUEST = LIVE_STREAM_WORKSPACE_MEMORY_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,interface {}]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,interface {}]
 *         properties:
 *             data:
 *                 pointer: true
 *                 type: any
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,interface {}]
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_WORKSPACE_MEMORY_RESPONSE = LIVE_STREAM_WORKSPACE_MEMORY_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_INTERFACE {};

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/xterm.WsConnectionRequest:
 *         name: mogenius-operator/src/xterm.WsConnectionRequest
 *         properties:
 *             channelId:
 *                 type: string
 *             cmdType:
 *                 type: string
 *             nodeName:
 *                 type: string
 *             podName:
 *                 type: string
 *             websocketHost:
 *                 type: string
 *             websocketScheme:
 *                 type: string
 *             workspace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/xterm.WsConnectionRequest
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_WORKSPACE_TRAFFIC_REQUEST = LIVE_STREAM_WORKSPACE_TRAFFIC_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,interface {}]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,interface {}]
 *         properties:
 *             data:
 *                 pointer: true
 *                 type: any
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.WsConnectionRequest,interface {}]
 *     type: struct
 * ```
 *
 */
export type LIVE_STREAM_WORKSPACE_TRAFFIC_RESPONSE = LIVE_STREAM_WORKSPACE_TRAFFIC_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_INTERFACE {};

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.PrometheusRequestRedis:
 *         name: mogenius-operator/src/core.PrometheusRequestRedis
 *         properties:
 *             controller:
 *                 type: string
 *             namespace:
 *                 type: string
 *             query:
 *                 type: string
 *             queryName:
 *                 type: string
 *             step:
 *                 type: int
 * typeInfo:
 *     structRef: mogenius-operator/src/core.PrometheusRequestRedis
 *     type: struct
 * ```
 *
 */
export type PROMETHEUS_CHARTS_ADD_REQUEST = PROMETHEUS_CHARTS_ADD_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDIS;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequestRedis,*string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequestRedis,*string]
 *         properties:
 *             data:
 *                 pointer: true
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequestRedis,*string]
 *     type: struct
 * ```
 *
 */
export type PROMETHEUS_CHARTS_ADD_RESPONSE = PROMETHEUS_CHARTS_ADD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDIS_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.PrometheusRequestRedis:
 *         name: mogenius-operator/src/core.PrometheusRequestRedis
 *         properties:
 *             controller:
 *                 type: string
 *             namespace:
 *                 type: string
 *             query:
 *                 type: string
 *             queryName:
 *                 type: string
 *             step:
 *                 type: int
 * typeInfo:
 *     structRef: mogenius-operator/src/core.PrometheusRequestRedis
 *     type: struct
 * ```
 *
 */
export type PROMETHEUS_CHARTS_GET_REQUEST = PROMETHEUS_CHARTS_GET_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDIS;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.PrometheusStoreObject:
 *         name: mogenius-operator/src/core.PrometheusStoreObject
 *         properties:
 *             createdAt:
 *                 structRef: time.Time
 *                 type: struct
 *             query:
 *                 type: string
 *             step:
 *                 type: int
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequestRedis,*mogenius-operator/src/core.PrometheusStoreObject]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequestRedis,*mogenius-operator/src/core.PrometheusStoreObject]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/core.PrometheusStoreObject
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequestRedis,*mogenius-operator/src/core.PrometheusStoreObject]
 *     type: struct
 * ```
 *
 */
export type PROMETHEUS_CHARTS_GET_RESPONSE = PROMETHEUS_CHARTS_GET_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDIS_MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSSTOREOBJECT;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.PrometheusRequestRedisList:
 *         name: mogenius-operator/src/core.PrometheusRequestRedisList
 *         properties:
 *             controller:
 *                 type: string
 *             namespace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.PrometheusRequestRedisList
 *     type: struct
 * ```
 *
 */
export type PROMETHEUS_CHARTS_LIST_REQUEST = PROMETHEUS_CHARTS_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDISLIST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.PrometheusStoreObject:
 *         name: mogenius-operator/src/core.PrometheusStoreObject
 *         properties:
 *             createdAt:
 *                 structRef: time.Time
 *                 type: struct
 *             query:
 *                 type: string
 *             step:
 *                 type: int
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequestRedisList,map[string]mogenius-operator/src/core.PrometheusStoreObject]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequestRedisList,map[string]mogenius-operator/src/core.PrometheusStoreObject]
 *         properties:
 *             data:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     structRef: mogenius-operator/src/core.PrometheusStoreObject
 *                     type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequestRedisList,map[string]mogenius-operator/src/core.PrometheusStoreObject]
 *     type: struct
 * ```
 *
 */
export type PROMETHEUS_CHARTS_LIST_RESPONSE = PROMETHEUS_CHARTS_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDISLIST_MAPSTRINGMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSSTOREOBJECT;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.PrometheusRequestRedis:
 *         name: mogenius-operator/src/core.PrometheusRequestRedis
 *         properties:
 *             controller:
 *                 type: string
 *             namespace:
 *                 type: string
 *             query:
 *                 type: string
 *             queryName:
 *                 type: string
 *             step:
 *                 type: int
 * typeInfo:
 *     structRef: mogenius-operator/src/core.PrometheusRequestRedis
 *     type: struct
 * ```
 *
 */
export type PROMETHEUS_CHARTS_REMOVE_REQUEST = PROMETHEUS_CHARTS_REMOVE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDIS;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequestRedis,*string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequestRedis,*string]
 *         properties:
 *             data:
 *                 pointer: true
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequestRedis,*string]
 *     type: struct
 * ```
 *
 */
export type PROMETHEUS_CHARTS_REMOVE_RESPONSE = PROMETHEUS_CHARTS_REMOVE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDIS_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.PrometheusRequest:
 *         name: mogenius-operator/src/core.PrometheusRequest
 *         properties:
 *             prometheusPass:
 *                 type: string
 *             prometheusToken:
 *                 type: string
 *             prometheusUrl:
 *                 type: string
 *             prometheusUser:
 *                 type: string
 *             query:
 *                 type: string
 *             step:
 *                 type: int
 *             timeOffsetSeconds:
 *                 type: int
 * typeInfo:
 *     structRef: mogenius-operator/src/core.PrometheusRequest
 *     type: struct
 * ```
 *
 */
export type PROMETHEUS_IS_REACHABLE_REQUEST = PROMETHEUS_IS_REACHABLE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequest,bool]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequest,bool]
 *         properties:
 *             data:
 *                 type: bool
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequest,bool]
 *     type: struct
 * ```
 *
 */
export type PROMETHEUS_IS_REACHABLE_RESPONSE = PROMETHEUS_IS_REACHABLE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUEST_BOOL;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.PrometheusRequest:
 *         name: mogenius-operator/src/core.PrometheusRequest
 *         properties:
 *             prometheusPass:
 *                 type: string
 *             prometheusToken:
 *                 type: string
 *             prometheusUrl:
 *                 type: string
 *             prometheusUser:
 *                 type: string
 *             query:
 *                 type: string
 *             step:
 *                 type: int
 *             timeOffsetSeconds:
 *                 type: int
 * typeInfo:
 *     structRef: mogenius-operator/src/core.PrometheusRequest
 *     type: struct
 * ```
 *
 */
export type PROMETHEUS_QUERY_REQUEST = PROMETHEUS_QUERY_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_2:
 *         properties:
 *             result:
 *                 elementType:
 *                     pointer: true
 *                     type: any
 *                 type: array
 *             resultType:
 *                 type: string
 *     mogenius-operator/src/core.PrometheusQueryResponse:
 *         name: mogenius-operator/src/core.PrometheusQueryResponse
 *         properties:
 *             data:
 *                 structRef: ANON_STRUCT_2
 *                 type: struct
 *             error:
 *                 type: string
 *             errorType:
 *                 type: string
 *             status:
 *                 type: string
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequest,*mogenius-operator/src/core.PrometheusQueryResponse]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequest,*mogenius-operator/src/core.PrometheusQueryResponse]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/core.PrometheusQueryResponse
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequest,*mogenius-operator/src/core.PrometheusQueryResponse]
 *     type: struct
 * ```
 *
 */
export type PROMETHEUS_QUERY_RESPONSE = PROMETHEUS_QUERY_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUEST_MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSQUERYRESPONSE;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.PrometheusRequest:
 *         name: mogenius-operator/src/core.PrometheusRequest
 *         properties:
 *             prometheusPass:
 *                 type: string
 *             prometheusToken:
 *                 type: string
 *             prometheusUrl:
 *                 type: string
 *             prometheusUser:
 *                 type: string
 *             query:
 *                 type: string
 *             step:
 *                 type: int
 *             timeOffsetSeconds:
 *                 type: int
 * typeInfo:
 *     structRef: mogenius-operator/src/core.PrometheusRequest
 *     type: struct
 * ```
 *
 */
export type PROMETHEUS_VALUES_REQUEST = PROMETHEUS_VALUES_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequest,[]string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequest,[]string]
 *         properties:
 *             data:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.PrometheusRequest,[]string]
 *     type: struct
 * ```
 *
 */
export type PROMETHEUS_VALUES_RESPONSE = PROMETHEUS_VALUES_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUEST_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type SEALED_SECRET_CREATE_FROM_EXISTING_REQUEST = SEALED_SECRET_CREATE_FROM_EXISTING_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *         properties:
 *             Object:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·36,*k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·36,*k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·36,*k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     type: struct
 * ```
 *
 */
export type SEALED_SECRET_CREATE_FROM_EXISTING_RESPONSE = SEALED_SECRET_CREATE_FROM_EXISTING_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST36_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type SEALED_SECRET_GET_CERTIFICATE_REQUEST = SEALED_SECRET_GET_CERTIFICATE_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/api/core/v1.Secret:
 *         name: k8s.io/api/core/v1.Secret
 *         properties:
 *             TypeMeta:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta
 *                 type: struct
 *             data:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     elementType:
 *                         type: uint
 *                     type: array
 *             immutable:
 *                 pointer: true
 *                 type: bool
 *             metadata:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
 *                 type: struct
 *             stringData:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             type:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1
 *         properties: {}
 *     k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry
 *         properties:
 *             apiVersion:
 *                 type: string
 *             fieldsType:
 *                 type: string
 *             fieldsV1:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.FieldsV1
 *                 type: struct
 *             manager:
 *                 type: string
 *             operation:
 *                 type: string
 *             subresource:
 *                 type: string
 *             time:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *     k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
 *         properties:
 *             annotations:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             creationTimestamp:
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             deletionGracePeriodSeconds:
 *                 pointer: true
 *                 type: int
 *             deletionTimestamp:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *                 type: struct
 *             finalizers:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             generateName:
 *                 type: string
 *             generation:
 *                 type: int
 *             labels:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: string
 *             managedFields:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1.ManagedFieldsEntry
 *                     type: struct
 *                 type: array
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             ownerReferences:
 *                 elementType:
 *                     structRef: k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference
 *                     type: struct
 *                 type: array
 *             resourceVersion:
 *                 type: string
 *             selfLink:
 *                 type: string
 *             uid:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference
 *         properties:
 *             apiVersion:
 *                 type: string
 *             blockOwnerDeletion:
 *                 pointer: true
 *                 type: bool
 *             controller:
 *                 pointer: true
 *                 type: bool
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             uid:
 *                 type: string
 *     k8s.io/apimachinery/pkg/apis/meta/v1.Time:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.Time
 *         properties:
 *             Time:
 *                 structRef: time.Time
 *                 type: struct
 *     k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,*k8s.io/api/core/v1.Secret]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,*k8s.io/api/core/v1.Secret]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: k8s.io/api/core/v1.Secret
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,*k8s.io/api/core/v1.Secret]
 *     type: struct
 * ```
 *
 */
export type SEALED_SECRET_GET_CERTIFICATE_RESPONSE = SEALED_SECRET_GET_CERTIFICATE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_K8S_IO_API_CORE_V1_SECRET;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/xterm.PodCmdConnectionRequest:
 *         name: mogenius-operator/src/xterm.PodCmdConnectionRequest
 *         properties:
 *             container:
 *                 type: string
 *             controller:
 *                 type: string
 *             logTail:
 *                 type: string
 *             namespace:
 *                 type: string
 *             pod:
 *                 type: string
 *             wsConnectionRequest:
 *                 structRef: mogenius-operator/src/xterm.WsConnectionRequest
 *                 type: struct
 *     mogenius-operator/src/xterm.WsConnectionRequest:
 *         name: mogenius-operator/src/xterm.WsConnectionRequest
 *         properties:
 *             channelId:
 *                 type: string
 *             cmdType:
 *                 type: string
 *             nodeName:
 *                 type: string
 *             podName:
 *                 type: string
 *             websocketHost:
 *                 type: string
 *             websocketScheme:
 *                 type: string
 *             workspace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/xterm.PodCmdConnectionRequest
 *     type: struct
 * ```
 *
 */
export type SERVICE_EXEC_SH_CONNECTION_REQUEST_REQUEST = SERVICE_EXEC_SH_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_PODCMDCONNECTIONREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_1:
 *         properties: {}
 *     mogenius-operator/src/core.Result[mogenius-operator/src/xterm.PodCmdConnectionRequest,mogenius-operator/src/core.Void]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.PodCmdConnectionRequest,mogenius-operator/src/core.Void]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: ANON_STRUCT_1
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.PodCmdConnectionRequest,mogenius-operator/src/core.Void]
 *     type: struct
 * ```
 *
 */
export type SERVICE_EXEC_SH_CONNECTION_REQUEST_RESPONSE = SERVICE_EXEC_SH_CONNECTION_REQUEST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_PODCMDCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/xterm.PodCmdConnectionRequest:
 *         name: mogenius-operator/src/xterm.PodCmdConnectionRequest
 *         properties:
 *             container:
 *                 type: string
 *             controller:
 *                 type: string
 *             logTail:
 *                 type: string
 *             namespace:
 *                 type: string
 *             pod:
 *                 type: string
 *             wsConnectionRequest:
 *                 structRef: mogenius-operator/src/xterm.WsConnectionRequest
 *                 type: struct
 *     mogenius-operator/src/xterm.WsConnectionRequest:
 *         name: mogenius-operator/src/xterm.WsConnectionRequest
 *         properties:
 *             channelId:
 *                 type: string
 *             cmdType:
 *                 type: string
 *             nodeName:
 *                 type: string
 *             podName:
 *                 type: string
 *             websocketHost:
 *                 type: string
 *             websocketScheme:
 *                 type: string
 *             workspace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/xterm.PodCmdConnectionRequest
 *     type: struct
 * ```
 *
 */
export type SERVICE_LOG_STREAM_CONNECTION_REQUEST_REQUEST = SERVICE_LOG_STREAM_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_PODCMDCONNECTIONREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_1:
 *         properties: {}
 *     mogenius-operator/src/core.Result[mogenius-operator/src/xterm.PodCmdConnectionRequest,mogenius-operator/src/core.Void]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.PodCmdConnectionRequest,mogenius-operator/src/core.Void]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: ANON_STRUCT_1
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.PodCmdConnectionRequest,mogenius-operator/src/core.Void]
 *     type: struct
 * ```
 *
 */
export type SERVICE_LOG_STREAM_CONNECTION_REQUEST_RESPONSE = SERVICE_LOG_STREAM_CONNECTION_REQUEST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_PODCMDCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/xterm.PodEventConnectionRequest:
 *         name: mogenius-operator/src/xterm.PodEventConnectionRequest
 *         properties:
 *             controller:
 *                 type: string
 *             namespace:
 *                 type: string
 *             wsConnectionRequest:
 *                 structRef: mogenius-operator/src/xterm.WsConnectionRequest
 *                 type: struct
 *     mogenius-operator/src/xterm.WsConnectionRequest:
 *         name: mogenius-operator/src/xterm.WsConnectionRequest
 *         properties:
 *             channelId:
 *                 type: string
 *             cmdType:
 *                 type: string
 *             nodeName:
 *                 type: string
 *             podName:
 *                 type: string
 *             websocketHost:
 *                 type: string
 *             websocketScheme:
 *                 type: string
 *             workspace:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/xterm.PodEventConnectionRequest
 *     type: struct
 * ```
 *
 */
export type SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST_REQUEST = SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_PODEVENTCONNECTIONREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_1:
 *         properties: {}
 *     mogenius-operator/src/core.Result[mogenius-operator/src/xterm.PodEventConnectionRequest,mogenius-operator/src/core.Void]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.PodEventConnectionRequest,mogenius-operator/src/core.Void]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: ANON_STRUCT_1
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/xterm.PodEventConnectionRequest,mogenius-operator/src/core.Void]
 *     type: struct
 * ```
 *
 */
export type SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST_RESPONSE = SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_PODEVENTCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             timeOffsetMinutes:
 *                 type: int
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type STATS_POD_ALL_FOR_CONTROLLER_REQUEST = STATS_POD_ALL_FOR_CONTROLLER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·7,*[]mogenius-operator/src/structs.PodStats]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·7,*[]mogenius-operator/src/structs.PodStats]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/structs.PodStats
 *                     type: struct
 *                 pointer: true
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/structs.PodStats:
 *         name: mogenius-operator/src/structs.PodStats
 *         properties:
 *             containerName:
 *                 type: string
 *             cpu:
 *                 type: int
 *             cpuLimit:
 *                 type: int
 *             createdAt:
 *                 structRef: time.Time
 *                 type: struct
 *             ephemeralStorage:
 *                 type: int
 *             ephemeralStorageLimit:
 *                 type: int
 *             memory:
 *                 type: int
 *             memoryLimit:
 *                 type: int
 *             namespace:
 *                 type: string
 *             podName:
 *                 type: string
 *             startTime:
 *                 structRef: time.Time
 *                 type: struct
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·7,*[]mogenius-operator/src/structs.PodStats]
 *     type: struct
 * ```
 *
 */
export type STATS_POD_ALL_FOR_CONTROLLER_RESPONSE = STATS_POD_ALL_FOR_CONTROLLER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST7_MOGENIUS_OPERATOR_SRC_STRUCTS_PODSTATS;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             timeOffsetMinutes:
 *                 type: int
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type STATS_TRAFFIC_ALL_FOR_CONTROLLER_REQUEST = STATS_TRAFFIC_ALL_FOR_CONTROLLER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·7,*[]mogenius-operator/src/networkmonitor.PodNetworkStats]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·7,*[]mogenius-operator/src/networkmonitor.PodNetworkStats]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/networkmonitor.PodNetworkStats
 *                     type: struct
 *                 pointer: true
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/networkmonitor.PodNetworkStats:
 *         name: mogenius-operator/src/networkmonitor.PodNetworkStats
 *         properties:
 *             createdAt:
 *                 structRef: time.Time
 *                 type: struct
 *             namespace:
 *                 type: string
 *             pod:
 *                 type: string
 *             receivedBytes:
 *                 type: uint
 *             receivedPackets:
 *                 type: uint
 *             receivedStartBytes:
 *                 type: uint
 *             transmitBytes:
 *                 type: uint
 *             transmitPackets:
 *                 type: uint
 *             transmitStartBytes:
 *                 type: uint
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·7,*[]mogenius-operator/src/networkmonitor.PodNetworkStats]
 *     type: struct
 * ```
 *
 */
export type STATS_TRAFFIC_ALL_FOR_CONTROLLER_RESPONSE = STATS_TRAFFIC_ALL_FOR_CONTROLLER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST7_MOGENIUS_OPERATOR_SRC_NETWORKMONITOR_PODNETWORKSTATS;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             timeOffsetMinutes:
 *                 type: int
 *             workspaceName:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type STATS_WORKSPACE_CPU_UTILIZATION_REQUEST = STATS_WORKSPACE_CPU_UTILIZATION_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.GenericChartEntry:
 *         name: mogenius-operator/src/core.GenericChartEntry
 *         properties:
 *             pods:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: float
 *             time:
 *                 structRef: time.Time
 *                 type: struct
 *             value:
 *                 type: float
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·8,[]mogenius-operator/src/core.GenericChartEntry]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·8,[]mogenius-operator/src/core.GenericChartEntry]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/core.GenericChartEntry
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·8,[]mogenius-operator/src/core.GenericChartEntry]
 *     type: struct
 * ```
 *
 */
export type STATS_WORKSPACE_CPU_UTILIZATION_RESPONSE = STATS_WORKSPACE_CPU_UTILIZATION_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST8_MOGENIUS_OPERATOR_SRC_CORE_GENERICCHARTENTRY;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             timeOffsetMinutes:
 *                 type: int
 *             workspaceName:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type STATS_WORKSPACE_MEMORY_UTILIZATION_REQUEST = STATS_WORKSPACE_MEMORY_UTILIZATION_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.GenericChartEntry:
 *         name: mogenius-operator/src/core.GenericChartEntry
 *         properties:
 *             pods:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: float
 *             time:
 *                 structRef: time.Time
 *                 type: struct
 *             value:
 *                 type: float
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·8,[]mogenius-operator/src/core.GenericChartEntry]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·8,[]mogenius-operator/src/core.GenericChartEntry]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/core.GenericChartEntry
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·8,[]mogenius-operator/src/core.GenericChartEntry]
 *     type: struct
 * ```
 *
 */
export type STATS_WORKSPACE_MEMORY_UTILIZATION_RESPONSE = STATS_WORKSPACE_MEMORY_UTILIZATION_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST8_MOGENIUS_OPERATOR_SRC_CORE_GENERICCHARTENTRY;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             timeOffsetMinutes:
 *                 type: int
 *             workspaceName:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type STATS_WORKSPACE_TRAFFIC_UTILIZATION_REQUEST = STATS_WORKSPACE_TRAFFIC_UTILIZATION_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.GenericChartEntry:
 *         name: mogenius-operator/src/core.GenericChartEntry
 *         properties:
 *             pods:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     type: float
 *             time:
 *                 structRef: time.Time
 *                 type: struct
 *             value:
 *                 type: float
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·8,[]mogenius-operator/src/core.GenericChartEntry]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·8,[]mogenius-operator/src/core.GenericChartEntry]
 *         properties:
 *             data:
 *                 elementType:
 *                     structRef: mogenius-operator/src/core.GenericChartEntry
 *                     type: struct
 *                 type: array
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·8,[]mogenius-operator/src/core.GenericChartEntry]
 *     type: struct
 * ```
 *
 */
export type STATS_WORKSPACE_TRAFFIC_UTILIZATION_RESPONSE = STATS_WORKSPACE_TRAFFIC_UTILIZATION_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST8_MOGENIUS_OPERATOR_SRC_CORE_GENERICCHARTENTRY;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/services.NfsVolumeRequest:
 *         name: mogenius-operator/src/services.NfsVolumeRequest
 *         properties:
 *             namespaceName:
 *                 type: string
 *             sizeInGb:
 *                 type: int
 *             volumeName:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/services.NfsVolumeRequest
 *     type: struct
 * ```
 *
 */
export type STORAGE_CREATE_VOLUME_REQUEST = STORAGE_CREATE_VOLUME_REQUEST__MOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMEREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/services.NfsVolumeRequest,bool]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/services.NfsVolumeRequest,bool]
 *         properties:
 *             data:
 *                 type: bool
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/services.NfsVolumeRequest,bool]
 *     type: struct
 * ```
 *
 */
export type STORAGE_CREATE_VOLUME_RESPONSE = STORAGE_CREATE_VOLUME_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMEREQUEST_BOOL;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/services.NfsVolumeRequest:
 *         name: mogenius-operator/src/services.NfsVolumeRequest
 *         properties:
 *             namespaceName:
 *                 type: string
 *             sizeInGb:
 *                 type: int
 *             volumeName:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/services.NfsVolumeRequest
 *     type: struct
 * ```
 *
 */
export type STORAGE_DELETE_VOLUME_REQUEST = STORAGE_DELETE_VOLUME_REQUEST__MOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMEREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/services.NfsVolumeRequest,bool]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/services.NfsVolumeRequest,bool]
 *         properties:
 *             data:
 *                 type: bool
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/services.NfsVolumeRequest,bool]
 *     type: struct
 * ```
 *
 */
export type STORAGE_DELETE_VOLUME_RESPONSE = STORAGE_DELETE_VOLUME_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMEREQUEST_BOOL;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/services.NfsVolumeStatsRequest:
 *         name: mogenius-operator/src/services.NfsVolumeStatsRequest
 *         properties:
 *             namespaceName:
 *                 type: string
 *             volumeName:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/services.NfsVolumeStatsRequest
 *     type: struct
 * ```
 *
 */
export type STORAGE_STATS_REQUEST = STORAGE_STATS_REQUEST__MOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMESTATSREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/services.NfsVolumeStatsRequest,mogenius-operator/src/services.NfsVolumeStatsResponse]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/services.NfsVolumeStatsRequest,mogenius-operator/src/services.NfsVolumeStatsResponse]
 *         properties:
 *             data:
 *                 structRef: mogenius-operator/src/services.NfsVolumeStatsResponse
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/services.NfsVolumeStatsResponse:
 *         name: mogenius-operator/src/services.NfsVolumeStatsResponse
 *         properties:
 *             freeBytes:
 *                 type: uint
 *             totalBytes:
 *                 type: uint
 *             usedBytes:
 *                 type: uint
 *             volumeName:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/services.NfsVolumeStatsRequest,mogenius-operator/src/services.NfsVolumeStatsResponse]
 *     type: struct
 * ```
 *
 */
export type STORAGE_STATS_RESPONSE = STORAGE_STATS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMESTATSREQUEST_MOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMESTATSRESPONSE;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/services.NfsStatusRequest:
 *         name: mogenius-operator/src/services.NfsStatusRequest
 *         properties:
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             type:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/services.NfsStatusRequest
 *     type: struct
 * ```
 *
 */
export type STORAGE_STATUS_REQUEST = STORAGE_STATUS_REQUEST__MOGENIUS_OPERATOR_SRC_SERVICES_NFSSTATUSREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/services.NfsStatusRequest,mogenius-operator/src/services.NfsStatusResponse]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/services.NfsStatusRequest,mogenius-operator/src/services.NfsStatusResponse]
 *         properties:
 *             data:
 *                 structRef: mogenius-operator/src/services.NfsStatusResponse
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/services.NfsStatusResponse:
 *         name: mogenius-operator/src/services.NfsStatusResponse
 *         properties:
 *             freeBytes:
 *                 type: uint
 *             messages:
 *                 elementType:
 *                     structRef: mogenius-operator/src/services.VolumeStatusMessage
 *                     type: struct
 *                 type: array
 *             namespaceName:
 *                 type: string
 *             status:
 *                 type: string
 *             totalBytes:
 *                 type: uint
 *             usedByPods:
 *                 elementType:
 *                     type: string
 *                 type: array
 *             usedBytes:
 *                 type: uint
 *             volumeName:
 *                 type: string
 *     mogenius-operator/src/services.VolumeStatusMessage:
 *         name: mogenius-operator/src/services.VolumeStatusMessage
 *         properties:
 *             message:
 *                 type: string
 *             type:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/services.NfsStatusRequest,mogenius-operator/src/services.NfsStatusResponse]
 *     type: struct
 * ```
 *
 */
export type STORAGE_STATUS_RESPONSE = STORAGE_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_SERVICES_NFSSTATUSREQUEST_MOGENIUS_OPERATOR_SRC_SERVICES_NFSSTATUSRESPONSE;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type SYSTEM_CHECK_REQUEST = SYSTEM_CHECK_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,mogenius-operator/src/services.SystemCheckResponse]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,mogenius-operator/src/services.SystemCheckResponse]
 *         properties:
 *             data:
 *                 structRef: mogenius-operator/src/services.SystemCheckResponse
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/services.SystemCheckEntry:
 *         name: mogenius-operator/src/services.SystemCheckEntry
 *         properties:
 *             checkName:
 *                 type: string
 *             description:
 *                 type: string
 *             errorMessage:
 *                 pointer: true
 *                 type: string
 *             helmStatus:
 *                 type: string
 *             installPattern:
 *                 type: string
 *             isRequired:
 *                 type: bool
 *             isRunning:
 *                 type: bool
 *             processTimeInMs:
 *                 type: int
 *             solutionMessage:
 *                 type: string
 *             successMessage:
 *                 type: string
 *             uninstallPattern:
 *                 type: string
 *             upgradePattern:
 *                 type: string
 *             versionAvailable:
 *                 type: string
 *             versionInstalled:
 *                 type: string
 *             wantsToBeInstalled:
 *                 type: bool
 *     mogenius-operator/src/services.SystemCheckResponse:
 *         name: mogenius-operator/src/services.SystemCheckResponse
 *         properties:
 *             entries:
 *                 elementType:
 *                     structRef: mogenius-operator/src/services.SystemCheckEntry
 *                     type: struct
 *                 type: array
 *             terminalString:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,mogenius-operator/src/services.SystemCheckResponse]
 *     type: struct
 * ```
 *
 */
export type SYSTEM_CHECK_RESPONSE = SYSTEM_CHECK_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_SERVICES_SYSTEMCHECKRESPONSE;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/utils.ResourceDescriptor:
 *         name: mogenius-operator/src/utils.ResourceDescriptor
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *             namespaced:
 *                 type: bool
 *             plural:
 *                 type: string
 *     mogenius-operator/src/utils.WorkloadSingleRequest:
 *         name: mogenius-operator/src/utils.WorkloadSingleRequest
 *         properties:
 *             ResourceDescriptor:
 *                 structRef: mogenius-operator/src/utils.ResourceDescriptor
 *                 type: struct
 *             namespace:
 *                 type: string
 *             resourceName:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/utils.WorkloadSingleRequest
 *     type: struct
 * ```
 *
 */
export type TRIGGER_WORKLOAD_REQUEST = TRIGGER_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *         properties:
 *             Object:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadSingleRequest,*k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadSingleRequest,*k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadSingleRequest,*k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     type: struct
 * ```
 *
 */
export type TRIGGER_WORKLOAD_RESPONSE = TRIGGER_WORKLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type UNINSTALL_CERT_MANAGER_REQUEST = UNINSTALL_CERT_MANAGER_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type UNINSTALL_CERT_MANAGER_RESPONSE = UNINSTALL_CERT_MANAGER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type UNINSTALL_CLUSTER_ISSUER_REQUEST = UNINSTALL_CLUSTER_ISSUER_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type UNINSTALL_CLUSTER_ISSUER_RESPONSE = UNINSTALL_CLUSTER_ISSUER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type UNINSTALL_INGRESS_CONTROLLER_TRAEFIK_REQUEST = UNINSTALL_INGRESS_CONTROLLER_TRAEFIK_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type UNINSTALL_INGRESS_CONTROLLER_TRAEFIK_RESPONSE = UNINSTALL_INGRESS_CONTROLLER_TRAEFIK_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type UNINSTALL_KEPLER_REQUEST = UNINSTALL_KEPLER_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type UNINSTALL_KEPLER_RESPONSE = UNINSTALL_KEPLER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type UNINSTALL_METALLB_REQUEST = UNINSTALL_METALLB_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type UNINSTALL_METALLB_RESPONSE = UNINSTALL_METALLB_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type UNINSTALL_METRICS_SERVER_REQUEST = UNINSTALL_METRICS_SERVER_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type UNINSTALL_METRICS_SERVER_RESPONSE = UNINSTALL_METRICS_SERVER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             grantee:
 *                 type: string
 *             name:
 *                 type: string
 *             role:
 *                 type: string
 *             targetName:
 *                 type: string
 *             targetType:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type UPDATE_GRANT_REQUEST = UPDATE_GRANT_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·31,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·31,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·31,string]
 *     type: struct
 * ```
 *
 */
export type UPDATE_GRANT_RESPONSE = UPDATE_GRANT_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST31_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/api/rbac/v1.Subject:
 *         name: k8s.io/api/rbac/v1.Subject
 *         properties:
 *             apiGroup:
 *                 type: string
 *             kind:
 *                 type: string
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             email:
 *                 type: string
 *             firstName:
 *                 type: string
 *             lastName:
 *                 type: string
 *             name:
 *                 type: string
 *             subject:
 *                 pointer: true
 *                 structRef: k8s.io/api/rbac/v1.Subject
 *                 type: struct
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type UPDATE_USER_REQUEST = UPDATE_USER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·26,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·26,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·26,string]
 *     type: struct
 * ```
 *
 */
export type UPDATE_USER_RESPONSE = UPDATE_USER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST26_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/utils.ResourceDescriptor:
 *         name: mogenius-operator/src/utils.ResourceDescriptor
 *         properties:
 *             apiVersion:
 *                 type: string
 *             kind:
 *                 type: string
 *             namespaced:
 *                 type: bool
 *             plural:
 *                 type: string
 *     mogenius-operator/src/utils.WorkloadChangeRequest:
 *         name: mogenius-operator/src/utils.WorkloadChangeRequest
 *         properties:
 *             ResourceDescriptor:
 *                 structRef: mogenius-operator/src/utils.ResourceDescriptor
 *                 type: struct
 *             namespace:
 *                 type: string
 *             yamlData:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/utils.WorkloadChangeRequest
 *     type: struct
 * ```
 *
 */
export type UPDATE_WORKLOAD_REQUEST = UPDATE_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_WORKLOADCHANGEREQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured:
 *         name: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *         properties:
 *             Object:
 *                 keyType:
 *                     type: string
 *                 type: map
 *                 valueType:
 *                     pointer: true
 *                     type: any
 *     ? mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadChangeRequest,*k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     :   name: mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadChangeRequest,*k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/utils.WorkloadChangeRequest,*k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.Unstructured]
 *     type: struct
 * ```
 *
 */
export type UPDATE_WORKLOAD_RESPONSE = UPDATE_WORKLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_UTILS_WORKLOADCHANGEREQUEST_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             displayName:
 *                 type: string
 *             name:
 *                 type: string
 *             resources:
 *                 elementType:
 *                     structRef: mogenius-operator/src/crds/v1alpha1.WorkspaceResourceIdentifier
 *                     type: struct
 *                 type: array
 *     mogenius-operator/src/crds/v1alpha1.WorkspaceResourceIdentifier:
 *         name: mogenius-operator/src/crds/v1alpha1.WorkspaceResourceIdentifier
 *         properties:
 *             id:
 *                 type: string
 *             namespace:
 *                 type: string
 *             type:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type UPDATE_WORKSPACE_REQUEST = UPDATE_WORKSPACE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·21,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·21,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·21,string]
 *     type: struct
 * ```
 *
 */
export type UPDATE_WORKSPACE_RESPONSE = UPDATE_WORKSPACE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST21_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             command:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type UPGRADEK8SMANAGER_REQUEST = UPGRADEK8SMANAGER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·4,*mogenius-operator/src/structs.Job]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·4,*mogenius-operator/src/structs.Job]
 *         properties:
 *             data:
 *                 pointer: true
 *                 structRef: mogenius-operator/src/structs.Job
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 *     mogenius-operator/src/structs.Command:
 *         name: mogenius-operator/src/structs.Command
 *         properties:
 *             command:
 *                 type: string
 *             finished:
 *                 structRef: time.Time
 *                 type: struct
 *             id:
 *                 type: string
 *             message:
 *                 type: string
 *             started:
 *                 structRef: time.Time
 *                 type: struct
 *             state:
 *                 type: string
 *             title:
 *                 type: string
 *     mogenius-operator/src/structs.Job:
 *         name: mogenius-operator/src/structs.Job
 *         properties:
 *             commands:
 *                 elementType:
 *                     pointer: true
 *                     structRef: mogenius-operator/src/structs.Command
 *                     type: struct
 *                 type: array
 *             containerName:
 *                 type: string
 *             controllerName:
 *                 type: string
 *             finished:
 *                 structRef: time.Time
 *                 type: struct
 *             id:
 *                 type: string
 *             message:
 *                 type: string
 *             namespaceName:
 *                 type: string
 *             projectId:
 *                 type: string
 *             started:
 *                 structRef: time.Time
 *                 type: struct
 *             state:
 *                 type: string
 *             title:
 *                 type: string
 *     time.Time:
 *         name: time.Time
 *         properties: {}
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·4,*mogenius-operator/src/structs.Job]
 *     type: struct
 * ```
 *
 */
export type UPGRADEK8SMANAGER_RESPONSE = UPGRADEK8SMANAGER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST4_MOGENIUS_OPERATOR_SRC_STRUCTS_JOB;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type UPGRADE_CERT_MANAGER_REQUEST = UPGRADE_CERT_MANAGER_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type UPGRADE_CERT_MANAGER_RESPONSE = UPGRADE_CERT_MANAGER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type UPGRADE_INGRESS_CONTROLLER_TRAEFIK_REQUEST = UPGRADE_INGRESS_CONTROLLER_TRAEFIK_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type UPGRADE_INGRESS_CONTROLLER_TRAEFIK_RESPONSE = UPGRADE_INGRESS_CONTROLLER_TRAEFIK_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type UPGRADE_KEPLER_REQUEST = UPGRADE_KEPLER_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type UPGRADE_KEPLER_RESPONSE = UPGRADE_KEPLER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type UPGRADE_METALLB_REQUEST = UPGRADE_METALLB_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type UPGRADE_METALLB_RESPONSE = UPGRADE_METALLB_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     ANON_STRUCT_0:
 *         properties: {}
 * typeInfo:
 *     pointer: true
 *     structRef: ANON_STRUCT_0
 *     type: struct
 * ```
 *
 */
export type UPGRADE_METRICS_SERVER_REQUEST = UPGRADE_METRICS_SERVER_REQUEST__ANON_STRUCT_0|undefined;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *         properties:
 *             data:
 *                 type: string
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Void,string]
 *     type: struct
 * ```
 *
 */
export type UPGRADE_METRICS_SERVER_RESPONSE = UPGRADE_METRICS_SERVER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.Request:
 *         name: mogenius-operator/src/core.Request
 *         properties:
 *             configMaps:
 *                 type: bool
 *             dryRun:
 *                 type: bool
 *             ingresses:
 *                 type: bool
 *             jobs:
 *                 type: bool
 *             name:
 *                 type: string
 *             pods:
 *                 type: bool
 *             replicaSets:
 *                 type: bool
 *             secrets:
 *                 type: bool
 *             services:
 *                 type: bool
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Request
 *     type: struct
 * ```
 *
 */
export type WORKSPACE_CLEAN_UP_REQUEST = WORKSPACE_CLEAN_UP_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST;

/**
 * #### Source
 *
 * ```yaml
 * structs:
 *     mogenius-operator/src/core.CleanUpResult:
 *         name: mogenius-operator/src/core.CleanUpResult
 *         properties:
 *             configMaps:
 *                 elementType:
 *                     structRef: mogenius-operator/src/core.CleanUpResultEntry
 *                     type: struct
 *                 type: array
 *             ingresses:
 *                 elementType:
 *                     structRef: mogenius-operator/src/core.CleanUpResultEntry
 *                     type: struct
 *                 type: array
 *             jobs:
 *                 elementType:
 *                     structRef: mogenius-operator/src/core.CleanUpResultEntry
 *                     type: struct
 *                 type: array
 *             pods:
 *                 elementType:
 *                     structRef: mogenius-operator/src/core.CleanUpResultEntry
 *                     type: struct
 *                 type: array
 *             replicaSets:
 *                 elementType:
 *                     structRef: mogenius-operator/src/core.CleanUpResultEntry
 *                     type: struct
 *                 type: array
 *             secrets:
 *                 elementType:
 *                     structRef: mogenius-operator/src/core.CleanUpResultEntry
 *                     type: struct
 *                 type: array
 *             services:
 *                 elementType:
 *                     structRef: mogenius-operator/src/core.CleanUpResultEntry
 *                     type: struct
 *                 type: array
 *     mogenius-operator/src/core.CleanUpResultEntry:
 *         name: mogenius-operator/src/core.CleanUpResultEntry
 *         properties:
 *             name:
 *                 type: string
 *             namespace:
 *                 type: string
 *             reason:
 *                 type: string
 *     mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·20,mogenius-operator/src/core.CleanUpResult]:
 *         name: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·20,mogenius-operator/src/core.CleanUpResult]
 *         properties:
 *             data:
 *                 structRef: mogenius-operator/src/core.CleanUpResult
 *                 type: struct
 *             message:
 *                 type: string
 *             status:
 *                 type: string
 * typeInfo:
 *     structRef: mogenius-operator/src/core.Result[mogenius-operator/src/core.Request·20,mogenius-operator/src/core.CleanUpResult]
 *     type: struct
 * ```
 *
 */
export type WORKSPACE_CLEAN_UP_RESPONSE = WORKSPACE_CLEAN_UP_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST20_MOGENIUS_OPERATOR_SRC_CORE_CLEANUPRESULT;


//===============================================================
//===================== Struct Definitions ======================
//===============================================================

export type AUDIT_LOG_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"limit": number,"offset": number,"workspaceName": string};
export type AUDIT_LOG_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESPONSE = {"data": AUDIT_LOG_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_STORE_AUDITLOGRESPONSE,"message": string,"status": string};
export type AUDIT_LOG_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST34_MOGENIUS_OPERATOR_SRC_CORE_RESPONSE35 = {"data": AUDIT_LOG_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESPONSE,"message": string,"status": string};
export type AUDIT_LOG_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_STORE_AUDITLOGENTRY = {"createdAt": AUDIT_LOG_LIST_RESPONSE__TIME_TIME,"diff": string,"error": string,"pattern": string,"payload": any,"result": any,"user": AUDIT_LOG_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_USER,"workspace": string};
export type AUDIT_LOG_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_STORE_AUDITLOGRESPONSE = {"data": AUDIT_LOG_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_STORE_AUDITLOGENTRY[],"totalCount": number};
export type AUDIT_LOG_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_USER = {"email": string,"firstName": string,"lastName": string,"source": string};
export type AUDIT_LOG_LIST_RESPONSE__TIME_TIME = {};
export type CLUSTER_ARGO_CD_APPLICATION_REFRESH_REQUEST__MOGENIUS_OPERATOR_SRC_ARGOCD_ARGOCDAPPLICATIONREFRESHREQUEST = {"applicationName": string,"username": string};
export type CLUSTER_ARGO_CD_APPLICATION_REFRESH_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_ARGOCD_ARGOCDAPPLICATIONREFRESHREQUEST_BOOL = {"data": boolean,"message": string,"status": string};
export type CLUSTER_ARGO_CD_CREATE_API_TOKEN_REQUEST__MOGENIUS_OPERATOR_SRC_ARGOCD_ARGOCDCREATEAPITOKENREQUEST = {"username": string};
export type CLUSTER_ARGO_CD_CREATE_API_TOKEN_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_ARGOCD_ARGOCDCREATEAPITOKENREQUEST_BOOL = {"data": boolean,"message": string,"status": string};
export type CLUSTER_CLEAR_VALKEY_CACHE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"includeNodeStats": boolean,"includePodStats": boolean,"includeTraffic": boolean};
export type CLUSTER_CLEAR_VALKEY_CACHE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST5_STRING = {"data": string,"message": string,"status": string};
export type CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_COMPONENTLOGCONNECTIONREQUEST = {"component": string,"controller": string|undefined,"namespace": string|undefined,"release": string|undefined,"wsConnectionRequest": CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST};
export type CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST = {"channelId": string,"cmdType": string,"nodeName": string,"podName": string,"websocketHost": string,"websocketScheme": string,"workspace": string};
export type CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST_RESPONSE__ANON_STRUCT_1 = {};
export type CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_COMPONENTLOGCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID = {"data": CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST_RESPONSE__ANON_STRUCT_1|undefined,"message": string,"status": string};
export type CLUSTER_FORCE_DISCONNECT_REQUEST__ANON_STRUCT_0 = {};
export type CLUSTER_FORCE_DISCONNECT_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_BOOL = {"data": boolean,"message": string,"status": string};
export type CLUSTER_FORCE_RECONNECT_REQUEST__ANON_STRUCT_0 = {};
export type CLUSTER_FORCE_RECONNECT_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_BOOL = {"data": boolean,"message": string,"status": string};
export type CLUSTER_HELM_CHART_INSTALL_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINSTALLUPGRADEREQUEST = {"chart": string,"dryRun": boolean,"namespace": string,"release": string,"values": string,"version": string};
export type CLUSTER_HELM_CHART_INSTALL_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINSTALLUPGRADEREQUEST_STRING = {"data": string,"message": string,"status": string};
export type CLUSTER_HELM_CHART_INSTALL_OCI_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTOCIINSTALLUPGRADEREQUEST = {"chart": string,"dryRun": boolean,"namespace": string,"password": string,"registryUrl": string,"release": string,"username": string,"values": string,"version": string};
export type CLUSTER_HELM_CHART_INSTALL_OCI_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMCHARTOCIINSTALLUPGRADEREQUEST_STRING = {"data": string,"message": string,"status": string};
export type CLUSTER_HELM_CHART_REMOVE_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMREPOREMOVEREQUEST = {"name": string};
export type CLUSTER_HELM_CHART_REMOVE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMREPOREMOVEREQUEST_STRING = {"data": string,"message": string,"status": string};
export type CLUSTER_HELM_CHART_SEARCH_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTSEARCHREQUEST = {"name": string};
export type CLUSTER_HELM_CHART_SEARCH_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMCHARTSEARCHREQUEST_MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINFO = {"data": CLUSTER_HELM_CHART_SEARCH_RESPONSE__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINFO[],"message": string,"status": string};
export type CLUSTER_HELM_CHART_SEARCH_RESPONSE__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINFO = {"app_version": string,"description": string,"name": string,"version": string};
export type CLUSTER_HELM_CHART_SHOW_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTSHOWREQUEST = {"chart": string,"format": string,"version": string};
export type CLUSTER_HELM_CHART_SHOW_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMCHARTSHOWREQUEST_STRING = {"data": string,"message": string,"status": string};
export type CLUSTER_HELM_CHART_VERSIONS_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTVERSIONREQUEST = {"chart": string};
export type CLUSTER_HELM_CHART_VERSIONS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMCHARTVERSIONREQUEST_MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINFO = {"data": CLUSTER_HELM_CHART_VERSIONS_RESPONSE__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINFO[],"message": string,"status": string};
export type CLUSTER_HELM_CHART_VERSIONS_RESPONSE__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINFO = {"app_version": string,"description": string,"name": string,"version": string};
export type CLUSTER_HELM_RELEASE_GET_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEGETREQUEST = {"getFormat": string,"namespace": string,"release": string};
export type CLUSTER_HELM_RELEASE_GET_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEGETREQUEST_STRING = {"data": string,"message": string,"status": string};
export type CLUSTER_HELM_RELEASE_GET_WORKLOADS_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEGETWORKLOADSREQUEST = {"namespace": string,"release": string,"whitelist": CLUSTER_HELM_RELEASE_GET_WORKLOADS_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR|undefined[]};
export type CLUSTER_HELM_RELEASE_GET_WORKLOADS_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR = {"apiVersion": string,"kind": string,"namespaced": boolean,"plural": string};
export type CLUSTER_HELM_RELEASE_GET_WORKLOADS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"Object": Record<string, any>};
export type CLUSTER_HELM_RELEASE_GET_WORKLOADS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEGETWORKLOADSREQUEST_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"data": CLUSTER_HELM_RELEASE_GET_WORKLOADS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED[],"message": string,"status": string};
export type CLUSTER_HELM_RELEASE_HISTORY_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEHISTORYREQUEST = {"namespace": string,"release": string};
export type CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_COMMON_FILE = {"data": number[],"modtime": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__TIME_TIME,"name": string};
export type CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_CHART = {"files": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_COMMON_FILE|undefined[],"lock": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_LOCK|undefined,"metadata": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_METADATA|undefined,"modtime": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__TIME_TIME,"schema": number[],"schemamodtime": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__TIME_TIME,"templates": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_COMMON_FILE|undefined[],"values": Record<string, any>};
export type CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_DEPENDENCY = {"alias": string,"condition": string,"enabled": boolean,"import-values": any[],"name": string,"repository": string,"tags": string[],"version": string};
export type CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_LOCK = {"dependencies": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_DEPENDENCY|undefined[],"digest": string,"generated": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__TIME_TIME};
export type CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_MAINTAINER = {"email": string,"name": string,"url": string};
export type CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_METADATA = {"annotations": Record<string, string>,"apiVersion": string,"appVersion": string,"condition": string,"dependencies": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_DEPENDENCY|undefined[],"deprecated": boolean,"description": string,"home": string,"icon": string,"keywords": string[],"kubeVersion": string,"maintainers": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_MAINTAINER|undefined[],"name": string,"sources": string[],"tags": string,"type": string,"version": string};
export type CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_RELEASE_V1_HOOK = {"delete_policies": string[],"events": string[],"kind": string,"last_run": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_RELEASE_V1_HOOKEXECUTION,"manifest": string,"name": string,"output_log_policies": string[],"path": string,"weight": number};
export type CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_RELEASE_V1_HOOKEXECUTION = {"completed_at": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__TIME_TIME,"phase": string,"started_at": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__TIME_TIME};
export type CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_RELEASE_V1_INFO = {"deleted": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__TIME_TIME,"description": string,"first_deployed": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__TIME_TIME,"last_deployed": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__TIME_TIME,"notes": string,"resources": Record<string, any[]>,"status": string};
export type CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_RELEASE_V1_RELEASE = {"apply_method": string,"chart": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_CHART|undefined,"config": Record<string, any>,"hooks": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_RELEASE_V1_HOOK|undefined[],"info": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_RELEASE_V1_INFO|undefined,"manifest": string,"name": string,"namespace": string,"version": number};
export type CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEHISTORYREQUEST_HELM_SH_HELM_V4_PKG_RELEASE_V1_RELEASE = {"data": CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__HELM_SH_HELM_V4_PKG_RELEASE_V1_RELEASE[],"message": string,"status": string};
export type CLUSTER_HELM_RELEASE_HISTORY_RESPONSE__TIME_TIME = {};
export type CLUSTER_HELM_RELEASE_LINK_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASELINKREQUEST = {"namespace": string,"releaseName": string,"repoName": string};
export type CLUSTER_HELM_RELEASE_LINK_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASELINKREQUEST_STRING = {"data": string,"message": string,"status": string};
export type CLUSTER_HELM_RELEASE_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASELISTREQUEST = {"namespace": string};
export type CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_COMMON_FILE = {"data": number[],"modtime": CLUSTER_HELM_RELEASE_LIST_RESPONSE__TIME_TIME,"name": string};
export type CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_CHART = {"files": CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_COMMON_FILE|undefined[],"lock": CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_LOCK|undefined,"metadata": CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_METADATA|undefined,"modtime": CLUSTER_HELM_RELEASE_LIST_RESPONSE__TIME_TIME,"schema": number[],"schemamodtime": CLUSTER_HELM_RELEASE_LIST_RESPONSE__TIME_TIME,"templates": CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_COMMON_FILE|undefined[],"values": Record<string, any>};
export type CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_DEPENDENCY = {"alias": string,"condition": string,"enabled": boolean,"import-values": any[],"name": string,"repository": string,"tags": string[],"version": string};
export type CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_LOCK = {"dependencies": CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_DEPENDENCY|undefined[],"digest": string,"generated": CLUSTER_HELM_RELEASE_LIST_RESPONSE__TIME_TIME};
export type CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_MAINTAINER = {"email": string,"name": string,"url": string};
export type CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_METADATA = {"annotations": Record<string, string>,"apiVersion": string,"appVersion": string,"condition": string,"dependencies": CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_DEPENDENCY|undefined[],"deprecated": boolean,"description": string,"home": string,"icon": string,"keywords": string[],"kubeVersion": string,"maintainers": CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_MAINTAINER|undefined[],"name": string,"sources": string[],"tags": string,"type": string,"version": string};
export type CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_RELEASE_V1_HOOK = {"delete_policies": string[],"events": string[],"kind": string,"last_run": CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_RELEASE_V1_HOOKEXECUTION,"manifest": string,"name": string,"output_log_policies": string[],"path": string,"weight": number};
export type CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_RELEASE_V1_HOOKEXECUTION = {"completed_at": CLUSTER_HELM_RELEASE_LIST_RESPONSE__TIME_TIME,"phase": string,"started_at": CLUSTER_HELM_RELEASE_LIST_RESPONSE__TIME_TIME};
export type CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_RELEASE_V1_INFO = {"deleted": CLUSTER_HELM_RELEASE_LIST_RESPONSE__TIME_TIME,"description": string,"first_deployed": CLUSTER_HELM_RELEASE_LIST_RESPONSE__TIME_TIME,"last_deployed": CLUSTER_HELM_RELEASE_LIST_RESPONSE__TIME_TIME,"notes": string,"resources": Record<string, any[]>,"status": string};
export type CLUSTER_HELM_RELEASE_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASELISTREQUEST_MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASE = {"data": CLUSTER_HELM_RELEASE_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASE|undefined[],"message": string,"status": string};
export type CLUSTER_HELM_RELEASE_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASE = {"chart": CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_CHART_V2_CHART|undefined,"config": Record<string, any>,"hooks": CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_RELEASE_V1_HOOK|undefined[],"info": CLUSTER_HELM_RELEASE_LIST_RESPONSE__HELM_SH_HELM_V4_PKG_RELEASE_V1_INFO|undefined,"manifest": string,"name": string,"namespace": string,"repoName": string,"version": number};
export type CLUSTER_HELM_RELEASE_LIST_RESPONSE__TIME_TIME = {};
export type CLUSTER_HELM_RELEASE_ROLLBACK_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEROLLBACKREQUEST = {"namespace": string,"release": string,"revision": number};
export type CLUSTER_HELM_RELEASE_ROLLBACK_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEROLLBACKREQUEST_STRING = {"data": string,"message": string,"status": string};
export type CLUSTER_HELM_RELEASE_STATUS_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASESTATUSREQUEST = {"namespace": string,"release": string};
export type CLUSTER_HELM_RELEASE_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASESTATUSREQUEST_MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASESTATUSINFO = {"data": CLUSTER_HELM_RELEASE_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASESTATUSINFO|undefined,"message": string,"status": string};
export type CLUSTER_HELM_RELEASE_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASESTATUSINFO = {"chart": string,"lastDeployed": CLUSTER_HELM_RELEASE_STATUS_RESPONSE__TIME_TIME,"name": string,"namespace": string,"status": string,"version": number};
export type CLUSTER_HELM_RELEASE_STATUS_RESPONSE__TIME_TIME = {};
export type CLUSTER_HELM_RELEASE_UNINSTALL_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEUNINSTALLREQUEST = {"dryRun": boolean,"namespace": string,"release": string};
export type CLUSTER_HELM_RELEASE_UNINSTALL_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMRELEASEUNINSTALLREQUEST_STRING = {"data": string,"message": string,"status": string};
export type CLUSTER_HELM_RELEASE_UPGRADE_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINSTALLUPGRADEREQUEST = {"chart": string,"dryRun": boolean,"namespace": string,"release": string,"values": string,"version": string};
export type CLUSTER_HELM_RELEASE_UPGRADE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMCHARTINSTALLUPGRADEREQUEST_STRING = {"data": string,"message": string,"status": string};
export type CLUSTER_HELM_REPO_ADD_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMREPOADDREQUEST = {"insecureSkipTLSverify": boolean,"name": string,"passCredentialsAll": boolean,"password": string,"url": string,"username": string};
export type CLUSTER_HELM_REPO_ADD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMREPOADDREQUEST_STRING = {"data": string,"message": string,"status": string};
export type CLUSTER_HELM_REPO_LIST_REQUEST__ANON_STRUCT_0 = {};
export type CLUSTER_HELM_REPO_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_HELM_HELMENTRYWITHOUTPASSWORD = {"data": CLUSTER_HELM_REPO_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_HELM_HELMENTRYWITHOUTPASSWORD|undefined[],"message": string,"status": string};
export type CLUSTER_HELM_REPO_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_HELM_HELMENTRYWITHOUTPASSWORD = {"insecure_skip_tls_verify": boolean,"name": string,"pass_credentials_all": boolean,"url": string};
export type CLUSTER_HELM_REPO_PATCH_REQUEST__MOGENIUS_OPERATOR_SRC_HELM_HELMREPOPATCHREQUEST = {"insecureSkipTLSverify": boolean,"name": string,"newName": string,"passCredentialsAll": boolean,"password": string,"url": string,"username": string};
export type CLUSTER_HELM_REPO_PATCH_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_HELM_HELMREPOPATCHREQUEST_STRING = {"data": string,"message": string,"status": string};
export type CLUSTER_HELM_REPO_UPDATE_REQUEST__ANON_STRUCT_0 = {};
export type CLUSTER_HELM_REPO_UPDATE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_HELM_HELMENTRYSTATUS = {"data": CLUSTER_HELM_REPO_UPDATE_RESPONSE__MOGENIUS_OPERATOR_SRC_HELM_HELMENTRYSTATUS[],"message": string,"status": string};
export type CLUSTER_HELM_REPO_UPDATE_RESPONSE__MOGENIUS_OPERATOR_SRC_HELM_HELMENTRYSTATUS = {"entry": CLUSTER_HELM_REPO_UPDATE_RESPONSE__MOGENIUS_OPERATOR_SRC_HELM_HELMENTRYWITHOUTPASSWORD|undefined,"message": string,"status": string};
export type CLUSTER_HELM_REPO_UPDATE_RESPONSE__MOGENIUS_OPERATOR_SRC_HELM_HELMENTRYWITHOUTPASSWORD = {"insecure_skip_tls_verify": boolean,"name": string,"pass_credentials_all": boolean,"url": string};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_REQUEST__MOGENIUS_OPERATOR_SRC_SERVICES_CLUSTERLISTWORKLOADS = {"labelSelector": string,"namespace": string,"prefix": string};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_MODIFYVOLUMESTATUS = {"status": string,"targetVolumeAttributesClassName": string};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_PERSISTENTVOLUMECLAIM = {"TypeMeta": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TYPEMETA,"metadata": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OBJECTMETA,"spec": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_PERSISTENTVOLUMECLAIMSPEC,"status": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_PERSISTENTVOLUMECLAIMSTATUS};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_PERSISTENTVOLUMECLAIMCONDITION = {"lastProbeTime": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME,"lastTransitionTime": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME,"message": string,"reason": string,"status": string,"type": string};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_PERSISTENTVOLUMECLAIMSPEC = {"accessModes": string[],"dataSource": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_TYPEDLOCALOBJECTREFERENCE|undefined,"dataSourceRef": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_TYPEDOBJECTREFERENCE|undefined,"resources": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_VOLUMERESOURCEREQUIREMENTS,"selector": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_LABELSELECTOR|undefined,"storageClassName": string|undefined,"volumeAttributesClassName": string|undefined,"volumeMode": string|undefined,"volumeName": string};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_PERSISTENTVOLUMECLAIMSTATUS = {"accessModes": string[],"allocatedResourceStatuses": Record<string, string>,"allocatedResources": Record<string, CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_API_RESOURCE_QUANTITY>,"capacity": Record<string, CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_API_RESOURCE_QUANTITY>,"conditions": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_PERSISTENTVOLUMECLAIMCONDITION[],"currentVolumeAttributesClassName": string|undefined,"modifyVolumeStatus": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_MODIFYVOLUMESTATUS|undefined,"phase": string};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_TYPEDLOCALOBJECTREFERENCE = {"apiGroup": string|undefined,"kind": string,"name": string};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_TYPEDOBJECTREFERENCE = {"apiGroup": string|undefined,"kind": string,"name": string,"namespace": string|undefined};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_VOLUMERESOURCEREQUIREMENTS = {"limits": Record<string, CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_API_RESOURCE_QUANTITY>,"requests": Record<string, CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_API_RESOURCE_QUANTITY>};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_API_RESOURCE_QUANTITY = {"Format": string};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_FIELDSV1 = {};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_LABELSELECTOR = {"matchExpressions": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_LABELSELECTORREQUIREMENT[],"matchLabels": Record<string, string>};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_LABELSELECTORREQUIREMENT = {"key": string,"operator": string,"values": string[]};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MANAGEDFIELDSENTRY = {"apiVersion": string,"fieldsType": string,"fieldsV1": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_FIELDSV1|undefined,"manager": string,"operation": string,"subresource": string,"time": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME|undefined};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OBJECTMETA = {"annotations": Record<string, string>,"creationTimestamp": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME,"deletionGracePeriodSeconds": number|undefined,"deletionTimestamp": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME|undefined,"finalizers": string[],"generateName": string,"generation": number,"labels": Record<string, string>,"managedFields": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MANAGEDFIELDSENTRY[],"name": string,"namespace": string,"ownerReferences": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OWNERREFERENCE[],"resourceVersion": string,"selfLink": string,"uid": string};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OWNERREFERENCE = {"apiVersion": string,"blockOwnerDeletion": boolean|undefined,"controller": boolean|undefined,"kind": string,"name": string,"uid": string};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME = {"Time": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__TIME_TIME};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TYPEMETA = {"apiVersion": string,"kind": string};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_SERVICES_CLUSTERLISTWORKLOADS_K8S_IO_API_CORE_V1_PERSISTENTVOLUMECLAIM = {"data": CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__K8S_IO_API_CORE_V1_PERSISTENTVOLUMECLAIM[],"message": string,"status": string};
export type CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE__TIME_TIME = {};
export type CLUSTER_MACHINE_STATS_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"nodes": string[]};
export type CLUSTER_MACHINE_STATS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST16_MOGENIUS_OPERATOR_SRC_STRUCTS_MACHINESTATS = {"data": CLUSTER_MACHINE_STATS_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_MACHINESTATS[],"message": string,"status": string};
export type CLUSTER_MACHINE_STATS_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_MACHINESTATS = {"btfSupport": boolean};
export type CLUSTER_RESOURCE_INFO_REQUEST__ANON_STRUCT_0 = {};
export type CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_CLUSTERRESOURCEINFO = {"cniConfig": CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_CNIDATA[],"country": CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_UTILS_COUNTRYDETAILS|undefined,"error": string[],"loadBalancerExternalIps": string[],"nodeStats": CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_DTOS_NODESTAT[],"provider": string};
export type CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_CORE_CLUSTERRESOURCEINFO3 = {"data": CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_CLUSTERRESOURCEINFO,"message": string,"status": string};
export type CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_DTOS_NODESTAT = {"architecture": string,"cpuInCores": number,"cpuInCoresLimited": number,"cpuInCoresRequested": number,"cpuInCoresUtilized": number,"ephemeralInBytes": number,"kubletVersion": string,"machineStats": CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_MACHINESTATS|undefined,"maschineId": string,"maxPods": number,"memoryInBytes": number,"memoryInBytesLimited": number,"memoryInBytesRequested": number,"memoryInBytesUtilized": number,"name": string,"osImage": string,"osKernelVersion": string,"osType": string,"totalPods": number};
export type CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_CNICAPABILITIES = {"bandwidth": boolean,"portMappings": boolean};
export type CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_CNIDATA = {"cniVersion": string,"name": string,"node": string,"plugins": CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_PLUGIN[]};
export type CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_CNIIPAM = {"type": string};
export type CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_CNIPOLICY = {"type": string};
export type CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_MACHINESTATS = {"btfSupport": boolean};
export type CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_PLUGIN = {"capabilities": CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_CNICAPABILITIES|undefined,"datastore_type": string,"ipam": CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_CNIIPAM|undefined,"log_file_path": string,"log_level": string,"mtu": number,"nodename": string,"policy": CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_CNIPOLICY|undefined,"snat": boolean|undefined,"type": string};
export type CLUSTER_RESOURCE_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_UTILS_COUNTRYDETAILS = {"capitalCity": string,"capitalCityLat": number,"capitalCityLng": number,"code": string,"code3": string,"continent": string,"currency": string,"currencyName": string,"domainTld": string,"isActive": boolean,"isEuMember": boolean,"isoId": number,"languages": string[],"name": string,"phoneNumberPrefix": string,"taxPercent": number};
export type CREATE_GRANT_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"grantee": string,"name": string,"role": string,"targetName": string,"targetType": string};
export type CREATE_GRANT_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST29_STRING = {"data": string,"message": string,"status": string};
export type CREATE_NEW_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR = {"apiVersion": string,"kind": string,"namespaced": boolean,"plural": string};
export type CREATE_NEW_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_WORKLOADCHANGEREQUEST = {"ResourceDescriptor": CREATE_NEW_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR,"namespace": string,"yamlData": string};
export type CREATE_NEW_WORKLOAD_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"Object": Record<string, any>};
export type CREATE_NEW_WORKLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_UTILS_WORKLOADCHANGEREQUEST_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"data": CREATE_NEW_WORKLOAD_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED|undefined,"message": string,"status": string};
export type CREATE_USER_REQUEST__K8S_IO_API_RBAC_V1_SUBJECT = {"apiGroup": string,"kind": string,"name": string,"namespace": string};
export type CREATE_USER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"email": string,"firstName": string,"lastName": string,"name": string,"subject": CREATE_USER_REQUEST__K8S_IO_API_RBAC_V1_SUBJECT|undefined};
export type CREATE_USER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST24_STRING = {"data": string,"message": string,"status": string};
export type CREATE_WORKSPACE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"displayName": string,"name": string,"resources": CREATE_WORKSPACE_REQUEST__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_WORKSPACERESOURCEIDENTIFIER[]};
export type CREATE_WORKSPACE_REQUEST__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_WORKSPACERESOURCEIDENTIFIER = {"id": string,"namespace": string,"type": string};
export type CREATE_WORKSPACE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST18_STRING = {"data": string,"message": string,"status": string};
export type DELETE_GRANT_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"name": string};
export type DELETE_GRANT_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST32_STRING = {"data": string,"message": string,"status": string};
export type DELETE_USER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"name": string};
export type DELETE_USER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST27_STRING = {"data": string,"message": string,"status": string};
export type DELETE_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR = {"apiVersion": string,"kind": string,"namespaced": boolean,"plural": string};
export type DELETE_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST = {"ResourceDescriptor": DELETE_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR,"namespace": string,"resourceName": string};
export type DELETE_WORKLOAD_RESPONSE__ANON_STRUCT_1 = {};
export type DELETE_WORKLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID = {"data": DELETE_WORKLOAD_RESPONSE__ANON_STRUCT_1|undefined,"message": string,"status": string};
export type DELETE_WORKSPACE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"name": string};
export type DELETE_WORKSPACE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST22_STRING = {"data": string,"message": string,"status": string};
export type DESCRIBE_REQUEST__ANON_STRUCT_0 = {};
export type DESCRIBE_RESPONSE__ANON_STRUCT_2 = {"buildType": string,"version": DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_VERSION_VERSION};
export type DESCRIBE_RESPONSE__ANON_STRUCT_4 = {};
export type DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_PATTERNCONFIG = {"deprecated": boolean,"deprecatedMessage": string,"legacyResponseLayout": boolean,"needsUser": boolean,"requestSchema": DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_SCHEMA_SCHEMA|undefined,"responseSchema": DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_SCHEMA_SCHEMA|undefined};
export type DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESPONSE = {"buildInfo": DESCRIBE_RESPONSE__ANON_STRUCT_2,"features": DESCRIBE_RESPONSE__ANON_STRUCT_4,"patterns": Record<string, DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_PATTERNCONFIG>};
export type DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_CORE_RESPONSE2 = {"data": DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESPONSE,"message": string,"status": string};
export type DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_SCHEMA_SCHEMA = {"structs": Record<string, DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_SCHEMA_STRUCTLAYOUT>,"typeInfo": DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_SCHEMA_TYPEINFO|undefined};
export type DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_SCHEMA_STRUCTLAYOUT = {"name": string,"properties": Record<string, DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_SCHEMA_TYPEINFO|undefined>};
export type DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_SCHEMA_TYPEINFO = {"elementType": DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_SCHEMA_TYPEINFO|undefined,"keyType": DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_SCHEMA_TYPEINFO|undefined,"pointer": boolean,"structRef": string,"type": string,"valueType": DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_SCHEMA_TYPEINFO|undefined};
export type DESCRIBE_RESPONSE__MOGENIUS_OPERATOR_SRC_VERSION_VERSION = {"arch": string,"branch": string,"buildTimestamp": string,"gitCommitHash": string,"os": string,"version": string};
export type DESCRIBE_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR = {"apiVersion": string,"kind": string,"namespaced": boolean,"plural": string};
export type DESCRIBE_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST = {"ResourceDescriptor": DESCRIBE_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR,"namespace": string,"resourceName": string};
export type DESCRIBE_WORKLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST_STRING = {"data": string,"message": string,"status": string};
export type FILES_CHMOD_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"file": FILES_CHMOD_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO,"mode": string};
export type FILES_CHMOD_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO = {"path": string,"volumeName": string,"volumeNamespace": string};
export type FILES_CHMOD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST13_BOOL = {"data": boolean,"message": string,"status": string};
export type FILES_CHOWN_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"file": FILES_CHOWN_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO,"gid": string,"uid": string};
export type FILES_CHOWN_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO = {"path": string,"volumeName": string,"volumeNamespace": string};
export type FILES_CHOWN_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST12_BOOL = {"data": boolean,"message": string,"status": string};
export type FILES_CREATE_FOLDER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"folder": FILES_CREATE_FOLDER_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO};
export type FILES_CREATE_FOLDER_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO = {"path": string,"volumeName": string,"volumeNamespace": string};
export type FILES_CREATE_FOLDER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST10_BOOL = {"data": boolean,"message": string,"status": string};
export type FILES_DELETE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"file": FILES_DELETE_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO};
export type FILES_DELETE_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO = {"path": string,"volumeName": string,"volumeNamespace": string};
export type FILES_DELETE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST14_BOOL = {"data": boolean,"message": string,"status": string};
export type FILES_DOWNLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"file": FILES_DOWNLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO,"postTo": string};
export type FILES_DOWNLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO = {"path": string,"volumeName": string,"volumeNamespace": string};
export type FILES_DOWNLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST15_MOGENIUS_OPERATOR_SRC_SERVICES_FILESDOWNLOADRESPONSE = {"data": FILES_DOWNLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_SERVICES_FILESDOWNLOADRESPONSE,"message": string,"status": string};
export type FILES_DOWNLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_SERVICES_FILESDOWNLOADRESPONSE = {"error": string,"sizeInBytes": number};
export type FILES_INFO_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO = {"path": string,"volumeName": string,"volumeNamespace": string};
export type FILES_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO_MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEDTO = {"data": FILES_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEDTO,"message": string,"status": string};
export type FILES_INFO_RESPONSE__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEDTO = {"contentType": string,"createdAt": string,"extension": string,"hash": string,"mimeType": string,"mode": string,"modifiedAt": string,"name": string,"relativePath": string,"size": string,"sizeInBytes": number,"type": string,"uid_gid": string};
export type FILES_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"folder": FILES_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO};
export type FILES_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO = {"path": string,"volumeName": string,"volumeNamespace": string};
export type FILES_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST9_MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEDTO = {"data": FILES_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEDTO[],"message": string,"status": string};
export type FILES_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEDTO = {"contentType": string,"createdAt": string,"extension": string,"hash": string,"mimeType": string,"mode": string,"modifiedAt": string,"name": string,"relativePath": string,"size": string,"sizeInBytes": number,"type": string,"uid_gid": string};
export type FILES_RENAME_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"file": FILES_RENAME_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO,"newName": string};
export type FILES_RENAME_REQUEST__MOGENIUS_OPERATOR_SRC_DTOS_PERSISTENTFILEREQUESTDTO = {"path": string,"volumeName": string,"volumeNamespace": string};
export type FILES_RENAME_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST11_BOOL = {"data": boolean,"message": string,"status": string};
export type GET_GRANT_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"name": string};
export type GET_GRANT_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_FIELDSV1 = {};
export type GET_GRANT_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MANAGEDFIELDSENTRY = {"apiVersion": string,"fieldsType": string,"fieldsV1": GET_GRANT_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_FIELDSV1|undefined,"manager": string,"operation": string,"subresource": string,"time": GET_GRANT_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME|undefined};
export type GET_GRANT_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OBJECTMETA = {"annotations": Record<string, string>,"creationTimestamp": GET_GRANT_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME,"deletionGracePeriodSeconds": number|undefined,"deletionTimestamp": GET_GRANT_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME|undefined,"finalizers": string[],"generateName": string,"generation": number,"labels": Record<string, string>,"managedFields": GET_GRANT_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MANAGEDFIELDSENTRY[],"name": string,"namespace": string,"ownerReferences": GET_GRANT_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OWNERREFERENCE[],"resourceVersion": string,"selfLink": string,"uid": string};
export type GET_GRANT_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OWNERREFERENCE = {"apiVersion": string,"blockOwnerDeletion": boolean|undefined,"controller": boolean|undefined,"kind": string,"name": string,"uid": string};
export type GET_GRANT_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME = {"Time": GET_GRANT_RESPONSE__TIME_TIME};
export type GET_GRANT_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TYPEMETA = {"apiVersion": string,"kind": string};
export type GET_GRANT_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST30_MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANT = {"data": GET_GRANT_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANT|undefined,"message": string,"status": string};
export type GET_GRANT_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANT = {"TypeMeta": GET_GRANT_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TYPEMETA,"metadata": GET_GRANT_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OBJECTMETA,"spec": GET_GRANT_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANTSPEC,"status": GET_GRANT_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANTSTATUS};
export type GET_GRANT_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANTSPEC = {"grantee": string,"role": string,"targetName": string,"targetType": string};
export type GET_GRANT_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANTSTATUS = {};
export type GET_GRANT_RESPONSE__TIME_TIME = {};
export type GET_GRANTS_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"targetName": string|undefined,"targetType": string|undefined};
export type GET_GRANTS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_FIELDSV1 = {};
export type GET_GRANTS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MANAGEDFIELDSENTRY = {"apiVersion": string,"fieldsType": string,"fieldsV1": GET_GRANTS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_FIELDSV1|undefined,"manager": string,"operation": string,"subresource": string,"time": GET_GRANTS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME|undefined};
export type GET_GRANTS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OBJECTMETA = {"annotations": Record<string, string>,"creationTimestamp": GET_GRANTS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME,"deletionGracePeriodSeconds": number|undefined,"deletionTimestamp": GET_GRANTS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME|undefined,"finalizers": string[],"generateName": string,"generation": number,"labels": Record<string, string>,"managedFields": GET_GRANTS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MANAGEDFIELDSENTRY[],"name": string,"namespace": string,"ownerReferences": GET_GRANTS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OWNERREFERENCE[],"resourceVersion": string,"selfLink": string,"uid": string};
export type GET_GRANTS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OWNERREFERENCE = {"apiVersion": string,"blockOwnerDeletion": boolean|undefined,"controller": boolean|undefined,"kind": string,"name": string,"uid": string};
export type GET_GRANTS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME = {"Time": GET_GRANTS_RESPONSE__TIME_TIME};
export type GET_GRANTS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TYPEMETA = {"apiVersion": string,"kind": string};
export type GET_GRANTS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST28_MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANT = {"data": GET_GRANTS_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANT[],"message": string,"status": string};
export type GET_GRANTS_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANT = {"TypeMeta": GET_GRANTS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TYPEMETA,"metadata": GET_GRANTS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OBJECTMETA,"spec": GET_GRANTS_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANTSPEC,"status": GET_GRANTS_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANTSTATUS};
export type GET_GRANTS_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANTSPEC = {"grantee": string,"role": string,"targetName": string,"targetType": string};
export type GET_GRANTS_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_GRANTSTATUS = {};
export type GET_GRANTS_RESPONSE__TIME_TIME = {};
export type GET_LABELED_WORKLOAD_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_KUBERNETES_GETUNSTRUCTUREDLABELEDRESOURCELISTREQUEST = {"blacklist": GET_LABELED_WORKLOAD_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR|undefined[],"label": string,"whitelist": GET_LABELED_WORKLOAD_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR|undefined[]};
export type GET_LABELED_WORKLOAD_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR = {"apiVersion": string,"kind": string,"namespaced": boolean,"plural": string};
export type GET_LABELED_WORKLOAD_LIST_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"Object": Record<string, any>};
export type GET_LABELED_WORKLOAD_LIST_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTUREDLIST = {"Object": Record<string, any>,"items": GET_LABELED_WORKLOAD_LIST_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED[]};
export type GET_LABELED_WORKLOAD_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_KUBERNETES_GETUNSTRUCTUREDLABELEDRESOURCELISTREQUEST_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTUREDLIST = {"data": GET_LABELED_WORKLOAD_LIST_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTUREDLIST,"message": string,"status": string};
export type GET_NAMESPACE_WORKLOAD_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_KUBERNETES_GETUNSTRUCTUREDNAMESPACERESOURCELISTREQUEST = {"blacklist": GET_NAMESPACE_WORKLOAD_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR|undefined[],"namespace": string,"whitelist": GET_NAMESPACE_WORKLOAD_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR|undefined[]};
export type GET_NAMESPACE_WORKLOAD_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR = {"apiVersion": string,"kind": string,"namespaced": boolean,"plural": string};
export type GET_NAMESPACE_WORKLOAD_LIST_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"Object": Record<string, any>};
export type GET_NAMESPACE_WORKLOAD_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_KUBERNETES_GETUNSTRUCTUREDNAMESPACERESOURCELISTREQUEST_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"data": GET_NAMESPACE_WORKLOAD_LIST_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED[],"message": string,"status": string};
export type GET_NODES_METRICS_REQUEST__ANON_STRUCT_0 = {};
export type GET_NODES_METRICS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_NODEMETRICS = {"cpu": Record<string, any>,"memory": Record<string, any>,"nodeName": string,"traffic": GET_NODES_METRICS_RESPONSE__MOGENIUS_OPERATOR_SRC_NETWORKMONITOR_PODNETWORKSTATS[]};
export type GET_NODES_METRICS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESPONSE = {"nodes": GET_NODES_METRICS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_NODEMETRICS[]};
export type GET_NODES_METRICS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_CORE_RESPONSE38 = {"data": GET_NODES_METRICS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESPONSE,"message": string,"status": string};
export type GET_NODES_METRICS_RESPONSE__MOGENIUS_OPERATOR_SRC_NETWORKMONITOR_PODNETWORKSTATS = {"createdAt": GET_NODES_METRICS_RESPONSE__TIME_TIME,"namespace": string,"pod": string,"receivedBytes": number,"receivedPackets": number,"receivedStartBytes": number,"transmitBytes": number,"transmitPackets": number,"transmitStartBytes": number};
export type GET_NODES_METRICS_RESPONSE__TIME_TIME = {};
export type GET_USER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"name": string};
export type GET_USER_RESPONSE__K8S_IO_API_RBAC_V1_SUBJECT = {"apiGroup": string,"kind": string,"name": string,"namespace": string};
export type GET_USER_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_FIELDSV1 = {};
export type GET_USER_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MANAGEDFIELDSENTRY = {"apiVersion": string,"fieldsType": string,"fieldsV1": GET_USER_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_FIELDSV1|undefined,"manager": string,"operation": string,"subresource": string,"time": GET_USER_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME|undefined};
export type GET_USER_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OBJECTMETA = {"annotations": Record<string, string>,"creationTimestamp": GET_USER_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME,"deletionGracePeriodSeconds": number|undefined,"deletionTimestamp": GET_USER_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME|undefined,"finalizers": string[],"generateName": string,"generation": number,"labels": Record<string, string>,"managedFields": GET_USER_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MANAGEDFIELDSENTRY[],"name": string,"namespace": string,"ownerReferences": GET_USER_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OWNERREFERENCE[],"resourceVersion": string,"selfLink": string,"uid": string};
export type GET_USER_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OWNERREFERENCE = {"apiVersion": string,"blockOwnerDeletion": boolean|undefined,"controller": boolean|undefined,"kind": string,"name": string,"uid": string};
export type GET_USER_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME = {"Time": GET_USER_RESPONSE__TIME_TIME};
export type GET_USER_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TYPEMETA = {"apiVersion": string,"kind": string};
export type GET_USER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST25_MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USER = {"data": GET_USER_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USER|undefined,"message": string,"status": string};
export type GET_USER_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USER = {"TypeMeta": GET_USER_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TYPEMETA,"metadata": GET_USER_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OBJECTMETA,"spec": GET_USER_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USERSPEC,"status": GET_USER_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USERSTATUS};
export type GET_USER_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USERSPEC = {"email": string,"firstName": string,"lastName": string,"subject": GET_USER_RESPONSE__K8S_IO_API_RBAC_V1_SUBJECT|undefined};
export type GET_USER_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USERSTATUS = {};
export type GET_USER_RESPONSE__TIME_TIME = {};
export type GET_USERS_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"email": string|undefined};
export type GET_USERS_RESPONSE__K8S_IO_API_RBAC_V1_SUBJECT = {"apiGroup": string,"kind": string,"name": string,"namespace": string};
export type GET_USERS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_FIELDSV1 = {};
export type GET_USERS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MANAGEDFIELDSENTRY = {"apiVersion": string,"fieldsType": string,"fieldsV1": GET_USERS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_FIELDSV1|undefined,"manager": string,"operation": string,"subresource": string,"time": GET_USERS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME|undefined};
export type GET_USERS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OBJECTMETA = {"annotations": Record<string, string>,"creationTimestamp": GET_USERS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME,"deletionGracePeriodSeconds": number|undefined,"deletionTimestamp": GET_USERS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME|undefined,"finalizers": string[],"generateName": string,"generation": number,"labels": Record<string, string>,"managedFields": GET_USERS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MANAGEDFIELDSENTRY[],"name": string,"namespace": string,"ownerReferences": GET_USERS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OWNERREFERENCE[],"resourceVersion": string,"selfLink": string,"uid": string};
export type GET_USERS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OWNERREFERENCE = {"apiVersion": string,"blockOwnerDeletion": boolean|undefined,"controller": boolean|undefined,"kind": string,"name": string,"uid": string};
export type GET_USERS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME = {"Time": GET_USERS_RESPONSE__TIME_TIME};
export type GET_USERS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TYPEMETA = {"apiVersion": string,"kind": string};
export type GET_USERS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST23_MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USER = {"data": GET_USERS_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USER[],"message": string,"status": string};
export type GET_USERS_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USER = {"TypeMeta": GET_USERS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TYPEMETA,"metadata": GET_USERS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OBJECTMETA,"spec": GET_USERS_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USERSPEC,"status": GET_USERS_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USERSTATUS};
export type GET_USERS_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USERSPEC = {"email": string,"firstName": string,"lastName": string,"subject": GET_USERS_RESPONSE__K8S_IO_API_RBAC_V1_SUBJECT|undefined};
export type GET_USERS_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_USERSTATUS = {};
export type GET_USERS_RESPONSE__TIME_TIME = {};
export type GET_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR = {"apiVersion": string,"kind": string,"namespaced": boolean,"plural": string};
export type GET_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST = {"ResourceDescriptor": GET_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR,"namespace": string,"resourceName": string};
export type GET_WORKLOAD_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"Object": Record<string, any>};
export type GET_WORKLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"data": GET_WORKLOAD_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED|undefined,"message": string,"status": string};
export type GET_WORKLOAD_EXAMPLE_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR = {"apiVersion": string,"kind": string,"namespaced": boolean,"plural": string};
export type GET_WORKLOAD_EXAMPLE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR_STRING = {"data": string,"message": string,"status": string};
export type GET_WORKLOAD_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"apiVersion": string,"kind": string,"namespace": string|undefined,"plural": string,"withData": boolean|undefined};
export type GET_WORKLOAD_LIST_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"Object": Record<string, any>};
export type GET_WORKLOAD_LIST_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTUREDLIST = {"Object": Record<string, any>,"items": GET_WORKLOAD_LIST_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED[]};
export type GET_WORKLOAD_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST17_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTUREDLIST = {"data": GET_WORKLOAD_LIST_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTUREDLIST,"message": string,"status": string};
export type GET_WORKLOAD_STATUS_REQUEST__MOGENIUS_OPERATOR_SRC_KUBERNETES_GETWORKLOADSTATUSHELMRELEASENAMEREQUEST = {"namespace": string,"release": string};
export type GET_WORKLOAD_STATUS_REQUEST__MOGENIUS_OPERATOR_SRC_KUBERNETES_GETWORKLOADSTATUSREQUEST = {"helmReleases": GET_WORKLOAD_STATUS_REQUEST__MOGENIUS_OPERATOR_SRC_KUBERNETES_GETWORKLOADSTATUSHELMRELEASENAMEREQUEST[]|undefined,"ignoreDependentResources": boolean|undefined,"namespaces": string[]|undefined,"resourceDescriptor": GET_WORKLOAD_STATUS_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR|undefined,"resourceNames": string[]|undefined};
export type GET_WORKLOAD_STATUS_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR = {"apiVersion": string,"kind": string,"namespaced": boolean,"plural": string};
export type GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_API_CORE_V1_EVENT = {"TypeMeta": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TYPEMETA,"action": string,"count": number,"eventTime": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MICROTIME,"firstTimestamp": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME,"involvedObject": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_API_CORE_V1_OBJECTREFERENCE,"lastTimestamp": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME,"message": string,"metadata": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OBJECTMETA,"reason": string,"related": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_API_CORE_V1_OBJECTREFERENCE|undefined,"reportingComponent": string,"reportingInstance": string,"series": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_API_CORE_V1_EVENTSERIES|undefined,"source": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_API_CORE_V1_EVENTSOURCE,"type": string};
export type GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_API_CORE_V1_EVENTSERIES = {"count": number,"lastObservedTime": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MICROTIME};
export type GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_API_CORE_V1_EVENTSOURCE = {"component": string,"host": string};
export type GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_API_CORE_V1_OBJECTREFERENCE = {"apiVersion": string,"fieldPath": string,"kind": string,"name": string,"namespace": string,"resourceVersion": string,"uid": string};
export type GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_FIELDSV1 = {};
export type GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MANAGEDFIELDSENTRY = {"apiVersion": string,"fieldsType": string,"fieldsV1": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_FIELDSV1|undefined,"manager": string,"operation": string,"subresource": string,"time": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME|undefined};
export type GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MICROTIME = {"Time": GET_WORKLOAD_STATUS_RESPONSE__TIME_TIME};
export type GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OBJECTMETA = {"annotations": Record<string, string>,"creationTimestamp": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME,"deletionGracePeriodSeconds": number|undefined,"deletionTimestamp": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME|undefined,"finalizers": string[],"generateName": string,"generation": number,"labels": Record<string, string>,"managedFields": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MANAGEDFIELDSENTRY[],"name": string,"namespace": string,"ownerReferences": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OWNERREFERENCE[],"resourceVersion": string,"selfLink": string,"uid": string};
export type GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OWNERREFERENCE = {"apiVersion": string,"blockOwnerDeletion": boolean|undefined,"controller": boolean|undefined,"kind": string,"name": string,"uid": string};
export type GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME = {"Time": GET_WORKLOAD_STATUS_RESPONSE__TIME_TIME};
export type GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TYPEMETA = {"apiVersion": string,"kind": string};
export type GET_WORKLOAD_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_KUBERNETES_GETWORKLOADSTATUSREQUEST_MOGENIUS_OPERATOR_SRC_KUBERNETES_WORKLOADSTATUSDTO = {"data": GET_WORKLOAD_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_KUBERNETES_WORKLOADSTATUSDTO[],"message": string,"status": string};
export type GET_WORKLOAD_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_KUBERNETES_WORKLOADSTATUSDTO = {"items": GET_WORKLOAD_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_KUBERNETES_WORKLOADSTATUSITEMDTO[]};
export type GET_WORKLOAD_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_KUBERNETES_WORKLOADSTATUSITEMDTO = {"apiVersion": string,"creationTimestamp": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME,"endpoints": any,"events": GET_WORKLOAD_STATUS_RESPONSE__K8S_IO_API_CORE_V1_EVENT[],"kind": string,"name": string,"namespace": string,"ownerReferences": any,"replicas": number|undefined,"specClusterIP": string,"specType": string,"status": any,"uid": string};
export type GET_WORKLOAD_STATUS_RESPONSE__TIME_TIME = {};
export type GET_WORKSPACE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"name": string,"namespace": string};
export type GET_WORKSPACE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME = {"Time": GET_WORKSPACE_RESPONSE__TIME_TIME};
export type GET_WORKSPACE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_GETWORKSPACERESULT = {"creationTimestamp": GET_WORKSPACE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME,"name": string,"resources": GET_WORKSPACE_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_WORKSPACERESOURCEIDENTIFIER[]};
export type GET_WORKSPACE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST19_MOGENIUS_OPERATOR_SRC_CORE_GETWORKSPACERESULT = {"data": GET_WORKSPACE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_GETWORKSPACERESULT|undefined,"message": string,"status": string};
export type GET_WORKSPACE_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_WORKSPACERESOURCEIDENTIFIER = {"id": string,"namespace": string,"type": string};
export type GET_WORKSPACE_RESPONSE__TIME_TIME = {};
export type GET_WORKSPACES_REQUEST__ANON_STRUCT_0 = {};
export type GET_WORKSPACES_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME = {"Time": GET_WORKSPACES_RESPONSE__TIME_TIME};
export type GET_WORKSPACES_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_GETWORKSPACERESULT = {"creationTimestamp": GET_WORKSPACES_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME,"name": string,"resources": GET_WORKSPACES_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_WORKSPACERESOURCEIDENTIFIER[]};
export type GET_WORKSPACES_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_CORE_GETWORKSPACERESULT = {"data": GET_WORKSPACES_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_GETWORKSPACERESULT[],"message": string,"status": string};
export type GET_WORKSPACES_RESPONSE__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_WORKSPACERESOURCEIDENTIFIER = {"id": string,"namespace": string,"type": string};
export type GET_WORKSPACES_RESPONSE__TIME_TIME = {};
export type GET_WORKSPACE_WORKLOADS_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"blacklist": GET_WORKSPACE_WORKLOADS_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR|undefined[],"namespaceWhitelist": string[],"whitelist": GET_WORKSPACE_WORKLOADS_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR|undefined[],"workspaceName": string};
export type GET_WORKSPACE_WORKLOADS_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR = {"apiVersion": string,"kind": string,"namespaced": boolean,"plural": string};
export type GET_WORKSPACE_WORKLOADS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"Object": Record<string, any>};
export type GET_WORKSPACE_WORKLOADS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST33_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"data": GET_WORKSPACE_WORKLOADS_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED[],"message": string,"status": string};
export type INSTALL_CERT_MANAGER_REQUEST__ANON_STRUCT_0 = {};
export type INSTALL_CERT_MANAGER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type INSTALL_CLUSTER_ISSUER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"email": string};
export type INSTALL_CLUSTER_ISSUER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST6_STRING = {"data": string,"message": string,"status": string};
export type INSTALL_INGRESS_CONTROLLER_TRAEFIK_REQUEST__ANON_STRUCT_0 = {};
export type INSTALL_INGRESS_CONTROLLER_TRAEFIK_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type INSTALL_KEPLER_REQUEST__ANON_STRUCT_0 = {};
export type INSTALL_KEPLER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type INSTALL_METALLB_REQUEST__ANON_STRUCT_0 = {};
export type INSTALL_METALLB_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type INSTALL_METRICS_SERVER_REQUEST__ANON_STRUCT_0 = {};
export type INSTALL_METRICS_SERVER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type LIST_ALL_RESOURCE_DESCRIPTORS_REQUEST__ANON_STRUCT_0 = {};
export type LIST_ALL_RESOURCE_DESCRIPTORS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR = {"data": LIST_ALL_RESOURCE_DESCRIPTORS_RESPONSE__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR[],"message": string,"status": string};
export type LIST_ALL_RESOURCE_DESCRIPTORS_RESPONSE__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR = {"apiVersion": string,"kind": string,"namespaced": boolean,"plural": string};
export type LIVE_STREAM_NODES_CPU_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST = {"channelId": string,"cmdType": string,"nodeName": string,"podName": string,"websocketHost": string,"websocketScheme": string,"workspace": string};
export type LIVE_STREAM_NODES_CPU_RESPONSE__ANON_STRUCT_1 = {};
export type LIVE_STREAM_NODES_CPU_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID = {"data": LIVE_STREAM_NODES_CPU_RESPONSE__ANON_STRUCT_1|undefined,"message": string,"status": string};
export type LIVE_STREAM_NODES_MEMORY_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST = {"channelId": string,"cmdType": string,"nodeName": string,"podName": string,"websocketHost": string,"websocketScheme": string,"workspace": string};
export type LIVE_STREAM_NODES_MEMORY_RESPONSE__ANON_STRUCT_1 = {};
export type LIVE_STREAM_NODES_MEMORY_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID = {"data": LIVE_STREAM_NODES_MEMORY_RESPONSE__ANON_STRUCT_1|undefined,"message": string,"status": string};
export type LIVE_STREAM_NODES_TRAFFIC_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST = {"channelId": string,"cmdType": string,"nodeName": string,"podName": string,"websocketHost": string,"websocketScheme": string,"workspace": string};
export type LIVE_STREAM_NODES_TRAFFIC_RESPONSE__ANON_STRUCT_1 = {};
export type LIVE_STREAM_NODES_TRAFFIC_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID = {"data": LIVE_STREAM_NODES_TRAFFIC_RESPONSE__ANON_STRUCT_1|undefined,"message": string,"status": string};
export type LIVE_STREAM_POD_CPU_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST = {"channelId": string,"cmdType": string,"nodeName": string,"podName": string,"websocketHost": string,"websocketScheme": string,"workspace": string};
export type LIVE_STREAM_POD_CPU_RESPONSE__ANON_STRUCT_1 = {};
export type LIVE_STREAM_POD_CPU_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID = {"data": LIVE_STREAM_POD_CPU_RESPONSE__ANON_STRUCT_1|undefined,"message": string,"status": string};
export type LIVE_STREAM_POD_MEMORY_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST = {"channelId": string,"cmdType": string,"nodeName": string,"podName": string,"websocketHost": string,"websocketScheme": string,"workspace": string};
export type LIVE_STREAM_POD_MEMORY_RESPONSE__ANON_STRUCT_1 = {};
export type LIVE_STREAM_POD_MEMORY_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID = {"data": LIVE_STREAM_POD_MEMORY_RESPONSE__ANON_STRUCT_1|undefined,"message": string,"status": string};
export type LIVE_STREAM_POD_TRAFFIC_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST = {"channelId": string,"cmdType": string,"nodeName": string,"podName": string,"websocketHost": string,"websocketScheme": string,"workspace": string};
export type LIVE_STREAM_POD_TRAFFIC_RESPONSE__ANON_STRUCT_1 = {};
export type LIVE_STREAM_POD_TRAFFIC_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID = {"data": LIVE_STREAM_POD_TRAFFIC_RESPONSE__ANON_STRUCT_1|undefined,"message": string,"status": string};
export type LIVE_STREAM_WORKSPACE_CPU_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST = {"channelId": string,"cmdType": string,"nodeName": string,"podName": string,"websocketHost": string,"websocketScheme": string,"workspace": string};
export type LIVE_STREAM_WORKSPACE_CPU_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_INTERFACE {} = {"data": any,"message": string,"status": string};
export type LIVE_STREAM_WORKSPACE_MEMORY_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST = {"channelId": string,"cmdType": string,"nodeName": string,"podName": string,"websocketHost": string,"websocketScheme": string,"workspace": string};
export type LIVE_STREAM_WORKSPACE_MEMORY_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_INTERFACE {} = {"data": any,"message": string,"status": string};
export type LIVE_STREAM_WORKSPACE_TRAFFIC_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST = {"channelId": string,"cmdType": string,"nodeName": string,"podName": string,"websocketHost": string,"websocketScheme": string,"workspace": string};
export type LIVE_STREAM_WORKSPACE_TRAFFIC_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST_INTERFACE {} = {"data": any,"message": string,"status": string};
export type PROMETHEUS_CHARTS_ADD_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDIS = {"controller": string,"namespace": string,"query": string,"queryName": string,"step": number};
export type PROMETHEUS_CHARTS_ADD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDIS_STRING = {"data": string|undefined,"message": string,"status": string};
export type PROMETHEUS_CHARTS_GET_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDIS = {"controller": string,"namespace": string,"query": string,"queryName": string,"step": number};
export type PROMETHEUS_CHARTS_GET_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSSTOREOBJECT = {"createdAt": PROMETHEUS_CHARTS_GET_RESPONSE__TIME_TIME,"query": string,"step": number};
export type PROMETHEUS_CHARTS_GET_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDIS_MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSSTOREOBJECT = {"data": PROMETHEUS_CHARTS_GET_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSSTOREOBJECT|undefined,"message": string,"status": string};
export type PROMETHEUS_CHARTS_GET_RESPONSE__TIME_TIME = {};
export type PROMETHEUS_CHARTS_LIST_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDISLIST = {"controller": string,"namespace": string};
export type PROMETHEUS_CHARTS_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSSTOREOBJECT = {"createdAt": PROMETHEUS_CHARTS_LIST_RESPONSE__TIME_TIME,"query": string,"step": number};
export type PROMETHEUS_CHARTS_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDISLIST_MAPSTRINGMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSSTOREOBJECT = {"data": Record<string, PROMETHEUS_CHARTS_LIST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSSTOREOBJECT>,"message": string,"status": string};
export type PROMETHEUS_CHARTS_LIST_RESPONSE__TIME_TIME = {};
export type PROMETHEUS_CHARTS_REMOVE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDIS = {"controller": string,"namespace": string,"query": string,"queryName": string,"step": number};
export type PROMETHEUS_CHARTS_REMOVE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUESTREDIS_STRING = {"data": string|undefined,"message": string,"status": string};
export type PROMETHEUS_IS_REACHABLE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUEST = {"prometheusPass": string,"prometheusToken": string,"prometheusUrl": string,"prometheusUser": string,"query": string,"step": number,"timeOffsetSeconds": number};
export type PROMETHEUS_IS_REACHABLE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUEST_BOOL = {"data": boolean,"message": string,"status": string};
export type PROMETHEUS_QUERY_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUEST = {"prometheusPass": string,"prometheusToken": string,"prometheusUrl": string,"prometheusUser": string,"query": string,"step": number,"timeOffsetSeconds": number};
export type PROMETHEUS_QUERY_RESPONSE__ANON_STRUCT_2 = {"result": any[],"resultType": string};
export type PROMETHEUS_QUERY_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSQUERYRESPONSE = {"data": PROMETHEUS_QUERY_RESPONSE__ANON_STRUCT_2,"error": string,"errorType": string,"status": string};
export type PROMETHEUS_QUERY_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUEST_MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSQUERYRESPONSE = {"data": PROMETHEUS_QUERY_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSQUERYRESPONSE|undefined,"message": string,"status": string};
export type PROMETHEUS_VALUES_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUEST = {"prometheusPass": string,"prometheusToken": string,"prometheusUrl": string,"prometheusUser": string,"query": string,"step": number,"timeOffsetSeconds": number};
export type PROMETHEUS_VALUES_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_PROMETHEUSREQUEST_STRING = {"data": string[],"message": string,"status": string};
export type SEALED_SECRET_CREATE_FROM_EXISTING_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"name": string,"namespace": string};
export type SEALED_SECRET_CREATE_FROM_EXISTING_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"Object": Record<string, any>};
export type SEALED_SECRET_CREATE_FROM_EXISTING_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST36_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"data": SEALED_SECRET_CREATE_FROM_EXISTING_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED|undefined,"message": string,"status": string};
export type SEALED_SECRET_GET_CERTIFICATE_REQUEST__ANON_STRUCT_0 = {};
export type SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_API_CORE_V1_SECRET = {"TypeMeta": SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TYPEMETA,"data": Record<string, number[]>,"immutable": boolean|undefined,"metadata": SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OBJECTMETA,"stringData": Record<string, string>,"type": string};
export type SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_FIELDSV1 = {};
export type SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MANAGEDFIELDSENTRY = {"apiVersion": string,"fieldsType": string,"fieldsV1": SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_FIELDSV1|undefined,"manager": string,"operation": string,"subresource": string,"time": SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME|undefined};
export type SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OBJECTMETA = {"annotations": Record<string, string>,"creationTimestamp": SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME,"deletionGracePeriodSeconds": number|undefined,"deletionTimestamp": SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME|undefined,"finalizers": string[],"generateName": string,"generation": number,"labels": Record<string, string>,"managedFields": SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_MANAGEDFIELDSENTRY[],"name": string,"namespace": string,"ownerReferences": SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OWNERREFERENCE[],"resourceVersion": string,"selfLink": string,"uid": string};
export type SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_OWNERREFERENCE = {"apiVersion": string,"blockOwnerDeletion": boolean|undefined,"controller": boolean|undefined,"kind": string,"name": string,"uid": string};
export type SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TIME = {"Time": SEALED_SECRET_GET_CERTIFICATE_RESPONSE__TIME_TIME};
export type SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_TYPEMETA = {"apiVersion": string,"kind": string};
export type SEALED_SECRET_GET_CERTIFICATE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_K8S_IO_API_CORE_V1_SECRET = {"data": SEALED_SECRET_GET_CERTIFICATE_RESPONSE__K8S_IO_API_CORE_V1_SECRET|undefined,"message": string,"status": string};
export type SEALED_SECRET_GET_CERTIFICATE_RESPONSE__TIME_TIME = {};
export type SERVICE_EXEC_SH_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_PODCMDCONNECTIONREQUEST = {"container": string,"controller": string,"logTail": string,"namespace": string,"pod": string,"wsConnectionRequest": SERVICE_EXEC_SH_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST};
export type SERVICE_EXEC_SH_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST = {"channelId": string,"cmdType": string,"nodeName": string,"podName": string,"websocketHost": string,"websocketScheme": string,"workspace": string};
export type SERVICE_EXEC_SH_CONNECTION_REQUEST_RESPONSE__ANON_STRUCT_1 = {};
export type SERVICE_EXEC_SH_CONNECTION_REQUEST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_PODCMDCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID = {"data": SERVICE_EXEC_SH_CONNECTION_REQUEST_RESPONSE__ANON_STRUCT_1|undefined,"message": string,"status": string};
export type SERVICE_LOG_STREAM_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_PODCMDCONNECTIONREQUEST = {"container": string,"controller": string,"logTail": string,"namespace": string,"pod": string,"wsConnectionRequest": SERVICE_LOG_STREAM_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST};
export type SERVICE_LOG_STREAM_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST = {"channelId": string,"cmdType": string,"nodeName": string,"podName": string,"websocketHost": string,"websocketScheme": string,"workspace": string};
export type SERVICE_LOG_STREAM_CONNECTION_REQUEST_RESPONSE__ANON_STRUCT_1 = {};
export type SERVICE_LOG_STREAM_CONNECTION_REQUEST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_PODCMDCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID = {"data": SERVICE_LOG_STREAM_CONNECTION_REQUEST_RESPONSE__ANON_STRUCT_1|undefined,"message": string,"status": string};
export type SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_PODEVENTCONNECTIONREQUEST = {"controller": string,"namespace": string,"wsConnectionRequest": SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST};
export type SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST_REQUEST__MOGENIUS_OPERATOR_SRC_XTERM_WSCONNECTIONREQUEST = {"channelId": string,"cmdType": string,"nodeName": string,"podName": string,"websocketHost": string,"websocketScheme": string,"workspace": string};
export type SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST_RESPONSE__ANON_STRUCT_1 = {};
export type SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_XTERM_PODEVENTCONNECTIONREQUEST_MOGENIUS_OPERATOR_SRC_CORE_VOID = {"data": SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST_RESPONSE__ANON_STRUCT_1|undefined,"message": string,"status": string};
export type STATS_POD_ALL_FOR_CONTROLLER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"kind": string,"name": string,"namespace": string,"timeOffsetMinutes": number};
export type STATS_POD_ALL_FOR_CONTROLLER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST7_MOGENIUS_OPERATOR_SRC_STRUCTS_PODSTATS = {"data": STATS_POD_ALL_FOR_CONTROLLER_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_PODSTATS[]|undefined,"message": string,"status": string};
export type STATS_POD_ALL_FOR_CONTROLLER_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_PODSTATS = {"containerName": string,"cpu": number,"cpuLimit": number,"createdAt": STATS_POD_ALL_FOR_CONTROLLER_RESPONSE__TIME_TIME,"ephemeralStorage": number,"ephemeralStorageLimit": number,"memory": number,"memoryLimit": number,"namespace": string,"podName": string,"startTime": STATS_POD_ALL_FOR_CONTROLLER_RESPONSE__TIME_TIME};
export type STATS_POD_ALL_FOR_CONTROLLER_RESPONSE__TIME_TIME = {};
export type STATS_TRAFFIC_ALL_FOR_CONTROLLER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"kind": string,"name": string,"namespace": string,"timeOffsetMinutes": number};
export type STATS_TRAFFIC_ALL_FOR_CONTROLLER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST7_MOGENIUS_OPERATOR_SRC_NETWORKMONITOR_PODNETWORKSTATS = {"data": STATS_TRAFFIC_ALL_FOR_CONTROLLER_RESPONSE__MOGENIUS_OPERATOR_SRC_NETWORKMONITOR_PODNETWORKSTATS[]|undefined,"message": string,"status": string};
export type STATS_TRAFFIC_ALL_FOR_CONTROLLER_RESPONSE__MOGENIUS_OPERATOR_SRC_NETWORKMONITOR_PODNETWORKSTATS = {"createdAt": STATS_TRAFFIC_ALL_FOR_CONTROLLER_RESPONSE__TIME_TIME,"namespace": string,"pod": string,"receivedBytes": number,"receivedPackets": number,"receivedStartBytes": number,"transmitBytes": number,"transmitPackets": number,"transmitStartBytes": number};
export type STATS_TRAFFIC_ALL_FOR_CONTROLLER_RESPONSE__TIME_TIME = {};
export type STATS_WORKSPACE_CPU_UTILIZATION_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"timeOffsetMinutes": number,"workspaceName": string};
export type STATS_WORKSPACE_CPU_UTILIZATION_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_GENERICCHARTENTRY = {"pods": Record<string, number>,"time": STATS_WORKSPACE_CPU_UTILIZATION_RESPONSE__TIME_TIME,"value": number};
export type STATS_WORKSPACE_CPU_UTILIZATION_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST8_MOGENIUS_OPERATOR_SRC_CORE_GENERICCHARTENTRY = {"data": STATS_WORKSPACE_CPU_UTILIZATION_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_GENERICCHARTENTRY[],"message": string,"status": string};
export type STATS_WORKSPACE_CPU_UTILIZATION_RESPONSE__TIME_TIME = {};
export type STATS_WORKSPACE_MEMORY_UTILIZATION_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"timeOffsetMinutes": number,"workspaceName": string};
export type STATS_WORKSPACE_MEMORY_UTILIZATION_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_GENERICCHARTENTRY = {"pods": Record<string, number>,"time": STATS_WORKSPACE_MEMORY_UTILIZATION_RESPONSE__TIME_TIME,"value": number};
export type STATS_WORKSPACE_MEMORY_UTILIZATION_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST8_MOGENIUS_OPERATOR_SRC_CORE_GENERICCHARTENTRY = {"data": STATS_WORKSPACE_MEMORY_UTILIZATION_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_GENERICCHARTENTRY[],"message": string,"status": string};
export type STATS_WORKSPACE_MEMORY_UTILIZATION_RESPONSE__TIME_TIME = {};
export type STATS_WORKSPACE_TRAFFIC_UTILIZATION_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"timeOffsetMinutes": number,"workspaceName": string};
export type STATS_WORKSPACE_TRAFFIC_UTILIZATION_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_GENERICCHARTENTRY = {"pods": Record<string, number>,"time": STATS_WORKSPACE_TRAFFIC_UTILIZATION_RESPONSE__TIME_TIME,"value": number};
export type STATS_WORKSPACE_TRAFFIC_UTILIZATION_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST8_MOGENIUS_OPERATOR_SRC_CORE_GENERICCHARTENTRY = {"data": STATS_WORKSPACE_TRAFFIC_UTILIZATION_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_GENERICCHARTENTRY[],"message": string,"status": string};
export type STATS_WORKSPACE_TRAFFIC_UTILIZATION_RESPONSE__TIME_TIME = {};
export type STORAGE_CREATE_VOLUME_REQUEST__MOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMEREQUEST = {"namespaceName": string,"sizeInGb": number,"volumeName": string};
export type STORAGE_CREATE_VOLUME_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMEREQUEST_BOOL = {"data": boolean,"message": string,"status": string};
export type STORAGE_DELETE_VOLUME_REQUEST__MOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMEREQUEST = {"namespaceName": string,"sizeInGb": number,"volumeName": string};
export type STORAGE_DELETE_VOLUME_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMEREQUEST_BOOL = {"data": boolean,"message": string,"status": string};
export type STORAGE_STATS_REQUEST__MOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMESTATSREQUEST = {"namespaceName": string,"volumeName": string};
export type STORAGE_STATS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMESTATSREQUEST_MOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMESTATSRESPONSE = {"data": STORAGE_STATS_RESPONSE__MOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMESTATSRESPONSE,"message": string,"status": string};
export type STORAGE_STATS_RESPONSE__MOGENIUS_OPERATOR_SRC_SERVICES_NFSVOLUMESTATSRESPONSE = {"freeBytes": number,"totalBytes": number,"usedBytes": number,"volumeName": string};
export type STORAGE_STATUS_REQUEST__MOGENIUS_OPERATOR_SRC_SERVICES_NFSSTATUSREQUEST = {"name": string,"namespace": string,"type": string};
export type STORAGE_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_SERVICES_NFSSTATUSREQUEST_MOGENIUS_OPERATOR_SRC_SERVICES_NFSSTATUSRESPONSE = {"data": STORAGE_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_SERVICES_NFSSTATUSRESPONSE,"message": string,"status": string};
export type STORAGE_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_SERVICES_NFSSTATUSRESPONSE = {"freeBytes": number,"messages": STORAGE_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_SERVICES_VOLUMESTATUSMESSAGE[],"namespaceName": string,"status": string,"totalBytes": number,"usedByPods": string[],"usedBytes": number,"volumeName": string};
export type STORAGE_STATUS_RESPONSE__MOGENIUS_OPERATOR_SRC_SERVICES_VOLUMESTATUSMESSAGE = {"message": string,"type": string};
export type SYSTEM_CHECK_REQUEST__ANON_STRUCT_0 = {};
export type SYSTEM_CHECK_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_MOGENIUS_OPERATOR_SRC_SERVICES_SYSTEMCHECKRESPONSE = {"data": SYSTEM_CHECK_RESPONSE__MOGENIUS_OPERATOR_SRC_SERVICES_SYSTEMCHECKRESPONSE,"message": string,"status": string};
export type SYSTEM_CHECK_RESPONSE__MOGENIUS_OPERATOR_SRC_SERVICES_SYSTEMCHECKENTRY = {"checkName": string,"description": string,"errorMessage": string|undefined,"helmStatus": string,"installPattern": string,"isRequired": boolean,"isRunning": boolean,"processTimeInMs": number,"solutionMessage": string,"successMessage": string,"uninstallPattern": string,"upgradePattern": string,"versionAvailable": string,"versionInstalled": string,"wantsToBeInstalled": boolean};
export type SYSTEM_CHECK_RESPONSE__MOGENIUS_OPERATOR_SRC_SERVICES_SYSTEMCHECKRESPONSE = {"entries": SYSTEM_CHECK_RESPONSE__MOGENIUS_OPERATOR_SRC_SERVICES_SYSTEMCHECKENTRY[],"terminalString": string};
export type TRIGGER_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR = {"apiVersion": string,"kind": string,"namespaced": boolean,"plural": string};
export type TRIGGER_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST = {"ResourceDescriptor": TRIGGER_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR,"namespace": string,"resourceName": string};
export type TRIGGER_WORKLOAD_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"Object": Record<string, any>};
export type TRIGGER_WORKLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_UTILS_WORKLOADSINGLEREQUEST_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"data": TRIGGER_WORKLOAD_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED|undefined,"message": string,"status": string};
export type UNINSTALL_CERT_MANAGER_REQUEST__ANON_STRUCT_0 = {};
export type UNINSTALL_CERT_MANAGER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type UNINSTALL_CLUSTER_ISSUER_REQUEST__ANON_STRUCT_0 = {};
export type UNINSTALL_CLUSTER_ISSUER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type UNINSTALL_INGRESS_CONTROLLER_TRAEFIK_REQUEST__ANON_STRUCT_0 = {};
export type UNINSTALL_INGRESS_CONTROLLER_TRAEFIK_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type UNINSTALL_KEPLER_REQUEST__ANON_STRUCT_0 = {};
export type UNINSTALL_KEPLER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type UNINSTALL_METALLB_REQUEST__ANON_STRUCT_0 = {};
export type UNINSTALL_METALLB_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type UNINSTALL_METRICS_SERVER_REQUEST__ANON_STRUCT_0 = {};
export type UNINSTALL_METRICS_SERVER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type UPDATE_GRANT_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"grantee": string,"name": string,"role": string,"targetName": string,"targetType": string};
export type UPDATE_GRANT_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST31_STRING = {"data": string,"message": string,"status": string};
export type UPDATE_USER_REQUEST__K8S_IO_API_RBAC_V1_SUBJECT = {"apiGroup": string,"kind": string,"name": string,"namespace": string};
export type UPDATE_USER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"email": string,"firstName": string,"lastName": string,"name": string,"subject": UPDATE_USER_REQUEST__K8S_IO_API_RBAC_V1_SUBJECT|undefined};
export type UPDATE_USER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST26_STRING = {"data": string,"message": string,"status": string};
export type UPDATE_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR = {"apiVersion": string,"kind": string,"namespaced": boolean,"plural": string};
export type UPDATE_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_WORKLOADCHANGEREQUEST = {"ResourceDescriptor": UPDATE_WORKLOAD_REQUEST__MOGENIUS_OPERATOR_SRC_UTILS_RESOURCEDESCRIPTOR,"namespace": string,"yamlData": string};
export type UPDATE_WORKLOAD_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"Object": Record<string, any>};
export type UPDATE_WORKLOAD_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_UTILS_WORKLOADCHANGEREQUEST_K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED = {"data": UPDATE_WORKLOAD_RESPONSE__K8S_IO_APIMACHINERY_PKG_APIS_META_V1_UNSTRUCTURED_UNSTRUCTURED|undefined,"message": string,"status": string};
export type UPDATE_WORKSPACE_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"displayName": string,"name": string,"resources": UPDATE_WORKSPACE_REQUEST__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_WORKSPACERESOURCEIDENTIFIER[]};
export type UPDATE_WORKSPACE_REQUEST__MOGENIUS_OPERATOR_SRC_CRDS_V1ALPHA1_WORKSPACERESOURCEIDENTIFIER = {"id": string,"namespace": string,"type": string};
export type UPDATE_WORKSPACE_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST21_STRING = {"data": string,"message": string,"status": string};
export type UPGRADEK8SMANAGER_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"command": string};
export type UPGRADEK8SMANAGER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST4_MOGENIUS_OPERATOR_SRC_STRUCTS_JOB = {"data": UPGRADEK8SMANAGER_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_JOB|undefined,"message": string,"status": string};
export type UPGRADEK8SMANAGER_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_COMMAND = {"command": string,"finished": UPGRADEK8SMANAGER_RESPONSE__TIME_TIME,"id": string,"message": string,"started": UPGRADEK8SMANAGER_RESPONSE__TIME_TIME,"state": string,"title": string};
export type UPGRADEK8SMANAGER_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_JOB = {"commands": UPGRADEK8SMANAGER_RESPONSE__MOGENIUS_OPERATOR_SRC_STRUCTS_COMMAND|undefined[],"containerName": string,"controllerName": string,"finished": UPGRADEK8SMANAGER_RESPONSE__TIME_TIME,"id": string,"message": string,"namespaceName": string,"projectId": string,"started": UPGRADEK8SMANAGER_RESPONSE__TIME_TIME,"state": string,"title": string};
export type UPGRADEK8SMANAGER_RESPONSE__TIME_TIME = {};
export type UPGRADE_CERT_MANAGER_REQUEST__ANON_STRUCT_0 = {};
export type UPGRADE_CERT_MANAGER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type UPGRADE_INGRESS_CONTROLLER_TRAEFIK_REQUEST__ANON_STRUCT_0 = {};
export type UPGRADE_INGRESS_CONTROLLER_TRAEFIK_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type UPGRADE_KEPLER_REQUEST__ANON_STRUCT_0 = {};
export type UPGRADE_KEPLER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type UPGRADE_METALLB_REQUEST__ANON_STRUCT_0 = {};
export type UPGRADE_METALLB_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type UPGRADE_METRICS_SERVER_REQUEST__ANON_STRUCT_0 = {};
export type UPGRADE_METRICS_SERVER_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_VOID_STRING = {"data": string,"message": string,"status": string};
export type WORKSPACE_CLEAN_UP_REQUEST__MOGENIUS_OPERATOR_SRC_CORE_REQUEST = {"configMaps": boolean,"dryRun": boolean,"ingresses": boolean,"jobs": boolean,"name": string,"pods": boolean,"replicaSets": boolean,"secrets": boolean,"services": boolean};
export type WORKSPACE_CLEAN_UP_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_CLEANUPRESULT = {"configMaps": WORKSPACE_CLEAN_UP_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_CLEANUPRESULTENTRY[],"ingresses": WORKSPACE_CLEAN_UP_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_CLEANUPRESULTENTRY[],"jobs": WORKSPACE_CLEAN_UP_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_CLEANUPRESULTENTRY[],"pods": WORKSPACE_CLEAN_UP_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_CLEANUPRESULTENTRY[],"replicaSets": WORKSPACE_CLEAN_UP_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_CLEANUPRESULTENTRY[],"secrets": WORKSPACE_CLEAN_UP_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_CLEANUPRESULTENTRY[],"services": WORKSPACE_CLEAN_UP_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_CLEANUPRESULTENTRY[]};
export type WORKSPACE_CLEAN_UP_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_CLEANUPRESULTENTRY = {"name": string,"namespace": string,"reason": string};
export type WORKSPACE_CLEAN_UP_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_RESULTMOGENIUS_OPERATOR_SRC_CORE_REQUEST20_MOGENIUS_OPERATOR_SRC_CORE_CLEANUPRESULT = {"data": WORKSPACE_CLEAN_UP_RESPONSE__MOGENIUS_OPERATOR_SRC_CORE_CLEANUPRESULT,"message": string,"status": string};

//===============================================================
//==================== Pattern Type Mapping =====================
//===============================================================

export interface IPatternConfig {
  [Pattern.AUDIT_LOG_LIST]: {
    Request: AUDIT_LOG_LIST_REQUEST;
    Response: AUDIT_LOG_LIST_RESPONSE;
  };
  [Pattern.CLUSTER_ARGO_CD_APPLICATION_REFRESH]: {
    Request: CLUSTER_ARGO_CD_APPLICATION_REFRESH_REQUEST;
    Response: CLUSTER_ARGO_CD_APPLICATION_REFRESH_RESPONSE;
  };
  [Pattern.CLUSTER_ARGO_CD_CREATE_API_TOKEN]: {
    Request: CLUSTER_ARGO_CD_CREATE_API_TOKEN_REQUEST;
    Response: CLUSTER_ARGO_CD_CREATE_API_TOKEN_RESPONSE;
  };
  [Pattern.CLUSTER_CLEAR_VALKEY_CACHE]: {
    Request: CLUSTER_CLEAR_VALKEY_CACHE_REQUEST;
    Response: CLUSTER_CLEAR_VALKEY_CACHE_RESPONSE;
  };
  [Pattern.CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST]: {
    Request: CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST_REQUEST;
    Response: CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST_RESPONSE;
  };
  [Pattern.CLUSTER_FORCE_DISCONNECT]: {
    Request: CLUSTER_FORCE_DISCONNECT_REQUEST;
    Response: CLUSTER_FORCE_DISCONNECT_RESPONSE;
  };
  [Pattern.CLUSTER_FORCE_RECONNECT]: {
    Request: CLUSTER_FORCE_RECONNECT_REQUEST;
    Response: CLUSTER_FORCE_RECONNECT_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_CHART_INSTALL]: {
    Request: CLUSTER_HELM_CHART_INSTALL_REQUEST;
    Response: CLUSTER_HELM_CHART_INSTALL_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_CHART_INSTALL_OCI]: {
    Request: CLUSTER_HELM_CHART_INSTALL_OCI_REQUEST;
    Response: CLUSTER_HELM_CHART_INSTALL_OCI_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_CHART_REMOVE]: {
    Request: CLUSTER_HELM_CHART_REMOVE_REQUEST;
    Response: CLUSTER_HELM_CHART_REMOVE_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_CHART_SEARCH]: {
    Request: CLUSTER_HELM_CHART_SEARCH_REQUEST;
    Response: CLUSTER_HELM_CHART_SEARCH_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_CHART_SHOW]: {
    Request: CLUSTER_HELM_CHART_SHOW_REQUEST;
    Response: CLUSTER_HELM_CHART_SHOW_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_CHART_VERSIONS]: {
    Request: CLUSTER_HELM_CHART_VERSIONS_REQUEST;
    Response: CLUSTER_HELM_CHART_VERSIONS_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_RELEASE_GET]: {
    Request: CLUSTER_HELM_RELEASE_GET_REQUEST;
    Response: CLUSTER_HELM_RELEASE_GET_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_RELEASE_GET_WORKLOADS]: {
    Request: CLUSTER_HELM_RELEASE_GET_WORKLOADS_REQUEST;
    Response: CLUSTER_HELM_RELEASE_GET_WORKLOADS_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_RELEASE_HISTORY]: {
    Request: CLUSTER_HELM_RELEASE_HISTORY_REQUEST;
    Response: CLUSTER_HELM_RELEASE_HISTORY_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_RELEASE_LINK]: {
    Request: CLUSTER_HELM_RELEASE_LINK_REQUEST;
    Response: CLUSTER_HELM_RELEASE_LINK_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_RELEASE_LIST]: {
    Request: CLUSTER_HELM_RELEASE_LIST_REQUEST;
    Response: CLUSTER_HELM_RELEASE_LIST_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_RELEASE_ROLLBACK]: {
    Request: CLUSTER_HELM_RELEASE_ROLLBACK_REQUEST;
    Response: CLUSTER_HELM_RELEASE_ROLLBACK_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_RELEASE_STATUS]: {
    Request: CLUSTER_HELM_RELEASE_STATUS_REQUEST;
    Response: CLUSTER_HELM_RELEASE_STATUS_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_RELEASE_UNINSTALL]: {
    Request: CLUSTER_HELM_RELEASE_UNINSTALL_REQUEST;
    Response: CLUSTER_HELM_RELEASE_UNINSTALL_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_RELEASE_UPGRADE]: {
    Request: CLUSTER_HELM_RELEASE_UPGRADE_REQUEST;
    Response: CLUSTER_HELM_RELEASE_UPGRADE_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_REPO_ADD]: {
    Request: CLUSTER_HELM_REPO_ADD_REQUEST;
    Response: CLUSTER_HELM_REPO_ADD_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_REPO_LIST]: {
    Request: CLUSTER_HELM_REPO_LIST_REQUEST;
    Response: CLUSTER_HELM_REPO_LIST_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_REPO_PATCH]: {
    Request: CLUSTER_HELM_REPO_PATCH_REQUEST;
    Response: CLUSTER_HELM_REPO_PATCH_RESPONSE;
  };
  [Pattern.CLUSTER_HELM_REPO_UPDATE]: {
    Request: CLUSTER_HELM_REPO_UPDATE_REQUEST;
    Response: CLUSTER_HELM_REPO_UPDATE_RESPONSE;
  };
  [Pattern.CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS]: {
    Request: CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_REQUEST;
    Response: CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS_RESPONSE;
  };
  [Pattern.CLUSTER_MACHINE_STATS]: {
    Request: CLUSTER_MACHINE_STATS_REQUEST;
    Response: CLUSTER_MACHINE_STATS_RESPONSE;
  };
  [Pattern.CLUSTER_RESOURCE_INFO]: {
    Request: CLUSTER_RESOURCE_INFO_REQUEST;
    Response: CLUSTER_RESOURCE_INFO_RESPONSE;
  };
  [Pattern.CREATE_GRANT]: {
    Request: CREATE_GRANT_REQUEST;
    Response: CREATE_GRANT_RESPONSE;
  };
  [Pattern.CREATE_NEW_WORKLOAD]: {
    Request: CREATE_NEW_WORKLOAD_REQUEST;
    Response: CREATE_NEW_WORKLOAD_RESPONSE;
  };
  [Pattern.CREATE_USER]: {
    Request: CREATE_USER_REQUEST;
    Response: CREATE_USER_RESPONSE;
  };
  [Pattern.CREATE_WORKSPACE]: {
    Request: CREATE_WORKSPACE_REQUEST;
    Response: CREATE_WORKSPACE_RESPONSE;
  };
  [Pattern.DELETE_GRANT]: {
    Request: DELETE_GRANT_REQUEST;
    Response: DELETE_GRANT_RESPONSE;
  };
  [Pattern.DELETE_USER]: {
    Request: DELETE_USER_REQUEST;
    Response: DELETE_USER_RESPONSE;
  };
  [Pattern.DELETE_WORKLOAD]: {
    Request: DELETE_WORKLOAD_REQUEST;
    Response: DELETE_WORKLOAD_RESPONSE;
  };
  [Pattern.DELETE_WORKSPACE]: {
    Request: DELETE_WORKSPACE_REQUEST;
    Response: DELETE_WORKSPACE_RESPONSE;
  };
  [Pattern.DESCRIBE]: {
    Request: DESCRIBE_REQUEST;
    Response: DESCRIBE_RESPONSE;
  };
  [Pattern.DESCRIBE_WORKLOAD]: {
    Request: DESCRIBE_WORKLOAD_REQUEST;
    Response: DESCRIBE_WORKLOAD_RESPONSE;
  };
  [Pattern.FILES_CHMOD]: {
    Request: FILES_CHMOD_REQUEST;
    Response: FILES_CHMOD_RESPONSE;
  };
  [Pattern.FILES_CHOWN]: {
    Request: FILES_CHOWN_REQUEST;
    Response: FILES_CHOWN_RESPONSE;
  };
  [Pattern.FILES_CREATE_FOLDER]: {
    Request: FILES_CREATE_FOLDER_REQUEST;
    Response: FILES_CREATE_FOLDER_RESPONSE;
  };
  [Pattern.FILES_DELETE]: {
    Request: FILES_DELETE_REQUEST;
    Response: FILES_DELETE_RESPONSE;
  };
  [Pattern.FILES_DOWNLOAD]: {
    Request: FILES_DOWNLOAD_REQUEST;
    Response: FILES_DOWNLOAD_RESPONSE;
  };
  [Pattern.FILES_INFO]: {
    Request: FILES_INFO_REQUEST;
    Response: FILES_INFO_RESPONSE;
  };
  [Pattern.FILES_LIST]: {
    Request: FILES_LIST_REQUEST;
    Response: FILES_LIST_RESPONSE;
  };
  [Pattern.FILES_RENAME]: {
    Request: FILES_RENAME_REQUEST;
    Response: FILES_RENAME_RESPONSE;
  };
  [Pattern.GET_GRANT]: {
    Request: GET_GRANT_REQUEST;
    Response: GET_GRANT_RESPONSE;
  };
  [Pattern.GET_GRANTS]: {
    Request: GET_GRANTS_REQUEST;
    Response: GET_GRANTS_RESPONSE;
  };
  [Pattern.GET_LABELED_WORKLOAD_LIST]: {
    Request: GET_LABELED_WORKLOAD_LIST_REQUEST;
    Response: GET_LABELED_WORKLOAD_LIST_RESPONSE;
  };
  [Pattern.GET_NAMESPACE_WORKLOAD_LIST]: {
    Request: GET_NAMESPACE_WORKLOAD_LIST_REQUEST;
    Response: GET_NAMESPACE_WORKLOAD_LIST_RESPONSE;
  };
  [Pattern.GET_NODES_METRICS]: {
    Request: GET_NODES_METRICS_REQUEST;
    Response: GET_NODES_METRICS_RESPONSE;
  };
  [Pattern.GET_USER]: {
    Request: GET_USER_REQUEST;
    Response: GET_USER_RESPONSE;
  };
  [Pattern.GET_USERS]: {
    Request: GET_USERS_REQUEST;
    Response: GET_USERS_RESPONSE;
  };
  [Pattern.GET_WORKLOAD]: {
    Request: GET_WORKLOAD_REQUEST;
    Response: GET_WORKLOAD_RESPONSE;
  };
  [Pattern.GET_WORKLOAD_EXAMPLE]: {
    Request: GET_WORKLOAD_EXAMPLE_REQUEST;
    Response: GET_WORKLOAD_EXAMPLE_RESPONSE;
  };
  [Pattern.GET_WORKLOAD_LIST]: {
    Request: GET_WORKLOAD_LIST_REQUEST;
    Response: GET_WORKLOAD_LIST_RESPONSE;
  };
  [Pattern.GET_WORKLOAD_STATUS]: {
    Request: GET_WORKLOAD_STATUS_REQUEST;
    Response: GET_WORKLOAD_STATUS_RESPONSE;
  };
  [Pattern.GET_WORKSPACE]: {
    Request: GET_WORKSPACE_REQUEST;
    Response: GET_WORKSPACE_RESPONSE;
  };
  [Pattern.GET_WORKSPACES]: {
    Request: GET_WORKSPACES_REQUEST;
    Response: GET_WORKSPACES_RESPONSE;
  };
  [Pattern.GET_WORKSPACE_WORKLOADS]: {
    Request: GET_WORKSPACE_WORKLOADS_REQUEST;
    Response: GET_WORKSPACE_WORKLOADS_RESPONSE;
  };
  [Pattern.INSTALL_CERT_MANAGER]: {
    Request: INSTALL_CERT_MANAGER_REQUEST;
    Response: INSTALL_CERT_MANAGER_RESPONSE;
  };
  [Pattern.INSTALL_CLUSTER_ISSUER]: {
    Request: INSTALL_CLUSTER_ISSUER_REQUEST;
    Response: INSTALL_CLUSTER_ISSUER_RESPONSE;
  };
  [Pattern.INSTALL_INGRESS_CONTROLLER_TRAEFIK]: {
    Request: INSTALL_INGRESS_CONTROLLER_TRAEFIK_REQUEST;
    Response: INSTALL_INGRESS_CONTROLLER_TRAEFIK_RESPONSE;
  };
  [Pattern.INSTALL_KEPLER]: {
    Request: INSTALL_KEPLER_REQUEST;
    Response: INSTALL_KEPLER_RESPONSE;
  };
  [Pattern.INSTALL_METALLB]: {
    Request: INSTALL_METALLB_REQUEST;
    Response: INSTALL_METALLB_RESPONSE;
  };
  [Pattern.INSTALL_METRICS_SERVER]: {
    Request: INSTALL_METRICS_SERVER_REQUEST;
    Response: INSTALL_METRICS_SERVER_RESPONSE;
  };
  [Pattern.LIST_ALL_RESOURCE_DESCRIPTORS]: {
    Request: LIST_ALL_RESOURCE_DESCRIPTORS_REQUEST;
    Response: LIST_ALL_RESOURCE_DESCRIPTORS_RESPONSE;
  };
  [Pattern.LIVE_STREAM_NODES_CPU]: {
    Request: LIVE_STREAM_NODES_CPU_REQUEST;
    Response: LIVE_STREAM_NODES_CPU_RESPONSE;
  };
  [Pattern.LIVE_STREAM_NODES_MEMORY]: {
    Request: LIVE_STREAM_NODES_MEMORY_REQUEST;
    Response: LIVE_STREAM_NODES_MEMORY_RESPONSE;
  };
  [Pattern.LIVE_STREAM_NODES_TRAFFIC]: {
    Request: LIVE_STREAM_NODES_TRAFFIC_REQUEST;
    Response: LIVE_STREAM_NODES_TRAFFIC_RESPONSE;
  };
  [Pattern.LIVE_STREAM_POD_CPU]: {
    Request: LIVE_STREAM_POD_CPU_REQUEST;
    Response: LIVE_STREAM_POD_CPU_RESPONSE;
  };
  [Pattern.LIVE_STREAM_POD_MEMORY]: {
    Request: LIVE_STREAM_POD_MEMORY_REQUEST;
    Response: LIVE_STREAM_POD_MEMORY_RESPONSE;
  };
  [Pattern.LIVE_STREAM_POD_TRAFFIC]: {
    Request: LIVE_STREAM_POD_TRAFFIC_REQUEST;
    Response: LIVE_STREAM_POD_TRAFFIC_RESPONSE;
  };
  [Pattern.LIVE_STREAM_WORKSPACE_CPU]: {
    Request: LIVE_STREAM_WORKSPACE_CPU_REQUEST;
    Response: LIVE_STREAM_WORKSPACE_CPU_RESPONSE;
  };
  [Pattern.LIVE_STREAM_WORKSPACE_MEMORY]: {
    Request: LIVE_STREAM_WORKSPACE_MEMORY_REQUEST;
    Response: LIVE_STREAM_WORKSPACE_MEMORY_RESPONSE;
  };
  [Pattern.LIVE_STREAM_WORKSPACE_TRAFFIC]: {
    Request: LIVE_STREAM_WORKSPACE_TRAFFIC_REQUEST;
    Response: LIVE_STREAM_WORKSPACE_TRAFFIC_RESPONSE;
  };
  [Pattern.PROMETHEUS_CHARTS_ADD]: {
    Request: PROMETHEUS_CHARTS_ADD_REQUEST;
    Response: PROMETHEUS_CHARTS_ADD_RESPONSE;
  };
  [Pattern.PROMETHEUS_CHARTS_GET]: {
    Request: PROMETHEUS_CHARTS_GET_REQUEST;
    Response: PROMETHEUS_CHARTS_GET_RESPONSE;
  };
  [Pattern.PROMETHEUS_CHARTS_LIST]: {
    Request: PROMETHEUS_CHARTS_LIST_REQUEST;
    Response: PROMETHEUS_CHARTS_LIST_RESPONSE;
  };
  [Pattern.PROMETHEUS_CHARTS_REMOVE]: {
    Request: PROMETHEUS_CHARTS_REMOVE_REQUEST;
    Response: PROMETHEUS_CHARTS_REMOVE_RESPONSE;
  };
  [Pattern.PROMETHEUS_IS_REACHABLE]: {
    Request: PROMETHEUS_IS_REACHABLE_REQUEST;
    Response: PROMETHEUS_IS_REACHABLE_RESPONSE;
  };
  [Pattern.PROMETHEUS_QUERY]: {
    Request: PROMETHEUS_QUERY_REQUEST;
    Response: PROMETHEUS_QUERY_RESPONSE;
  };
  [Pattern.PROMETHEUS_VALUES]: {
    Request: PROMETHEUS_VALUES_REQUEST;
    Response: PROMETHEUS_VALUES_RESPONSE;
  };
  [Pattern.SEALED_SECRET_CREATE_FROM_EXISTING]: {
    Request: SEALED_SECRET_CREATE_FROM_EXISTING_REQUEST;
    Response: SEALED_SECRET_CREATE_FROM_EXISTING_RESPONSE;
  };
  [Pattern.SEALED_SECRET_GET_CERTIFICATE]: {
    Request: SEALED_SECRET_GET_CERTIFICATE_REQUEST;
    Response: SEALED_SECRET_GET_CERTIFICATE_RESPONSE;
  };
  [Pattern.SERVICE_EXEC_SH_CONNECTION_REQUEST]: {
    Request: SERVICE_EXEC_SH_CONNECTION_REQUEST_REQUEST;
    Response: SERVICE_EXEC_SH_CONNECTION_REQUEST_RESPONSE;
  };
  [Pattern.SERVICE_LOG_STREAM_CONNECTION_REQUEST]: {
    Request: SERVICE_LOG_STREAM_CONNECTION_REQUEST_REQUEST;
    Response: SERVICE_LOG_STREAM_CONNECTION_REQUEST_RESPONSE;
  };
  [Pattern.SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST]: {
    Request: SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST_REQUEST;
    Response: SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST_RESPONSE;
  };
  [Pattern.STATS_POD_ALL_FOR_CONTROLLER]: {
    Request: STATS_POD_ALL_FOR_CONTROLLER_REQUEST;
    Response: STATS_POD_ALL_FOR_CONTROLLER_RESPONSE;
  };
  [Pattern.STATS_TRAFFIC_ALL_FOR_CONTROLLER]: {
    Request: STATS_TRAFFIC_ALL_FOR_CONTROLLER_REQUEST;
    Response: STATS_TRAFFIC_ALL_FOR_CONTROLLER_RESPONSE;
  };
  [Pattern.STATS_WORKSPACE_CPU_UTILIZATION]: {
    Request: STATS_WORKSPACE_CPU_UTILIZATION_REQUEST;
    Response: STATS_WORKSPACE_CPU_UTILIZATION_RESPONSE;
  };
  [Pattern.STATS_WORKSPACE_MEMORY_UTILIZATION]: {
    Request: STATS_WORKSPACE_MEMORY_UTILIZATION_REQUEST;
    Response: STATS_WORKSPACE_MEMORY_UTILIZATION_RESPONSE;
  };
  [Pattern.STATS_WORKSPACE_TRAFFIC_UTILIZATION]: {
    Request: STATS_WORKSPACE_TRAFFIC_UTILIZATION_REQUEST;
    Response: STATS_WORKSPACE_TRAFFIC_UTILIZATION_RESPONSE;
  };
  [Pattern.STORAGE_CREATE_VOLUME]: {
    Request: STORAGE_CREATE_VOLUME_REQUEST;
    Response: STORAGE_CREATE_VOLUME_RESPONSE;
  };
  [Pattern.STORAGE_DELETE_VOLUME]: {
    Request: STORAGE_DELETE_VOLUME_REQUEST;
    Response: STORAGE_DELETE_VOLUME_RESPONSE;
  };
  [Pattern.STORAGE_STATS]: {
    Request: STORAGE_STATS_REQUEST;
    Response: STORAGE_STATS_RESPONSE;
  };
  [Pattern.STORAGE_STATUS]: {
    Request: STORAGE_STATUS_REQUEST;
    Response: STORAGE_STATUS_RESPONSE;
  };
  [Pattern.SYSTEM_CHECK]: {
    Request: SYSTEM_CHECK_REQUEST;
    Response: SYSTEM_CHECK_RESPONSE;
  };
  [Pattern.TRIGGER_WORKLOAD]: {
    Request: TRIGGER_WORKLOAD_REQUEST;
    Response: TRIGGER_WORKLOAD_RESPONSE;
  };
  [Pattern.UNINSTALL_CERT_MANAGER]: {
    Request: UNINSTALL_CERT_MANAGER_REQUEST;
    Response: UNINSTALL_CERT_MANAGER_RESPONSE;
  };
  [Pattern.UNINSTALL_CLUSTER_ISSUER]: {
    Request: UNINSTALL_CLUSTER_ISSUER_REQUEST;
    Response: UNINSTALL_CLUSTER_ISSUER_RESPONSE;
  };
  [Pattern.UNINSTALL_INGRESS_CONTROLLER_TRAEFIK]: {
    Request: UNINSTALL_INGRESS_CONTROLLER_TRAEFIK_REQUEST;
    Response: UNINSTALL_INGRESS_CONTROLLER_TRAEFIK_RESPONSE;
  };
  [Pattern.UNINSTALL_KEPLER]: {
    Request: UNINSTALL_KEPLER_REQUEST;
    Response: UNINSTALL_KEPLER_RESPONSE;
  };
  [Pattern.UNINSTALL_METALLB]: {
    Request: UNINSTALL_METALLB_REQUEST;
    Response: UNINSTALL_METALLB_RESPONSE;
  };
  [Pattern.UNINSTALL_METRICS_SERVER]: {
    Request: UNINSTALL_METRICS_SERVER_REQUEST;
    Response: UNINSTALL_METRICS_SERVER_RESPONSE;
  };
  [Pattern.UPDATE_GRANT]: {
    Request: UPDATE_GRANT_REQUEST;
    Response: UPDATE_GRANT_RESPONSE;
  };
  [Pattern.UPDATE_USER]: {
    Request: UPDATE_USER_REQUEST;
    Response: UPDATE_USER_RESPONSE;
  };
  [Pattern.UPDATE_WORKLOAD]: {
    Request: UPDATE_WORKLOAD_REQUEST;
    Response: UPDATE_WORKLOAD_RESPONSE;
  };
  [Pattern.UPDATE_WORKSPACE]: {
    Request: UPDATE_WORKSPACE_REQUEST;
    Response: UPDATE_WORKSPACE_RESPONSE;
  };
  [Pattern.UPGRADEK8SMANAGER]: {
    Request: UPGRADEK8SMANAGER_REQUEST;
    Response: UPGRADEK8SMANAGER_RESPONSE;
  };
  [Pattern.UPGRADE_CERT_MANAGER]: {
    Request: UPGRADE_CERT_MANAGER_REQUEST;
    Response: UPGRADE_CERT_MANAGER_RESPONSE;
  };
  [Pattern.UPGRADE_INGRESS_CONTROLLER_TRAEFIK]: {
    Request: UPGRADE_INGRESS_CONTROLLER_TRAEFIK_REQUEST;
    Response: UPGRADE_INGRESS_CONTROLLER_TRAEFIK_RESPONSE;
  };
  [Pattern.UPGRADE_KEPLER]: {
    Request: UPGRADE_KEPLER_REQUEST;
    Response: UPGRADE_KEPLER_RESPONSE;
  };
  [Pattern.UPGRADE_METALLB]: {
    Request: UPGRADE_METALLB_REQUEST;
    Response: UPGRADE_METALLB_RESPONSE;
  };
  [Pattern.UPGRADE_METRICS_SERVER]: {
    Request: UPGRADE_METRICS_SERVER_REQUEST;
    Response: UPGRADE_METRICS_SERVER_RESPONSE;
  };
  [Pattern.WORKSPACE_CLEAN_UP]: {
    Request: WORKSPACE_CLEAN_UP_REQUEST;
    Response: WORKSPACE_CLEAN_UP_RESPONSE;
  };
};

export type PatternKey = keyof IPatternConfig;

export function PatternInfo<K extends PatternKey>(_: K): IPatternConfig[K] {
  return {} as IPatternConfig[K];
};

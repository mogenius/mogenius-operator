package services

// TODO: GET THE TYPE RIGHT
// I NEE AN ENUM !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

// type COMMAND string

// const (
// 	K8SNOTIFICATION     COMMAND = "K8sNotification"
// 	CLUSTERSTATUS       COMMAND = "ClusterStatus"
// 	CLUSTERRESOURCEINFO COMMAND = "ClusterResourceInfo"
// 	KUBERNETESEVENT     COMMAND = "KubernetesEvent"
// 	UPGRADEK8SMANAGER   COMMAND = "UpgradeK8sManager"
// 	SERVICE_POD_EXISTS  COMMAND = "SERVICE_POD_EXISTS"
// 	SERVICE_PODS        COMMAND = "SERVICE_PODS"
// 	FILES_LIST          COMMAND = "files/list"
// 	FILES_DOWNLOAD      COMMAND = "files/download"
// 	FILES_CREATE_FOLDER COMMAND = "files/create-folder"
// 	FILES_RENAME        COMMAND = "files/rename"
// 	FILES_CHOWN         COMMAND = "files/chown"
// 	FILES_CHMOD         COMMAND = "files/chmod"
// 	FILES_DELETE        COMMAND = "files/delete"
// )

// var COMMAND_REQUESTS1 = []COMMAND{
// 	K8SNOTIFICATION,
// 	CLUSTERSTATUS,
// 	CLUSTERRESOURCEINFO,
// 	KUBERNETESEVENT,
// 	UPGRADEK8SMANAGER,
// 	SERVICE_POD_EXISTS,
// 	SERVICE_PODS,
// 	FILES_LIST,
// 	FILES_DOWNLOAD,
// 	FILES_CREATE_FOLDER,
// 	FILES_RENAME,
// 	FILES_CHOWN,
// 	FILES_CHMOD,
// 	FILES_DELETE,
// }

// var COMMAND_REQUESTS = []string{
// 	"cluster/execute-helm-chart-task",
// 	"cluster/uninstall-helm-chart",
// 	"cluster/tcp-udp-configuration",

// 	"namespace/create",
// 	"namespace/delete",
// 	"namespace/shutdown",
// 	"namespace/pod-ids",
// 	"namespace/validate-cluster-pods",
// 	"namespace/validate-ports",
// 	"namespace/list-all",
// 	"namespace/gather-all-resources",
// 	"namespace/backup",
// 	"namespace/restore",

// 	"service/create",
// 	"service/delete",
// 	"service/pod-ids",
// 	"service/set-image",
// 	"service/log",
// 	"service/log-error",
// 	"service/resource-status",
// 	"service/restart",
// 	"service/stop",
// 	"service/start",
// 	"service/update-service",
// 	"service/spectrum-bind",
// 	"service/spectrum-unbind",
// 	"service/spectrum-configmaps",

// 	"service/log-stream",

// 	"list/namespaces",
// 	"list/deployments",
// 	"list/services",
// 	"list/pods",
// 	"list/ingresses",
// 	"list/configmaps",
// 	"list/secrets",
// 	"list/nodes",
// 	"list/daemonsets",
// 	"list/statefulsets",
// 	"list/jobs",
// 	"list/cronjobs",
// 	"list/replicasets",
// 	"list/persistent_volumes",
// 	"list/persistent_volume_claims",
// 	"list/horizontal_pod_autoscalers",
// 	"list/events",
// 	"list/certificates",
// 	"list/certificaterequests",
// 	"list/orders",
// 	"list/issuers",
// 	"list/clusterissuers",
// 	"list/service_account",
// 	"list/role",
// 	"list/role_binding",
// 	"list/cluster_role",
// 	"list/cluster_role_binding",
// 	"list/volume_attachment",

// 	"update/deployment",
// 	"update/service",
// 	"update/pod",
// 	"update/ingress",
// 	"update/configmap",
// 	"update/secret",
// 	"update/daemonset",
// 	"update/statefulset",
// 	"update/job",
// 	"update/cronjob",
// 	"update/replicaset",
// 	"update/persistent_volume",
// 	"update/persistent_volume_claim",
// 	"update/horizontal_pod_autoscalers",
// 	"update/certificates",
// 	"update/certificaterequests",
// 	"update/orders",
// 	"update/issuers",
// 	"update/clusterissuers",
// 	"update/service_account",
// 	"update/role",
// 	"update/role_binding",
// 	"update/cluster_role",
// 	"update/cluster_role_binding",
// 	"update/volume_attachment",

// 	"delete/namespace",
// 	"delete/deployment",
// 	"delete/service",
// 	"delete/pod",
// 	"delete/ingress",
// 	"delete/configmap",
// 	"delete/secret",
// 	"delete/daemonset",
// 	"delete/statefulset",
// 	"delete/job",
// 	"delete/cronjob",
// 	"delete/replicaset",
// 	"delete/persistent_volume",
// 	"delete/persistent_volume_claim",
// 	"delete/certificates",
// 	"delete/certificaterequests",
// 	"delete/orders",
// 	"delete/issuers",
// 	"delete/clusterissuers",
// 	"delete/service_account",
// 	"delete/role",
// 	"delete/role_binding",
// 	"delete/cluster_role",
// 	"delete/cluster_role_binding",
// 	"delete/volume_attachment",

// 	"storage/create-volume",
// 	"storage/delete-volume",
// 	"storage/backup-volume",
// 	"storage/restore-volume",
// 	"storage/stats",
// 	"storage/namespace/stats",

// 	"popeye-console",
// }

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
	PAT_INSTALL_CONTAINER_REGISTRY           string = "install-container-registry"
	PAT_UNINSTALL_CONTAINER_REGISTRY         string = "uninstall-container-registry"
	PAT_INSTALL_EXTERNAL_SECRETS             string = "install-external-secrets"
	PAT_UNINSTALL_EXTERNAL_SECRETS           string = "uninstall-external-secrets"
	PAT_INSTALL_METALLB                      string = "install-metallb"
	PAT_UNINSTALL_METALLB                    string = "uninstall-metallb"
	PAT_INSTALL_RENOVATE_OPERATOR            string = "install-renovate-operator"
	PAT_UNINSTALL_RENOVATE_OPERATOR          string = "uninstall-renovate-operator"
	PAT_UPGRADE_RENOVATE_OPERATOR            string = "upgrade-renovate-operator"
	PAT_INSTALL_PROMETHEUS                   string = "install-prometheus"
	PAT_UNINSTALL_PROMETHEUS                 string = "uninstall-prometheus"
	PAT_UPGRADE_PROMETHEUS                   string = "upgrade-prometheus"
	PAT_INSTALL_ALERTMANAGER                 string = "install-alertmanager"
	PAT_UNINSTALL_ALERTMANAGER               string = "uninstall-alertmanager"
	PAT_UPGRADE_ALERTMANAGER                 string = "upgrade-alertmanager"
)

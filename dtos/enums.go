package dtos

type GitConnectionTypeEnum string

const (
	GitConGitHub    GitConnectionTypeEnum = "GIT_HUB"
	GitConGitLab    GitConnectionTypeEnum = "GIT_LAB"
	GitConBitBucket GitConnectionTypeEnum = "BITBUCKET"
)

type K8sServiceTypeEnum string

const (
	GIT_REPOSITORY_TEMPLATE               K8sServiceTypeEnum = "GIT_REPOSITORY_TEMPLATE"
	GIT_REPOSITORY                        K8sServiceTypeEnum = "GIT_REPOSITORY"
	CONTAINER_IMAGE_TEMPLATE              K8sServiceTypeEnum = "CONTAINER_IMAGE_TEMPLATE"
	CONTAINER_IMAGE                       K8sServiceTypeEnum = "CONTAINER_IMAGE"
	K8S_DEPLOYMENT                        K8sServiceTypeEnum = "K8S_DEPLOYMENT"
	K8S_DAEMONSET                         K8sServiceTypeEnum = "K8S_DAEMONSET"
	K8S_STATEFULSET                       K8sServiceTypeEnum = "K8S_STATEFULSET"
	K8S_REPLICASET                        K8sServiceTypeEnum = "K8S_REPLICASET"
	K8S_JOB                               K8sServiceTypeEnum = "K8S_JOB"
	DOCKER_COMPOSE                        K8sServiceTypeEnum = "DOCKER_COMPOSE"
	K8S_CRON_JOB_GIT_REPOSITORY_TEMPLATE  K8sServiceTypeEnum = "K8S_CRON_JOB_GIT_REPOSITORY_TEMPLATE"
	K8S_CRON_JOB_GIT_REPOSITORY           K8sServiceTypeEnum = "K8S_CRON_JOB_GIT_REPOSITORY"
	K8S_CRON_JOB_CONTAINER_IMAGE_TEMPLATE K8sServiceTypeEnum = "K8S_CRON_JOB_CONTAINER_IMAGE_TEMPLATE"
	K8S_CRON_JOB_CONTAINER_IMAGE          K8sServiceTypeEnum = "K8S_CRON_JOB_CONTAINER_IMAGE"
)

type ImagePullPolicyEnum string

const (
	PullPolicyAlways       ImagePullPolicyEnum = "Always"
	PullPolicyNever        ImagePullPolicyEnum = "Never"
	PullPolicyIfNotPresent ImagePullPolicyEnum = "IfNotPresent"
)

type DeploymentStrategyEnum string

const (
	StrategyRecreate DeploymentStrategyEnum = "recreate"
	StrategyRolling  DeploymentStrategyEnum = "rolling"
)

type K8sEnvVarDtoEnum string

const (
	EnvVarPlainText       K8sEnvVarDtoEnum = "PLAINTEXT"
	EnvVarKeyVault        K8sEnvVarDtoEnum = "KEY_VAULT"
	EnvVarVolumeMount     K8sEnvVarDtoEnum = "VOLUME_MOUNT"
	EnvVarVolumeMountSeed K8sEnvVarDtoEnum = "VOLUME_MOUNT_SEED"
	EnvVarChangeOwner     K8sEnvVarDtoEnum = "CHANGE_OWNER"
	EnvVarHostname        K8sEnvVarDtoEnum = "HOSTNAME"
)

type PortTypeEnum string

const (
	PortTypeHTTPS PortTypeEnum = "HTTPS"
	PortTypeTCP   PortTypeEnum = "TCP"
	PortTypeUDP   PortTypeEnum = "UDP"
)

type K8sServiceControllerEnum string

const (
	DEPLOYMENT   K8sServiceControllerEnum = "Deployment"
	REPLICA_SET  K8sServiceControllerEnum = "ReplicaSet"
	STATEFUL_SET K8sServiceControllerEnum = "StatefulSet"
	DAEMON_SET   K8sServiceControllerEnum = "DaemonSet"
	JOB          K8sServiceControllerEnum = "Job"
	CRON_JOB     K8sServiceControllerEnum = "CronJob"
)

type K8sContainerTypeEnum string

const (
	CONTAINER_GIT_REPOSITORY  K8sContainerTypeEnum = "GIT_REPOSITORY"
	CONTAINER_CONTAINER_IMAGE K8sContainerTypeEnum = "CONTAINER_IMAGE"
)

const (
	KindNamespaces               string = "namespaces"
	KindDeployments              string = "deployments"
	KindPods                     string = "pods"
	KindStatefulSets             string = "statefulsets"
	KindServices                 string = "services"
	KindIngresses                string = "ingresses"
	KindConfigMaps               string = "configmaps"
	KindSecrets                  string = "secrets"
	KindJobs                     string = "jobs"
	KindCronJobs                 string = "cronjobs"
	KindDaemonSets               string = "daemonsets"
	KindNetworkPolicies          string = "networkpolicies"
	KindHorizontalPodAutoscalers string = "horizontalpodautoscalers"
)

var AvailableSyncWorkloadKinds = []string{
	KindNamespaces,
	KindPods,
	KindDeployments,
	KindStatefulSets,
	KindServices,
	KindIngresses,
	KindConfigMaps,
	KindSecrets,
	KindJobs,
	KindCronJobs,
	KindDaemonSets,
	KindNetworkPolicies,
	KindHorizontalPodAutoscalers,
}

func DefaultIgnoredNamespaces() []string {
	return []string{
		"kube-system",
		"kube-public",
		"kube-node-lease",
	}
}

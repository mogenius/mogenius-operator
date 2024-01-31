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

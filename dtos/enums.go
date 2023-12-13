package dtos

type GitConnectionTypeEnum string

const (
	GitConGitHub    GitConnectionTypeEnum = "GIT_HUB"
	GitConGitLab    GitConnectionTypeEnum = "GIT_LAB"
	GitConBitBucket GitConnectionTypeEnum = "BITBUCKET"
)

type K8sServiceTypeEnum string

const (
	GitRepositoryTemplate  K8sServiceTypeEnum = "GIT_REPOSITORY_TEMPLATE"
	GitRepository          K8sServiceTypeEnum = "GIT_REPOSITORY"
	ContainerImageTemplate K8sServiceTypeEnum = "CONTAINER_IMAGE_TEMPLATE"
	ContainerImage         K8sServiceTypeEnum = "CONTAINER_IMAGE"
	K8SDeployment          K8sServiceTypeEnum = "K8S_DEPLOYMENT"
	K8SCronJob             K8sServiceTypeEnum = "K8S_CRON_JOB"
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

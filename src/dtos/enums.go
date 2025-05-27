package dtos

type GitConnectionTypeEnum string

const (
	GitConGitHub    GitConnectionTypeEnum = "GIT_HUB"
	GitConGitLab    GitConnectionTypeEnum = "GIT_LAB"
	GitConBitBucket GitConnectionTypeEnum = "BITBUCKET"
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
	EnvVarPlainText   K8sEnvVarDtoEnum = "PLAINTEXT"
	EnvVarKeyVault    K8sEnvVarDtoEnum = "KEY_VAULT"
	EnvVarVolumeMount K8sEnvVarDtoEnum = "VOLUME_MOUNT"
	// EnvVarExternalSecret  K8sEnvVarDtoEnum = "EXTERNAL_SECRET_STORE"
	// EnvVarVolumeMountSeed K8sEnvVarDtoEnum = "VOLUME_MOUNT_SEED"
	// EnvVarChangeOwner     K8sEnvVarDtoEnum = "CHANGE_OWNER"
	EnvVarHostname K8sEnvVarDtoEnum = "HOSTNAME"
)

type EnvVarVaultTypeEnum string

const (
	EnvVarVaultTypeMogeniusVault          EnvVarVaultTypeEnum = "MOGENIUS_VAULT"
	EnvVarVaultTypeHashicorpExternalVault EnvVarVaultTypeEnum = "HASHICORP_EXTERNAL_VAULT"
)

type PortTypeEnum string

const (
	PortTypeHTTPS PortTypeEnum = "HTTPS"
	PortTypeTCP   PortTypeEnum = "TCP"
	PortTypeUDP   PortTypeEnum = "UDP"
	PortTypeSCTP  PortTypeEnum = "SCTP"
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

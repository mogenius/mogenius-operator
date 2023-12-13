package structs

type HelmTaskEnum string

const (
	HelmInstall   HelmTaskEnum = "install"
	HelmUpgrade   HelmTaskEnum = "upgrade"
	HelmUninstall HelmTaskEnum = "uninstall"
)

type BuildJobStateEnum string

const (
	BuildJobStateFailed    BuildJobStateEnum = "FAILED"
	BuildJobStateSucceeded BuildJobStateEnum = "SUCCEEDED"
	BuildJobStateStarted   BuildJobStateEnum = "STARTED"
	BuildJobStatePending   BuildJobStateEnum = "PENDING"
	BuildJobStateCanceled  BuildJobStateEnum = "CANCELED"
	BuildJobStateTimeout   BuildJobStateEnum = "TIMEOUT"
)

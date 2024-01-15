package structs

type HelmTaskEnum string

const (
	HelmInstall   HelmTaskEnum = "install"
	HelmUpgrade   HelmTaskEnum = "upgrade"
	HelmUninstall HelmTaskEnum = "uninstall"
)

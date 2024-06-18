package services

type Installable struct {
	Name               string
	InstallErrMsg      string
	InstallPattern     string
	IsRequired         bool
	WantsToBeInstalled bool
}
type Uninstallable struct {
	UninstallPattern string
}

type Upgradable struct {
	UpgradePattern string
}

type Deployable struct {
	deployName string
}

func (d *Deployable, Ins) ExecuteSystemCheck() string {
	if d.CustomDeployName != "" {
		return d.CustomDeployName
	}
	return d.Name
}

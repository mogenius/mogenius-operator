package gitops

type fluxInstaller struct{}

func (a *fluxInstaller) Install(component string, artifact GitOpsArtifact) error {
	return nil
}

func (a *fluxInstaller) UnInstall(component string) error {
	return nil
}

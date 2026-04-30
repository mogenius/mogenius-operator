package gitops

type argocdInstaller struct{}

func (a *argocdInstaller) Install(component string, artifact GitOpsArtifact) error {
	return nil
}

func (a *argocdInstaller) UnInstall(component string) error {
	return nil
}

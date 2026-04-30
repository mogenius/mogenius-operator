package gitops

type GitOpsArtifact struct {
	Namespace    string
	Values       string
	HelmChart    HelmChartReference
	ExtraObjects []any
}

type HelmChartReference struct {
	Repository string
	Chart      string
	Name       string
	Version    string
}

type GitOpsInstaller interface {
	Install(string, GitOpsArtifact) error
	UnInstall(string) error
}

func NewGitOpsInstaller(engine string) GitOpsInstaller {
	switch engine {
	case "argocd":
		return &argocdInstaller{}
	case "flux":
		return &fluxInstaller{}
	default:
		return nil
	}
}

func defaultLabels(component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "mogenius-operator",
		"app.kubernetes.io/component":  component,
	}
}

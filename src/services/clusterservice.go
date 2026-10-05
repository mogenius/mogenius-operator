package services

const (
	MogeniusHelmIndex = "https://helm.mogenius.com/public"
)

type ClusterListWorkloads struct {
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"labelSelector"`
	Prefix        string `json:"prefix"`
}

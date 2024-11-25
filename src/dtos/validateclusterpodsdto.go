package dtos

type ValidateClusterPodsDto struct {
	InDbButNotInCluster []string `json:"inDbButNotInCluster"`
	InClusterButNotInDb []string `json:"inClusterButNotInDb"`
}

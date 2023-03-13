package dtos

type ClusterResourceInfoDto struct {
	LoadBalancerIps []string   `json:"loadBalancerIps"`
	NodeStats       []NodeStat `json:"nodeStats"`
}

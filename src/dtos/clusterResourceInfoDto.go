package dtos

type ClusterResourceInfoDto struct {
	LoadBalancerExternalIps []string   `json:"loadBalancerExternalIps"`
	NodeStats               []NodeStat `json:"nodeStats"`
	Provider                string     `json:"provider"`
}

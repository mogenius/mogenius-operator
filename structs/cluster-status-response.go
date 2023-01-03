package structs

type ClusterStatusResponse struct {
	TCPResponse
	ClusterName string `json:"clusterName"`
}

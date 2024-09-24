package dtos

// type of ingress or egress constant, literal
type K8sNetworkPolicyType string

const (
	Egress  K8sNetworkPolicyType = "egress"
	Ingress K8sNetworkPolicyType = "ingress"
)

type K8sLabeledNetworkPolicyParams struct {
	Name  string               `json:"name" validate:"required"`
	Type  K8sNetworkPolicyType `json:"type" validate:"required"`
	Ports []K8sPortsDto        `json:"ports" validate:"required"`
}

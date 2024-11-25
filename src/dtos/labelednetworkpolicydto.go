package dtos

// type of ingress or egress constant, literal
type K8sNetworkPolicyType string

const (
	Egress  K8sNetworkPolicyType = "egress"
	Ingress K8sNetworkPolicyType = "ingress"
)

type K8sLabeledNetworkPolicyDto struct {
	Name     string               `json:"name" validate:"required"`
	Type     K8sNetworkPolicyType `json:"type" validate:"required"`
	Port     uint16               `json:"port" validate:"required"`
	PortType PortTypeEnum         `json:"portType" validate:"required"`
}

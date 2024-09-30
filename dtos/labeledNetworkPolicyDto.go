package dtos

// type of ingress or egress constant, literal
type K8sNetworkPolicyType string

const (
	Egress  K8sNetworkPolicyType = "egress"
	Ingress K8sNetworkPolicyType = "ingress"
)

type K8sLabeledPortDto struct {
	Port     int32        `json:"port" validate:"required"`
	PortType PortTypeEnum `json:"portType" validate:"required"`
}

type K8sLabeledNetworkPolicies struct {
	Name  string               `json:"name" validate:"required"`
	Type  K8sNetworkPolicyType `json:"type" validate:"required"`
	Ports []K8sLabeledPortDto  `json:"ports" validate:"required"`
}

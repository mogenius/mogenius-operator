package dtos

import v1 "k8s.io/api/networking/v1"

// type Port struct {
// 	Protocol string `json:"protocol" validate:"required"`
// 	Port     int    `json:"port" validate:"required"`
// }

// type of ingress or egress constant, literal
type NetworkPolicyType string

const (
	Egress  NetworkPolicyType = "egress"
	Ingress NetworkPolicyType = "ingress"
)

type LabeledNetworkPolicyParams struct {
	Name  string                 `json:"name" validate:"required"`
	Type  NetworkPolicyType      `json:"type" validate:"required"`
	Ports []v1.NetworkPolicyPort `json:"ports" validate:"required"`
}

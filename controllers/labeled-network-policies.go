package controllers

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/kubernetes"
)

type CreateLabeledNetworkPolicyRequest struct {
	NamespaceName        string                          `json:"namespaceName" validate:"required"`
	LabeledNetworkPolicy dtos.K8sLabeledNetworkPolicyDto `json:"labeledNetworkPolicy" validate:"required"`
}

type CreateLabeledNetworkPolicyResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
}

type LabeledNetworkPoliciesListResponse []dtos.K8sLabeledNetworkPolicyDto

func CreateLabeledNetworkPolicy(data CreateLabeledNetworkPolicyRequest) CreateLabeledNetworkPolicyResponse {
	err := kubernetes.CreateLabeledNetworkPolicy(data.NamespaceName, data.LabeledNetworkPolicy)
	if err != nil {
		return CreateLabeledNetworkPolicyResponse{
			Status:       "FAILURE",
			ErrorMessage: fmt.Sprintf("Failed to create network policy, err: %v", err.Error()),
		}
	}
	return CreateLabeledNetworkPolicyResponse{
		Status: "SUCCESS",
	}
}

func ListLabeledNetworkPolicyPortsExample() LabeledNetworkPoliciesListResponse {
	return []dtos.K8sLabeledNetworkPolicyDto{
		{
			Name: "mogenius-policy-123",
			Type: dtos.Ingress,
			Ports: []dtos.K8sLabeledPortDto{
				{
					Port:     80,
					PortType: dtos.PortTypeHTTPS,
				},
				{
					Port:     443,
					PortType: dtos.PortTypeTCP,
				},
			},
		},
		{
			Name: "mogenius-policy-098",
			Type: dtos.Egress,
			Ports: []dtos.K8sLabeledPortDto{
				{
					Port:     13333,
					PortType: dtos.PortTypeSCTP,
				},
			},
		},
	}
}

func ListLabeledNetworkPolicyPorts() LabeledNetworkPoliciesListResponse {
	return kubernetes.ReadNetworkPolicyPorts()
}

package controllers

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/kubernetes"
)

type CreateLabeledNetworkPolicyRequest struct {
	ControllerName       string                          `json:"controllerName" validate:"required"`
	ControllerType       dtos.K8sServiceControllerEnum   `json:"controllerType" validate:"required"`
	NamespaceName        string                          `json:"namespaceName" validate:"required"`
	LabeledNetworkPolicy dtos.K8sLabeledNetworkPolicyDto `json:"labeledNetworkPolicy" validate:"required"`
}

type status string

const (
	success status = "success"
	failure status = "failure"
)

type CreateLabeledNetworkPolicyResponse struct {
	Status  status `json:"status"`
	Message string `json:"message"`
}

type LabeledNetworkPoliciesListResponse []dtos.K8sLabeledNetworkPolicyDto

func CreateLabeledNetworkPolicy(data CreateLabeledNetworkPolicyRequest) CreateLabeledNetworkPolicyResponse {
	err := kubernetes.EnsureLabeledNetworkPolicy(data.NamespaceName, data.LabeledNetworkPolicy)
	if err != nil {
		return CreateLabeledNetworkPolicyResponse{
			Status:  failure,
			Message: fmt.Sprintf("Failed to create network policy, err: %v", err.Error()),
		}
	}
	err = kubernetes.AttachLabeledNetworkPolicy(data.ControllerName, data.ControllerType, data.NamespaceName, data.LabeledNetworkPolicy)
	if err != nil {
		return CreateLabeledNetworkPolicyResponse{
			Status:  failure,
			Message: fmt.Sprintf("Failed to attach network policy, err: %v", err.Error()),
		}
	}

	return CreateLabeledNetworkPolicyResponse{
		Status: success,
	}
}

func ListLabeledNetworkPolicyPortsExample() LabeledNetworkPoliciesListResponse {
	return []dtos.K8sLabeledNetworkPolicyDto{
		{
			Name:     "mogenius-policy-123",
			Type:     dtos.Ingress,
			Port:     80,
			PortType: dtos.PortTypeHTTPS,
		},
		{
			Name:     "mogenius-policy-098",
			Type:     dtos.Egress,
			Port:     13333,
			PortType: dtos.PortTypeSCTP,
		},
	}
}

func ListLabeledNetworkPolicyPorts() LabeledNetworkPoliciesListResponse {
	return kubernetes.ReadNetworkPolicyPorts()
}

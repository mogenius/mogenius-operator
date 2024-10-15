package controllers

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/kubernetes"
)

type AttachLabeledNetworkPolicyRequest struct {
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

type AttachLabeledNetworkPolicyResponse struct {
	Status  status `json:"status"`
	Message string `json:"message"`
}

type LabeledNetworkPoliciesListResponse []dtos.K8sLabeledNetworkPolicyDto

func AttachLabeledNetworkPolicy(data AttachLabeledNetworkPolicyRequest) AttachLabeledNetworkPolicyResponse {
	err := kubernetes.EnsureLabeledNetworkPolicy(data.NamespaceName, data.LabeledNetworkPolicy)
	if err != nil {
		return AttachLabeledNetworkPolicyResponse{
			Status:  failure,
			Message: fmt.Sprintf("Failed to create network policy, err: %v", err.Error()),
		}
	}
	err = kubernetes.AttachLabeledNetworkPolicy(data.ControllerName, data.ControllerType, data.NamespaceName, data.LabeledNetworkPolicy)
	if err != nil {
		return AttachLabeledNetworkPolicyResponse{
			Status:  failure,
			Message: fmt.Sprintf("Failed to attach network policy, err: %v", err.Error()),
		}
	}

	return AttachLabeledNetworkPolicyResponse{
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
			Type:     dtos.Ingress,
			Port:     13333,
			PortType: dtos.PortTypeSCTP,
		},
	}
}

func ListLabeledNetworkPolicyPorts() LabeledNetworkPoliciesListResponse {
	return kubernetes.ReadNetworkPolicyPorts()
}

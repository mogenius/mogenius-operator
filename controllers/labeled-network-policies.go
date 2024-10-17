package controllers

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/kubernetes"
)

type status string

const (
	success status = "success"
	failure status = "failure"
)

type DetachLabeledNetworkPolicyResponse struct {
	Status  status `json:"status"`
	Message string `json:"message"`
}
type DetachLabeledNetworkPolicyRequest struct {
	ControllerName       string                          `json:"controllerName" validate:"required"`
	ControllerType       dtos.K8sServiceControllerEnum   `json:"controllerType" validate:"required"`
	NamespaceName        string                          `json:"namespaceName" validate:"required"`
	LabeledNetworkPolicy dtos.K8sLabeledNetworkPolicyDto `json:"labeledNetworkPolicy" validate:"required"`
}

type LabeledNetworkPoliciesListResponse []dtos.K8sLabeledNetworkPolicyDto

func DetachLabeledNetworkPolicy(data DetachLabeledNetworkPolicyRequest) DetachLabeledNetworkPolicyResponse {
	err := kubernetes.DetachLabeledNetworkPolicy(data.ControllerName, data.ControllerType, data.NamespaceName, data.LabeledNetworkPolicy)
	if err != nil {
		return DetachLabeledNetworkPolicyResponse{
			Status:  failure,
			Message: fmt.Sprintf("Failed to detach network policy, err: %v", err.Error()),
		}
	}

	return DetachLabeledNetworkPolicyResponse{
		Status: success,
	}
}

type AttachLabeledNetworkPolicyRequest struct {
	ControllerName       string                          `json:"controllerName" validate:"required"`
	ControllerType       dtos.K8sServiceControllerEnum   `json:"controllerType" validate:"required"`
	NamespaceName        string                          `json:"namespaceName" validate:"required"`
	LabeledNetworkPolicy dtos.K8sLabeledNetworkPolicyDto `json:"labeledNetworkPolicy" validate:"required"`
}

type AttachLabeledNetworkPolicyResponse struct {
	Status  status `json:"status"`
	Message string `json:"message"`
}

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

type RemoveConflictingNetworkPoliciesRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
}

type RemoveConflictingNetworkPoliciesResponse struct {
	Status  status `json:"status"`
	Message string `json:"message"`
}

func RemoveConflictingNetworkPolicies(data RemoveConflictingNetworkPoliciesRequest) RemoveConflictingNetworkPoliciesResponse {
	err := kubernetes.RemoveAllConflictingNetworkPolicies(data.NamespaceName)
	if err != nil {
		return RemoveConflictingNetworkPoliciesResponse{
			Status:  failure,
			Message: fmt.Sprintf("Failed to delete all network policies, err: %v", err.Error()),
		}
	}

	return RemoveConflictingNetworkPoliciesResponse{
		Status: success,
	}
}

package controllers

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/kubernetes"

	punqUtils "github.com/mogenius/punq/utils"
	v1 "k8s.io/api/networking/v1"
)

type DetachLabeledNetworkPolicyRequest struct {
	ControllerName       string                          `json:"controllerName" validate:"required"`
	ControllerType       dtos.K8sServiceControllerEnum   `json:"controllerType" validate:"required"`
	NamespaceName        string                          `json:"namespaceName" validate:"required"`
	LabeledNetworkPolicy dtos.K8sLabeledNetworkPolicyDto `json:"labeledNetworkPolicy" validate:"required"`
}

//type LabeledNetworkPoliciesListResponse []dtos.K8sLabeledNetworkPolicyDto

func DetachLabeledNetworkPolicy(data DetachLabeledNetworkPolicyRequest) (string, error) {
	err := kubernetes.DetachLabeledNetworkPolicy(data.ControllerName, data.ControllerType, data.NamespaceName, data.LabeledNetworkPolicy)
	if err != nil {
		return "", fmt.Errorf("failed to detach network policy, err: %s", err.Error())
	}

	return "", nil
}

type AttachLabeledNetworkPolicyRequest struct {
	ControllerName       string                          `json:"controllerName" validate:"required"`
	ControllerType       dtos.K8sServiceControllerEnum   `json:"controllerType" validate:"required"`
	NamespaceName        string                          `json:"namespaceName" validate:"required"`
	LabeledNetworkPolicy dtos.K8sLabeledNetworkPolicyDto `json:"labeledNetworkPolicy" validate:"required"`
}

func AttachLabeledNetworkPolicy(data AttachLabeledNetworkPolicyRequest) (string, error) {
	err := kubernetes.EnsureLabeledNetworkPolicy(data.NamespaceName, data.LabeledNetworkPolicy)
	if err != nil {
		return "", fmt.Errorf("failed to create network policy, err: %s", err.Error())
	}
	err = kubernetes.AttachLabeledNetworkPolicy(data.ControllerName, data.ControllerType, data.NamespaceName, data.LabeledNetworkPolicy)
	if err != nil {
		return "", fmt.Errorf("failed to attach network policy, err: %s", err.Error())
	}

	return "", nil
}

func ListLabeledNetworkPolicyPortsExample() []dtos.K8sLabeledNetworkPolicyDto {
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

func ListLabeledNetworkPolicyPorts() ([]dtos.K8sLabeledNetworkPolicyDto, error) {
	list, err := kubernetes.ReadNetworkPolicyPorts()
	if err != nil {
		return []dtos.K8sLabeledNetworkPolicyDto{}, fmt.Errorf("failed to list all network policies, err: %s", err.Error())
	}
	return list, nil
}

type RemoveConflictingNetworkPoliciesRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
}

func RemoveConflictingNetworkPolicies(data RemoveConflictingNetworkPoliciesRequest) (string, error) {
	err := kubernetes.RemoveAllConflictingNetworkPolicies(data.NamespaceName)
	if err != nil {
		return "", fmt.Errorf("failed to delete all network policies, err: %s", err.Error())
	}

	return "", nil
}

type ListConflictingNetworkPoliciesRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
}

type K8sConflictingNetworkPolicyDto struct {
	NamespaceName string               `json:"namespaceName"`
	Name          *string              `json:"name,omitempty"`
	Spec          v1.NetworkPolicySpec `json:"spec"`
	// NetworkPolicy  v1.NetworkPolicy               `json:"networkPolicy"`
}

func ListAllConflictingNetworkPolicies(data ListConflictingNetworkPoliciesRequest) ([]K8sConflictingNetworkPolicyDto, error) {
	policies, err := kubernetes.ListAllConflictingNetworkPolicies(data.NamespaceName)
	if err != nil {
		return []K8sConflictingNetworkPolicyDto{}, fmt.Errorf("failed to list all network policies, err: %s", err.Error())
	}

	var dataList []K8sConflictingNetworkPolicyDto
	for _, policy := range policies.Items {
		data := K8sConflictingNetworkPolicyDto{
			Name:          punqUtils.Pointer(policy.Name),
			NamespaceName: policy.Namespace,
			Spec:          policy.Spec,
			// NetworkPolicy:  policy,
		}
		dataList = append(dataList, data)
	}

	return dataList, nil
}

type ListControllerLabeledNetworkPoliciesRequest struct {
	ControllerName string                        `json:"controllerName" validate:"required"`
	ControllerType dtos.K8sServiceControllerEnum `json:"controllerType" validate:"required"`
	NamespaceName  string                        `json:"namespaceName" validate:"required"`
}

type ListControllerLabeledNetworkPoliciesResponse struct {
	ControllerName         string                            `json:"controllerName" validate:"required"`
	ControllerType         dtos.K8sServiceControllerEnum     `json:"controllerType" validate:"required"`
	NamespaceName          string                            `json:"namespaceName" validate:"required"`
	LabeledNetworkPolicies []dtos.K8sLabeledNetworkPolicyDto `json:"labeledNetworkPolicy" validate:"required"`
}

func ListControllerLabeledNetwork(data ListControllerLabeledNetworkPoliciesRequest) (ListControllerLabeledNetworkPoliciesResponse, error) {
	policies, err := kubernetes.ListControllerLabeledNetworkPolicies(data.NamespaceName)
	if err != nil {
		return ListControllerLabeledNetworkPoliciesResponse{}, err
	}

	return ListControllerLabeledNetworkPoliciesResponse{
		ControllerName:         data.ControllerName,
		ControllerType:         data.ControllerType,
		NamespaceName:          data.NamespaceName,
		LabeledNetworkPolicies: policies,
	}, nil
}

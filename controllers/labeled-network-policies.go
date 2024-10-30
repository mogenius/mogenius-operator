package controllers

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/kubernetes"
	"strings"

	punqUtils "github.com/mogenius/punq/utils"
	v1 "k8s.io/api/networking/v1"
)

const logType = "NETWORK POLICY"

type DetachLabeledNetworkPolicyRequest struct {
	ControllerName         string                            `json:"controllerName" validate:"required"`
	ControllerType         dtos.K8sServiceControllerEnum     `json:"controllerType" validate:"required"`
	NamespaceName          string                            `json:"namespaceName" validate:"required"`
	LabeledNetworkPolicies []dtos.K8sLabeledNetworkPolicyDto `json:"labeledNetworkPolicies" validate:"required"`
}

//type LabeledNetworkPoliciesListResponse []dtos.K8sLabeledNetworkPolicyDto

func DetachLabeledNetworkPolicy(data DetachLabeledNetworkPolicyRequest) (string, error) {
	if len(data.LabeledNetworkPolicies) == 0 {
		return "", nil
	}

	// log
	logWithFields := ControllerLogger.With("namespace", data.NamespaceName, "controllerName", data.ControllerName)
	var labeledNetworkPolicyNameStrings []string
	for _, labeledNetworkPolicy := range data.LabeledNetworkPolicies {
		labeledNetworkPolicyNameStrings = append(labeledNetworkPolicyNameStrings, labeledNetworkPolicy.Name)
	}
	labeledNetworkPolicyNames := strings.Join(labeledNetworkPolicyNameStrings, ", ")
	logWithFields.Info(fmt.Sprintf("   %s Detach network policy %s from %s", logType, labeledNetworkPolicyNames, data.ControllerName))

	err := kubernetes.DetachLabeledNetworkPolicies(data.ControllerName, data.ControllerType, data.NamespaceName, data.LabeledNetworkPolicies)
	if err != nil {
		logWithFields.Error(fmt.Sprintf("  %s failed to detach network policy, err: %s", logType, err.Error()))
		return "", fmt.Errorf("failed to detach network policy, err: %s", err.Error())
	}

	logWithFields.Info(fmt.Sprintf("   %s Network policy %s detached from %s", logType, labeledNetworkPolicyNames, data.ControllerName))
	return "", nil
}

type AttachLabeledNetworkPolicyRequest struct {
	ControllerName         string                            `json:"controllerName" validate:"required"`
	ControllerType         dtos.K8sServiceControllerEnum     `json:"controllerType" validate:"required"`
	NamespaceName          string                            `json:"namespaceName" validate:"required"`
	LabeledNetworkPolicies []dtos.K8sLabeledNetworkPolicyDto `json:"labeledNetworkPolicies" validate:"required"`
}

func AttachLabeledNetworkPolicy(data AttachLabeledNetworkPolicyRequest) (string, error) {
	if len(data.LabeledNetworkPolicies) == 0 {
		return "", nil
	}

	if data.NamespaceName == "kube-system" {
		return "", fmt.Errorf("cannot attach network policy to kube-system namespace")
	}

	// log
	logWithFields := ControllerLogger.With("namespace", data.NamespaceName, "controllerName", data.ControllerName)
	var labeledNetworkPolicyNameStrings []string
	for _, labeledNetworkPolicy := range data.LabeledNetworkPolicies {
		labeledNetworkPolicyNameStrings = append(labeledNetworkPolicyNameStrings, labeledNetworkPolicy.Name)
	}
	labeledNetworkPolicyNames := strings.Join(labeledNetworkPolicyNameStrings, ", ")
	logWithFields.Info(fmt.Sprintf("   %s Attach network policy %s to %s", logType, labeledNetworkPolicyNames, data.ControllerName))

	// create kind: NetworkPolicy
	err := kubernetes.EnsureLabeledNetworkPolicies(data.NamespaceName, data.LabeledNetworkPolicies)
	if err != nil {
		logWithFields.Error(fmt.Sprintf("  %s Failed to create network policy, err: %s", logType, err.Error()))
		return "", fmt.Errorf("failed to create network policy, err: %s", err.Error())
	}
	err = kubernetes.AttachLabeledNetworkPolicies(data.ControllerName, data.ControllerType, data.NamespaceName, data.LabeledNetworkPolicies)
	if err != nil {
		logWithFields.Error(fmt.Sprintf("   %s Failed to attach network policy, err: %s", logType, err.Error()))
		return "", fmt.Errorf("failed to attach network policy, err: %s", err.Error())
	}

	logWithFields.Info(fmt.Sprintf("   %s Network policy %s attached to %s", logType, labeledNetworkPolicyNames, data.ControllerName))
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
	LabeledNetworkPolicies []dtos.K8sLabeledNetworkPolicyDto `json:"labeledNetworkPolicies" validate:"required"`
}

func ListControllerLabeledNetwork(data ListControllerLabeledNetworkPoliciesRequest) (ListControllerLabeledNetworkPoliciesResponse, error) {
	// log
	logWithFields := ControllerLogger.With("namespace", data.NamespaceName, "controllerName", data.ControllerName)

	policies, err := kubernetes.ListControllerLabeledNetworkPolicies(data.ControllerName, data.ControllerType, data.NamespaceName)
	if err != nil {
		logWithFields.Error(fmt.Sprintf("  %s failed to list network policies, err: %s", logType, err.Error()))
		return ListControllerLabeledNetworkPoliciesResponse{}, err
	}

	return ListControllerLabeledNetworkPoliciesResponse{
		ControllerName:         data.ControllerName,
		ControllerType:         data.ControllerType,
		NamespaceName:          data.NamespaceName,
		LabeledNetworkPolicies: policies,
	}, nil
}

package controllers

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/kubernetes"
	"strings"

	punqUtils "github.com/mogenius/punq/utils"
	v1Core "k8s.io/api/core/v1"
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
	logWithFields := controllerLogger.With("namespace", data.NamespaceName, "controllerName", data.ControllerName)
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
	logWithFields := controllerLogger.With("namespace", data.NamespaceName, "controllerName", data.ControllerName)
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
	logWithFields := controllerLogger.With("namespace", data.NamespaceName, "controllerName", data.ControllerName)

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

func UpdateNetworkPolicyTemplate(policies []kubernetes.NetworkPolicy) error {
	return kubernetes.UpdateNetworkPolicyTemplate(policies)
}

type ListNetworkPolicyResponse struct {
	Namespaces []ListNetworkPolicyNamespace `json:"namespaces" validate:"required"`
}

type ListNetworkPolicyNamespace struct {
	Id                string                           `json:"id" validate:"required"`
	DisplayName       string                           `json:"displayName" validate:"required"`
	Name              string                           `json:"name" validate:"required"`
	ProjectId         string                           `json:"projectId" validate:"required"`
	Controllers       []ListNetworkPolicyController    `json:"controllers" validate:"required"`
	UnmanagedPolicies []K8sConflictingNetworkPolicyDto `json:"unmanagedPolicies" validate:"required"`
}

type ListNetworkPolicyController struct {
	ControllerName         string                            `json:"controllerName" validate:"required"`
	ControllerType         dtos.K8sServiceControllerEnum     `json:"controllerType" validate:"required"`
	ServiceId              string                            `json:"serviceId" validate:"required"`
	LabeledNetworkPolicies []dtos.K8sLabeledNetworkPolicyDto `json:"networkPolicies" validate:"required"`
}

func createNetworkPolicyDto(name string, spec v1.NetworkPolicySpec) dtos.K8sLabeledNetworkPolicyDto {
	var typeOf dtos.K8sNetworkPolicyType
	var port uint16
	var portType dtos.PortTypeEnum

	if len(spec.Ingress) > 0 {
		typeOf = dtos.Ingress

		port = uint16(spec.Ingress[0].Ports[0].Port.IntVal)

		if spec.Ingress[0].Ports[0].Protocol == nil {
			portType = dtos.PortTypeTCP
		} else {
			switch *spec.Ingress[0].Ports[0].Protocol {
			case v1Core.ProtocolUDP:
				portType = dtos.PortTypeUDP
			case v1Core.ProtocolSCTP:
				portType = dtos.PortTypeSCTP
			default:
				portType = dtos.PortTypeTCP
			}
		}
	} else if len(spec.Egress) > 0 {
		typeOf = dtos.Egress

		port = uint16(spec.Egress[0].Ports[0].Port.IntVal)

		if spec.Egress[0].Ports[0].Protocol == nil {
			portType = dtos.PortTypeTCP
		} else {
			switch *spec.Egress[0].Ports[0].Protocol {
			case v1Core.ProtocolUDP:
				portType = dtos.PortTypeUDP
			case v1Core.ProtocolSCTP:
				portType = dtos.PortTypeSCTP
			default:
				portType = dtos.PortTypeTCP
			}
		}
	}

	return dtos.K8sLabeledNetworkPolicyDto{
		Name:     name,
		Type:     typeOf,
		Port:     port,
		PortType: portType,
	}
}

func ListAllNetworkPolicies() ([]ListNetworkPolicyNamespace, error) {
	//
	namespaces, err := kubernetes.ListAllNamespaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list all namespaces, err: %s", err.Error())
	}

	// ignore errors
	policies, _ := kubernetes.ListAllNetworkPolicies("")

	managedMap := make(map[string]int)
	unmanagedMap := make(map[string][]int)

	for idx, policy := range policies {
		isManaged := policy.Labels != nil && policy.Labels[kubernetes.NetpolLabel] == "true"
		if isManaged {
			// managed
			managedKey := fmt.Sprintf("%s--%s", policy.Namespace, policy.Name)
			managedMap[managedKey] = idx
		} else {
			// unmanaged
			unmanagedKey := policy.Namespace
			unmanagedMap[unmanagedKey] = append(unmanagedMap[unmanagedKey], idx)
		}
	}

	var namespacesDto []ListNetworkPolicyNamespace
	for _, namespace := range namespaces {
		namespaceDto := ListNetworkPolicyNamespace{
			Name: namespace.Name,
		}

		// ignore errors
		deployments, _ := kubernetes.ListAllDeployments(namespace.Name)
		for _, deployment := range deployments {
			controllerDto := ListNetworkPolicyController{
				ControllerName: deployment.Name,
				ControllerType: dtos.DEPLOYMENT,
			}

			if deployment.Spec.Template.Labels != nil {
				for key, _ := range deployment.Spec.Template.Labels {
					managedKey := fmt.Sprintf("%s--%s", namespace.Name, key)
					if idx, ok := managedMap[managedKey]; ok {
						networkPolicyDto := createNetworkPolicyDto(policies[idx].Name, policies[idx].Spec)
						controllerDto.LabeledNetworkPolicies = append(controllerDto.LabeledNetworkPolicies, networkPolicyDto)
					}
				}
			}

			namespaceDto.Controllers = append(namespaceDto.Controllers, controllerDto)
		}

		// ignore errors
		daemonsets, _ := kubernetes.ListAllDaemonSets(namespace.Name)
		for _, daemonset := range daemonsets {
			controllerDto := ListNetworkPolicyController{
				ControllerName: daemonset.Name,
				ControllerType: dtos.DAEMON_SET,
			}

			if daemonset.Spec.Template.Labels != nil {
				for key, _ := range daemonset.Spec.Template.Labels {
					managedKey := fmt.Sprintf("%s--%s", namespace.Name, key)
					if idx, ok := managedMap[managedKey]; ok {
						networkPolicyDto := createNetworkPolicyDto(policies[idx].Name, policies[idx].Spec)
						controllerDto.LabeledNetworkPolicies = append(controllerDto.LabeledNetworkPolicies, networkPolicyDto)
					}
				}
			}

			namespaceDto.Controllers = append(namespaceDto.Controllers, controllerDto)
		}

		// ignore errors
		statefulsets, _ := kubernetes.ListAllStatefulSets(namespace.Name)
		for _, statefulset := range statefulsets {
			controllerDto := ListNetworkPolicyController{
				ControllerName: statefulset.Name,
				ControllerType: dtos.STATEFUL_SET,
			}

			if statefulset.Spec.Template.Labels != nil {
				for key, _ := range statefulset.Spec.Template.Labels {
					managedKey := fmt.Sprintf("%s--%s", namespace.Name, key)
					if idx, ok := managedMap[managedKey]; ok {
						networkPolicyDto := createNetworkPolicyDto(policies[idx].Name, policies[idx].Spec)
						controllerDto.LabeledNetworkPolicies = append(controllerDto.LabeledNetworkPolicies, networkPolicyDto)
					}
				}
			}

			namespaceDto.Controllers = append(namespaceDto.Controllers, controllerDto)
		}

		for _, idx := range unmanagedMap[namespace.Name] {
			policy := policies[idx]

			conflictingNetworkPolicyDto := K8sConflictingNetworkPolicyDto{
				Name:          punqUtils.Pointer(policy.Name),
				NamespaceName: policy.Namespace,
				Spec:          policy.Spec,
				// NetworkPolicy:  policy,
			}

			namespaceDto.UnmanagedPolicies = append(namespaceDto.UnmanagedPolicies, conflictingNetworkPolicyDto)
		}

		namespacesDto = append(namespacesDto, namespaceDto)
	}

	return namespacesDto, nil
}

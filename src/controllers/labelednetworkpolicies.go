package controllers

import (
	"fmt"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"
	"sort"
	"strings"

	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var config cfg.ConfigModule

const logType = "NETWORK POLICY"

func ignoreNamespaces() []string {
	return []string{"kube-public", "kube-system", "kube-node-lease", config.Get("MO_OWN_NAMESPACE")}
}

type DetachLabeledNetworkPolicyRequest struct {
	ControllerName         string                            `json:"controllerName" validate:"required"`
	ControllerType         dtos.K8sServiceControllerEnum     `json:"controllerType" validate:"required"`
	NamespaceName          string                            `json:"namespaceName" validate:"required"`
	LabeledNetworkPolicies []dtos.K8sLabeledNetworkPolicyDto `json:"labeledNetworkPolicies" validate:"required"`
}

func ignoreNamespace(namespaceName string) error {
	for _, ns := range ignoreNamespaces() {
		if namespaceName == ns {
			return fmt.Errorf("cannot attach network policy to %s namespace", namespaceName)
		}
	}
	return nil
}

func DetachLabeledNetworkPolicy(data DetachLabeledNetworkPolicyRequest) (string, error) {
	if len(data.LabeledNetworkPolicies) == 0 {
		return "", nil
	}

	// ignore namespace
	err := ignoreNamespace(data.NamespaceName)
	if err != nil {
		return "", err
	}

	// log
	logWithFields := controllerLogger.With("namespace", data.NamespaceName, "controllerName", data.ControllerName)
	var labeledNetworkPolicyNameStrings []string
	for _, labeledNetworkPolicy := range data.LabeledNetworkPolicies {
		labeledNetworkPolicyNameStrings = append(labeledNetworkPolicyNameStrings, labeledNetworkPolicy.Name)
	}
	labeledNetworkPolicyNames := strings.Join(labeledNetworkPolicyNameStrings, ", ")
	logWithFields.Info("detach network policy", "logType", logType, "policies", labeledNetworkPolicyNames, "controller", data.ControllerName)

	err = kubernetes.DetachLabeledNetworkPolicies(data.ControllerName, data.ControllerType, data.NamespaceName, data.LabeledNetworkPolicies)
	if err != nil {
		logWithFields.Error("failed to detach network policy", "logType", logType, "error", err)
		return "", fmt.Errorf("failed to detach network policy, err: %s", err.Error())
	}

	logWithFields.Info("Network policy detached", "logType", logType, "policies", labeledNetworkPolicyNames, "controller", data.ControllerName)
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

	// ignore namespace
	err := ignoreNamespace(data.NamespaceName)
	if err != nil {
		return "", err
	}

	// log
	logWithFields := controllerLogger.With("namespace", data.NamespaceName, "controllerName", data.ControllerName)
	var labeledNetworkPolicyNameStrings []string
	for _, labeledNetworkPolicy := range data.LabeledNetworkPolicies {
		labeledNetworkPolicyNameStrings = append(labeledNetworkPolicyNameStrings, labeledNetworkPolicy.Name)
	}
	labeledNetworkPolicyNames := strings.Join(labeledNetworkPolicyNameStrings, ", ")
	logWithFields.Info("attach network policy", "logType", logType, "policies", labeledNetworkPolicyNames, "controller", data.ControllerName)

	// create kind: NetworkPolicy
	err = kubernetes.EnsureLabeledNetworkPolicies(data.NamespaceName, data.LabeledNetworkPolicies)
	if err != nil {
		logWithFields.Error("failed to create network policy", "logType", logType, "error", err)
		return "", fmt.Errorf("failed to create network policy, err: %s", err.Error())
	}
	err = kubernetes.AttachLabeledNetworkPolicies(data.ControllerName, data.ControllerType, data.NamespaceName, data.LabeledNetworkPolicies)
	if err != nil {
		logWithFields.Error("failed to attach network policy", "logType", logType, "error", err)
		return "", fmt.Errorf("failed to attach network policy, err: %s", err.Error())
	}

	logWithFields.Info("Network policy attached", "logType", logType, "policies", labeledNetworkPolicyNames, "controller", data.ControllerName)
	return "", nil
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
	// ignore namespace
	err := ignoreNamespace(data.NamespaceName)
	if err != nil {
		return "", err
	}

	err = kubernetes.RemoveAllConflictingNetworkPolicies(data.NamespaceName)
	if err != nil {
		return "", fmt.Errorf("failed to delete all network policies, err: %s", err.Error())
	}

	return "", nil
}

type ListConflictingNetworkPoliciesRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
}

type K8sNetworkPolicyDto struct {
	NamespaceName string               `json:"namespaceName"`
	Name          *string              `json:"name,omitempty"`
	Spec          v1.NetworkPolicySpec `json:"spec"`
	// NetworkPolicy  v1.NetworkPolicy               `json:"networkPolicy"`
}

type K8sConflictingNetworkPolicyDto = K8sNetworkPolicyDto

func ListAllConflictingNetworkPolicies(data ListConflictingNetworkPoliciesRequest) ([]K8sConflictingNetworkPolicyDto, error) {
	policies, err := kubernetes.ListAllConflictingNetworkPolicies(data.NamespaceName)
	if err != nil {
		return []K8sConflictingNetworkPolicyDto{}, fmt.Errorf("failed to list all network policies, err: %s", err.Error())
	}

	var dataList []K8sConflictingNetworkPolicyDto
	for _, policy := range policies.Items {
		data := K8sConflictingNetworkPolicyDto{
			Name:          utils.Pointer(policy.Name),
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
		logWithFields.Error("failed to list network policies", "logType", logType, "error", err)
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
	ManagedPolicies   []K8sNetworkPolicyDto            `json:"managedPolicies" validate:"required"`
}

type ListNetworkPolicyController struct {
	ControllerName         string                            `json:"controllerName" validate:"required"`
	ControllerType         dtos.K8sServiceControllerEnum     `json:"controllerType" validate:"required"`
	ServiceId              string                            `json:"serviceId" validate:"required"`
	LabeledNetworkPolicies []dtos.K8sLabeledNetworkPolicyDto `json:"networkPolicies" validate:"required"`
}

type ListManagedAndUnmanagedNetworkPolicyNamespace struct {
	Id                string                           `json:"id" validate:"required"`
	DisplayName       string                           `json:"displayName" validate:"required"`
	Name              string                           `json:"name" validate:"required"`
	ProjectId         string                           `json:"projectId" validate:"required"`
	UnmanagedPolicies []K8sConflictingNetworkPolicyDto `json:"unmanagedPolicies" validate:"required"`
	ManagedPolicies   []K8sNetworkPolicyDto            `json:"managedPolicies" validate:"required"`
}

func createNetworkPolicyDto(name string, spec v1.NetworkPolicySpec) dtos.K8sLabeledNetworkPolicyDto {
	var typeOf dtos.K8sNetworkPolicyType
	var port uint16
	var portType dtos.PortTypeEnum

	if len(spec.Ingress) > 0 && len(spec.Ingress[0].Ports) == 1 && spec.Ingress[0].Ports[0].Port != nil {
		typeOf = dtos.Ingress
		port = uint16(spec.Ingress[0].Ports[0].Port.IntVal)
		portType = dtos.PortTypeEnum(extractNetworkPolicyPortProtocol(spec.Ingress[0].Ports[0].Protocol))
	} else if len(spec.Egress) > 0 && len(spec.Egress[0].Ports) == 1 && spec.Egress[0].Ports[0].Port != nil {
		typeOf = dtos.Egress
		port = uint16(spec.Egress[0].Ports[0].Port.IntVal)
		portType = dtos.PortTypeEnum(extractNetworkPolicyPortProtocol(spec.Egress[0].Ports[0].Protocol))
	}

	return dtos.K8sLabeledNetworkPolicyDto{
		Name:     name,
		Type:     typeOf,
		Port:     port,
		PortType: portType,
	}
}

func extractNetworkPolicyPortProtocol(protocol *v1Core.Protocol) dtos.PortTypeEnum {
	var portType dtos.PortTypeEnum
	if protocol == nil {
		portType = dtos.PortTypeTCP
	} else {
		switch *protocol {
		case v1Core.ProtocolUDP:
			portType = dtos.PortTypeUDP
		case v1Core.ProtocolSCTP:
			portType = dtos.PortTypeSCTP
		default:
			portType = dtos.PortTypeTCP
		}
	}
	return portType
}

type ListNamespaceLabeledNetworkPoliciesRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
}

type EnforceNetworkPolicyManagerRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
}

type DisableNetworkPolicyManagerRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
}

func EnforceNetworkPolicyManager(namespaceName string) error {
	// ignore namespace
	err := ignoreNamespace(namespaceName)
	if err != nil {
		return err
	}
	return kubernetes.EnforceNetworkPolicyManagerForNamespace(namespaceName)
}

func DisableNetworkPolicyManager(namespaceName string) error {
	// ignore namespace
	err := ignoreNamespace(namespaceName)
	if err != nil {
		return err
	}
	return kubernetes.DisableNetworkPolicyManagerForNamespace(namespaceName)
}

func ListNamespaceNetworkPolicies(valkeyClient valkeyclient.ValkeyClient, data ListNamespaceLabeledNetworkPoliciesRequest) ([]ListNetworkPolicyNamespace, error) {
	namespace := store.GetNamespace(valkeyClient, data.NamespaceName, controllerLogger)
	if namespace == nil {
		return nil, fmt.Errorf("failed to get namespace")
	}

	policies, err := store.ListNetworkPolicies(valkeyClient, data.NamespaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to list network policies, err: %s", err.Error())
	}

	return listNetworkPoliciesByNamespaces([]v1Core.Namespace{*namespace}, policies)
}

func ListAllNetworkPolicies(valkeyClient valkeyclient.ValkeyClient) ([]ListNetworkPolicyNamespace, error) {
	namespaces, err := store.ListAllNamespaces(valkeyClient)
	if err != nil {
		return nil, fmt.Errorf("failed to list all namespaces, err: %s", err.Error())
	}

	policies, err := store.ListNetworkPolicies(valkeyClient, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list network policies, err: %s", err.Error())
	}

	return listNetworkPoliciesByNamespaces(namespaces, policies)
}

func listNetworkPoliciesByNamespaces(namespaces []v1Core.Namespace, policies []v1.NetworkPolicy) ([]ListNetworkPolicyNamespace, error) {
	managedControllerMap := make(map[string]int)
	unmanagedMap := make(map[string][]int)
	managedMap := make(map[string][]int)

	for idx, policy := range policies {
		hasLabels := policy.ObjectMeta.Labels != nil
		isManagedMogeniusControllerNetworkPolicy := hasLabels && policy.ObjectMeta.Labels[kubernetes.NetpolLabel] == "true" && policy.Name != kubernetes.DenyAllIngressNetPolName && policy.Name != kubernetes.AllowNamespaceCommunicationNetPolName
		isManagedMogeniusNamespaceNetworkPolicy := hasLabels && policy.ObjectMeta.Labels[kubernetes.NetpolLabel] == "true" && (policy.Name == kubernetes.DenyAllIngressNetPolName || policy.Name == kubernetes.AllowNamespaceCommunicationNetPolName)
		isManagedLegacyMogeniusNamespaceNetworkPolicy := hasLabels && policy.ObjectMeta.Labels["mo-created-by"] == "mogenius-k8s-manager" && func() bool {
			_, exists := policy.ObjectMeta.Labels["mo-app"]
			return !exists
		}()
		if isManagedMogeniusControllerNetworkPolicy {
			// managed controller
			managedKey := fmt.Sprintf("%s--%s", policy.Namespace, policy.Name)
			managedControllerMap[managedKey] = idx
		} else if isManagedMogeniusNamespaceNetworkPolicy || isManagedLegacyMogeniusNamespaceNetworkPolicy {
			// managed namespace
			managedKey := policy.Namespace
			managedMap[managedKey] = append(managedMap[managedKey], idx)
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

		controllers := []unstructured.Unstructured{}
		depls, _ := kubernetes.GetUnstructuredResourceList("apps/v1", "", "deployments", &namespace.Name)
		if depls != nil {
			controllers = append(controllers, depls.Items...)
		}
		dmSets, _ := kubernetes.GetUnstructuredResourceList("apps/v1", "", "daemonsets", &namespace.Name)
		if dmSets != nil {
			controllers = append(controllers, dmSets.Items...)
		}
		sfs, _ := kubernetes.GetUnstructuredResourceList("apps/v1", "", "statefulsets", &namespace.Name)
		if sfs != nil {
			controllers = append(controllers, sfs.Items...)
		}

		for _, ctrl := range controllers {
			controllerDto := ListNetworkPolicyController{
				ControllerName: ctrl.GetName(),
				ControllerType: dtos.K8sServiceControllerEnum(ctrl.GetKind()),
			}

			if ctrl.GetLabels() != nil {
				for key := range ctrl.GetLabels() {
					managedKey := fmt.Sprintf("%s--%s", namespace.Name, key)
					if idx, ok := managedControllerMap[managedKey]; ok {
						networkPolicyDto := createNetworkPolicyDto(policies[idx].Name, policies[idx].Spec)
						controllerDto.LabeledNetworkPolicies = append(controllerDto.LabeledNetworkPolicies, networkPolicyDto)
					}
				}
			}

			sort.Slice(controllerDto.LabeledNetworkPolicies, func(i, j int) bool {
				// sort by port
				if controllerDto.LabeledNetworkPolicies[i].Port != controllerDto.LabeledNetworkPolicies[j].Port {
					return controllerDto.LabeledNetworkPolicies[i].Port < controllerDto.LabeledNetworkPolicies[j].Port
				}
				// sort type
				return controllerDto.LabeledNetworkPolicies[i].Type < controllerDto.LabeledNetworkPolicies[j].Type
			})

			namespaceDto.Controllers = append(namespaceDto.Controllers, controllerDto)
		}

		for _, idx := range managedMap[namespace.Name] {
			policy := policies[idx]

			networkPolicyDto := K8sNetworkPolicyDto{
				Name:          utils.Pointer(policy.Name),
				NamespaceName: policy.Namespace,
				Spec:          policy.Spec,
			}

			namespaceDto.ManagedPolicies = append(namespaceDto.ManagedPolicies, networkPolicyDto)
		}

		for _, idx := range unmanagedMap[namespace.Name] {
			policy := policies[idx]

			conflictingNetworkPolicyDto := K8sConflictingNetworkPolicyDto{
				Name:          utils.Pointer(policy.Name),
				NamespaceName: policy.Namespace,
				Spec:          policy.Spec,
			}

			namespaceDto.UnmanagedPolicies = append(namespaceDto.UnmanagedPolicies, conflictingNetworkPolicyDto)
		}

		namespacesDto = append(namespacesDto, namespaceDto)
	}

	return namespacesDto, nil
}

func ListManagedAndUnmanagedNamespaceNetworkPolicies(valkeyClient valkeyclient.ValkeyClient, data ListNamespaceLabeledNetworkPoliciesRequest) ([]ListManagedAndUnmanagedNetworkPolicyNamespace, error) {
	namespace := store.GetNamespace(valkeyClient, data.NamespaceName, controllerLogger)
	if namespace == nil {
		return nil, fmt.Errorf("failed to get namespace")
	}

	policies, err := store.ListNetworkPolicies(valkeyClient, data.NamespaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to list network policies, err: %s", err.Error())
	}

	return listManagedAndUnmanagedNamespaceNetworkPoliciesByNamespaces([]v1Core.Namespace{*namespace}, policies)
}

func listManagedAndUnmanagedNamespaceNetworkPoliciesByNamespaces(namespaces []v1Core.Namespace, policies []v1.NetworkPolicy) ([]ListManagedAndUnmanagedNetworkPolicyNamespace, error) {
	unmanagedMap := make(map[string][]int)
	managedMap := make(map[string][]int)

	for idx, policy := range policies {
		hasLabels := policy.ObjectMeta.Labels != nil
		isManagedMogeniusControllerNetworkPolicy := hasLabels && policy.ObjectMeta.Labels[kubernetes.NetpolLabel] == "true" && policy.Name != kubernetes.DenyAllIngressNetPolName && policy.Name != kubernetes.AllowNamespaceCommunicationNetPolName
		isManagedMogeniusNamespaceNetworkPolicy := hasLabels && policy.ObjectMeta.Labels[kubernetes.NetpolLabel] == "true" && (policy.Name == kubernetes.DenyAllIngressNetPolName || policy.Name == kubernetes.AllowNamespaceCommunicationNetPolName)
		isManagedLegacyMogeniusNamespaceNetworkPolicy := hasLabels && policy.ObjectMeta.Labels["mo-created-by"] == "mogenius-k8s-manager" && func() bool {
			_, exists := policy.ObjectMeta.Labels["mo-app"]
			return !exists
		}()

		if isManagedMogeniusControllerNetworkPolicy {
			// managed controller
		} else if isManagedMogeniusNamespaceNetworkPolicy || isManagedLegacyMogeniusNamespaceNetworkPolicy {
			// managed namespace
			managedKey := policy.Namespace
			managedMap[managedKey] = append(managedMap[managedKey], idx)
		} else {
			// unmanaged
			unmanagedKey := policy.Namespace
			unmanagedMap[unmanagedKey] = append(unmanagedMap[unmanagedKey], idx)
		}
	}

	var namespacesDto []ListManagedAndUnmanagedNetworkPolicyNamespace

	for _, namespace := range namespaces {
		namespaceDto := ListManagedAndUnmanagedNetworkPolicyNamespace{
			Name: namespace.Name,
		}

		for _, idx := range managedMap[namespace.Name] {
			policy := policies[idx]

			networkPolicyDto := K8sNetworkPolicyDto{
				Name:          utils.Pointer(policy.Name),
				NamespaceName: policy.Namespace,
				Spec:          policy.Spec,
			}

			namespaceDto.ManagedPolicies = append(namespaceDto.ManagedPolicies, networkPolicyDto)
		}

		for _, idx := range unmanagedMap[namespace.Name] {
			policy := policies[idx]

			conflictingNetworkPolicyDto := K8sConflictingNetworkPolicyDto{
				Name:          utils.Pointer(policy.Name),
				NamespaceName: policy.Namespace,
				Spec:          policy.Spec,
			}

			namespaceDto.UnmanagedPolicies = append(namespaceDto.UnmanagedPolicies, conflictingNetworkPolicyDto)
		}

		namespacesDto = append(namespacesDto, namespaceDto)
	}

	return namespacesDto, nil
}

type RemoveUnmanagedNetworkPoliciesRequest struct {
	Namespace string   `json:"namespaceName" validate:"required"`
	Policies  []string `json:"policies" validate:"required"`
}

func RemoveUnmanagedNetworkPolicies(data RemoveUnmanagedNetworkPoliciesRequest) error {
	// ignore namespace
	err := ignoreNamespace(data.Namespace)
	if err != nil {
		return err
	}

	err = kubernetes.RemoveUnmanagedNetworkPolicies(data.Namespace, data.Policies)
	if err != nil {
		return fmt.Errorf("failed to remove unmanaged network policies, err: %s", err.Error())
	}

	return nil
}

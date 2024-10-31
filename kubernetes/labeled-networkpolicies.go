package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/store"
	"mogenius-k8s-manager/utils"
	"reflect"
	"strings"

	v2 "k8s.io/api/apps/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"

	appsv1 "k8s.io/api/apps/v1"
	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
)

// The first rule of NetworkPolicy-Club is: you do not talk about NetworkPolicy-Club.
// The second rule of NetworkPolicy-Club is: Mogenius Policies have only one Port (Single Responsibility Principle).
// The third rule of NetworkPolicy-Club is: We do not delete NetworkPolicies, we only add them (if not hook to a controller, they do not do any damage).

const (
	// all policies
	NetpolLabel string = "mogenius-network-policy"

	// deny policy
	DenyAllNetPolName string = "deny-all"
	MarkerLabel              = "using-" + DenyAllNetPolName

	// allow policies
	PoliciesLabelPrefix string = "mo-netpol"

	PolicyConfigMapKey  string = "network-ports"
	PolicyConfigMapName string = "network-ports-config"
)

func AttachLabeledNetworkPolicy(controllerName string,
	controllerType dtos.K8sServiceControllerEnum,
	namespaceName string,
	labelPolicy dtos.K8sLabeledNetworkPolicyDto,
) error {
	client := GetAppClient()
	label := getNetworkPolicyName(labelPolicy)

	switch controllerType {
	case dtos.DEPLOYMENT:
		deployment, err := client.Deployments(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		if deployment.Spec.Template.ObjectMeta.Labels == nil {
			deployment.Spec.Template.ObjectMeta.Labels = make(map[string]string)
		}
		if deployment.ObjectMeta.Labels == nil {
			deployment.ObjectMeta.Labels = make(map[string]string)
		}
		deployment.Spec.Template.ObjectMeta.Labels[label] = "true"
		deployment.ObjectMeta.Labels[label] = "true"

		_, err = client.Deployments(namespaceName).Update(context.TODO(), deployment, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.DAEMON_SET:
		daemonset, err := client.DaemonSets(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		if daemonset.Spec.Template.ObjectMeta.Labels == nil {
			daemonset.Spec.Template.ObjectMeta.Labels = make(map[string]string)
		}
		if daemonset.ObjectMeta.Labels == nil {
			daemonset.ObjectMeta.Labels = make(map[string]string)
		}
		daemonset.Spec.Template.ObjectMeta.Labels[label] = "true"
		daemonset.ObjectMeta.Labels[label] = "true"

		_, err = client.DaemonSets(namespaceName).Update(context.TODO(), daemonset, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.STATEFUL_SET:
		statefulset, err := client.StatefulSets(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		if statefulset.Spec.Template.ObjectMeta.Labels == nil {
			statefulset.Spec.Template.ObjectMeta.Labels = make(map[string]string)
		}
		if statefulset.ObjectMeta.Labels == nil {
			statefulset.ObjectMeta.Labels = make(map[string]string)
		}
		statefulset.Spec.Template.ObjectMeta.Labels[label] = "true"
		statefulset.ObjectMeta.Labels[label] = "true"

		_, err = client.StatefulSets(namespaceName).Update(context.TODO(), statefulset, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	default:
		return fmt.Errorf("unsupported controller type %s", controllerType)
	}
	return nil
}

func AttachLabeledNetworkPolicies(controllerName string,
	controllerType dtos.K8sServiceControllerEnum,
	namespaceName string,
	labelPolicy []dtos.K8sLabeledNetworkPolicyDto,
) error {
	client := GetAppClient()
	var deployment *v2.Deployment
	var daemonSet *v2.DaemonSet
	var statefulSet *v2.StatefulSet
	var err error

	switch controllerType {
	case dtos.DEPLOYMENT:
		deployment, err = client.Deployments(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		if deployment.Spec.Template.ObjectMeta.Labels == nil {
			deployment.Spec.Template.ObjectMeta.Labels = make(map[string]string)
		}
		if deployment.ObjectMeta.Labels == nil {
			deployment.ObjectMeta.Labels = make(map[string]string)
		}
	case dtos.DAEMON_SET:
		daemonSet, err = client.DaemonSets(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		if daemonSet.Spec.Template.ObjectMeta.Labels == nil {
			daemonSet.Spec.Template.ObjectMeta.Labels = make(map[string]string)
		}
		if daemonSet.ObjectMeta.Labels == nil {
			daemonSet.ObjectMeta.Labels = make(map[string]string)
		}
	case dtos.STATEFUL_SET:
		statefulSet, err = client.StatefulSets(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		if statefulSet.Spec.Template.ObjectMeta.Labels == nil {
			statefulSet.Spec.Template.ObjectMeta.Labels = make(map[string]string)
		}
		if statefulSet.ObjectMeta.Labels == nil {
			statefulSet.ObjectMeta.Labels = make(map[string]string)
		}
	default:
		return fmt.Errorf("unsupported controller type %s", controllerType)
	}

	for _, labelPolicy := range labelPolicy {
		label := getNetworkPolicyName(labelPolicy)
		switch controllerType {
		case dtos.DEPLOYMENT:
			deployment.Spec.Template.ObjectMeta.Labels[label] = "true"
			deployment.ObjectMeta.Labels[label] = "true"
		case dtos.DAEMON_SET:
			daemonSet.Spec.Template.ObjectMeta.Labels[label] = "true"
			daemonSet.ObjectMeta.Labels[label] = "true"
		case dtos.STATEFUL_SET:
			statefulSet.Spec.Template.ObjectMeta.Labels[label] = "true"
			statefulSet.ObjectMeta.Labels[label] = "true"
		default:
			return fmt.Errorf("unsupported controller type %s", controllerType)
		}
	}

	switch controllerType {
	case dtos.DEPLOYMENT:
		_, err = client.Deployments(namespaceName).Update(context.TODO(), deployment, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.DAEMON_SET:
		_, err = client.DaemonSets(namespaceName).Update(context.TODO(), daemonSet, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.STATEFUL_SET:
		_, err = client.StatefulSets(namespaceName).Update(context.TODO(), statefulSet, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	default:
		return fmt.Errorf("unsupported controller type %s", controllerType)
	}

	return nil
}

func DetachLabeledNetworkPolicy(controllerName string,
	controllerType dtos.K8sServiceControllerEnum,
	namespaceName string,
	labelPolicy dtos.K8sLabeledNetworkPolicyDto,
) error {
	client := GetAppClient()
	label := getNetworkPolicyName(labelPolicy)

	switch controllerType {
	case dtos.DEPLOYMENT:
		deployment, err := client.Deployments(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		delete(deployment.Spec.Template.ObjectMeta.Labels, label)
		delete(deployment.ObjectMeta.Labels, label)
		_, err = client.Deployments(namespaceName).Update(context.TODO(), deployment, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.DAEMON_SET:
		daemonset, err := client.DaemonSets(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		delete(daemonset.Spec.Template.ObjectMeta.Labels, label)
		delete(daemonset.ObjectMeta.Labels, label)
		_, err = client.DaemonSets(namespaceName).Update(context.TODO(), daemonset, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.STATEFUL_SET:
		statefulset, err := client.StatefulSets(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		delete(statefulset.Spec.Template.ObjectMeta.Labels, label)
		delete(statefulset.ObjectMeta.Labels, label)
		_, err = client.StatefulSets(namespaceName).Update(context.TODO(), statefulset, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	default:
		return fmt.Errorf("unsupported controller type %s", controllerType)
	}
	return nil
}

func DetachLabeledNetworkPolicies(controllerName string,
	controllerType dtos.K8sServiceControllerEnum,
	namespaceName string,
	labelPolicy []dtos.K8sLabeledNetworkPolicyDto,
) error {
	client := GetAppClient()
	var deployment *v2.Deployment
	var daemonSet *v2.DaemonSet
	var statefulSet *v2.StatefulSet
	var err error

	switch controllerType {
	case dtos.DEPLOYMENT:
		deployment, err = client.Deployments(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.DAEMON_SET:
		daemonSet, err = client.DaemonSets(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.STATEFUL_SET:
		statefulSet, err = client.StatefulSets(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	default:
		return fmt.Errorf("unsupported controller type %s", controllerType)
	}

	for _, policy := range labelPolicy {
		switch controllerType {
		case dtos.DEPLOYMENT:
			if deployment.Spec.Template.ObjectMeta.Labels != nil {
				delete(deployment.ObjectMeta.Labels, policy.Name)
				delete(deployment.Spec.Template.ObjectMeta.Labels, policy.Name)
			}
		case dtos.DAEMON_SET:
			if daemonSet.Spec.Template.ObjectMeta.Labels != nil {
				delete(daemonSet.ObjectMeta.Labels, policy.Name)
				delete(daemonSet.Spec.Template.ObjectMeta.Labels, policy.Name)
			}
		case dtos.STATEFUL_SET:
			if statefulSet.Spec.Template.ObjectMeta.Labels != nil {
				delete(statefulSet.ObjectMeta.Labels, policy.Name)
				delete(statefulSet.Spec.Template.ObjectMeta.Labels, policy.Name)
			}
		default:
			return fmt.Errorf("unsupported controller type %s", controllerType)
		}
	}

	switch controllerType {
	case dtos.DEPLOYMENT:
		_, err = client.Deployments(namespaceName).Update(context.TODO(), deployment, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.DAEMON_SET:
		_, err = client.DaemonSets(namespaceName).Update(context.TODO(), daemonSet, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.STATEFUL_SET:
		_, err = client.StatefulSets(namespaceName).Update(context.TODO(), statefulSet, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	default:
		return fmt.Errorf("unsupported controller type %s", controllerType)
	}

	// cleanup unused network policies
	err = CleanupLabeledNetworkPolicies(namespaceName)

	return err
}

func CleanupLabeledNetworkPolicies(namespaceName string) error {
	client := GetAppClient()
	deployments, err := client.Deployments(namespaceName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("CleanupLabeledNetworkPolicies getDeployments ERROR: %s", err)
	}
	daemonSet, err := client.DaemonSets(namespaceName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("CleanupLabeledNetworkPolicies getDaemonSets ERROR: %s", err)
	}
	statefulSet, err := client.StatefulSets(namespaceName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("CleanupLabeledNetworkPolicies getStatefulSets ERROR: %s", err)
	}

	// create list of all in-use labels
	inUseLabels := make(map[string]bool)
	for _, deployment := range deployments.Items {
		inUseLabels = findInuseLabels(deployment.ObjectMeta, inUseLabels)
		inUseLabels = findInuseLabels(deployment.Spec.Template.ObjectMeta, inUseLabels)
	}
	for _, daemonSet := range daemonSet.Items {
		inUseLabels = findInuseLabels(daemonSet.ObjectMeta, inUseLabels)
		inUseLabels = findInuseLabels(daemonSet.Spec.Template.ObjectMeta, inUseLabels)
	}
	for _, statefulSet := range statefulSet.Items {
		inUseLabels = findInuseLabels(statefulSet.ObjectMeta, inUseLabels)
		inUseLabels = findInuseLabels(statefulSet.Spec.Template.ObjectMeta, inUseLabels)
	}

	// list all network policies created by mogenius
	netClient := GetNetworkingClient()
	netPolList, err := netClient.NetworkPolicies(namespaceName).List(context.TODO(), metav1.ListOptions{LabelSelector: NetpolLabel + "=true"})
	if err != nil {
		return fmt.Errorf("CleanupLabeledNetworkPolicies getNetworkPolicies ERROR: %s", err)
	}

	// delete all network policies that are not in use
	cleanupCounter := 0
	for _, netPol := range netPolList.Items {
		if netPol.Name == DenyAllNetPolName {
			continue
		}
		if _, ok := inUseLabels[netPol.Name]; !ok {
			err = netClient.NetworkPolicies(namespaceName).Delete(context.TODO(), netPol.Name, metav1.DeleteOptions{})
			if err != nil {
				K8sLogger.Error("CleanupLabeledNetworkPolicies deleteNetworkPolicy ERROR", "error", err)
			} else {
				cleanupCounter++
			}
		}
	}
	K8sLogger.Info("unused mogenius network policies deleted.", "amount", cleanupCounter)
	return nil
}

func findInuseLabels(meta metav1.ObjectMeta, currentList map[string]bool) map[string]bool {
	for label := range meta.Labels {
		if strings.Contains(label, PoliciesLabelPrefix) {
			currentList[label] = true
		}
	}
	return currentList
}

func EnsureLabeledNetworkPolicy(namespaceName string, labelPolicy dtos.K8sLabeledNetworkPolicyDto) error {
	netpol := v1.NetworkPolicy{}

	// clean traffic rules
	netpol.Spec.Ingress = []v1.NetworkPolicyIngressRule{}
	netpol.Spec.Egress = []v1.NetworkPolicyEgressRule{}

	netpol.ObjectMeta.Name = getNetworkPolicyName(labelPolicy)
	netpol.ObjectMeta.Namespace = namespaceName

	netpol.Spec.PodSelector.MatchLabels = map[string]string{getNetworkPolicyName(labelPolicy): "true"}

	// this label is marking all netpols that "need" a deny-all rule
	netpol.ObjectMeta.Labels = map[string]string{MarkerLabel: "true"}
	// general label for all mogenius netpols
	netpol.ObjectMeta.Labels[NetpolLabel] = "true"

	port := intstr.FromInt32(int32(labelPolicy.Port))
	var proto v1Core.Protocol

	switch labelPolicy.PortType {
	case "UDP":
		proto = v1Core.ProtocolUDP
	case "SCTP":
		proto = v1Core.ProtocolSCTP
	default:
		proto = v1Core.ProtocolTCP
	}

	if labelPolicy.Type == dtos.Ingress {
		var rule v1.NetworkPolicyIngressRule = v1.NetworkPolicyIngressRule{}
		rule.From = append(rule.From, v1.NetworkPolicyPeer{
			IPBlock: &v1.IPBlock{
				CIDR: "0.0.0.0/0",
			},
		})
		rule.Ports = append(rule.Ports, v1.NetworkPolicyPort{
			Port: &port, Protocol: &proto,
		})
		netpol.Spec.Ingress = append(netpol.Spec.Ingress, rule)
	} else {
		var rule v1.NetworkPolicyEgressRule = v1.NetworkPolicyEgressRule{}
		rule.To = append(rule.To, v1.NetworkPolicyPeer{
			IPBlock: &v1.IPBlock{
				CIDR: "0.0.0.0/0",
			},
		})
		rule.Ports = append(rule.Ports, v1.NetworkPolicyPort{
			Port: &port, Protocol: &proto,
		})
		netpol.Spec.Egress = append(netpol.Spec.Egress, rule)
	}

	netPolClient := GetNetworkingClient().NetworkPolicies(namespaceName)
	_, err := netPolClient.Create(context.TODO(), &netpol, MoCreateOptions())
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		K8sLogger.Error("CreateNetworkPolicyServiceWithLabel ERROR: %s, trying to create labelPolicy %v ", err.Error(), labelPolicy)
		return err
	}

	err = ensureDenyAllRule(namespaceName)
	if err != nil {
		return err
	}
	return nil
}

func EnsureLabeledNetworkPolicies(namespaceName string, labelPolicy []dtos.K8sLabeledNetworkPolicyDto) error {
	for _, labelPolicy := range labelPolicy {
		netpol := v1.NetworkPolicy{}

		// clean traffic rules
		netpol.Spec.Ingress = []v1.NetworkPolicyIngressRule{}
		netpol.Spec.Egress = []v1.NetworkPolicyEgressRule{}

		netpol.ObjectMeta.Name = getNetworkPolicyName(labelPolicy)
		netpol.ObjectMeta.Namespace = namespaceName

		label := getNetworkPolicyName(labelPolicy)
		netpol.Spec.PodSelector.MatchLabels = map[string]string{label: "true"}

		// this label is marking all netpols that "need" a deny-all rule
		netpol.ObjectMeta.Labels = map[string]string{MarkerLabel: "true"}
		// general label for all mogenius netpols
		netpol.ObjectMeta.Labels[NetpolLabel] = "true"

		port := intstr.FromInt32(int32(labelPolicy.Port))
		var proto v1Core.Protocol

		switch labelPolicy.PortType {
		case "UDP":
			proto = v1Core.ProtocolUDP
		case "SCTP":
			proto = v1Core.ProtocolSCTP
		default:
			proto = v1Core.ProtocolTCP
		}

		if labelPolicy.Type == dtos.Ingress {
			var rule v1.NetworkPolicyIngressRule = v1.NetworkPolicyIngressRule{}
			rule.From = append(rule.From, v1.NetworkPolicyPeer{
				IPBlock: &v1.IPBlock{
					CIDR: "0.0.0.0/0",
				},
			})
			rule.Ports = append(rule.Ports, v1.NetworkPolicyPort{
				Port: &port, Protocol: &proto,
			})
			netpol.Spec.Ingress = append(netpol.Spec.Ingress, rule)
		} else {
			var rule v1.NetworkPolicyEgressRule = v1.NetworkPolicyEgressRule{}
			rule.To = append(rule.To, v1.NetworkPolicyPeer{
				IPBlock: &v1.IPBlock{
					CIDR: "0.0.0.0/0",
				},
			})
			rule.Ports = append(rule.Ports, v1.NetworkPolicyPort{
				Port: &port, Protocol: &proto,
			})
			netpol.Spec.Egress = append(netpol.Spec.Egress, rule)
		}

		netPolClient := GetNetworkingClient().NetworkPolicies(namespaceName)
		_, err := netPolClient.Create(context.TODO(), &netpol, MoCreateOptions())
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			K8sLogger.Error("CreateNetworkPolicyServiceWithLabel ERROR: %s, trying to create labelPolicy %v ", err.Error(), labelPolicy)
			return err
		}
	}

	err := ensureDenyAllRule(namespaceName)
	if err != nil {
		return err
	}
	return nil
}

func CreateDenyAllNetworkPolicy(namespaceName string) error {
	netpol := v1.NetworkPolicy{}
	netpol.ObjectMeta.Name = DenyAllNetPolName
	netpol.ObjectMeta.Namespace = namespaceName
	netpol.Spec.PodSelector = metav1.LabelSelector{} // An empty podSelector matches all pods in this namespace.
	netpol.Spec.Ingress = []v1.NetworkPolicyIngressRule{}

	// general label for all mogenius netpols
	netpol.ObjectMeta.Labels = make(map[string]string)
	netpol.ObjectMeta.Labels[NetpolLabel] = "true"

	netPolClient := GetNetworkingClient().NetworkPolicies(namespaceName)
	_, err := netPolClient.Create(context.TODO(), &netpol, MoCreateOptions())
	if err != nil {
		K8sLogger.Error("CreateDenyAllNetworkPolicy", "error", err)
		return err
	}
	return nil
}

func cleanupUnusedDenyAll(namespaceName string) {
	// list all network policies and find all that have the marker label
	policies, err := listAllNetworkPolicies(namespaceName)
	if err != nil {
		K8sLogger.Error("cleanupNetworkPolicies", "error", err)
		return
	}

	// filter out all policies that are not created by mogenius
	var netpols []v1.NetworkPolicy
	for _, policy := range policies {
		if policy.ObjectMeta.Labels != nil && policy.ObjectMeta.Labels[MarkerLabel] == "true" {
			continue
		}
		netpols = append(netpols, policy)
	}

	if len(netpols) == 0 {
		client := GetNetworkingClient()
		netPolClient := client.NetworkPolicies(namespaceName)

		err = netPolClient.Delete(context.TODO(), DenyAllNetPolName, metav1.DeleteOptions{})
		if err != nil {
			K8sLogger.Error("cleanupNetworkPolicies", "error", err)
		}
	}
}

func InitNetworkPolicyConfigMap() error {
	configMap := readDefaultConfigMap()

	return EnsureConfigMapExists(configMap.Namespace, *configMap)
}

func readDefaultConfigMap() *v1Core.ConfigMap {
	yamlString := utils.InitNetworkPolicyDefaultsYaml()

	// marshal yaml to struct
	var configMap v1Core.ConfigMap
	err := yaml.Unmarshal([]byte(yamlString), &configMap)
	if err != nil {
		K8sLogger.Error("InitNetworkPolicyConfigMap", "error", err)
		return nil
	}
	return &configMap
}

type NetworkPolicy struct {
	Name     string `yaml:"name"`
	Protocol string `yaml:"protocol"`
	Port     uint16 `yaml:"port"`
}

func checkForDuplicatedItems(items []NetworkPolicy) []NetworkPolicy {
	seen := make(map[string]bool)
	duplicates := []NetworkPolicy{}

	for _, item := range items {
		if !seen[item.Name] {
			seen[item.Name] = true
		} else {
			duplicates = append(duplicates, item)
			K8sLogger.Warn("Duplicate network policy name", "networkpolicy", item.Name)
		}
	}

	return duplicates
}

func UpdateNetworkPolicyTemplate(policies []NetworkPolicy) error {
	duplicates := checkForDuplicatedItems(policies)
	if len(duplicates) > 0 {
		return fmt.Errorf("found duplicate network policy names: %v", duplicates)
	}

	client := GetCoreClient().ConfigMaps(utils.CONFIG.Kubernetes.OwnNamespace)

	cfgMap := readDefaultConfigMap()

	yamlStr, err := yaml.Marshal(policies)
	if err != nil {
		K8sLogger.Error("UpdateNetworkPolicyTemplate", "error", err)
		return err
	}

	cfgMap.Data[PolicyConfigMapKey] = string(yamlStr)

	// check if the configmap already exists
	_, err = client.Update(context.TODO(), cfgMap, metav1.UpdateOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = client.Create(context.TODO(), cfgMap, MoCreateOptions())
			if err != nil {
				K8sLogger.Error("InitNetworkPolicyConfigMap", "error", err)
				return err
			}
		} else {
			K8sLogger.Error("InitNetworkPolicyConfigMap", "error", err)
			return err
		}
	}
	return nil
}

func ReadNetworkPolicyPorts() ([]dtos.K8sLabeledNetworkPolicyDto, error) {
	ClusterConfigMap := GetConfigMap(utils.CONFIG.Kubernetes.OwnNamespace, PolicyConfigMapName)

	var result []dtos.K8sLabeledNetworkPolicyDto
	var policies []NetworkPolicy
	err := yaml.Unmarshal([]byte(ClusterConfigMap.Data[PolicyConfigMapKey]), &policies)

	if err != nil {
		K8sLogger.Error("Error unmarshalling YAML", "error", err)
		return nil, err
	}
	for _, policy := range policies {
		result = append(result, dtos.K8sLabeledNetworkPolicyDto{
			Name:     policy.Name,
			Type:     dtos.Ingress,
			Port:     uint16(policy.Port),
			PortType: dtos.PortTypeEnum(policy.Protocol),
		})
	}
	return result, nil
}

func RemoveAllConflictingNetworkPolicies(namespaceName string) error {
	if namespaceName == "kube-system" {
		return fmt.Errorf("cannot remove network-policies from kube-system namespace")
	}

	netpols, err := ListAllConflictingNetworkPolicies(namespaceName)
	if err != nil {
		return fmt.Errorf("failed to list all network policies: %v", err)
	}

	client := GetNetworkingClient()
	netPolClient := client.NetworkPolicies(namespaceName)

	errors := []error{}
	for _, netpol := range netpols.Items {
		err = netPolClient.Delete(context.TODO(), netpol.Name, metav1.DeleteOptions{})
		if err != nil {
			K8sLogger.Error("RemoveAllConflictingNetworkPolicies", "error", err)
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("failed to remove all network policies: %v", errors)
	}
	return nil
}

func ListAllConflictingNetworkPolicies(namespaceName string) (*v1.NetworkPolicyList, error) {
	policies, err := listAllNetworkPolicies(namespaceName)
	if err != nil {
		return nil, err
	}

	// filter out all policies that are not created by mogenius
	var netpols []v1.NetworkPolicy
	for _, policy := range policies {
		if policy.ObjectMeta.Labels != nil && policy.ObjectMeta.Labels[NetpolLabel] != "true" {
			continue
		}
		netpols = append(netpols, policy)
	}

	// create list *v1.NetworkPolicyList
	return &v1.NetworkPolicyList{
		Items: netpols,
	}, nil
}

func listAllNetworkPolicies(namespaceName string) ([]v1.NetworkPolicy, error) {
	result := []v1.NetworkPolicy{}

	policies, err := store.GlobalStore.SearchByPrefix(reflect.TypeOf(v1.NetworkPolicy{}), namespaceName)
	if err != nil {
		K8sLogger.Error("ListAllNetworkPolicies", "error", err)
		return result, err
	}

	for _, ref := range policies {
		if ref == nil {
			continue
		}

		netpol := ref.(*v1.NetworkPolicy)
		if netpol == nil {
			continue
		}

		result = append(result, *netpol)
	}

	return result, nil
}

func ListAllNetworkPolicies(namespaceName string) punqUtils.K8sWorkloadResult {
	result, err := listAllNetworkPolicies(namespaceName)
	if err != nil {
		return punq.WorkloadResult(nil, err)
	}

	return punq.WorkloadResult(result, nil)
}

func extractLabels(maps ...map[string]string) map[string]string {
	mergedLabels := make(map[string]string)

	for _, m := range maps {
		for key, value := range m {
			mergedLabels[key] = value
		}
	}

	return mergedLabels
}

func ListControllerLabeledNetworkPolicies(
	controllerName string,
	controllerType dtos.K8sServiceControllerEnum,
	namespaceName string,
) ([]dtos.K8sLabeledNetworkPolicyDto, error) {
	// get all labels from the controller
	var labels map[string]string
	switch controllerType {
	case dtos.DEPLOYMENT:
		ref := store.GlobalStore.GetByKeyParts(reflect.TypeOf(appsv1.Deployment{}), "Deployment", namespaceName, controllerName)
		if ref == nil {
			return nil, fmt.Errorf("ListControllerLabeledNetworkPolicies %s ERROR: %s", controllerType, "deployment not found")
		}
		deployment := ref.(*appsv1.Deployment)
		if deployment == nil {
			return nil, fmt.Errorf("ListControllerLabeledNetworkPolicies %s ERROR: %s", controllerType, "deployment not found")
		}
		labels = extractLabels(deployment.ObjectMeta.Labels, deployment.Spec.Template.ObjectMeta.Labels)
	case dtos.DAEMON_SET:
		ref := store.GlobalStore.GetByKeyParts(reflect.TypeOf(appsv1.DaemonSet{}), "DaemonSet", namespaceName, controllerName)
		if ref == nil {
			return nil, fmt.Errorf("ListControllerLabeledNetworkPolicies %s ERROR: %s", controllerType, "daemonset not found")
		}
		daemonset := ref.(*appsv1.DaemonSet)
		if daemonset == nil {
			return nil, fmt.Errorf("ListControllerLabeledNetworkPolicies %s ERROR: %s", controllerType, "daemonset not found")
		}
		labels = extractLabels(daemonset.ObjectMeta.Labels, daemonset.Spec.Template.ObjectMeta.Labels)
	case dtos.STATEFUL_SET:
		ref := store.GlobalStore.GetByKeyParts(reflect.TypeOf(appsv1.StatefulSet{}), "StatefulSet", namespaceName, controllerName)
		if ref == nil {
			return nil, fmt.Errorf("ListControllerLabeledNetworkPolicies %s ERROR: %s", controllerType, "statefulset not found")
		}
		statefulset := ref.(*appsv1.StatefulSet)
		if statefulset == nil {
			return nil, fmt.Errorf("ListControllerLabeledNetworkPolicies %s ERROR: %s", controllerType, "statefulset not found")
		}
		labels = extractLabels(statefulset.ObjectMeta.Labels, statefulset.Spec.Template.ObjectMeta.Labels)
	default:
		return nil, fmt.Errorf("unsupported controller type %s", controllerType)
	}

	netpols := []dtos.K8sLabeledNetworkPolicyDto{}

	policies, err := listAllNetworkPolicies(namespaceName)
	if err != nil {
		return nil, err
	}

	for label := range labels {
		if !strings.Contains(label, PoliciesLabelPrefix) {
			continue
		}

		var netpol v1.NetworkPolicy

		found := false
		for _, policy := range policies {
			if policy.Name == label {
				netpol = policy
				found = true
				break
			}
		}

		if !found {
			continue
		}

		if strings.Contains(netpol.Name, "egress") {
			var port uint16
			var pType dtos.PortTypeEnum
			// our netpols only have one rule
			if len(netpol.Spec.Egress) == 1 && len(netpol.Spec.Egress[0].Ports) == 1 && netpol.Spec.Egress[0].Ports[0].Port != nil {
				port = uint16(netpol.Spec.Egress[0].Ports[0].Port.IntVal)
				pType = dtos.PortTypeEnum(*netpol.Spec.Egress[0].Ports[0].Protocol)
			}
			netpols = append(netpols, dtos.K8sLabeledNetworkPolicyDto{
				Name:     netpol.Name,
				Type:     dtos.Egress,
				Port:     port,
				PortType: pType,
			})
		} else {
			var port uint16
			var pType dtos.PortTypeEnum
			// our netpols only have one rule
			if len(netpol.Spec.Ingress) == 1 && len(netpol.Spec.Ingress[0].Ports) == 1 && netpol.Spec.Ingress[0].Ports[0].Port != nil {
				port = uint16(netpol.Spec.Ingress[0].Ports[0].Port.IntVal)
				pType = dtos.PortTypeEnum(*netpol.Spec.Ingress[0].Ports[0].Protocol)
			}
			netpols = append(netpols, dtos.K8sLabeledNetworkPolicyDto{
				Name:     netpol.Name,
				Type:     dtos.Ingress,
				Port:     port,
				PortType: pType,
			})
		}
	}
	return netpols, nil
}

func getNetworkPolicyName(labelPolicy dtos.K8sLabeledNetworkPolicyDto) string {
	return strings.ToLower(
		fmt.Sprintf("%s-%s-%s", PoliciesLabelPrefix, labelPolicy.Name, labelPolicy.Type),
	)
}

func ensureDenyAllRule(namespaceName string) error {
	netPolClient := GetNetworkingClient().NetworkPolicies(namespaceName)

	_, err := netPolClient.Get(context.TODO(), DenyAllNetPolName, metav1.GetOptions{})
	if err != nil {
		K8sLogger.Info("networkpolicy not found, it will be created.", "networkpolicy", DenyAllNetPolName)

		err = CreateDenyAllNetworkPolicy(namespaceName)
		if err != nil {
			K8sLogger.Error("failed to create networkpolicy", "networkpolicy", DenyAllNetPolName, "error", err)
			return err
		}
	}
	return nil
}

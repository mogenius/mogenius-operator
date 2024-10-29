package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/utils"
	"sort"
	"strings"
	"time"

	v2 "k8s.io/api/apps/v1"

	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"

	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func toDeleteFunc(namespaceName string, labels map[string]string) map[string]string {
	netClient := GetNetworkingClient()
	netPolClient := netClient.NetworkPolicies(namespaceName)
	var toDelete []string
	for label := range labels {
		if !strings.Contains(label, PoliciesLabelPrefix) {
			continue
		}

		_, err := netPolClient.Get(context.TODO(), label, metav1.GetOptions{})
		if err != nil {
			K8sLogger.Errorf("ListControllerLabeledNetworkPolicies ERROR: %s", err)
			toDelete = append(toDelete, label)
		}
	}
	for _, key := range toDelete {
		delete(labels, key)
	}
	return labels
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
		deployment.Spec.Template.ObjectMeta.Labels = toDeleteFunc(namespaceName, deployment.Spec.Template.ObjectMeta.Labels)
		deployment.ObjectMeta.Labels = toDeleteFunc(namespaceName, deployment.ObjectMeta.Labels)
		_, err = client.Deployments(namespaceName).Update(context.TODO(), deployment, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.DAEMON_SET:
		daemonSet.Spec.Template.ObjectMeta.Labels = toDeleteFunc(namespaceName, daemonSet.Spec.Template.ObjectMeta.Labels)
		daemonSet.ObjectMeta.Labels = toDeleteFunc(namespaceName, daemonSet.ObjectMeta.Labels)
		_, err = client.DaemonSets(namespaceName).Update(context.TODO(), daemonSet, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.STATEFUL_SET:
		statefulSet.Spec.Template.ObjectMeta.Labels = toDeleteFunc(namespaceName, statefulSet.Spec.Template.ObjectMeta.Labels)
		statefulSet.ObjectMeta.Labels = toDeleteFunc(namespaceName, statefulSet.ObjectMeta.Labels)
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
		deployment.Spec.Template.ObjectMeta.Labels = toDeleteFunc(namespaceName, deployment.Spec.Template.ObjectMeta.Labels)
		deployment.ObjectMeta.Labels = toDeleteFunc(namespaceName, deployment.ObjectMeta.Labels)
		_, err = client.Deployments(namespaceName).Update(context.TODO(), deployment, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.DAEMON_SET:
		daemonSet.Spec.Template.ObjectMeta.Labels = toDeleteFunc(namespaceName, daemonSet.Spec.Template.ObjectMeta.Labels)
		daemonSet.ObjectMeta.Labels = toDeleteFunc(namespaceName, daemonSet.ObjectMeta.Labels)
		_, err = client.DaemonSets(namespaceName).Update(context.TODO(), daemonSet, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.STATEFUL_SET:
		statefulSet.Spec.Template.ObjectMeta.Labels = toDeleteFunc(namespaceName, statefulSet.Spec.Template.ObjectMeta.Labels)
		statefulSet.ObjectMeta.Labels = toDeleteFunc(namespaceName, statefulSet.ObjectMeta.Labels)
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
				K8sLogger.Errorf("CleanupLabeledNetworkPolicies deleteNetworkPolicy ERROR: %s", err)
			} else {
				cleanupCounter++
			}
		}
	}
	K8sLogger.Infof("%d unused mogenius network policies deleted.", cleanupCounter)
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
		K8sLogger.Errorf("CreateNetworkPolicyServiceWithLabel ERROR: %s, trying to create labelPolicy %v ", err.Error(), labelPolicy)
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
			K8sLogger.Errorf("CreateNetworkPolicyServiceWithLabel ERROR: %s, trying to create labelPolicy %v ", err.Error(), labelPolicy)
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
		K8sLogger.Errorf("CreateDenyAllNetworkPolicy ERROR: %s", err)
		return err
	}
	return nil
}

func cleanupUnusedDenyAll(namespaceName string) {
	client := GetNetworkingClient()
	netPolClient := client.NetworkPolicies(namespaceName)

	// list all network policies and find all that have the marker label
	netpols, err := netPolClient.List(context.TODO(), metav1.ListOptions{
		LabelSelector: MarkerLabel + "=true",
	})
	if err != nil {
		K8sLogger.Errorf("cleanupNetworkPolicies ERROR: %s", err)
		return
	}

	if len(netpols.Items) == 0 {
		err = netPolClient.Delete(context.TODO(), DenyAllNetPolName, metav1.DeleteOptions{})
		if err != nil {
			K8sLogger.Errorf("cleanupNetworkPolicies ERROR: %s", err)
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
		K8sLogger.Errorf("InitNetworkPolicyConfigMap ERROR: %s", err)
		return nil
	}
	return &configMap
}

type NetworkPolicy struct {
	Name      string    `yaml:"name"`
	Protocol  string    `yaml:"protocol"`
	Port      uint16    `yaml:"port"`
	CreatedAt time.Time `yaml:"createdAt"`
}

func uniqueItemsByName(items []NetworkPolicy) []NetworkPolicy {
	seen := make(map[string]bool)
	result := []NetworkPolicy{}

	for _, item := range items {
		if !seen[item.Name] {
			seen[item.Name] = true
			result = append(result, item)
		} else {
			K8sLogger.Warnf("Duplicate network policy name: %s", item.Name)
		}
	}

	return result
}

func ReadNetworkPolicyPorts() ([]dtos.K8sLabeledNetworkPolicyDto, error) {
	configMap := readDefaultConfigMap()
	ClusterConfigMap := GetConfigMap(configMap.Namespace, configMap.Name)

	var result []dtos.K8sLabeledNetworkPolicyDto
	var policies []NetworkPolicy
	policiesRaw := ClusterConfigMap.Data["network-ports"]
	err := yaml.Unmarshal([]byte(policiesRaw), &policies)

	sort.Slice(policies, func(i, j int) bool {
		return policies[i].CreatedAt.Before(policies[j].CreatedAt)
	})

	policies = uniqueItemsByName(policies)

	if err != nil {
		K8sLogger.Errorf("Error unmarshalling YAML: %s\n", err)
		return nil, err
	}
	for _, policy := range policies {
		result = append(result, dtos.K8sLabeledNetworkPolicyDto{
			Name:     policy.Name,
			Type:     dtos.Ingress, // TODO should maybe be deleted
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
			K8sLogger.Errorf("RemoveAllConflictingNetworkPolicies ERROR: %s", err)
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("failed to remove all network policies: %v", errors)
	}
	return nil
}

func ListAllConflictingNetworkPolicies(namespaceName string) (*v1.NetworkPolicyList, error) {
	client := GetNetworkingClient()
	netPolClient := client.NetworkPolicies(namespaceName)

	netpols, err := netPolClient.List(context.TODO(), metav1.ListOptions{
		LabelSelector: NetpolLabel + "!=true",
	})
	if err != nil {
		K8sLogger.Errorf("ListAllConflictingNetworkPolicies ERROR: %s", err)
		return nil, nil
	}
	return netpols, err
}

func ListControllerLabeledNetworkPolicies(
	controllerName string,
	controllerType dtos.K8sServiceControllerEnum,
	namespaceName string,
) ([]dtos.K8sLabeledNetworkPolicyDto, error) {

	client := GetAppClient()

	// get all labels from the controller
	var labels map[string]string
	switch controllerType {
	case dtos.DEPLOYMENT:
		deployment, err := client.Deployments(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("ListControllerLabeledNetworkPolicies %s ERROR: %s", controllerType, err.Error())
		}
		labels = deployment.Spec.Template.ObjectMeta.Labels
	case dtos.DAEMON_SET:
		daemonset, err := client.DaemonSets(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("ListControllerLabeledNetworkPolicies %s ERROR: %s", controllerType, err.Error())
		}
		labels = daemonset.Spec.Template.ObjectMeta.Labels
	case dtos.STATEFUL_SET:
		statefulset, err := client.StatefulSets(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("ListControllerLabeledNetworkPolicies %s ERROR: %s", controllerType, err.Error())
		}
		labels = statefulset.Spec.Template.ObjectMeta.Labels
	default:
		return nil, fmt.Errorf("unsupported controller type %s", controllerType)
	}

	// get all network policies based on mo-netpol labels
	netClient := GetNetworkingClient()
	netPolClient := netClient.NetworkPolicies(namespaceName)

	netpols := []dtos.K8sLabeledNetworkPolicyDto{}
	for label := range labels {
		if !strings.Contains(label, PoliciesLabelPrefix) {
			continue
		}

		netpol, err := netPolClient.Get(context.TODO(), label, metav1.GetOptions{})
		if err != nil {
			K8sLogger.Errorf("ListControllerLabeledNetworkPolicies(get single one) ERROR: %s", err)
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
		K8sLogger.Infof("%s not found, it will be created.", DenyAllNetPolName)

		err = CreateDenyAllNetworkPolicy(namespaceName)
		if err != nil {
			K8sLogger.Errorf("ERROR creating: %s:  %v , abort NetPol creation!", DenyAllNetPolName, err.Error())
			return err
		}
	}
	return nil
}

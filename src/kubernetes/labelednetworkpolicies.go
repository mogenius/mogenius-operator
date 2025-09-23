package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/utils"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"

	appsv1 "k8s.io/api/apps/v1"
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
	DenyAllIngressNetPolName              string = "deny-all-ingress"
	AllowNamespaceCommunicationNetPolName string = "allow-namespace-communication"
	MarkerLabel                                  = "using-" + DenyAllIngressNetPolName

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
	client := clientProvider.K8sClientSet().AppsV1()
	label := GetNetworkPolicyName(labelPolicy)

	switch controllerType {
	case dtos.DEPLOYMENT:
		deployment, err := client.Deployments(namespaceName).Get(context.Background(), controllerName, metav1.GetOptions{})
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

		_, err = client.Deployments(namespaceName).Update(context.Background(), deployment, MoUpdateOptions(config))
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.DAEMON_SET:
		daemonset, err := client.DaemonSets(namespaceName).Get(context.Background(), controllerName, metav1.GetOptions{})
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

		_, err = client.DaemonSets(namespaceName).Update(context.Background(), daemonset, MoUpdateOptions(config))
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.STATEFUL_SET:
		statefulset, err := client.StatefulSets(namespaceName).Get(context.Background(), controllerName, metav1.GetOptions{})
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

		_, err = client.StatefulSets(namespaceName).Update(context.Background(), statefulset, MoUpdateOptions(config))
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
	client := clientProvider.K8sClientSet().AppsV1()
	var deployment *appsv1.Deployment
	var daemonSet *appsv1.DaemonSet
	var statefulSet *appsv1.StatefulSet
	var err error

	switch controllerType {
	case dtos.DEPLOYMENT:
		deployment, err = client.Deployments(namespaceName).Get(context.Background(), controllerName, metav1.GetOptions{})
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
		daemonSet, err = client.DaemonSets(namespaceName).Get(context.Background(), controllerName, metav1.GetOptions{})
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
		statefulSet, err = client.StatefulSets(namespaceName).Get(context.Background(), controllerName, metav1.GetOptions{})
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
		label := GetNetworkPolicyName(labelPolicy)
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
		_, err = client.Deployments(namespaceName).Update(context.Background(), deployment, MoUpdateOptions(config))
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.DAEMON_SET:
		_, err = client.DaemonSets(namespaceName).Update(context.Background(), daemonSet, MoUpdateOptions(config))
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.STATEFUL_SET:
		_, err = client.StatefulSets(namespaceName).Update(context.Background(), statefulSet, MoUpdateOptions(config))
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
	client := clientProvider.K8sClientSet().AppsV1()
	label := GetNetworkPolicyName(labelPolicy)

	switch controllerType {
	case dtos.DEPLOYMENT:
		deployment, err := client.Deployments(namespaceName).Get(context.Background(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		delete(deployment.Spec.Template.ObjectMeta.Labels, label)
		delete(deployment.ObjectMeta.Labels, label)
		_, err = client.Deployments(namespaceName).Update(context.Background(), deployment, MoUpdateOptions(config))
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.DAEMON_SET:
		daemonset, err := client.DaemonSets(namespaceName).Get(context.Background(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		delete(daemonset.Spec.Template.ObjectMeta.Labels, label)
		delete(daemonset.ObjectMeta.Labels, label)
		_, err = client.DaemonSets(namespaceName).Update(context.Background(), daemonset, MoUpdateOptions(config))
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.STATEFUL_SET:
		statefulset, err := client.StatefulSets(namespaceName).Get(context.Background(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		delete(statefulset.Spec.Template.ObjectMeta.Labels, label)
		delete(statefulset.ObjectMeta.Labels, label)
		_, err = client.StatefulSets(namespaceName).Update(context.Background(), statefulset, MoUpdateOptions(config))
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
	client := clientProvider.K8sClientSet().AppsV1()
	var deployment *appsv1.Deployment
	var daemonSet *appsv1.DaemonSet
	var statefulSet *appsv1.StatefulSet
	var err error

	switch controllerType {
	case dtos.DEPLOYMENT:
		deployment, err = client.Deployments(namespaceName).Get(context.Background(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.DAEMON_SET:
		daemonSet, err = client.DaemonSets(namespaceName).Get(context.Background(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.STATEFUL_SET:
		statefulSet, err = client.StatefulSets(namespaceName).Get(context.Background(), controllerName, metav1.GetOptions{})
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
		_, err = client.Deployments(namespaceName).Update(context.Background(), deployment, MoUpdateOptions(config))
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.DAEMON_SET:
		_, err = client.DaemonSets(namespaceName).Update(context.Background(), daemonSet, MoUpdateOptions(config))
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.STATEFUL_SET:
		_, err = client.StatefulSets(namespaceName).Update(context.Background(), statefulSet, MoUpdateOptions(config))
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
	clientset := clientProvider.K8sClientSet()
	deployments, err := clientset.AppsV1().Deployments(namespaceName).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("CleanupLabeledNetworkPolicies getDeployments ERROR: %s", err)
	}
	daemonSet, err := clientset.AppsV1().DaemonSets(namespaceName).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("CleanupLabeledNetworkPolicies getDaemonSets ERROR: %s", err)
	}
	statefulSet, err := clientset.AppsV1().StatefulSets(namespaceName).List(context.Background(), metav1.ListOptions{})
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
	netPolList, err := clientset.NetworkingV1().NetworkPolicies(namespaceName).List(context.Background(), metav1.ListOptions{LabelSelector: NetpolLabel + "=true"})
	if err != nil {
		return fmt.Errorf("CleanupLabeledNetworkPolicies getNetworkPolicies ERROR: %s", err)
	}

	// delete all network policies that are not in use
	cleanupCounter := 0
	for _, netPol := range netPolList.Items {
		if netPol.Name == DenyAllIngressNetPolName || netPol.Name == AllowNamespaceCommunicationNetPolName {
			continue
		}
		if _, ok := inUseLabels[netPol.Name]; !ok {
			err = clientset.NetworkingV1().NetworkPolicies(namespaceName).Delete(context.Background(), netPol.Name, metav1.DeleteOptions{})
			if err != nil {
				k8sLogger.Error("CleanupLabeledNetworkPolicies deleteNetworkPolicy ERROR", "error", err)
			} else {
				cleanupCounter++
			}
		}
	}
	k8sLogger.Info("unused mogenius network policies deleted.", "amount", cleanupCounter)
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

	netpol.ObjectMeta.Name = GetNetworkPolicyName(labelPolicy)
	netpol.ObjectMeta.Namespace = namespaceName

	netpol.Spec.PodSelector.MatchLabels = map[string]string{GetNetworkPolicyName(labelPolicy): "true"}

	// this label is marking all netpols that "need" a deny-all-ingress rule
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

	clientset := clientProvider.K8sClientSet()
	netPolClient := clientset.NetworkingV1().NetworkPolicies(namespaceName)
	_, err := netPolClient.Create(context.Background(), &netpol, MoCreateOptions(config))
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		k8sLogger.Error("CreateNetworkPolicyServiceWithLabel ERROR: %s, trying to create labelPolicy %v ", err.Error(), labelPolicy)
		return err
	}

	err = ensureDenyAllIngressRule(namespaceName)
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

		netpol.ObjectMeta.Name = GetNetworkPolicyName(labelPolicy)
		netpol.ObjectMeta.Namespace = namespaceName

		label := GetNetworkPolicyName(labelPolicy)
		netpol.Spec.PodSelector.MatchLabels = map[string]string{label: "true"}

		// this label is marking all netpols that "need" a deny-all-ingress rule
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

		clientset := clientProvider.K8sClientSet()
		netPolClient := clientset.NetworkingV1().NetworkPolicies(namespaceName)
		_, err := netPolClient.Create(context.Background(), &netpol, MoCreateOptions(config))
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			k8sLogger.Error("CreateNetworkPolicyServiceWithLabel ERROR: %s, trying to create labelPolicy %v ", err.Error(), labelPolicy)
			return err
		}
	}

	err := ensureDenyAllIngressRule(namespaceName)
	if err != nil {
		return err
	}
	return nil
}

func CreateDenyAllIngressNetworkPolicy(namespaceName string) error {
	// check if the deny-all-ingress policy already exists

	netpol := v1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DenyAllIngressNetPolName,
			Namespace: namespaceName,
		},
		Spec: v1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{}, // Matches all pods in this namespace.
			Ingress: []v1.NetworkPolicyIngressRule{
				{
					From: []v1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": config.Get("MO_OWN_NAMESPACE"),
								},
							},
						},
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "kube-system",
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"acme.cert-manager.io/http01-solver": "true",
								},
							},
						},
					},
					Ports: []v1.NetworkPolicyPort{
						{
							Protocol: func() *v1Core.Protocol {
								tcp := v1Core.ProtocolTCP
								return &tcp
							}(),
							Port: func() *intstr.IntOrString {
								port := intstr.FromInt(8089)
								return &port
							}(),
						},
					},
				},
			},
		},
	}

	// general label for all mogenius netpols
	netpol.ObjectMeta.Labels = make(map[string]string)
	netpol.ObjectMeta.Labels[NetpolLabel] = "true"

	clientset := clientProvider.K8sClientSet()
	netPolClient := clientset.NetworkingV1().NetworkPolicies(namespaceName)
	_, err := netPolClient.Get(context.Background(), DenyAllIngressNetPolName, metav1.GetOptions{})
	if err != nil {
		// create the deny-all-ingress policy
		_, err := netPolClient.Create(context.Background(), &netpol, MoCreateOptions(config))
		if err != nil {
			k8sLogger.Error("CreateDenyAllIngressNetworkPolicy", "error", err)
			return err
		}
		return nil
	}

	// update the deny-all-ingress policy
	_, err = netPolClient.Update(context.Background(), &netpol, MoUpdateOptions(config))
	if err != nil {
		k8sLogger.Error("CreateDenyAllIngressNetworkPolicy", "error", err)
		return err
	}

	return nil
}

func CreateAllowNamespaceCommunicationNetworkPolicy(namespaceName string) error {
	netpol := v1.NetworkPolicy{}
	netpol.ObjectMeta.Name = AllowNamespaceCommunicationNetPolName
	netpol.ObjectMeta.Namespace = namespaceName

	netpol.Spec.PodSelector = metav1.LabelSelector{}
	netpol.Spec.PodSelector.MatchLabels = make(map[string]string)
	netpol.Spec.PodSelector.MatchLabels["ns"] = namespaceName

	netpol.Spec.Ingress = make([]v1.NetworkPolicyIngressRule, 1)
	netpol.Spec.Ingress[0].From = make([]v1.NetworkPolicyPeer, 1)
	netpol.Spec.Ingress[0].From[0] = v1.NetworkPolicyPeer{
		PodSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"ns": namespaceName,
			},
		},
	}

	// general label for all mogenius netpols
	netpol.ObjectMeta.Labels = make(map[string]string)
	netpol.ObjectMeta.Labels[NetpolLabel] = "true"

	clientset := clientProvider.K8sClientSet()
	netPolClient := clientset.NetworkingV1().NetworkPolicies(namespaceName)
	_, err := netPolClient.Get(context.Background(), AllowNamespaceCommunicationNetPolName, metav1.GetOptions{})
	if err != nil {
		// create the deny-all-ingress policy
		_, err := netPolClient.Create(context.Background(), &netpol, MoCreateOptions(config))
		if err != nil {
			k8sLogger.Error("CreateAllowNamespaceCommunicationNetworkPolicy", "error", err)
			return err
		}
		return nil
	}

	// update the deny-all-ingress policy
	_, err = netPolClient.Update(context.Background(), &netpol, MoUpdateOptions(config))
	if err != nil {
		k8sLogger.Error("CreateAllowNamespaceCommunicationNetworkPolicy", "error", err)
		return err
	}
	return nil
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
		k8sLogger.Error("InitNetworkPolicyConfigMap", "error", err)
		return nil
	}
	return &configMap
}

type NetworkPolicy struct {
	Name     string `yaml:"name" validate:"required"`
	Protocol string `yaml:"protocol" validate:"required"`
	Port     uint16 `yaml:"port" validate:"required"`
	Type     string `yaml:"type" validate:"required"`
}

func makeListUnique(items []NetworkPolicy) []NetworkPolicy {
	seen := make(map[string]bool)
	unique := []NetworkPolicy{}

	for _, item := range items {
		if !seen[item.Name] {
			seen[item.Name] = true
			unique = append(unique, item)
		}
	}

	return unique
}

func UpdateNetworkPolicyTemplate(policies []NetworkPolicy) error {
	uniquePolicies := makeListUnique(policies)

	cfgMap := readDefaultConfigMap()

	yamlStr, err := yaml.Marshal(uniquePolicies)
	if err != nil {
		k8sLogger.Error("UpdateNetworkPolicyTemplate", "error", err)
		return err
	}

	cfgMap.Data[PolicyConfigMapKey] = string(yamlStr)

	// check if the configmap already exists
	clientset := clientProvider.K8sClientSet()
	client := clientset.CoreV1().ConfigMaps(config.Get("MO_OWN_NAMESPACE"))
	_, err = client.Update(context.Background(), cfgMap, metav1.UpdateOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = client.Create(context.Background(), cfgMap, MoCreateOptions(config))
			if err != nil {
				k8sLogger.Error("InitNetworkPolicyConfigMap", "error", err)
				return err
			}
		} else {
			k8sLogger.Error("InitNetworkPolicyConfigMap", "error", err)
			return err
		}
	}
	return nil
}

func ReadNetworkPolicyPorts() ([]dtos.K8sLabeledNetworkPolicyDto, error) {
	ClusterConfigMap := GetConfigMap(config.Get("MO_OWN_NAMESPACE"), PolicyConfigMapName)

	var result []dtos.K8sLabeledNetworkPolicyDto
	var policies []NetworkPolicy
	err := yaml.Unmarshal([]byte(ClusterConfigMap.Data[PolicyConfigMapKey]), &policies)

	if err != nil {
		k8sLogger.Error("Error unmarshalling YAML", "error", err)
		return nil, err
	}
	for _, policy := range policies {
		result = append(result, dtos.K8sLabeledNetworkPolicyDto{
			Name:     policy.Name,
			Type:     dtos.Ingress,
			Port:     uint16(policy.Port),
			PortType: dtos.PortTypeEnum(policy.Protocol),
		})
		result = append(result, dtos.K8sLabeledNetworkPolicyDto{
			Name:     policy.Name,
			Type:     dtos.Egress,
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

	clientset := clientProvider.K8sClientSet()
	netPolClient := clientset.NetworkingV1().NetworkPolicies(namespaceName)

	errors := []error{}
	if netpols != nil {
		for _, netpol := range netpols.Items {
			err = netPolClient.Delete(context.Background(), netpol.Name, metav1.DeleteOptions{})
			if err != nil {
				k8sLogger.Error("RemoveAllConflictingNetworkPolicies", "error", err)
				errors = append(errors, err)
			}
		}
		if len(errors) > 0 {
			return fmt.Errorf("failed to remove all network policies: %v", errors)
		}
	}
	return nil
}

func EnforceNetworkPolicyManagerForNamespace(namespaceName string) error {
	// delete all conflicting network policies
	err := RemoveAllConflictingNetworkPolicies(namespaceName)
	if err != nil {
		return fmt.Errorf("failed to remove all conflicting network policies: %v", err)
	}
	// remove deny-all network policy
	_ = DeleteNetworkPolicyByName(namespaceName, "deny-all")

	// add deny-all-ingress network policy
	err = CreateDenyAllIngressNetworkPolicy(namespaceName)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create deny-all-ingress network policy: %v", err)
	}

	// add allow-namespace-communication network policy
	err = CreateAllowNamespaceCommunicationNetworkPolicy(namespaceName)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create allow-namespace-communication network policy: %v", err)
	}

	// delete network policies named with namespace name
	policies, err := store.ListNetworkPolicies(valkeyClient, namespaceName)
	if err == nil {
		for _, policy := range policies {
			if policy.Name == namespaceName {
				err = DeleteNetworkPolicyByName(namespaceName, policy.Name)
				if err != nil {
					k8sLogger.Error("EnforceNetworkPolicyManagerForNamespace", "error", err)
				}
			}
		}
	}

	// delete all pods in namespace to enforce new network policies
	err = DeleteAllPodsInNamespace(namespaceName)
	if err != nil {
		return fmt.Errorf("failed to delete all pods in namespace: %v", err)
	}
	return nil
}

func DeleteNetworkPolicyByName(namespaceName string, policyName string) error {
	netPolClient := clientProvider.K8sClientSet().NetworkingV1().NetworkPolicies(namespaceName)
	err := netPolClient.Delete(context.Background(), policyName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete network policy %s: %v", policyName, err)
	}

	// find in deployment, daemonset, statefulset and remove label
	errors := []error{}
	controllers := []unstructured.Unstructured{}

	GetUnstructuredResourceList := func(group, version, name string, namespace *string) []unstructured.Unstructured {
		result, _ := GetUnstructuredResourceList(group, version, name, namespace)
		if result != nil {
			return result.Items
		}
		return nil
	}
	controllers = append(controllers, GetUnstructuredResourceList("apps/v1", "", "deployments", &namespaceName)...)
	controllers = append(controllers, GetUnstructuredResourceList("apps/v1", "", "daemonsets", &namespaceName)...)
	controllers = append(controllers, GetUnstructuredResourceList("apps/v1", "", "statefulsets", &namespaceName)...)

	dynamicClient := clientProvider.DynamicClient()

	for i := range controllers {
		ctrl := controllers[i]
		kind := ctrl.GetKind()
		name := ctrl.GetName()
		var gvr schema.GroupVersionResource
		switch kind {
		case "Deployment":
			gvr = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
		case "DaemonSet":
			gvr = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}
		case "StatefulSet":
			gvr = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
		default:
			k8sLogger.Error("DeleteNetworkPolicyByName", "error", fmt.Errorf("unsupported kind: %s", kind))
			errors = append(errors, err)
			continue
		}
		latestObject, err := dynamicClient.Resource(gvr).Namespace(namespaceName).Get(
			context.Background(),
			name,
			metav1.GetOptions{},
		)
		if err != nil {
			k8sLogger.Error("DeleteNetworkPolicyByName", "error", err)
			errors = append(errors, err)
			continue
		}

		// remove labels from metadata
		updatedLabels := latestObject.GetLabels()
		for key := range updatedLabels {
			if strings.HasPrefix(key, PoliciesLabelPrefix) {
				delete(updatedLabels, key)
			}
		}
		latestObject.SetLabels(updatedLabels)

		// remove labels from spec.template.metadata
		spec, ok := latestObject.Object["spec"].(map[string]any)
		if !ok {
			continue
		}
		template, ok := spec["template"].(map[string]any)
		if !ok {
			continue
		}
		metadata, ok := template["metadata"].(map[string]any)
		if !ok {
			continue
		}
		if metadata["labels"] != nil {
			labels := metadata["labels"].(map[string]any)
			for key := range labels {
				if strings.Contains(key, PoliciesLabelPrefix) {
					delete(labels, key)
				}
			}
		}

		// update the object
		_, err = dynamicClient.Resource(gvr).Namespace(namespaceName).Update(
			context.Background(),
			latestObject,
			MoUpdateOptions(config),
		)
		if err != nil {
			k8sLogger.Error("DeleteNetworkPolicyByName", "error", err)
			errors = append(errors, err)
			continue
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to remove labels from controllers: %v", errors)
	}

	return nil
}

func DisableNetworkPolicyManagerForNamespace(namespaceName string) error {
	// get all network policies in the namespace created by mogenius
	netPols, err := store.ListNetworkPolicies(valkeyClient, namespaceName)
	if err != nil {
		return fmt.Errorf("failed to list all network policies: %v", err)
	}
	errors := []error{}
	for _, netPol := range netPols {
		// if NetpolLabel exits in the labels and is set to false, skip
		if netPol.Labels == nil || netPol.ObjectMeta.Labels[NetpolLabel] == "false" {
			continue
		}
		// delete the network policy
		err = DeleteNetworkPolicyByName(namespaceName, netPol.Name)
		// err = netPolClient.Delete(context.Background(), netPol.Name, metav1.DeleteOptions{})
		if err != nil {
			k8sLogger.Error("DisableNetworkPolicyManagerForNamespace", "error", err)
			errors = append(errors, err)
		}
	}

	err = CleanupLabeledNetworkPolicies(namespaceName)
	if err != nil {
		k8sLogger.Error("CleanupLabeledNetworkPolicies", "error", err)
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to remove unmanaged network policies: %v", errors)
	}

	return nil
}

func ListAllConflictingNetworkPolicies(namespaceName string) (*v1.NetworkPolicyList, error) {
	policies, err := store.ListNetworkPolicies(valkeyClient, namespaceName)
	if err != nil {
		return nil, err
	}

	// filter out all policies that are not created by mogenius
	var netpols []v1.NetworkPolicy
	for _, policy := range policies {

		hasLabels := policy.ObjectMeta.Labels != nil
		isManagedMogeniusNetworkPolicy := hasLabels && policy.ObjectMeta.Labels[NetpolLabel] == "true"
		isLegacyMogeniusNamespaceNetworkPolicy := hasLabels && policy.ObjectMeta.Labels["mo-created-by"] == "mogenius-k8s-manager" && func() bool {
			_, exists := policy.ObjectMeta.Labels["mo-app"]
			return !exists
		}()
		isMogeniusNetworkPolicy := isManagedMogeniusNetworkPolicy || isLegacyMogeniusNamespaceNetworkPolicy
		if isMogeniusNetworkPolicy {
			continue
		}
		netpols = append(netpols, policy)
	}

	// create list *v1.NetworkPolicyList
	return &v1.NetworkPolicyList{
		Items: netpols,
	}, nil
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
		deployment, err := store.GetByKeyParts[appsv1.Deployment](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.DeploymentResource.Group, utils.DeploymentResource.Kind, namespaceName, controllerName)
		if err != nil {
			return nil, fmt.Errorf("ListControllerLabeledNetworkPolicies %s ERROR: %s", controllerType, err.Error())
		}
		labels = extractLabels(deployment.ObjectMeta.Labels, deployment.Spec.Template.ObjectMeta.Labels)
	case dtos.DAEMON_SET:
		daemonset, err := store.GetByKeyParts[appsv1.DaemonSet](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.DaemonSetResource.Group, utils.DaemonSetResource.Kind, namespaceName, controllerName)
		if err != nil {
			return nil, fmt.Errorf("ListControllerLabeledNetworkPolicies %s ERROR: %s", controllerType, err.Error())
		}
		labels = extractLabels(daemonset.ObjectMeta.Labels, daemonset.Spec.Template.ObjectMeta.Labels)
	case dtos.STATEFUL_SET:
		statefulset, err := store.GetByKeyParts[appsv1.StatefulSet](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.StatefulSetResource.Group, utils.StatefulSetResource.Kind, namespaceName, controllerName)
		if err != nil {
			return nil, fmt.Errorf("ListControllerLabeledNetworkPolicies %s ERROR: %s", controllerType, err.Error())
		}
		labels = extractLabels(statefulset.ObjectMeta.Labels, statefulset.Spec.Template.ObjectMeta.Labels)
	default:
		return nil, fmt.Errorf("unsupported controller type %s", controllerType)
	}

	netpols := []dtos.K8sLabeledNetworkPolicyDto{}

	policies, err := store.ListNetworkPolicies(valkeyClient, namespaceName)
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

func GetNetworkPolicyName(labelPolicy dtos.K8sLabeledNetworkPolicyDto) string {
	return strings.ToLower(
		fmt.Sprintf("%s-%s-%s", PoliciesLabelPrefix, labelPolicy.Name, labelPolicy.Type),
	)
}

func ensureDenyAllIngressRule(namespaceName string) error {
	if namespaceName == config.Get("MO_OWN_NAMESPACE") {
		return fmt.Errorf("cannot create network-policies in %s namespace", config.Get("MO_OWN_NAMESPACE"))
	}
	clientset := clientProvider.K8sClientSet()
	netPolClient := clientset.NetworkingV1().NetworkPolicies(namespaceName)

	_, err := netPolClient.Get(context.Background(), DenyAllIngressNetPolName, metav1.GetOptions{})
	if err != nil {
		k8sLogger.Info("networkpolicy not found, it will be created.", "networkpolicy", DenyAllIngressNetPolName)

		err = CreateDenyAllIngressNetworkPolicy(namespaceName)
		if err != nil {
			k8sLogger.Error("failed to create networkpolicy", "networkpolicy", DenyAllIngressNetPolName, "error", err)
			return err
		}
	}
	return nil
}

func RemoveUnmanagedNetworkPolicies(namespaceName string, policies []string) error {
	if namespaceName == "kube-system" {
		return fmt.Errorf("cannot remove network-policies from kube-system namespace")
	}

	netpols, err := ListAllConflictingNetworkPolicies(namespaceName)
	if err != nil {
		return fmt.Errorf("failed to list all network policies: %v", err)
	}

	clientset := clientProvider.K8sClientSet()
	netPolClient := clientset.NetworkingV1().NetworkPolicies(namespaceName)

	errors := []error{}
	for _, netpol := range netpols.Items {
		if !slices.Contains(policies, netpol.Name) {
			continue
		}

		err = netPolClient.Delete(context.Background(), netpol.Name, metav1.DeleteOptions{})
		if err != nil {
			k8sLogger.Error("RemoveUnmanagedNetworkPolicies", "error", err)
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("failed to remove unmanaged network policies: %v", errors)
	}

	return nil
}

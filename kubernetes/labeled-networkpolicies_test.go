package kubernetes

import (
	"context"
	"mogenius-k8s-manager/dtos"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PolicyName1 = "mogenius-policy-123"
	PolicyName2 = "mogenius-policy-098"
)

func TestCreateNetworkPolicyServiceWithLabel(t *testing.T) {
	var namespaceName = "mogenius"

	var labelPolicy1 = dtos.K8sLabeledNetworkPolicyDto{
		Name:     PolicyName1,
		Type:     dtos.Ingress,
		Port:     80,
		PortType: dtos.PortTypeHTTPS,
	}
	err := EnsureLabeledNetworkPolicy(namespaceName, labelPolicy1)
	if err != nil {
		t.Errorf("Error creating network policy: %s", err.Error())
	}

	var labelPolicy2 = dtos.K8sLabeledNetworkPolicyDto{
		Name:     PolicyName2,
		Type:     dtos.Egress,
		Port:     59999,
		PortType: dtos.PortTypeUDP,
	}

	err = EnsureLabeledNetworkPolicy(namespaceName, labelPolicy2)
	if err != nil {
		t.Errorf("Error creating network policy: %s", err.Error())
	}
}

func TestInitNetworkPolicyConfigMap(t *testing.T) {
	err := InitNetworkPolicyConfigMap()
	if err != nil {
		t.Errorf("Error initializing network policy config map: %s", err.Error())
	}
}

func TestReadNetworkPolicyPorts(t *testing.T) {
	ports, err := ReadNetworkPolicyPorts()
	if err != nil {
		t.Errorf("Error reading network policy ports: %s", err.Error())
	}
	if len(ports) == 0 {
		t.Errorf("Error reading network policy ports")
	}
	// check if ports contains a imap named port fo egress
	var found bool
	for _, port := range ports {
		if port.Name == "imap" && port.Type == dtos.Ingress && port.Port == 143 && port.PortType == dtos.PortTypeTCP {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Error reading network policy ports")
	}

}

func TestAttachAndDetachLabeledNetworkPolicy(t *testing.T) {
	var namespaceName = "mogenius"

	// create simple nginx deployment with k8s
	exampleDeploy := createNginxDeployment()

	client := GetAppClient()
	_, err := client.Deployments(namespaceName).Create(context.TODO(), exampleDeploy, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		t.Errorf("Error creating deployment: %s", err.Error())
	}
	// sleep for 5 seconds to allow the deployment to be created
	// real world scenario wouldn't have this problem, as we assume existing controllers
	time.Sleep(5 * time.Second)

	defer client.Deployments(namespaceName).Delete(context.TODO(), exampleDeploy.Name, metav1.DeleteOptions{})

	// attach network policy
	var labelPolicy = dtos.K8sLabeledNetworkPolicyDto{
		Name:     PolicyName1,
		Type:     dtos.Ingress,
		Port:     80,
		PortType: dtos.PortTypeHTTPS,
	}

	err = AttachLabeledNetworkPolicy(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName, labelPolicy)
	if err != nil {
		t.Errorf("Error attaching network policy: %s", err.Error())
	}

	// detach network policy
	err = DetachLabeledNetworkPolicy(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName, labelPolicy)
	if err != nil {
		t.Errorf("Error detaching network policy: %s", err.Error())
	}
}
func TestListAllConflictingNetworkPolicies(t *testing.T) {
	list, err := ListAllConflictingNetworkPolicies("mogenius")
	if err != nil {
		t.Errorf("Error listing conflicting network policies: %s", err.Error())
	}
	t.Log(list)
}

func TestRemoveAllNetworkPolicies(t *testing.T) {
	t.Skip("skipping this test for manual testing")

	RemoveAllConflictingNetworkPolicies("mogenius")
}

func TestListControllerLabeledNetworkPolicy(t *testing.T) {
	var namespaceName = "mogenius"

	// create simple nginx deployment with k8s
	exampleDeploy := createNginxDeployment()

	client := GetAppClient()
	_, err := client.Deployments(namespaceName).Create(context.TODO(), exampleDeploy, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		t.Errorf("Error creating deployment: %s", err.Error())
	}
	// sleep for 5 seconds to allow the deployment to be created
	// real world scenario wouldn't have this problem, as we assume existing controllers
	time.Sleep(10 * time.Second)

	defer client.Deployments(namespaceName).Delete(context.TODO(), exampleDeploy.Name, metav1.DeleteOptions{})
	// attach network policy
	var labelPolicy1 = dtos.K8sLabeledNetworkPolicyDto{
		Name:     PolicyName1,
		Type:     dtos.Ingress,
		Port:     80,
		PortType: dtos.PortTypeTCP,
	}

	err = AttachLabeledNetworkPolicy(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName, labelPolicy1)
	if err != nil {
		t.Errorf("Error attaching network policy: %s", err.Error())
	}
	// attach network policy
	var labelPolicy2 = dtos.K8sLabeledNetworkPolicyDto{
		Name:     PolicyName2,
		Type:     dtos.Egress,
		Port:     80,
		PortType: dtos.PortTypeHTTPS,
	}

	err = AttachLabeledNetworkPolicy(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName, labelPolicy2)
	if err != nil {
		t.Errorf("Error attaching network policy: %s", err.Error())
	}

	list, err := ListControllerLabeledNetworkPolicies(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName)
	if err != nil {
		t.Errorf("Error listing conflicting network policies: %s", err.Error())
	}
	t.Log(list)
}

func TestDeleteNetworkPolicy(t *testing.T) {
	var namespaceName = "mogenius"

	err := DeleteNetworkPolicy(namespaceName, PolicyName1)
	if err != nil {
		t.Errorf("Error deleting network policy: %s. %s", PolicyName1, err.Error())
	}
	err = DeleteNetworkPolicy(namespaceName, PolicyName2)
	if err != nil {
		t.Errorf("Error deleting network policy: %s. %s", PolicyName2, err.Error())
	}
}

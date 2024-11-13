package kubernetes_test

import (
	"context"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/utils"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PolicyName1 = "mogenius-policy-123"
	PolicyName2 = "mogenius-policy-098"
)

var labelPolicy1 = dtos.K8sLabeledNetworkPolicyDto{
	Name:     PolicyName1,
	Type:     dtos.Egress,
	Port:     80,
	PortType: dtos.PortTypeTCP,
}

var labelPolicy2 = dtos.K8sLabeledNetworkPolicyDto{
	Name:     PolicyName2,
	Type:     dtos.Ingress,
	Port:     59999,
	PortType: dtos.PortTypeUDP,
}

func createNginxDeployment() *v1.Deployment {
	replicas := int32(1)
	labels := map[string]string{
		"app": "nginx",
	}

	return &v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-deployment",
		},
		Spec: v1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestCreateNetworkPolicyServiceWithLabel(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})

	err := kubernetes.EnsureLabeledNetworkPolicy("default", labelPolicy1)
	if err != nil {
		t.Errorf("Error creating network policy: %s", err.Error())
	}

	err = kubernetes.EnsureLabeledNetworkPolicy("default", labelPolicy2)
	if err != nil {
		t.Errorf("Error creating network policy: %s", err.Error())
	}
}

func TestInitNetworkPolicyConfigMap(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)

	err := kubernetes.InitNetworkPolicyConfigMap()
	if err != nil {
		t.Errorf("Error initializing network policy config map: %s", err.Error())
	}
}

func TestReadNetworkPolicyPorts(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})

	ports, err := kubernetes.ReadNetworkPolicyPorts()
	if err != nil {
		t.Errorf("Error reading network policy ports: %s", err.Error())
	}
	if len(ports) == 0 {
		t.Errorf("Error reading network policy ports because they len() == 0")
	}
	// check if ports contains a imap named port fo egress
	var found bool
	for _, port := range ports {
		if port.Name == "imap-TCP" && port.Type == dtos.Ingress && port.Port == 143 && port.PortType == dtos.PortTypeTCP {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Networkpolicy port for imap not found. failing test.")
	}

}

func TestAttachAndDetachLabeledNetworkPolicy(t *testing.T) {
	var namespaceName = "mogenius"

	// create simple nginx deployment with k8s
	exampleDeploy := createNginxDeployment()

	client := kubernetes.GetAppClient()
	_, err := client.Deployments(namespaceName).Create(context.TODO(), exampleDeploy, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		t.Errorf("Error creating deployment: %s", err.Error())
	}
	// sleep to allow the deployment to be created
	// real world scenario wouldn't have this problem, as we assume existing controllers
	time.Sleep(5 * time.Second)

	defer func() {
		err := client.Deployments(namespaceName).Delete(context.TODO(), exampleDeploy.Name, metav1.DeleteOptions{})
		if err != nil {
			t.Error(err)
		}
	}()

	// attach network policy
	err = kubernetes.AttachLabeledNetworkPolicies(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName, []dtos.K8sLabeledNetworkPolicyDto{labelPolicy1})
	if err != nil {
		t.Errorf("Error attaching network policy: %s", err.Error())
	}

	// sleep to allow the deployment to be updated
	// real world scenario wouldn't have this problem, as we assume existing controllers
	time.Sleep(5 * time.Second)

	// detach network policy
	err = kubernetes.DetachLabeledNetworkPolicy(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName, labelPolicy1)
	if err != nil {
		t.Errorf("Error detaching network policy: %s", err.Error())
	}
}

func TestListAllConflictingNetworkPolicies(t *testing.T) {
	store.Start()
	list, err := kubernetes.ListAllConflictingNetworkPolicies("mogenius")
	if err != nil {
		t.Errorf("Error listing conflicting network policies: %s", err.Error())
	}
	t.Log(list)
}

func TestRemoveAllNetworkPolicies(t *testing.T) {
	t.Skip("skipping this test for manual testing")

	err := kubernetes.RemoveAllConflictingNetworkPolicies("mogenius")
	if err != nil {
		t.Error(err)
	}
}

func TestCleanupMogeniusNetworkPolicies(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)

	err := kubernetes.CleanupLabeledNetworkPolicies("mogenius")
	if err != nil {
		t.Errorf("Error TestCleanupMogeniusNetworkPolicies: %s", err.Error())
	}
}

func TestListControllerLabeledNetworkPolicy(t *testing.T) {
	var namespaceName = "mogenius"

	logManager := interfaces.NewMockSlogManager(t)
	store.Setup(logManager)
	store.Start()

	// create simple nginx deployment with k8s
	exampleDeploy := createNginxDeployment()

	client := kubernetes.GetAppClient()
	_, err := client.Deployments(namespaceName).Create(context.TODO(), exampleDeploy, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		t.Errorf("Error creating deployment: %s", err.Error())
	}
	// sleep to allow the deployment to be created
	// real world scenario wouldn't have this problem, as we assume existing controllers
	time.Sleep(5 * time.Second)
	defer func() {
		err := client.Deployments(namespaceName).Delete(context.TODO(), exampleDeploy.Name, metav1.DeleteOptions{})
		if err != nil {
			t.Errorf("Error deleting deployments: %s", err)
		}
	}()

	// attach network policy
	err = kubernetes.AttachLabeledNetworkPolicies(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName, []dtos.K8sLabeledNetworkPolicyDto{labelPolicy1})
	if err != nil {
		t.Errorf("Error attaching network policy: %s", err.Error())
	}

	// sleep to allow the deployment to be updated
	// real world scenario wouldn't have this problem, as we assume existing controllers
	time.Sleep(5 * time.Second)

	// attach network policy
	err = kubernetes.AttachLabeledNetworkPolicies(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName, []dtos.K8sLabeledNetworkPolicyDto{labelPolicy2})
	if err != nil {
		t.Errorf("Error attaching network policy: %s", err.Error())
	}

	list, err := kubernetes.ListControllerLabeledNetworkPolicies(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName)
	if err != nil {
		t.Errorf("Error listing conflicting network policies: %s", err.Error())
	}
	t.Log(list)
}

func TestDeleteNetworkPolicy(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)

	err := kubernetes.DeleteNetworkPolicy("mogenius", kubernetes.GetNetworkPolicyName(labelPolicy1))
	if err != nil {
		t.Errorf("Error deleting network policy: %s. %s", PolicyName1, err.Error())
	}
	err = kubernetes.DeleteNetworkPolicy("mogenius", kubernetes.GetNetworkPolicyName(labelPolicy2))
	if err != nil {
		t.Errorf("Error deleting network policy: %s. %s", PolicyName2, err.Error())
	}
}

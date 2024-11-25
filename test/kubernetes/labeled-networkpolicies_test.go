package kubernetes_test

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/assert"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/watcher"
	"path/filepath"
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
	logManager := logging.NewMockSlogManager(t)
	config := cfg.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_PATH",
		DefaultValue: utils.Pointer(filepath.Join(t.TempDir(), "mogenius.db")),
	})
	watcherModule := watcher.NewWatcher()
	err := kubernetes.Setup(logManager, config, watcherModule)
	assert.Assert(err == nil, err)

	err = kubernetes.EnsureLabeledNetworkPolicy("default", labelPolicy1)
	assert.Assert(err == nil, err)

	err = kubernetes.EnsureLabeledNetworkPolicy("default", labelPolicy2)
	assert.Assert(err == nil, err)
}

func TestInitNetworkPolicyConfigMap(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	config := cfg.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_PATH",
		DefaultValue: utils.Pointer(filepath.Join(t.TempDir(), "mogenius.db")),
	})
	watcherModule := watcher.NewWatcher()
	err := kubernetes.Setup(logManager, config, watcherModule)
	assert.Assert(err == nil, err)

	err = kubernetes.InitNetworkPolicyConfigMap()
	assert.Assert(err == nil, err)
}

func TestReadNetworkPolicyPorts(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	config := cfg.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_PATH",
		DefaultValue: utils.Pointer(filepath.Join(t.TempDir(), "mogenius.db")),
	})
	watcherModule := watcher.NewWatcher()
	err := kubernetes.Setup(logManager, config, watcherModule)
	assert.Assert(err == nil, err)

	ports, err := kubernetes.ReadNetworkPolicyPorts()
	assert.Assert(err == nil, err)
	assert.Assert(len(ports) > 0, "Error reading network policy ports")

	// check if ports contains a imap named port fo egress
	var found bool
	for _, port := range ports {
		if port.Name == "imap" && port.Type == dtos.Ingress && port.Port == 143 && port.PortType == dtos.PortTypeTCP {
			found = true
			break
		}
	}
	t.Log(ports)
	assert.Assert(found, "NetworkPolicy port for imap not found")
}

func TestAttachAndDetachLabeledNetworkPolicy(t *testing.T) {
	t.Skip("test currently relies on sleep introducing flakyness")
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

	t.Cleanup(func() {
		err := client.Deployments(namespaceName).Delete(context.TODO(), exampleDeploy.Name, metav1.DeleteOptions{})
		if err != nil {
			t.Error(err)
		}
	})

	// attach network policy
	err = kubernetes.AttachLabeledNetworkPolicies(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName, []dtos.K8sLabeledNetworkPolicyDto{labelPolicy1})
	assert.Assert(err == nil, err)

	// sleep to allow the deployment to be updated
	// real world scenario wouldn't have this problem, as we assume existing controllers
	time.Sleep(5 * time.Second)

	// detach network policy
	err = kubernetes.DetachLabeledNetworkPolicy(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName, labelPolicy1)
	assert.Assert(err == nil, err)
}

func TestListAllConflictingNetworkPolicies(t *testing.T) {
	store.Start()
	list, err := kubernetes.ListAllConflictingNetworkPolicies("mogenius")
	assert.Assert(err == nil, err)
	t.Log(list)
}

func TestRemoveAllNetworkPolicies(t *testing.T) {
	t.Skip("skipping this test for manual testing")
	err := kubernetes.RemoveAllConflictingNetworkPolicies("mogenius")
	assert.Assert(err == nil, err)
}

func TestCleanupMogeniusNetworkPolicies(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	config := cfg.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_PATH",
		DefaultValue: utils.Pointer(filepath.Join(t.TempDir(), "mogenius.db")),
	})
	watcherModule := watcher.NewWatcher()
	err := kubernetes.Setup(logManager, config, watcherModule)
	assert.Assert(err == nil, err)

	err = kubernetes.CleanupLabeledNetworkPolicies("mogenius")
	assert.Assert(err == nil, err)
}

func TestListControllerLabeledNetworkPolicy(t *testing.T) {
	t.Skip("test currently relies on sleep introducing flakyness")
	var namespaceName = "mogenius"

	logManager := logging.NewMockSlogManager(t)
	store.Setup(logManager)
	store.Start()

	// create simple nginx deployment with k8s
	exampleDeploy := createNginxDeployment()

	client := kubernetes.GetAppClient()
	_, err := client.Deployments(namespaceName).Create(context.Background(), exampleDeploy, metav1.CreateOptions{})
	assert.Assert(err == nil || apierrors.IsAlreadyExists(err))

	// sleep to allow the deployment to be created
	// real world scenario wouldn't have this problem, as we assume existing controllers
	// time.Sleep(5 * time.Second)
	t.Cleanup(func() {
		err := client.Deployments(namespaceName).Delete(context.TODO(), exampleDeploy.Name, metav1.DeleteOptions{})
		if err != nil {
			t.Error(err)
		}
	})

	// attach network policy
	err = kubernetes.AttachLabeledNetworkPolicies(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName, []dtos.K8sLabeledNetworkPolicyDto{labelPolicy1})
	assert.Assert(err == nil, err)

	// sleep to allow the deployment to be updated
	// real world scenario wouldn't have this problem, as we assume existing controllers
	time.Sleep(5 * time.Second)

	// attach network policy
	err = kubernetes.AttachLabeledNetworkPolicies(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName, []dtos.K8sLabeledNetworkPolicyDto{labelPolicy2})
	assert.Assert(err == nil, err)

	list, err := kubernetes.ListControllerLabeledNetworkPolicies(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName)
	assert.Assert(err == nil, err)
	t.Log(list)
}

func TestDeleteNetworkPolicy(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	config := cfg.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_PATH",
		DefaultValue: utils.Pointer(filepath.Join(t.TempDir(), "mogenius.db")),
	})
	watcherModule := watcher.NewWatcher()
	err := kubernetes.Setup(logManager, config, watcherModule)
	assert.Assert(err == nil, err)

	err = kubernetes.DeleteNetworkPolicy("mogenius", kubernetes.GetNetworkPolicyName(labelPolicy1))
	assert.Assert(err != nil, err, fmt.Sprintf("error deleting network policy %s", PolicyName1))
	err = kubernetes.DeleteNetworkPolicy("mogenius", kubernetes.GetNetworkPolicyName(labelPolicy2))
	assert.Assert(err != nil, err, fmt.Sprintf("error deleting network policy %s", PolicyName2))
}

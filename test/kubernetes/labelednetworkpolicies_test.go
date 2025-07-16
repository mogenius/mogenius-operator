package kubernetes_test

import (
	"context"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"
	"os"
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
	logManager := logging.NewSlogManager(slog.LevelDebug, []slog.Handler{slog.NewJSONHandler(os.Stderr, nil)})
	config := cfg.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})
	config.Declare(cfg.ConfigDeclaration{
		Key:          "KUBERNETES_DEBUG",
		DefaultValue: utils.Pointer("false"),
	})
	config.Declare(cfg.ConfigDeclaration{
		Key:          "OWN_DEPLOYMENT_NAME",
		DefaultValue: utils.Pointer("local"),
	})
	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"), config)
	storeModule := valkeyclient.NewValkeyClient(logManager.CreateLogger("valkey"), config)
	err := kubernetes.Setup(logManager, config, clientProvider, storeModule)
	assert.AssertT(t, err == nil, err)

	err = kubernetes.EnsureLabeledNetworkPolicy("default", labelPolicy1)
	assert.AssertT(t, err == nil, err)

	err = kubernetes.EnsureLabeledNetworkPolicy("default", labelPolicy2)
	assert.AssertT(t, err == nil, err)
}

func TestInitNetworkPolicyConfigMap(t *testing.T) {
	logManager := logging.NewSlogManager(slog.LevelDebug, []slog.Handler{slog.NewJSONHandler(os.Stderr, nil)})
	config := cfg.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "KUBERNETES_DEBUG",
		DefaultValue: utils.Pointer("false"),
	})
	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"), config)
	storeModule := valkeyclient.NewValkeyClient(logManager.CreateLogger("valkey"), config)
	err := kubernetes.Setup(logManager, config, clientProvider, storeModule)
	assert.AssertT(t, err == nil, err)

	err = kubernetes.InitNetworkPolicyConfigMap()
	assert.AssertT(t, err == nil, err)
}

func TestReadNetworkPolicyPorts(t *testing.T) {
	logManager := logging.NewSlogManager(slog.LevelDebug, []slog.Handler{slog.NewJSONHandler(os.Stderr, nil)})
	config := cfg.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})
	config.Declare(cfg.ConfigDeclaration{
		Key:          "KUBERNETES_DEBUG",
		DefaultValue: utils.Pointer("false"),
	})
	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"), config)
	storeModule := valkeyclient.NewValkeyClient(logManager.CreateLogger("valkey"), config)
	err := kubernetes.Setup(logManager, config, clientProvider, storeModule)
	assert.AssertT(t, err == nil, err)

	ports, err := kubernetes.ReadNetworkPolicyPorts()
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, len(ports) > 0, "Error reading network policy ports")

	// check if ports contains a imap named port fo egress
	var found bool
	for _, port := range ports {
		if port.Name == "imap" && port.Type == dtos.Ingress && port.Port == 143 && port.PortType == dtos.PortTypeTCP {
			found = true
			break
		}
	}
	t.Log(ports)
	assert.AssertT(t, found, "NetworkPolicy port for imap not found")
}

func TestAttachAndDetachLabeledNetworkPolicy(t *testing.T) {
	t.Skip("test currently relies on sleep introducing flakyness")
	var namespaceName = "mogenius"

	// create simple nginx deployment with k8s
	exampleDeploy := createNginxDeployment()

	config := cfg.NewConfig()
	logManager := logging.NewSlogManager(slog.LevelDebug, []slog.Handler{slog.NewJSONHandler(os.Stderr, nil)})
	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"), config)
	clientset := clientProvider.K8sClientSet()
	client := clientset.AppsV1()
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
	assert.AssertT(t, err == nil, err)

	// sleep to allow the deployment to be updated
	// real world scenario wouldn't have this problem, as we assume existing controllers
	time.Sleep(5 * time.Second)

	// detach network policy
	err = kubernetes.DetachLabeledNetworkPolicy(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName, labelPolicy1)
	assert.AssertT(t, err == nil, err)
}

func TestListAllConflictingNetworkPolicies(t *testing.T) {
	logManager := logging.NewSlogManager(slog.LevelDebug, []slog.Handler{slog.NewJSONHandler(os.Stderr, nil)})
	config := cfg.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "KUBERNETES_DEBUG",
		DefaultValue: utils.Pointer("false"),
	})
	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"), config)
	storeModule := valkeyclient.NewValkeyClient(logManager.CreateLogger("valkey"), config)
	err := kubernetes.Setup(logManager, config, clientProvider, storeModule)
	assert.AssertT(t, err == nil, err)
	list, err := kubernetes.ListAllConflictingNetworkPolicies("mogenius")
	assert.AssertT(t, err == nil, err)
	t.Log(list)
}

func TestRemoveAllNetworkPolicies(t *testing.T) {
	t.Skip("skipping this test for manual testing")
	err := kubernetes.RemoveAllConflictingNetworkPolicies("mogenius")
	assert.AssertT(t, err == nil, err)
}

func TestCleanupMogeniusNetworkPolicies(t *testing.T) {
	logManager := logging.NewSlogManager(slog.LevelDebug, []slog.Handler{slog.NewJSONHandler(os.Stderr, nil)})
	config := cfg.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "KUBERNETES_DEBUG",
		DefaultValue: utils.Pointer("false"),
	})
	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"), config)
	storeModule := valkeyclient.NewValkeyClient(logManager.CreateLogger("valkey"), config)
	err := kubernetes.Setup(logManager, config, clientProvider, storeModule)
	assert.AssertT(t, err == nil, err)

	err = kubernetes.CleanupLabeledNetworkPolicies("mogenius")
	assert.AssertT(t, err == nil, err)
}

func TestListControllerLabeledNetworkPolicy(t *testing.T) {
	t.Skip("test currently relies on sleep introducing flakyness")
	var namespaceName = "mogenius"

	logManager := logging.NewSlogManager(slog.LevelDebug, []slog.Handler{slog.NewJSONHandler(os.Stderr, nil)})

	// create simple nginx deployment with k8s
	exampleDeploy := createNginxDeployment()

	config := cfg.NewConfig()
	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"), config)
	clientset := clientProvider.K8sClientSet()
	client := clientset.AppsV1()
	_, err := client.Deployments(namespaceName).Create(context.Background(), exampleDeploy, metav1.CreateOptions{})
	assert.AssertT(t, err == nil || apierrors.IsAlreadyExists(err))

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
	assert.AssertT(t, err == nil, err)

	// sleep to allow the deployment to be updated
	// real world scenario wouldn't have this problem, as we assume existing controllers
	time.Sleep(5 * time.Second)

	// attach network policy
	err = kubernetes.AttachLabeledNetworkPolicies(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName, []dtos.K8sLabeledNetworkPolicyDto{labelPolicy2})
	assert.AssertT(t, err == nil, err)

	list, err := kubernetes.ListControllerLabeledNetworkPolicies(exampleDeploy.Name, dtos.K8sServiceControllerEnum(exampleDeploy.Kind), namespaceName)
	assert.AssertT(t, err == nil, err)
	t.Log(list)
}

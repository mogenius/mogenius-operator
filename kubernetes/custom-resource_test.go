package kubernetes_test

import (
	"mogenius-k8s-manager/config"
	"mogenius-k8s-manager/interfaces"
	"mogenius-k8s-manager/kubernetes"
	"testing"
)

// test the functionality of the custom resource with a basic pod
func TestCustomResource(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logManager := interfaces.NewMockSlogManager()
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)

	yamlData := `apiVersion: v1
kind: Pod
metadata:
  name: mypod
spec:
  containers:
  - name: mycontainer
    image: busybox
    command: ['sh', '-c', 'echo Hello Kubernetes! && sleep 3600']
`
	// CREATE
	err := kubernetes.ApplyResource(yamlData, false)
	if err != nil {
		t.Errorf("Error applying resource: %s", err.Error())
	}

	// UPDATE (same resource), on second call the update client call is tested
	err = kubernetes.ApplyResource(yamlData, false)
	if err != nil {
		t.Errorf("Error applying resource: %s", err.Error())
	}

	// GET
	_, err = kubernetes.GetResource("", "v1", "Pods", "mypod", "default", false)
	if err != nil {
		t.Errorf("Error getting resource: %s", err.Error())
	}

	// LIST
	_, err = kubernetes.ListResources("", "v1", "Pods", "default", false)
	if err != nil {
		t.Errorf("Error listing resources: %s", err.Error())
	}

	// DELETE
	err = kubernetes.DeleteResource("", "v1", "Pods", "mypod", "default", false)
	if err != nil {
		t.Errorf("Error deleting resource: %s", err.Error())
	}
}

// test the functionality of the custom resource with a
// properly "custom" resource, the secret store
func TestSecretStoreResource(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logManager := interfaces.NewMockSlogManager()
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)

	yamlData := `apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: test-secret-store
spec:
  provider:
    vault:
      server: "http://vault.default.svc.cluster.local:8200"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "mogenius-external-secrets"
          serviceAccountRef:
            name: "external-secrets-sa"
`
	// prereq:
	err := kubernetes.ApplyServiceAccount("external-secrets-sa", "default", nil)
	if err != nil {
		t.Error(err)
	}

	// CREATE
	err = kubernetes.ApplyResource(yamlData, true)
	if err != nil {
		t.Errorf("Error applying resource: %s", err.Error())
	}

	// UPDATE (same resource), on second call the update client call is tested
	err = kubernetes.ApplyResource(yamlData, true)
	if err != nil {
		t.Errorf("Error applying resource: %s", err.Error())
	}

	// LIST
	_, err = kubernetes.ListResources("external-secrets.io", "v1beta1", "clustersecretstores", "", true)
	if err != nil {
		t.Errorf("Error listing resources: %s", err.Error())
	}

	// GET
	_, err = kubernetes.GetResource("external-secrets.io", "v1beta1", "clustersecretstores", "test-secret-store", "", true)
	if err != nil {
		t.Errorf("Error getting resource: %s", err.Error())
	}

	// DELETE
	err = kubernetes.DeleteResource("external-secrets.io", "v1beta1", "clustersecretstores", "test-secret-store", "", true)
	if err != nil {
		t.Errorf("Error deleting resource: %s", err.Error())
	}

	err = kubernetes.DeleteServiceAccount("external-secrets-sa", "default")
	if err != nil {
		t.Error(err)
	}
}

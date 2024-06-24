package kubernetes

import (
	"testing"

	"github.com/mogenius/punq/logger"
)

// test the functionality of the custom resource with a basic pod
func TestCustomResource(t *testing.T) {
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
	err := ApplyResource(yamlData, false)
	if err != nil {
		t.Errorf("Error applying resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource applied ✅")
	}

	// UPDATE (same resource), on second call the update client call is tested
	err = ApplyResource(yamlData, false)
	if err != nil {
		t.Errorf("Error applying resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource updated ✅")
	}

	// GET
	_, err = GetResource("", "v1", "Pods", "mypod", "default", false)
	if err != nil {
		t.Errorf("Error getting resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource retrieved ✅")
	}

	// LIST
	_, err = ListResources("", "v1", "Pods", "default", false)
	if err != nil {
		t.Errorf("Error listing resources: %s", err.Error())
	} else {
		logger.Log.Info("Resources listed ✅")
	}

	// DELETE
	err = DeleteResource("", "v1", "Pods", "mypod", "default", false)
	if err != nil {
		t.Errorf("Error deleting resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource deleted ✅")
	}
}

// test the functionality of the custom resource with a
// properly "custom" resource, the secret store
func TestSecretStoreResource(t *testing.T) {
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
	// CREATE
	err := ApplyResource(yamlData, true)
	if err != nil {
		t.Errorf("Error applying resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource applied ✅")
	}

	// UPDATE (same resource), on second call the update client call is tested
	err = ApplyResource(yamlData, true)
	if err != nil {
		t.Errorf("Error applying resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource updated ✅")
	}

	// LIST
	_, err = ListResources("external-secrets.io", "v1beta1", "clustersecretstores", "", true)
	if err != nil {
		t.Errorf("Error listing resources: %s", err.Error())
	} else {
		logger.Log.Info("Resources listed ✅")
	}

	// GET
	_, err = GetResource("external-secrets.io", "v1beta1", "clustersecretstores", "test-secret-store", "", true)
	if err != nil {
		t.Errorf("Error getting resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource retrieved ✅")
	}

	// DELETE
	err = DeleteResource("external-secrets.io", "v1beta1", "clustersecretstores", "test-secret-store", "", true)
	if err != nil {
		t.Errorf("Error deleting resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource deleted ✅")
	}
}

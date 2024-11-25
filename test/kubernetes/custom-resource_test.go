package kubernetes_test

import (
	"mogenius-k8s-manager/src/assert"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/watcher"
	"mogenius-k8s-manager/test"
	"path/filepath"
	"testing"
	"time"
)

// test the functionality of the custom resource with a basic pod
func TestCustomResource(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := cfg.NewConfig()
	watcherModule := watcher.NewWatcher()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_PATH",
		DefaultValue: utils.Pointer(filepath.Join(t.TempDir(), "mogenius.db")),
	})
	err := kubernetes.Setup(logManager, config, watcherModule)
	assert.Assert(err == nil, err)
	yamlData := test.YamlSanitize(`
	apiVersion: v1
	kind: Pod
	metadata:
	  name: mypod
	spec:
	  containers:
	  - name: mycontainer
	    image: busybox
	    command: ['sh', '-c', 'echo Hello Kubernetes! && sleep 3600']
	`)
	// CREATE
	err = kubernetes.ApplyResource(yamlData, false)
	assert.Assert(err == nil, err)

	// UPDATE (same resource), on second call the update client call is tested
	err = kubernetes.ApplyResource(yamlData, false)
	assert.Assert(err == nil, err)

	// GET
	_, err = kubernetes.GetResource("", "v1", "Pods", "mypod", "default", false)
	assert.Assert(err == nil, err)

	// LIST
	_, err = kubernetes.ListResources("", "v1", "Pods", "default", false)
	assert.Assert(err == nil, err)

	// DELETE
	err = kubernetes.DeleteResource("", "v1", "Pods", "mypod", "default", false)
	assert.Assert(err == nil, err)
}

// test the functionality of the custom resource with a
// properly "custom" resource, the secret store
func TestSecretStoreResource(t *testing.T) {
	t.Skip("test currently relies on sleep introducing flakyness")
	logManager := interfaces.NewMockSlogManager(t)
	config := cfg.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_PATH",
		DefaultValue: utils.Pointer(filepath.Join(t.TempDir(), "mogenius.db")),
	})
	watcherModule := watcher.NewWatcher()
	err := kubernetes.Setup(logManager, config, watcherModule)
	assert.Assert(err == nil, err)

	yamlData := test.YamlSanitize(`
	apiVersion: external-secrets.io/v1beta1
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
	`)
	// prereq:
	err = kubernetes.ApplyServiceAccount("external-secrets-sa", "default", nil)
	assert.Assert(err == nil, err)

	// CREATE
	err = kubernetes.ApplyResource(yamlData, true)
	assert.Assert(err == nil, err)

	// UPDATE (same resource), on second call the update client call is tested
	time.Sleep(5 * time.Second)
	err = kubernetes.ApplyResource(yamlData, true)
	assert.Assert(err == nil, err)

	// LIST
	_, err = kubernetes.ListResources("external-secrets.io", "v1beta1", "clustersecretstores", "", true)
	assert.Assert(err == nil, err)

	// GET
	_, err = kubernetes.GetResource("external-secrets.io", "v1beta1", "clustersecretstores", "test-secret-store", "", true)
	assert.Assert(err == nil, err)

	// DELETE
	err = kubernetes.DeleteResource("external-secrets.io", "v1beta1", "clustersecretstores", "test-secret-store", "", true)
	assert.Assert(err == nil, err)

	err = kubernetes.DeleteServiceAccount("external-secrets-sa", "default")
	assert.Assert(err == nil, err)
}

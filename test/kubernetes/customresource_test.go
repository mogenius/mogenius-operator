package kubernetes_test

import (
	"mogenius-k8s-manager/src/assert"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/redisstore"
	"mogenius-k8s-manager/test"
	"testing"
	"time"
)

// test the functionality of the custom resource with a basic pod
func TestCustomResource(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	config := cfg.NewConfig()
	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"))
	watcherModule := kubernetes.NewWatcher(logManager.CreateLogger("watcher"), clientProvider)
	redisStoreModule := redisstore.NewRedisStore(logManager.CreateLogger("redisstore"), config)
	err := kubernetes.Setup(logManager, config, watcherModule, clientProvider, redisStoreModule)
	assert.AssertT(t, err == nil, err)
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
	assert.AssertT(t, err == nil, err)

	// UPDATE (same resource), on second call the update client call is tested
	err = kubernetes.ApplyResource(yamlData, false)
	assert.AssertT(t, err == nil, err)

	// GET
	_, err = kubernetes.GetResource("", "v1", "Pods", "mypod", "default", false)
	assert.AssertT(t, err == nil, err)

	// LIST
	_, err = kubernetes.ListResources("", "v1", "Pods", "default", false)
	assert.AssertT(t, err == nil, err)

	// DELETE
	err = kubernetes.DeleteResource("", "v1", "Pods", "mypod", "default", false)
	assert.AssertT(t, err == nil, err)
}

// test the functionality of the custom resource with a
// properly "custom" resource, the secret store
func TestSecretStoreResource(t *testing.T) {
	t.Skip("test currently relies on sleep introducing flakyness")
	logManager := logging.NewMockSlogManager(t)
	config := cfg.NewConfig()
	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"))
	watcherModule := kubernetes.NewWatcher(logManager.CreateLogger("watcher"), clientProvider)
	redisStoreModule := redisstore.NewRedisStore(logManager.CreateLogger("redisstore"), config)
	err := kubernetes.Setup(logManager, config, watcherModule, clientProvider, redisStoreModule)
	assert.AssertT(t, err == nil, err)

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
	assert.AssertT(t, err == nil, err)

	// CREATE
	err = kubernetes.ApplyResource(yamlData, true)
	assert.AssertT(t, err == nil, err)

	// UPDATE (same resource), on second call the update client call is tested
	time.Sleep(5 * time.Second)
	err = kubernetes.ApplyResource(yamlData, true)
	assert.AssertT(t, err == nil, err)

	// LIST
	_, err = kubernetes.ListResources("external-secrets.io", "v1beta1", "clustersecretstores", "", true)
	assert.AssertT(t, err == nil, err)

	// GET
	_, err = kubernetes.GetResource("external-secrets.io", "v1beta1", "clustersecretstores", "test-secret-store", "", true)
	assert.AssertT(t, err == nil, err)

	// DELETE
	err = kubernetes.DeleteResource("external-secrets.io", "v1beta1", "clustersecretstores", "test-secret-store", "", true)
	assert.AssertT(t, err == nil, err)

	err = kubernetes.DeleteServiceAccount("external-secrets-sa", "default")
	assert.AssertT(t, err == nil, err)
}

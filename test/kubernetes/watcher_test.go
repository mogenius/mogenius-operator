package kubernetes_test

import (
	"mogenius-k8s-manager/src/assert"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/test"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func TestWatcher(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	config := cfg.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_DEBUG",
		DefaultValue: utils.Pointer("true"),
	})
	structs.Setup(logManager, config)

	createNewDeplString := test.YamlSanitize(`
	apiVersion: apps/v1
	kind: Deployment
	metadata:
	  name: testdepl
	  namespace: default
	  labels:
	    app: my-app
	spec:
	  replicas: 1
	  selector:
	    matchLabels:
	      app: my-app
	  template:
	    metadata:
	      labels:
	        app: my-app
	        addedlabel: newlabel
	    spec:
	      containers:
	        - name: my-container
	          image: nginx
	          ports:
	            - containerPort: 80
	`)

	updatedDeplString := test.YamlSanitize(`
	apiVersion: apps/v1
	kind: Deployment
	metadata:
	  name: testdepl
	  namespace: default
	  labels:
	    app: my-app
	    whoop: whoop
	spec:
	  replicas: 1
	  selector:
	    matchLabels:
	      app: my-app
	  template:
	    metadata:
	      labels:
	        app: my-app
	        addedlabel: newlabel
	    spec:
	      containers:
	        - name: my-container
	          image: nginx
	          ports:
	            - containerPort: 80
	`)

	// LIST ALL AVAILABLE
	_, err := kubernetes.GetAvailableResources()
	assert.Assert(err == nil, err)

	// LIST ITEMS IN WORKLOAD
	_, err = kubernetes.GetUnstructuredResourceList("apps/v1", "", "deployments", utils.Pointer(""))
	assert.Assert(err == nil, err)

	_, err = kubernetes.GetUnstructuredResourceList("", "v1", "pods", utils.Pointer(""))
	assert.Assert(err == nil, err)

	_, err = kubernetes.GetUnstructuredResourceList("", "v1", "secrets", utils.Pointer(""))
	assert.Assert(err == nil, err)

	_, err = kubernetes.GetUnstructuredResourceList("", "v1", "persistentvolumes", utils.Pointer(""))
	assert.Assert(err == nil, err)

	_, err = kubernetes.GetUnstructuredResourceList("", "v1", "namespaces", nil)
	assert.Assert(err == nil, err)

	_, err = kubernetes.GetUnstructuredResourceList("k3s.cattle.io/v1", "v1", "addons", utils.Pointer(""))
	assert.Assert(err == nil, err)

	// GET WORKLOAD
	_, err = kubernetes.GetUnstructuredResource("apps/v1", "", "deployments", "kube-system", "coredns")
	assert.Assert(err == nil, err)

	// DESCRIBE
	_, err = kubernetes.DescribeUnstructuredResource("apps/v1", "", "deployments", "kube-system", "coredns")
	assert.Assert(err == nil, err)

	// NEW WORKLOAD
	_, err = kubernetes.CreateUnstructuredResource("apps/v1", "", "deployments", utils.Pointer(""), createNewDeplString)
	assert.Assert(err == nil || apierrors.IsAlreadyExists(err), err, createNewDeplString)

	// UPDATE WORKLOAD
	_, err = kubernetes.UpdateUnstructuredResource("apps/v1", "", "deployments", utils.Pointer(""), updatedDeplString)
	assert.Assert(err == nil, err)
}

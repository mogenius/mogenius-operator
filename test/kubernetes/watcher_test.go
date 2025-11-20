package kubernetes_test

import (
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/test"
	"os"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func TestWatcher(t *testing.T) {
	logManager := logging.NewSlogManager(slog.LevelDebug, []slog.Handler{slog.NewJSONHandler(os.Stderr, nil)})
	structs.Setup(logManager)

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
	assert.AssertT(t, err == nil, err)

	// LIST ITEMS IN WORKLOAD
	_, err = kubernetes.GetUnstructuredResourceList("deployments", "apps/v1", utils.Pointer(""))
	assert.AssertT(t, err == nil, err)

	_, err = kubernetes.GetUnstructuredResourceList("pods", "v1", utils.Pointer(""))
	assert.AssertT(t, err == nil, err)

	_, err = kubernetes.GetUnstructuredResourceList("secrets", "v1", utils.Pointer(""))
	assert.AssertT(t, err == nil, err)

	_, err = kubernetes.GetUnstructuredResourceList("persistentvolumes", "v1", utils.Pointer(""))
	assert.AssertT(t, err == nil, err)

	_, err = kubernetes.GetUnstructuredResourceList("namespaces", "v1", nil)
	assert.AssertT(t, err == nil, err)

	_, err = kubernetes.GetUnstructuredResourceList("addons", "k3s.cattle.io/v1", utils.Pointer(""))
	assert.AssertT(t, err == nil, err)

	// GET WORKLOAD
	_, err = kubernetes.GetUnstructuredResource("apps/v1", "deployments", "kube-system", "coredns")
	assert.AssertT(t, err == nil, err)

	// DESCRIBE
	_, err = kubernetes.DescribeUnstructuredResource("apps/v1", "deployments", "kube-system", "coredns")
	assert.AssertT(t, err == nil, err)

	// NEW WORKLOAD
	_, err = kubernetes.CreateUnstructuredResource("apps/v1", "deployments", "", createNewDeplString)
	assert.AssertT(t, err == nil || apierrors.IsAlreadyExists(err), err, createNewDeplString)

	// UPDATE WORKLOAD
	_, err = kubernetes.UpdateUnstructuredResource("apps/v1", "deployments", "", updatedDeplString)
	assert.AssertT(t, err == nil, err)
}

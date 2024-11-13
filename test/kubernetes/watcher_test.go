package kubernetes_test

import (
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"testing"
)

func TestWatcher(t *testing.T) {
	createNewDeplString := `apiVersion: apps/v1
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
        - containerPort: 80`

	updatedDeplString := `apiVersion: apps/v1
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
        - containerPort: 80`

	// LIST ALL AVAILABLE
	resources, err := kubernetes.GetAvailableResources()
	if err != nil {
		t.Fatalf("Error GetAvailableResources: %s", err.Error())
	} else {
		t.Logf("%d resources found ✅", len(resources))
	}

	// LIST ITEMS IN WORKLOAD
	deplList, err := kubernetes.GetUnstructuredResourceList("apps/v1", "", "deployments", utils.Pointer(""))
	if err != nil {
		t.Fatalf("Error GetUnstructuredResourceList deployments: %s", err.Error())
	} else {
		t.Logf("%d deployments found ✅", len(deplList.Items))
	}
	podList, err := kubernetes.GetUnstructuredResourceList("", "v1", "pods", utils.Pointer(""))
	if err != nil {
		t.Fatalf("Error GetUnstructuredResourceList pods: %s", err.Error())
	} else {
		t.Logf("%d pods found ✅", len(podList.Items))
	}
	secList, err := kubernetes.GetUnstructuredResourceList("", "v1", "secrets", utils.Pointer(""))
	if err != nil {
		t.Fatalf("Error GetUnstructuredResourceList pods: %s", err.Error())
	} else {
		t.Logf("%d secrets found ✅", len(secList.Items))
	}
	pvList, err := kubernetes.GetUnstructuredResourceList("", "v1", "persistentvolumes", utils.Pointer(""))
	if err != nil {
		t.Fatalf("Error GetUnstructuredResourceList pods: %s", err.Error())
	} else {
		t.Logf("%d persistentvolumes found ✅", len(pvList.Items))
	}
	nsList, err := kubernetes.GetUnstructuredResourceList("", "v1", "namespaces", nil)
	if err != nil {
		t.Fatalf("Error GetUnstructuredResourceList namespaces: %s", err.Error())
	} else {
		t.Logf("%d namespaces found ✅", len(nsList.Items))
	}
	k3sAddonsList, err := kubernetes.GetUnstructuredResourceList("k3s.cattle.io/v1", "v1", "addons", utils.Pointer(""))
	if err != nil {
		t.Fatalf("Error GetUnstructuredResourceList k3sAddons: %s", err.Error())
	} else {
		t.Logf("%d k3s addons found ✅", len(k3sAddonsList.Items))
	}

	// GET WORKLOAD
	getObj, err := kubernetes.GetUnstructuredResource("apps/v1", "", "deployments", "kube-system", "coredns")
	if err != nil {
		t.Fatalf("Error describing deployments: %s", err.Error())
	} else {
		t.Log(getObj)
		t.Log("Get object success ✅")
	}

	// DESCRIBE
	describeStr, err := kubernetes.DescribeUnstructuredResource("apps/v1", "", "deployments", "kube-system", "coredns")
	if err != nil {
		t.Fatalf("Error describing deployments: %s", err.Error())
	} else {
		t.Log(describeStr)
		t.Log("Description generated ✅")
	}

	// NEW WORKLOAD
	depl, err := kubernetes.CreateUnstructuredResource("apps/v1", "", "deployments", utils.Pointer(""), createNewDeplString)
	if err != nil {
		t.Fatalf("Error creating deployment: %s", err.Error())
	} else {
		t.Logf("Deployment created: %s ✅", depl.GetName())
	}

	// UPDATE WORKLOAD
	deplUpdated, err := kubernetes.UpdateUnstructuredResource("apps/v1", "", "deployments", utils.Pointer(""), updatedDeplString)
	if err != nil {
		t.Fatalf("Error updating deployment: %s", err.Error())
	} else {
		t.Logf("Deployment updated: %s ✅", deplUpdated.GetName())
	}

	// DELETE WORKLOAD
	err = kubernetes.DeleteUnstructuredResource("apps/v1", "", "deployments", "default", updatedDeplString)
	if err != nil {
		t.Fatalf("Error deleting deployment: %s", err.Error())
	} else {
		t.Log("Deployment deleted ✅")
	}
}

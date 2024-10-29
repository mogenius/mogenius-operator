package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/interfaces"
	"testing"
)

// compile time check
func TestWatcherAdheresToInterface(t *testing.T) {
	watcher := NewWatcher()
	testfunc := func(w interfaces.KubernetesWatcher) {}
	testfunc(&watcher) // this checks if the typesystem allows to call it
}

func TestWatcher(t *testing.T) {
	t.Log("TestWatcher")

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

	yamlString := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: coredns
  namespace: kube-system
  labels:
    app: my-app
spec:
  containers:
  - name: my-container
    image: nginx
    ports:
    - containerPort: 80`

	// LIST ALL AVAILABLE
	resources, err := GetAvailableResources()
	if err != nil {
		t.Errorf("Error GetAvailableResources: %s", err.Error())
	} else {
		t.Logf("%d resources found ✅", len(resources))
	}

	// LIST ITEMS IN WORKLOAD
	deplList, err := GetUnstructuredResourceList("apps/v1", "", "deployments", true)
	if err != nil {
		t.Errorf("Error GetUnstructuredResourceList deployments: %s", err.Error())
	} else {
		t.Logf("%d deployments found ✅", len(deplList.Items))
	}
	podList, err := GetUnstructuredResourceList("", "v1", "pods", true)
	if err != nil {
		t.Errorf("Error GetUnstructuredResourceList pods: %s", err.Error())
	} else {
		t.Logf("%d pods found ✅", len(podList.Items))
	}
	secList, err := GetUnstructuredResourceList("", "v1", "secrets", true)
	if err != nil {
		t.Errorf("Error GetUnstructuredResourceList pods: %s", err.Error())
	} else {
		t.Logf("%d secrets found ✅", len(secList.Items))
	}
	pvList, err := GetUnstructuredResourceList("", "v1", "persistentvolumes", true)
	if err != nil {
		t.Errorf("Error GetUnstructuredResourceList pods: %s", err.Error())
	} else {
		t.Logf("%d persistentvolumes found ✅", len(pvList.Items))
	}

	// DESCRIBE
	describeStr, err := DescribeUnstructuredResource("apps/v1", "", "deployments", true, yamlString)
	if err != nil {
		t.Errorf("Error describing deployments: %s", err.Error())
	} else {
		fmt.Println(describeStr)
		t.Log("Describtion generated ✅")
	}

	// NEW WORKLOAD
	depl, err := CreateUnstructuredResource("apps/v1", "", "deployments", true, createNewDeplString)
	if err != nil {
		t.Errorf("Error creating deployment: %s", err.Error())
	} else {
		t.Logf("Deployment created: %s ✅", depl.GetName())
	}

	// UPDATE WORKLOAD
	deplUpdated, err := UpdateUnstructuredResource("apps/v1", "", "deployments", true, updatedDeplString)
	if err != nil {
		t.Errorf("Error updating deployment: %s", err.Error())
	} else {
		t.Logf("Deployment updated: %s ✅", deplUpdated.GetName())
	}

	// DELETE WORKLOAD
	err = DeleteUnstructuredResource("apps/v1", "", "deployments", true, updatedDeplString)
	if err != nil {
		t.Errorf("Error deleting deployment: %s", err.Error())
	} else {
		t.Log("Deployment deleted ✅")
	}
}

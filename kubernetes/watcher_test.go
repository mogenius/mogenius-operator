package kubernetes

import (
	"fmt"
	"testing"
)

func TestWatcher(t *testing.T) {
	t.Log("TestWatcher")

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

	// Test the example generator
	describeStr, err := DescribeUnstructuredResource("apps/v1", "", "deployments", true, yamlString)
	if err != nil {
		t.Errorf("Error generating example: %s", err.Error())
	} else {
		fmt.Println(describeStr)
		t.Log("Example generated ✅")
	}
}

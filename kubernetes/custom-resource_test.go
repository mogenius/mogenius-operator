package kubernetes

import (
	"testing"

	"github.com/mogenius/punq/logger"
)

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
	err := ApplyResource(yamlData)
	if err != nil {
		t.Errorf("Error applying resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource applied ✅")
	}

	// GET
	_, err = GetResource("", "v1", "Pods", "mypod", "default")
	if err != nil {
		t.Errorf("Error getting resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource retrieved ✅")
	}

	// LIST
	_, err = ListResources("", "v1", "Pods", "default")
	if err != nil {
		t.Errorf("Error listing resources: %s", err.Error())
	} else {
		logger.Log.Info("Resources listed ✅")
	}

	// DELETE
	err = DeleteResource("", "v1", "Pods", "mypod", "default")
	if err != nil {
		t.Errorf("Error deleting resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource deleted ✅")
	}
}

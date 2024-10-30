package kubernetes

import (
	"testing"
)

// test the functionality of the custom resource with a basic pod
func TestExample(t *testing.T) {
	// unknown resource
	yaml := GetResourceTemplateYaml("v1", "Pod", "mypod", "default", "mypod")
	if yaml == "" {
		t.Errorf("Error getting resource template")
	} else {
		K8sLogger.Info(yaml)
		K8sLogger.Info("Unknown Resource template retrieved ✅")
	}

	// known resource
	knownResourceYaml := GetResourceTemplateYaml("v1", "Deployment", "testtemplate", "default", "mypod")
	if knownResourceYaml == "" {
		t.Errorf("Error getting resource template")
	} else {
		K8sLogger.Info(knownResourceYaml)
		K8sLogger.Info("Known Resource template retrieved ✅")
	}
}

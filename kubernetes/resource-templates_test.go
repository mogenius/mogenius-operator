package kubernetes

import (
	utils "mogenius-k8s-manager/utils"
	"testing"
)

// test the functionality of the custom resource with a basic pod
func TestResourceTemplates(t *testing.T) {
	utils.CONFIG.Kubernetes.OwnNamespace = "mogenius"

	// CREATE
	err := CreateOrUpdateResourceTemplateConfigmap()
	if err != nil {
		t.Errorf("Error creating resource template configmap: %s", err.Error())
	} else {
		k8sLogger.Info("Resource template configmap created ✅")
	}

	// unknown resource
	yaml := GetResourceTemplateYaml("", "v1", "mypod", "Pod", "default", "mypod")
	if yaml == "" {
		t.Errorf("Error getting resource template")
	} else {
		k8sLogger.Info(yaml)
		k8sLogger.Info("Unknown Resource template retrieved ✅")
	}

	// known resource Deployment
	knownResourceYaml := GetResourceTemplateYaml("v1", "Deployment", "testtemplate", "Pod", "default", "mypod")
	if knownResourceYaml == "" {
		t.Errorf("Error getting resource template")
	} else {
		k8sLogger.Info(knownResourceYaml)
		k8sLogger.Info("Known Resource Deployment template retrieved ✅")
	}

	// known resource Certificate
	knownResourceYamlCert := GetResourceTemplateYaml("cert-manager.io/v1", "v1", "certificates", "Certificate", "default", "mypod")
	if knownResourceYamlCert == "" {
		t.Errorf("Error getting resource template")
	} else {
		k8sLogger.Info(knownResourceYamlCert)
		k8sLogger.Info("Known Resource Certificate template retrieved ✅")
	}
}

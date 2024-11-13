package kubernetes_test

import (
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"testing"
)

// test the functionality of the custom resource with a basic pod
func TestResourceTemplates(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logManager := interfaces.NewMockSlogManager()
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})

	// CREATE
	err := kubernetes.CreateOrUpdateResourceTemplateConfigmap()
	if err != nil {
		t.Errorf("Error creating resource template configmap: %s", err.Error())
	}

	// unknown resource
	yaml := kubernetes.GetResourceTemplateYaml("", "v1", "mypod", "Pod", "default", "mypod")
	if yaml == "" {
		t.Errorf("Error getting resource template")
	}

	// known resource Deployment
	knownResourceYaml := kubernetes.GetResourceTemplateYaml("v1", "Deployment", "testtemplate", "Pod", "default", "mypod")
	if knownResourceYaml == "" {
		t.Errorf("Error getting resource template")
	}

	// known resource Certificate
	knownResourceYamlCert := kubernetes.GetResourceTemplateYaml("cert-manager.io/v1", "v1", "certificates", "Certificate", "default", "mypod")
	if knownResourceYamlCert == "" {
		t.Errorf("Error getting resource template")
	}
}

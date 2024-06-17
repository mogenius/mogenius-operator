package kubernetes

import (
	"testing"

	"github.com/mogenius/punq/logger"
)

func TestSecretStoreResource(t *testing.T) {
	yamlData := `apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: secret-store-vault-role-based
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
`
	// CREATE
	err := ApplyResource(yamlData)
	if err != nil {
		t.Errorf("Error applying resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource applied ✅")
	}

	// // GET
	// _, err = GetResource("", "v1", "Pods", "mypod", "default")
	// if err != nil {
	// 	t.Errorf("Error getting resource: %s", err.Error())
	// } else {
	// 	logger.Log.Info("Resource retrieved ✅")
	// }

	// // LIST
	// _, err = ListResources("", "v1", "Pods", "default")
	// if err != nil {
	// 	t.Errorf("Error listing resources: %s", err.Error())
	// } else {
	// 	logger.Log.Info("Resources listed ✅")
	// }

	// // DELETE
	// err = DeleteResource("", "v1", "Pods", "mypod", "default")
	// if err != nil {
	// 	t.Errorf("Error deleting resource: %s", err.Error())
	// } else {
	// 	logger.Log.Info("Resource deleted ✅")
	// }
}

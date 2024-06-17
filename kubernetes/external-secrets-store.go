package kubernetes

import (
	"mogenius-k8s-manager/utils"
)

func CreateExternalSecretsStore() {
	// CreateOrUpdateYamlString(utils.InitExternalSecretsStoreYaml())
	ApplyResource(utils.InitExternalSecretsStoreYaml())

}

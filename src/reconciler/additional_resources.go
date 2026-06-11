package reconciler

import (
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/utils"
)

func externalSecretResource(name, namespace string, externalSecret v1alpha1.ExternalSecret) map[string]any {
	key := "token"
	if externalSecret.Key != "" {
		key = externalSecret.Key
	}

	return map[string]any{
		"apiVersion": utils.ExternalSecretResource.ApiVersion,
		"kind":       utils.ExternalSecretResource.Kind,
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"data": []map[string]any{
				{
					"remoteRef": map[string]any{
						"key":      externalSecret.Path,
						"property": key,
					},
					"secretKey": key,
				},
			},
			"secretStoreRef": map[string]any{
				"kind": utils.ClusterSecretStoreResource.Kind,
				"name": externalSecret.Vault,
			},
			"target": map[string]any{
				"creationPolicy": "Owner",
				"deletionPolicy": "Merge",
				"name":           name,
			},
		},
	}
}

package kubernetes

import (
	"fmt"
	"mogenius-operator/src/utils"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/yaml"
)

const (
	RESOURCE_TEMPLATE_CONFIGMAP = "mogenius-resource-templates"
)

func GetResourceTemplateYaml(apiVersion, kind string) string {
	// check if example data exists
	yamlStr, err := loadResourceTemplateData(kind)
	if err == nil {
		return yamlStr
	}

	// default response
	obj := unstructured.Unstructured{}
	obj.SetKind(kind)
	obj.SetAPIVersion(apiVersion)

	obj.SetLabels(map[string]string{
		"example": "label",
	})
	data, err := yaml.Marshal(obj.Object)
	if err != nil {
		return ""
	}

	return string(data)
}

func loadResourceTemplateData(kind string) (string, error) {
	// load example data from file
	configmap, err := GetUnstructuredResource(utils.ConfigMapResource.ApiVersion, utils.ConfigMapResource.Plural, config.Get("MO_OWN_NAMESPACE"), RESOURCE_TEMPLATE_CONFIGMAP)
	if err != nil {
		return "", err
	}
	configmapData, ok := configmap.Object["data"].(map[string]any)
	if !ok {
		return "", nil
	}
	for key, v := range configmapData {
		if key == kind {
			dataStr := v.(string)
			if dataStr != "" {
				obj := unstructured.Unstructured{}
				err := yaml.Unmarshal([]byte(dataStr), &obj)
				if err != nil {
					continue
				}

				data, err := yaml.Marshal(obj.Object)
				if err != nil {
					return "", err
				}
				return string(data), nil
			}
		}
	}
	return "", fmt.Errorf("resource template '%s' not found", kind)
}

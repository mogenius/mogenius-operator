package kubernetes

import (
	"fmt"
	utils "mogenius-k8s-manager/src/utils"

	punqUtils "github.com/mogenius/punq/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/yaml"
)

const (
	RESOURCE_TEMPLATE_CONFIGMAP = "mogenius-resource-templates"
)

func CreateOrUpdateResourceTemplateConfigmap() error {
	yamlData := utils.InitResourceTemplatesYaml()

	// Decode YAML data into a generic map
	var decodedData map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlData), &decodedData)
	if err != nil {
		return err
	}

	cfgMap := unstructured.Unstructured{Object: decodedData}
	cfgMap.SetNamespace(config.Get("MO_OWN_NAMESPACE"))
	cfgMap.SetName(RESOURCE_TEMPLATE_CONFIGMAP)

	// Marshal cfgMap back to YAML
	updatedYaml, err := yaml.Marshal(cfgMap.Object)
	if err != nil {
		return err
	}

	// check if configmap exists
	_, err = CreateUnstructuredResource("", "v1", "configmaps", punqUtils.Pointer(""), string(updatedYaml))
	if apierrors.IsAlreadyExists(err) {
		k8sLogger.Info("Resource template configmap already exists")
		return nil
	}

	return err
}

func GetResourceTemplateYaml(group, version, name, kind, namespace, resourcename string) string {
	// check if example data exists
	yamlStr, err := loadResourceTemplateData(kind, namespace, resourcename)
	if err == nil {
		return yamlStr
	}

	// default response
	obj := unstructured.Unstructured{}
	obj.SetKind(kind)

	if group != "" && version == "" {
		obj.SetAPIVersion(group)
	}
	if group != "" && version != "" {
		obj.SetAPIVersion(fmt.Sprintf("%s/%s", group, version))
	}

	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	if resourcename != "" {
		obj.SetName(resourcename)
	}
	obj.SetLabels(map[string]string{
		"example": "label",
	})
	data, err := yaml.Marshal(obj.Object)
	if err != nil {
		return ""
	}

	return string(data)
}

func loadResourceTemplateData(kind, namespace, resourcename string) (string, error) {
	// load example data from file
	configmap, err := GetUnstructuredResource("", "v1", "configmaps", config.Get("MO_OWN_NAMESPACE"), RESOURCE_TEMPLATE_CONFIGMAP)
	if err != nil {
		return "", err
	}
	configmapData, ok := configmap.Object["data"].(map[string]interface{})
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
				if namespace != "" {
					obj.SetNamespace(namespace)
				}
				if resourcename != "" {
					obj.SetName(resourcename)
				}

				data, err := yaml.Marshal(obj.Object)
				if err != nil {
					return "", err
				}
				return string(data), nil
			}
		}
	}
	return "", fmt.Errorf("Resource template '%s' not found", kind)
}

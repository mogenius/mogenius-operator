package kubernetes

import (
	"fmt"
	utils "mogenius-k8s-manager/utils"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/yaml"
)

const (
	RESOURCE_TEMPLATE_CONFIGMAP = "mogenius-resource-templates"
)

func CreateOrUpdateResourceTemplateConfigmap() error {
	yamlData := utils.InitResourceTemplatesYaml()

	cfgMap := unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(yamlData), cfgMap)
	if err != nil {
		return err
	}
	cfgMap.SetNamespace(utils.CONFIG.Kubernetes.OwnNamespace)
	cfgMap.SetName(RESOURCE_TEMPLATE_CONFIGMAP)

	updatedYaml, err := yaml.Marshal(cfgMap)
	if err != nil {
		return err
	}

	// check if configmap exists
	_, err = CreateUnstructuredResource("v1", "", "configmaps", true, string(updatedYaml))
	if apierrors.IsAlreadyExists(err) {
		K8sLogger.Info("Resource template configmap already exists")
	}

	return err
}

func GetResourceTemplateYaml(group, version, name, namespace, resourcename string) string {
	// check if example data exists
	yamlStr, err := loadResourceTemplateData(group, version, name, namespace, resourcename)
	if err == nil {
		return yamlStr
	}

	// default response
	obj := unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    name,
	})
	obj.SetNamespace(namespace)
	obj.SetName(resourcename)
	obj.SetLabels(map[string]string{
		"example": "label",
	})
	data, err := yaml.Marshal(obj)
	if err != nil {
		return ""
	}

	return string(data)
}

func loadResourceTemplateData(group, version, name, namespace, resourcename string) (string, error) {
	// load example data from file
	configmap, err := GetUnstructuredResource(group, version, name, utils.CONFIG.Kubernetes.OwnNamespace, RESOURCE_TEMPLATE_CONFIGMAP)
	if err != nil {
		return "", err
	}
	configmapData, ok := configmap.Object["data"].(map[string]string)
	if !ok {
		return "", nil
	}
	for key, v := range configmapData {
		if key == name {
			obj := unstructured.Unstructured{}
			err := yaml.Unmarshal([]byte(v), &obj)
			if err != nil {
				continue
			}
			obj.SetName(resourcename)
			obj.SetNamespace(namespace)

			data, err := yaml.Marshal(obj)
			if err != nil {
				return "", err
			}
			return string(data), nil
		}
	}
	return "", fmt.Errorf("Resource template '%s' not found", name)
}

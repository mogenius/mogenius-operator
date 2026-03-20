package kubernetes

import (
	"context"
	"log/slog"
	"mogenius-operator/src/ai"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"

	coreV1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

var config cfg.ConfigModule
var k8sLogger *slog.Logger
var clientProvider k8sclient.K8sClientProvider
var valkeyClient valkeyclient.ValkeyClient

func Setup(
	logManagerModule logging.SlogManager,
	configModule cfg.ConfigModule,
	clientProviderModule k8sclient.K8sClientProvider,
	valkey valkeyclient.ValkeyClient,
) error {
	k8sLogger = logManagerModule.CreateLogger("kubernetes")
	config = configModule
	clientProvider = clientProviderModule
	valkeyClient = valkey

	if utils.ClusterProviderCached == utils.UNKNOWN {
		foundProvider, err := GuessClusterProvider()
		if err != nil {
			k8sLogger.Error("GuessClusterProvider", "error", err)
		}
		utils.ClusterProviderCached = foundProvider
		k8sLogger.Debug("🎲 🎲 🎲 ClusterProvider", "foundProvider", string(foundProvider))
	}

	return nil
}

// GetSecret fetches a secret directly from the Kubernetes cluster
func GetSecret(namespace, name string) (*coreV1.Secret, error) {
	clientset := clientProvider.K8sClientSet()
	return clientset.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

// CreateOrUpdateAndMergePromptConfig updates the AI filters ConfigMap with new filter
// definitions from the platform, but preserves the isActive state from the existing
// ConfigMap so that user toggles survive platform pushes / reconnects.
func CreateOrUpdateAndMergePromptConfig(newPromptCfg ai.AiPromptConfig) (ai.AiPromptConfig, error) {
	namespace := config.Get("MO_OWN_NAMESPACE")

	// Read existing ConfigMap to preserve user-toggled isActive states
	existingFilters, existingUserFilters := readExistingFilterStates(namespace)
	mergeIsActiveState(newPromptCfg.Filters, existingFilters)
	mergeIsActiveState(newPromptCfg.UserFilters, existingUserFilters)

	err := writeFiltersConfigMap(namespace, newPromptCfg.Filters, newPromptCfg.UserFilters)
	return newPromptCfg, err
}

// UpdateFilterActiveStates updates only the isActive field of filters in the ConfigMap.
// This is used when the user explicitly toggles filters via the UI.
func UpdateFilterActiveStates(filters []ai.AiFilter, userFilters []ai.AiFilter) error {
	namespace := config.Get("MO_OWN_NAMESPACE")
	return writeFiltersConfigMap(namespace, filters, userFilters)
}

func writeFiltersConfigMap(namespace string, filters []ai.AiFilter, userFilters []ai.AiFilter) error {
	filterYaml, err := yaml.Marshal(filters)
	if err != nil {
		return err
	}
	userFiltersYaml, err := yaml.Marshal(userFilters)
	if err != nil {
		return err
	}

	cfgMap := coreV1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: utils.ConfigMapResource.ApiVersion,
			Kind:       utils.ConfigMapResource.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.AI_FILTERS_CONFIGMAP_NAME,
			Namespace: namespace,
		},
		Data: map[string]string{
			"filters":     string(filterYaml),
			"userFilters": string(userFiltersYaml),
		},
	}

	_, err = GetUnstructuredResource(utils.ConfigMapResource.ApiVersion, utils.ConfigMapResource.Plural, namespace, utils.AI_FILTERS_CONFIGMAP_NAME)
	if apierrors.IsNotFound(err) {
		cfgMapYaml, err := yaml.Marshal(cfgMap)
		if err != nil {
			return err
		}
		_, err = CreateUnstructuredResource(utils.ConfigMapResource.ApiVersion, utils.ConfigMapResource.Plural, true, string(cfgMapYaml))
		return err
	} else if err == nil {
		cfgMapYaml, err := yaml.Marshal(cfgMap)
		if err != nil {
			return err
		}
		_, err = UpdateUnstructuredResource(utils.ConfigMapResource.ApiVersion, utils.ConfigMapResource.Plural, true, string(cfgMapYaml))
		return err
	}
	return err
}

// readExistingFilterStates reads the current isActive states from the existing ConfigMap.
func readExistingFilterStates(namespace string) (map[string]bool, map[string]bool) {
	existing, err := GetUnstructuredResource(utils.ConfigMapResource.ApiVersion, utils.ConfigMapResource.Plural, namespace, utils.AI_FILTERS_CONFIGMAP_NAME)
	if err != nil || existing == nil {
		return nil, nil
	}

	data, found, err := unstructured.NestedStringMap(existing.Object, "data")
	if err != nil || !found {
		return nil, nil
	}

	parseFilterStates := func(yamlStr string) map[string]bool {
		var filters []ai.AiFilter
		if err := yaml.Unmarshal([]byte(yamlStr), &filters); err != nil {
			return nil
		}
		states := make(map[string]bool, len(filters))
		for _, f := range filters {
			if f.Id != "" {
				states[f.Id] = f.IsActive
			}
		}
		return states
	}

	var filterStates, userFilterStates map[string]bool
	if filtersYaml, ok := data["filters"]; ok {
		filterStates = parseFilterStates(filtersYaml)
	}
	if userFiltersYaml, ok := data["userFilters"]; ok {
		userFilterStates = parseFilterStates(userFiltersYaml)
	}
	return filterStates, userFilterStates
}

// mergeIsActiveState preserves isActive from the existing ConfigMap for filters
// that already exist, so that user toggles survive platform pushes.
func mergeIsActiveState(filters []ai.AiFilter, existingStates map[string]bool) {
	if existingStates == nil {
		return
	}
	for i := range filters {
		if active, ok := existingStates[filters[i].Id]; ok {
			filters[i].IsActive = active
		}
	}
}

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

// create or update ConfigMap for AI prompt configuration
func CreateOrUpdateAndMergePromptConfig(newPromptCfg ai.AiPromptConfig) (ai.AiPromptConfig, error) {
	filterYaml, err := yaml.Marshal(newPromptCfg.Filters)
	if err != nil {
		return newPromptCfg, err
	}
	userFiltersYaml, err := yaml.Marshal(newPromptCfg.UserFilters)
	if err != nil {
		return newPromptCfg, err
	}

	cfgMap := coreV1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: utils.ConfigMapResource.ApiVersion,
			Kind:       utils.ConfigMapResource.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.AI_FILTERS_CONFIGMAP_NAME,
			Namespace: config.Get("MO_OWN_NAMESPACE"),
		},
		Data: map[string]string{
			"filters":     string(filterYaml),
			"userFilters": string(userFiltersYaml),
		},
	}

	// get existing configmap and merge if exists
	existingCfgMapUnstructured, err := GetUnstructuredResource(utils.ConfigMapResource.ApiVersion, utils.ConfigMapResource.Plural, config.Get("MO_OWN_NAMESPACE"), utils.AI_FILTERS_CONFIGMAP_NAME)
	if apierrors.IsNotFound(err) {
		// create if not exists
		cfgMapYaml, err := yaml.Marshal(cfgMap)
		if err != nil {
			return newPromptCfg, err
		}
		_, err = CreateUnstructuredResource(utils.ConfigMapResource.ApiVersion, utils.ConfigMapResource.Plural, true, string(cfgMapYaml))
		if err != nil {
			return newPromptCfg, err
		}
	} else {
		if err == nil {
			// update if exists
			configmapData, ok := existingCfgMapUnstructured.Object["data"].(map[string]any)
			if ok {
				filtersData, filtersFound := configmapData["filters"].(string)
				if filtersFound {
					var existingFilters []ai.AiFilter
					if unmarshalErr := yaml.Unmarshal([]byte(filtersData), &existingFilters); unmarshalErr != nil {
						return newPromptCfg, unmarshalErr
					}

					// transfer IsActive state from existing filters to new filters
					for _, eF := range existingFilters {
						for nfInfex, nF := range newPromptCfg.Filters {
							if eF.Id == nF.Id {
								newPromptCfg.Filters[nfInfex].IsActive = eF.IsActive
							}
						}
					}
				}
				userFiltersData, userFiltersFound := configmapData["userFilters"].(string)
				if userFiltersFound {
					var existingFilters []ai.AiFilter
					if unmarshalErr := yaml.Unmarshal([]byte(userFiltersData), &existingFilters); unmarshalErr != nil {
						return newPromptCfg, unmarshalErr
					}

					// transfer IsActive state from existing filters to new filters
					for _, eF := range existingFilters {
						for nfInfex, nF := range newPromptCfg.UserFilters {
							if eF.Id == nF.Id {
								newPromptCfg.UserFilters[nfInfex].IsActive = eF.IsActive
							}
						}
					}
				}
			}

			cfgMapYaml, err := yaml.Marshal(newPromptCfg)
			if err != nil {
				return newPromptCfg, err
			}
			_, err = UpdateUnstructuredResource(utils.ConfigMapResource.ApiVersion, utils.ConfigMapResource.Plural, true, string(cfgMapYaml))
			if err != nil {
				return newPromptCfg, err
			}
		}
	}

	return newPromptCfg, err
}

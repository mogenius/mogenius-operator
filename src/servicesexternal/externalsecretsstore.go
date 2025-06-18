package servicesexternal

import (
	"fmt"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"

	json "github.com/json-iterator/go"

	"mogenius-k8s-manager/src/utils"

	"strings"

	"sigs.k8s.io/yaml"
)

type ExternalSecretStoreProps struct {
	DisplayName    string
	ProjectId      string
	Role           string
	VaultServerUrl string
	SecretPath     string
	NamePrefix     string
	Name           string
	ServiceAccount string
}

func CreateExternalSecretsStore(props ExternalSecretStoreProps) error {
	// init some dynamic properties
	if props.NamePrefix == "" {
		props.NamePrefix = utils.NanoIdSmallLowerCase()
	}
	if strings.Contains(props.SecretPath, "/v1/") {
		props.SecretPath = strings.ReplaceAll(props.SecretPath, "/v1/", "")
	}
	props.Name = utils.GetSecretStoreName(props.NamePrefix)
	props.ServiceAccount = utils.GetServiceAccountName(props.Role)

	// create unique service account tag per role
	annotations := make(map[string]string)
	key := fmt.Sprintf("%srole-%s", utils.StoreAnnotationPrefix, props.Role)
	annotations[key] = fmt.Sprintf("Used to read secrets from vault path: %s", props.SecretPath)

	err := mokubernetes.ApplyServiceAccount(props.ServiceAccount, config.Get("MO_OWN_NAMESPACE"), annotations)
	if err != nil {
		esoLogger.Info("ServiceAccount apply failed", "error", err)
		return err
	}

	// create the secret store which connects to the vault and is able to fetch secrets
	err = mokubernetes.ApplyResource(
		RenderClusterSecretStore(
			utils.InitExternalSecretsStoreYaml(),
			props,
		),
		true,
	)
	if err != nil {
		return err
	}
	// create the external secret which will fetch all available secrets from vault
	// so that we can use them to offer them as UI options before binding them to a mogenius service
	err = mokubernetes.CreateExternalSecretList(mokubernetes.ExternalSecretListProps{
		NamePrefix:      props.NamePrefix,
		SecretStoreName: props.Name,
		SecretPath:      props.SecretPath,
	})
	if err != nil {
		return err
	}
	return nil
}

func GetExternalSecretsStore(name string) (*mokubernetes.SecretStoreSchema, error) {
	response, err := mokubernetes.GetResource("external-secrets.io", "v1beta1", "clustersecretstores", name, "", true)
	if err != nil {
		esoLogger.Error("GetResource failed for SecretStore: ", "error", name)
		return nil, err
	}

	esoLogger.Info("SecretStore retrieved", "name", response.GetName())

	yamlOutput, err := yaml.Marshal(response.Object)
	if err != nil {
		return nil, err
	}
	secretStore := mokubernetes.SecretStoreSchema{}
	err = yaml.Unmarshal([]byte(yamlOutput), &secretStore)
	if err != nil {
		return nil, err
	}
	return &secretStore, err
}

func ListAvailableExternalSecrets(namePrefix string) []string {
	response, err := mokubernetes.GetDecodedSecret(
		utils.GetSecretListName(namePrefix),
		config.Get("MO_OWN_NAMESPACE"),
	)
	if err != nil {
		esoLogger.Error("Getting secret list failed", "error", err)
	}
	// Initialize result with an empty slice for SecretsInProject
	result := []string{}
	// for the current prefix, there's only one secret list
	for _, secretValue := range response {
		var secretMap map[string]interface{}
		err := json.Unmarshal([]byte(secretValue), &secretMap)
		if err != nil {
			esoLogger.Error("Error unmarshalling secret", "error", err.Error())
			return nil
		}

		for key := range secretMap {
			result = append(result, key)
		}

	}
	return result
}

func DeleteExternalSecretsStore(name string) error {
	errors := []error{}

	// get the secret store
	store, err := mokubernetes.GetExternalSecretsStore(name)
	errors = append(errors, err)
	// delete the external secrets list
	errors = append(errors, mokubernetes.DeleteExternalSecretList(store.Prefix, store.ProjectId))

	// delete the secret store
	errors = append(errors, mokubernetes.DeleteResource(
		"external-secrets.io",
		"v1beta1",
		"clustersecretstores",
		utils.GetSecretStoreName(store.Prefix),
		"",
		true))

	// delete the service account if it has no annotations from another SecretStore
	errors = append(errors, deleteUnusedServiceAccount(store.Role, store.ProjectId, store.SharedPath))

	// if any of the above failed, return an error
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteUnusedServiceAccount(role, projectId, moSharedPath string) error {
	_ = moSharedPath
	serviceAccount, err := mokubernetes.GetServiceAccount(utils.GetServiceAccountName(role), config.Get("MO_OWN_NAMESPACE"))
	if err != nil {
		return err
	}
	esoLogger.Info("ServiceAccount retrieved", "namespace", serviceAccount.GetNamespace(), "name", serviceAccount.GetName())

	if serviceAccount.Annotations != nil {
		// remove current claim of using this service account
		removeKey := ""
		for key := range serviceAccount.Annotations {
			myKey := fmt.Sprintf("%s%s", utils.StoreAnnotationPrefix, projectId)
			if key == myKey {
				removeKey = key
				break // "my" claim found, remove it
			}
		}
		if removeKey != "" {
			delete(serviceAccount.Annotations, removeKey)
		}

		// check if there are any other claims
		canBeDeleted := true
		for key := range serviceAccount.Annotations {
			if strings.HasPrefix(key, utils.StoreAnnotationPrefix) {
				canBeDeleted = false
				break // one claim found, don't delete the sa
			}
		}
		if canBeDeleted {
			// no annotations left that indicate other usage, delete the sa
			err = mokubernetes.DeleteServiceAccount(serviceAccount.Name, serviceAccount.Namespace)
			if err != nil {
				return err
			}
		} else {
			// there are still claims, don't delete the service account
			err = mokubernetes.UpdateServiceAccount(serviceAccount)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func RenderClusterSecretStore(yamlTemplateString string, props ExternalSecretStoreProps) string {
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<VAULT_STORE_NAME>", props.Name)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<SECRET_PATH>", props.SecretPath)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<DISPLAY_NAME>", props.DisplayName)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<PREFIX>", strings.ToLower(props.NamePrefix))
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<PROJECT_ID>", strings.ToLower(props.ProjectId))
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<VAULT_SERVER_URL>", props.VaultServerUrl)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<ROLE>", props.Role)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<SERVICE_ACC>", props.ServiceAccount)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<MO_DEFAULT_NS>", config.Get("MO_OWN_NAMESPACE"))

	return yamlTemplateString
}

package servicesExternal

import (
	"encoding/json"
	"fmt"
	mokubernetes "mogenius-k8s-manager/kubernetes"

	"mogenius-k8s-manager/utils"

	punqUtils "github.com/mogenius/punq/utils"

	"strings"

	"github.com/mogenius/punq/logger"
	"gopkg.in/yaml.v2"
)

type ExternalSecretStoreProps struct {
	ProjectId      string
	Role           string
	VaultServerUrl string
	SecretPath     string
	NamePrefix     string
	Name           string
	ServiceAccount string
}

func externalSecretStorePropsExample() ExternalSecretStoreProps {
	return ExternalSecretStoreProps{
		ProjectId:      "jkhdfjk66-lkj4fdklfj-lkdsjfkl-4rt645-dalksf",
		Role:           "mogenius-external-secrets",
		VaultServerUrl: "http://vault.default.svc.cluster.local:8200",
		SecretPath:     "mogenius-external-secrets/data/phoenix",
	}
}

func CreateExternalSecretsStore(props ExternalSecretStoreProps) error {
	// init some dynamic properties
	props.NamePrefix = punqUtils.NanoIdSmallLowerCase()
	props.Name = utils.GetSecretStoreName(props.NamePrefix)
	props.ServiceAccount = utils.GetServiceAccountName(props.SecretPath)

	// create unique service account tag per project
	annotations := make(map[string]string)
	key := fmt.Sprintf("%s%s", utils.StoreAnnotationPrefix, props.ProjectId)
	annotations[key] = fmt.Sprintf("Used to read secrets from vault path: %s", props.SecretPath)

	err := mokubernetes.ApplyServiceAccount(props.ServiceAccount, utils.CONFIG.Kubernetes.OwnNamespace, annotations)
	if err != nil {
		logger.Log.Info("ServiceAccount apply failed")
		return err
	}

	// create the secret store which connects to the vault and is able to fetch secrets
	err = mokubernetes.ApplyResource(
		renderClusterSecretStore(
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
		Project:         props.ProjectId,
		SecretStoreName: props.Name,
		MoSharedPath:    props.SecretPath,
	})
	if err != nil {
		return err
	}
	return nil
}

func GetExternalSecretsStore(name string) (*mokubernetes.SecretStoreSchema, error) {
	response, err := mokubernetes.GetResource("external-secrets.io", "v1beta1", "clustersecretstores", name, "", true)
	if err != nil {
		logger.Log.Info("GetResource failed for SecretStore: " + name)
		return nil, err
	}

	logger.Log.Info(fmt.Sprintf("SecretStore retrieved name: %s", response.GetName()))

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

func ListAvailableExternalSecrets(namePrefix, projectName string) []string {
	response, err := mokubernetes.GetDecodedSecret(
		utils.GetSecretListName(namePrefix, projectName),
		utils.CONFIG.Kubernetes.OwnNamespace,
	)
	if err != nil {
		logger.Log.Error("Getting secret list failed")
	}
	// Initialize result with an empty slice for SecretsInProject
	result := []string{}
	for project, secretValue := range response {
		if project == projectName {
			var secretMap map[string]interface{}
			err := json.Unmarshal([]byte(secretValue), &secretMap)
			if err != nil {
				logger.Log.Error(err)
				return nil
			}

			for key := range secretMap {
				result = append(result, key)
			}
			break // there should only be one matching project
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
	errors = append(errors, deleteUnusedServiceAccount(store.ProjectId, store.SharedPath))

	// if any of the above failed, return an error
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteUnusedServiceAccount(projectName, moSharedPath string) error {
	serviceAccount, err := mokubernetes.GetServiceAccount(utils.GetServiceAccountName(moSharedPath), utils.CONFIG.Kubernetes.OwnNamespace)
	if err != nil {
		return err
	}
	logger.Log.Info(fmt.Sprintf("ServiceAccount retrieved ns: %s - name: %s", serviceAccount.GetNamespace(), serviceAccount.GetName()))

	if serviceAccount.Annotations != nil {
		// remove current claim of using this service account
		removeKey := ""
		for key := range serviceAccount.Annotations {
			myKey := fmt.Sprintf("%s%s", utils.StoreAnnotationPrefix, projectName)
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

func renderClusterSecretStore(yamlTemplateString string, props ExternalSecretStoreProps) string {
	yamlTemplateString = strings.Replace(yamlTemplateString, "<VAULT_STORE_NAME>", props.Name, -1)
	// secret stores are currently bound to the project settings
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<SECRET_PATH>", props.SecretPath)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<PREFIX>", strings.ToLower(props.NamePrefix))
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<PROJECT_ID>", strings.ToLower(props.ProjectId))
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<VAULT_SERVER_URL>", props.VaultServerUrl)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<ROLE>", props.Role)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<SERVICE_ACC>", props.ServiceAccount)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<MO_DEFAULT_NS>", utils.CONFIG.Kubernetes.OwnNamespace)

	return yamlTemplateString
}

package servicesExternal

import (
	"encoding/json"
	"fmt"
	mokubernetes "mogenius-k8s-manager/kubernetes"

	"mogenius-k8s-manager/utils"

	"strings"

	"github.com/mogenius/punq/logger"
	"gopkg.in/yaml.v2"
)

type ExternalSecretStoreProps struct {
	ProjectName    string
	Role           string
	VaultServerUrl string
	MoSharedPath   string
	NamePrefix     string
	Name           string
	ServiceAccount string
}

func externalSecretStorePropsExample() ExternalSecretStoreProps {
	return ExternalSecretStoreProps{
		ProjectName:    "phoenix",
		Role:           "mogenius-external-secrets",
		VaultServerUrl: "http://vault.default.svc.cluster.local:8200",
		MoSharedPath:   "mogenius-external-secrets",
		Name:           utils.GetSecretStoreName("Integration01", "phoenix"),
		ServiceAccount: utils.GetServiceAccountName("mogenius-external-secrets"),
	}
}

func CreateExternalSecretsStore(props ExternalSecretStoreProps) error {
	props.Name = utils.GetSecretStoreName(props.NamePrefix, props.ProjectName)
	props.ServiceAccount = utils.GetServiceAccountName(props.MoSharedPath)

	// create unique service account tag per project
	annotations := make(map[string]string)
	key := fmt.Sprintf("%s%s", utils.StoreAnnotationPrefix, props.ProjectName)
	annotations[key] = fmt.Sprintf("Used to read secrets from vault path: %s", props.MoSharedPath)

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
		Project:         props.ProjectName,
		SecretStoreName: props.Name,
		MoSharedPath:    props.MoSharedPath,
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

func DeleteExternalSecretsStore(namePrefix, projectName, moSharedPath string) error {
	errors := []error{}
	// delete the external secrets list
	errors = append(errors, mokubernetes.DeleteExternalSecretList(namePrefix, projectName))

	// delete the secret store
	errors = append(errors, mokubernetes.DeleteResource("external-secrets.io", "v1beta1", "clustersecretstores", utils.GetSecretStoreName(namePrefix, projectName), "", true))

	// delete the service account if it has no annotations from another SecretStore
	errors = append(errors, deleteUnusedServiceAccount(projectName, moSharedPath))

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
	yamlTemplateString = strings.Replace(yamlTemplateString, "<MO_SHARED_PATH_COMBINED>", utils.GetMoSharedPath(props.MoSharedPath, props.ProjectName), -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<VAULT_SERVER_URL>", props.VaultServerUrl, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<ROLE>", props.Role, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<SERVICE_ACC>", props.ServiceAccount, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<MO_DEFAULT_NS>", utils.CONFIG.Kubernetes.OwnNamespace, -1)

	return yamlTemplateString
}

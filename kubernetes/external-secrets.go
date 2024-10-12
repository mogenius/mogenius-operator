package kubernetes

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	"strings"

	jsoniter "github.com/json-iterator/go"

	punq "github.com/mogenius/punq/kubernetes"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type CreateExternalSecretProps struct {
	Namespace    string `json:"namespace" validate:"required"`
	PropertyName string `json:"propertyName" validate:"required"`
	NamePrefix   string `json:"namePrefix" validate:"required"`
	ServiceName  string `json:"serviceName" validate:"required"`
}

func CreateExternalSecretPropsExample() CreateExternalSecretProps {
	return CreateExternalSecretProps{
		Namespace:    "mogenius",
		PropertyName: "postgresURL",
		NamePrefix:   "3241lkjltg243",
		ServiceName:  "fe-mo-service",
	}
}

type ExternalSecretProps struct {
	CreateExternalSecretProps
	SecretName      string
	SecretStoreName string
	secretPath      string
}

// NewExternalSecret creates a new NewExternalSecret with default values.
func NewExternalSecret(data CreateExternalSecretProps) ExternalSecretProps {
	return ExternalSecretProps{
		CreateExternalSecretProps: data,
	}
}

type ExternalSecretListProps struct {
	NamePrefix      string
	SecretName      string
	SecretStoreName string
	SecretPath      string
}

func externalSecretListExample() ExternalSecretListProps {
	return ExternalSecretListProps{
		NamePrefix:      "fsd87fdh",
		SecretName:      "database-credentials",
		SecretStoreName: "fsd87fdh" + utils.SecretStoreSuffix,
		SecretPath:      "mogenius-external-secrets/data/team-blue",
	}
}

func CreateExternalSecretList(props ExternalSecretListProps) error {
	return ApplyResource(
		renderExternalSecretList(
			utils.InitExternalSecretListYaml(),
			props,
		),
		false,
	)
}

func CreateExternalSecret(data CreateExternalSecretProps) (string, error) {
	responsibleSecStore := utils.GetSecretStoreName(data.NamePrefix)

	secretPath, err := ReadSecretPathFromSecretStore(responsibleSecStore)
	if err != nil {
		return "", err
	}

	props := ExternalSecretProps{
		CreateExternalSecretProps: data,
		SecretName: utils.GetSecretName(
			data.NamePrefix, data.ServiceName, data.PropertyName,
		),
		SecretStoreName: responsibleSecStore,
		secretPath:      secretPath,
	}

	err = ApplyResource(
		renderExternalSecret(
			utils.InitExternalSecretYaml(),
			props,
		),
		false,
	)
	if err != nil {
		return "", err
	}
	return props.SecretName, nil
}

func GetSecretValueByPrefixControllerNameAndKey(namespaceName string, controllerName string, prefix string, key string) (string, error) {
	secretName := utils.GetSecretName(prefix, controllerName, key)

	secretClient := GetCoreClient().Secrets(namespaceName)

	data, err := secretClient.Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	base64Data := data.Data[key]
	if base64Data == nil {
		return "", fmt.Errorf("key %s not found in secret %s", key, secretName)
	}
	return string(base64Data), nil
}

func DeleteExternalSecretList(namePrefix string, projectName string) error {
	return DeleteExternalSecret(utils.GetSecretListName(namePrefix))
}

func DeleteUnusedSecretsForNamespace(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Delete Unused Secrets", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting unused secrets")

		deployments := punq.AllDeployments(namespace.Name, nil)

		mountedSecretNames := []string{}
		for _, deployment := range deployments {
			for _, volume := range deployment.Spec.Template.Spec.Volumes {
				if volume.Secret != nil {
					mountedSecretNames = append(mountedSecretNames, volume.Secret.SecretName)
				}
			}
		}

		// LIST ns secrets
		secrets, err := ListResources("external-secrets.io", "v1beta1", "externalsecrets", "", true)
		if err != nil {
			K8sLogger.Errorf("Error listing resources: %s", err.Error())
		}
		if secrets == nil {
			cmd.Success(job, "Deleted unused secrets")
			return
		}
		existingSecrets, err := parseExternalSecretsListing(secrets)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteUnusedSecretsForNamespace ERROR: %s", err.Error()))
			return
		}
		for _, secret := range existingSecrets {
			isMoExternalSecret := false
			for key := range secret.Labels {
				if key == "used-by-mo-service" {
					isMoExternalSecret = true
					break
				}
			}
			isUsedByDeployment := false
			for _, mountedSecretName := range mountedSecretNames {
				if mountedSecretName == secret.Name {
					isUsedByDeployment = true
					break
				}
			}

			if isMoExternalSecret && !isUsedByDeployment {
				err = DeleteExternalSecret(secret.Name)
				if err != nil {
					K8sLogger.Errorf("Error deleting unsed secret %s: %s", secret.Name, err.Error())
					break
				}
			}
		}
		cmd.Success(job, "Deleted unused secrets")
	}(wg)
}

func DeleteExternalSecret(name string) error {
	return DeleteResource(
		"external-secrets.io",
		"v1beta1",
		"externalsecrets",
		name,
		utils.CONFIG.Kubernetes.OwnNamespace,
		false,
	)
}

func renderExternalSecretList(yamlTemplateString string, props ExternalSecretListProps) string {
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<NAME>", utils.GetSecretListName(props.NamePrefix))
	// the list of all available secrets for a project is only ever read by the operator
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<NAMESPACE>", utils.CONFIG.Kubernetes.OwnNamespace)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<SECRET_STORE_NAME>", props.SecretStoreName)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<SECRET_PATH>", props.SecretPath)

	pathParts := strings.Split(props.SecretPath, "/")
	secretName := pathParts[len(pathParts)-1]
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<SECRET_NAME>", secretName)

	secretFolder := strings.Join(pathParts[:len(pathParts)-1], "/")
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<SECRET_FOLDER>", secretFolder)

	// log.Println(yamlTemplateString)
	return yamlTemplateString
}

func renderExternalSecret(yamlTemplateString string, props ExternalSecretProps) string {
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<NAME>", props.SecretName)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<SERVICE_NAME>", props.ServiceName)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<PROPERTY_FROM_SECRET>", props.PropertyName)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<NAMESPACE>", props.Namespace)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<SECRET_STORE_NAME>", props.SecretStoreName)
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<SECRET_PATH>", props.secretPath)

	return yamlTemplateString
}

type ExternalSecretListing struct {
	Name    string            `json:"name"`
	Role    string            `json:"role"`
	Message string            `json:"message"`
	Labels  map[string]string `json:"labels"`
}

type ExternalSecretListingSchema struct {
	Items []struct {
		Metadata struct {
			Name   string            `json:"name"`
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
		Spec struct {
			Provider struct {
				Vault struct {
					Auth struct {
						Kubernetes struct {
							Role string `json:"role"`
						} `json:"kubernetes"`
					} `json:"auth"`
				} `json:"vault"`
			} `json:"provider"`
		} `json:"spec"`
		Status struct {
			Conditions []struct {
				Message string `json:"message"`
			} `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
}

func parseExternalSecretsListing(list *unstructured.UnstructuredList) ([]ExternalSecretListing, error) {
	var stores []ExternalSecretListing

	for _, item := range list.Items {
		// Convert item to []byte
		itemBytes, err := item.MarshalJSON()
		if err != nil {
			K8sLogger.Error("Error converting item to []byte:", err)
			return nil, err
		}
		var ExternalSecrets ExternalSecretListingSchema
		err = jsoniter.Unmarshal(itemBytes, &ExternalSecrets)
		if err != nil {
			return nil, err
		}

		for _, item := range ExternalSecrets.Items {
			message := ""
			if len(item.Status.Conditions) > 0 {
				message = item.Status.Conditions[0].Message
			}

			store := ExternalSecretListing{
				Name:    item.Metadata.Name,
				Role:    item.Spec.Provider.Vault.Auth.Kubernetes.Role,
				Message: message,
				Labels:  item.Metadata.Labels,
			}
			stores = append(stores, store)
		}
	}

	return stores, nil
}

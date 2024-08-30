package kubernetes

import (
	"mogenius-k8s-manager/utils"

	"strings"

	jsoniter "github.com/json-iterator/go"

	punq "github.com/mogenius/punq/kubernetes"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type CreateExternalSecretProps struct {
	ServiceName           string `json:"serviceName" validate:"required"`
	Namespace             string `json:"namespace" validate:"required"`
	ProjectName           string `json:"projectName" validate:"required"`
	SecretStoreNamePrefix string `json:"namePrefix" validate:"required"`
	PropertyName          string `json:"propertyName" validate:"required"`
}

func CreateExternalSecretPropsExample() CreateExternalSecretProps {
	return CreateExternalSecretProps{
		ServiceName:           "customer-app01",
		Namespace:             "customer-app-namespace",
		ProjectName:           "phoenix",
		SecretStoreNamePrefix: "mo-test",
		PropertyName:          "postgresURL",
	}
}

type ExternalSecretProps struct {
	CreateExternalSecretProps
	SecretStoreName string
	secretPath      string
}

func externalExternalSecretExample() ExternalSecretProps {
	return NewExternalSecret(CreateExternalSecretPropsExample())
}

// NewExternalSecret creates a new NewExternalSecret with default values.
func NewExternalSecret(data CreateExternalSecretProps) ExternalSecretProps {
	return ExternalSecretProps{
		CreateExternalSecretProps: data,
	}
}

type ExternalSecretListProps struct {
	NamePrefix      string
	Project         string
	SecretStoreName string
	MoSharedPath    string
}

func externalSecretListExample() ExternalSecretListProps {
	return ExternalSecretListProps{
		NamePrefix:      "team-blue-secrets",
		Project:         "team-blue",
		SecretStoreName: "team-blue-secrets-" + utils.SecretStoreSuffix,
		MoSharedPath:    "mogenius-external-secrets",
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

func CreateExternalSecret(data CreateExternalSecretProps) error {
	responsibleSecStore := utils.GetSecretStoreName(data.SecretStoreNamePrefix)

	secretPath, err := ReadSecretPathFromSecretStore(responsibleSecStore)
	if err != nil {
		return err
	}

	props := ExternalSecretProps{
		CreateExternalSecretProps: data,
		SecretStoreName:           responsibleSecStore,
		secretPath:                secretPath,
	}

	err = ApplyResource(
		renderExternalSecret(
			utils.InitExternalSecretYaml(),
			props,
		),
		false,
	)
	if err != nil {
		return err
	}
	return nil
}

func DeleteExternalSecretList(namePrefix string, projectName string) error {
	return DeleteExternalSecret(utils.GetSecretListName(namePrefix))
}

func DeleteUnusedSecretsForNamespace(namespace string) error {
	// DEPLOYMENTs
	deployments := punq.AllDeployments(namespace, nil)

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

	existingSecrets, err := parseExternalSecretsListing(secrets)
	if err != nil {
		return err
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
				return err
			}
		}
	}
	return nil
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
	yamlTemplateString = strings.ReplaceAll(yamlTemplateString, "<MO_SHARED_PATH>", props.MoSharedPath)

	return yamlTemplateString
}

func renderExternalSecret(yamlTemplateString string, props ExternalSecretProps) string {
	yamlTemplateString = strings.Replace(yamlTemplateString, "<NAME>", utils.GetSecretName(
		props.SecretStoreNamePrefix, props.ServiceName, props.PropertyName,
	), -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<SERVICE_NAME>", props.ServiceName, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<PROPERTY_FROM_SECRET>", props.PropertyName, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<NAMESPACE>", props.Namespace, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<SECRET_STORE_NAME>", props.SecretStoreName, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<SECRET_PATH>", props.secretPath, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<PROJECT>", props.ProjectName, -1)

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

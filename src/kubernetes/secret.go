package kubernetes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/websocket"
	"sync"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ClusterImagePullSecretName = "cluster-img-pull-sec"
const ContainerImagePullSecretName = "container-img-pull-sec"

func AllSecrets(namespaceName string) []v1.Secret {
	result := []v1.Secret{}

	clientset := clientProvider.K8sClientSet()
	secretList, err := clientset.CoreV1().Secrets(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		k8sLogger.Error("AllSecrets", "error", err.Error())
		return result
	}

	for _, secret := range secretList.Items {
		secret.Kind = "Secret"
		secret.APIVersion = "v1"
		result = append(result, secret)

	}
	return result
}

func CreateSecret(namespace string, secret *v1.Secret) (*v1.Secret, error) {
	if secret == nil {
		var err error
		secret, err = exampleSecret(namespace)
		if err != nil {
			return nil, err
		}
	}

	clientset := clientProvider.K8sClientSet()
	return clientset.CoreV1().Secrets(namespace).Create(context.TODO(), secret, MoCreateOptions())
}

func exampleSecret(namespace string) (*v1.Secret, error) {
	jsonData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	jsonString, err := json.Marshal(jsonData)
	if err != nil {
		return nil, err
	}
	// encodedJson := base64.StdEncoding.EncodeToString(jsonString)
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "customer-blue-backend-project-vault-secret-list",
			Namespace: namespace,
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{
			"project001":      []byte(jsonString),
			"backend-project": []byte(jsonString),
		},
	}, nil

}

func GetDecodedSecret(secretName string, namespace string) (map[string]string, error) {
	clientset := clientProvider.K8sClientSet()
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s in namespace %s: %w", secretName, namespace, err)
	}

	decodedData := make(map[string]string)
	for key, value := range secret.Data {
		decodedData[key] = string(value)
	}

	return decodedData, nil
}

func DeleteK8sSecretBy(namespace string, name string) error {
	clientset := clientProvider.K8sClientSet()
	secretClient := clientset.CoreV1().Secrets(namespace)
	return secretClient.Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// -----------------------------------------------------
// Cluster Image Pull Secret
// -----------------------------------------------------

func CreateOrUpdateClusterImagePullSecret(eventClient websocket.WebsocketClient, job *structs.Job, project dtos.K8sProjectDto, namespace dtos.K8sNamespaceDto, wg *sync.WaitGroup) {
	secretName := utils.ParseK8sName(fmt.Sprintf("%s-%s", ClusterImagePullSecretName, namespace.Name))

	// DO NOT CREATE SECRET IF NO IMAGE REPO SECRET IS PROVIDED
	if project.ContainerRegistryUser == nil || project.ContainerRegistryPat == nil || project.ContainerRegistryUrl == nil {
		// delete if exists
		err := DeleteK8sSecretBy(namespace.Name, secretName)
		if err != nil {
			k8sLogger.Error("Error deleting secret", "namespace", namespace.Name, "secret", secretName, "error", err)
		}
		return
	}

	cmd := structs.CreateCommand(eventClient, "create", "Create Cluster ImagePullSecret", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(eventClient, job, "Creating Cluster ImagePullSecret")

		clientset := clientProvider.K8sClientSet()
		secretClient := clientset.CoreV1().Secrets(namespace.Name)

		secret := utils.InitContainerSecret()
		secret.ObjectMeta.Name = secretName
		secret.ObjectMeta.Namespace = namespace.Name
		secret.Labels = MoUpdateLabels(&secret.Labels, nil, nil, nil)

		secretStringData := make(map[string]string)

		authStr := fmt.Sprintf("%s:%s", *project.ContainerRegistryUser, *project.ContainerRegistryPat)
		authStrBase64 := base64.StdEncoding.EncodeToString([]byte(authStr))
		jsonData := fmt.Sprintf(`{"auths":{"%s":{"username":"%s","password":"%s","auth":"%s"}}}`, *project.ContainerRegistryUrl, *project.ContainerRegistryUser, *project.ContainerRegistryPat, authStrBase64)
		secretStringData[".dockerconfigjson"] = jsonData // base64.StdEncoding.EncodeToString([]byte(jsonData))

		secret.StringData = secretStringData

		// Check if exists
		_, err := secretClient.Update(context.TODO(), &secret, MoUpdateOptions())
		if err == nil {
			// UPDATED
			cmd.Success(eventClient, job, "Created Cluster ImagePullSecret")
		} else {
			if apierrors.IsNotFound(err) {
				_, err = secretClient.Create(context.TODO(), &secret, MoCreateOptions())
				if err != nil {
					cmd.Fail(eventClient, job, fmt.Sprintf("CreateOrUpdateClusterImagePullSecret (create) ERROR: %s", err.Error()))
				} else {
					// CREATED
					cmd.Success(eventClient, job, "Created Cluster ImagePullSecret")
				}
			} else {
				cmd.Fail(eventClient, job, fmt.Sprintf("CreateOrUpdateClusterImagePullSecret ERROR: %s", err.Error()))
			}
		}
	}(wg)
}

func ExistsClusterImagePullSecret(namespace string) bool {
	secretName := utils.ParseK8sName(fmt.Sprintf("%s-%s", ClusterImagePullSecretName, namespace))
	clientset := clientProvider.K8sClientSet()
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return secret != nil
}

// -----------------------------------------------------
// Container Image Pull Secret
// -----------------------------------------------------

func CreateOrUpdateContainerImagePullSecret(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	secretName := utils.ParseK8sName(fmt.Sprintf("%s-%s", ContainerImagePullSecretName, service.ControllerName))

	// DO NOT CREATE SECRET IF NO IMAGE REPO SECRET IS PROVIDED
	authStr := service.GetImageRepoSecretDecryptValue()
	if authStr == nil {
		// delete if exists
		err := DeleteK8sSecretBy(namespace.Name, secretName)
		if err != nil {
			k8sLogger.Error("Error deleting secret", "namespace", namespace.Name, "secret", secretName, "error", err)
		}
		return
	}

	cmd := structs.CreateCommand(eventClient, "create", "Create Container ImagePullSecret", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(eventClient, job, "Creating Container ImagePullSecret")

		if authStr != nil {
			err := ValidateContainerRegistryAuthString(*authStr)
			if err != nil {
				cmd.Fail(eventClient, job, fmt.Sprintf("The provided ImagePullSecret does not match the required format: %s", err.Error()))
				return
			}
		}

		clientset := clientProvider.K8sClientSet()
		secretClient := clientset.CoreV1().Secrets(namespace.Name)

		secret := utils.InitContainerSecret()
		secret.ObjectMeta.Name = secretName
		secret.ObjectMeta.Namespace = namespace.Name

		secretStringData := make(map[string]string)
		secretStringData[".dockerconfigjson"] = *service.GetImageRepoSecretDecryptValue()
		secret.StringData = secretStringData

		secret.Labels = MoUpdateLabels(&secret.Labels, nil, nil, nil)

		// Check if exists
		_, err := secretClient.Update(context.TODO(), &secret, MoUpdateOptions())
		if err == nil {
			// UPDATED
			cmd.Success(eventClient, job, "Created Container ImagePullSecret")
		} else {
			if apierrors.IsNotFound(err) {
				_, err = secretClient.Create(context.TODO(), &secret, MoCreateOptions())
				if err != nil {
					cmd.Fail(eventClient, job, fmt.Sprintf("CreateOrUpdateContainerImagePullSecret (create) ERROR: %s", err.Error()))
				} else {
					// CREATED
					cmd.Success(eventClient, job, "Created Container ImagePullSecret")
				}
			} else {
				cmd.Fail(eventClient, job, fmt.Sprintf("CreateOrUpdateContainerImagePullSecret ERROR: %s", err.Error()))
			}
		}
	}(wg)
}

func DeleteContainerImagePullSecret(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	secretName := utils.ParseK8sName(fmt.Sprintf("%s-%s", ContainerImagePullSecretName, service.ControllerName))

	cmd := structs.CreateCommand(eventClient, "delete", "Delete Container secret", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(eventClient, job, "Deleting Container secret")

		clientset := clientProvider.K8sClientSet()
		secretClient := clientset.CoreV1().Secrets(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: utils.Pointer[int64](5),
		}

		_, err := secretClient.Get(context.TODO(), secretName, metav1.GetOptions{})

		// ignore if not found
		if apierrors.IsNotFound(err) {
			cmd.Success(eventClient, job, "Deleted Container secret")
			return
		} else if err != nil {
			cmd.Fail(eventClient, job, fmt.Sprintf("DeleteContainerSecret ERROR: %s", err.Error()))
			return
		}

		err = secretClient.Delete(context.TODO(), secretName, deleteOptions)
		if err != nil {
			cmd.Fail(eventClient, job, fmt.Sprintf("DeleteContainerSecret ERROR: %s", err.Error()))
		} else {
			cmd.Success(eventClient, job, "Deleted Container secret")
		}
	}(wg)
}

// -----------------------------------------------------
// Service Secret
// -----------------------------------------------------

func UpdateOrCreateControllerSecret(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand(eventClient, "update", "Update Kubernetes secret", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(eventClient, job, "Updating secret")

		clientset := clientProvider.K8sClientSet()
		secretClient := clientset.CoreV1().Secrets(namespace.Name)
		secret := utils.InitSecret()
		secret.ObjectMeta.Name = service.ControllerName
		secret.ObjectMeta.Namespace = namespace.Name
		delete(secret.StringData, "exampleData") // delete example data

		for _, container := range service.Containers {
			for _, envVar := range container.EnvVars {
				if envVar.Type == dtos.EnvVarKeyVault && envVar.Data.VaultType == dtos.EnvVarVaultTypeMogeniusVault {
					secret.StringData[envVar.Name] = envVar.Value
					secret.Labels = make(map[string]string)
					secret.Labels[envVar.Name] = envVar.Data.Value
				}
				if envVar.Type == dtos.EnvVarPlainText ||
					envVar.Type == dtos.EnvVarHostname ||
					envVar.Type == dtos.EnvVarVolumeMount ||
					(envVar.Type == dtos.EnvVarKeyVault && envVar.Data.VaultType == dtos.EnvVarVaultTypeHashicorpExternalVault) {
					delete(secret.StringData, envVar.Name)
				}
			}
		}

		// delete secret if empty
		if len(secret.StringData) == 0 {
			_, err := secretClient.Get(context.TODO(), service.ControllerName, metav1.GetOptions{})

			// ignore if not found
			if apierrors.IsNotFound(err) {
				cmd.Success(eventClient, job, "Deleted unneeded secret")
				return
			} else if err != nil {
				cmd.Fail(eventClient, job, fmt.Sprintf("Deleted unneeded secret ERROR: %s", err.Error()))
				return
			}

			err = secretClient.Delete(context.TODO(), service.ControllerName, metav1.DeleteOptions{})
			if err != nil {
				cmd.Fail(eventClient, job, fmt.Sprintf("Deleted unneeded secret ERROR: %s", err.Error()))
			} else {
				cmd.Success(eventClient, job, "Deleted unneeded secret")
			}
			return
		}

		_, err := secretClient.Update(context.TODO(), &secret, MoUpdateOptions())
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = secretClient.Create(context.TODO(), &secret, MoCreateOptions())
				if err != nil {
					cmd.Fail(eventClient, job, fmt.Sprintf("UpdateOrCreateControllerSecrete ERROR: %s", err.Error()))
				} else {
					cmd.Success(eventClient, job, "Created secret")
				}
			} else {
				cmd.Fail(eventClient, job, fmt.Sprintf("UpdateOrCreateControllerSecrete ERROR: %s", err.Error()))
			}
		} else {
			cmd.Success(eventClient, job, "Update secret")
		}
	}(wg)
}

func DeleteControllerSecret(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	secretName := service.ControllerName

	cmd := structs.CreateCommand(eventClient, "delete", "Delete Controller secret", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(eventClient, job, "Deleting Controller secret")

		clientset := clientProvider.K8sClientSet()
		secretClient := clientset.CoreV1().Secrets(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: utils.Pointer[int64](5),
		}

		_, err := secretClient.Get(context.TODO(), secretName, metav1.GetOptions{})

		// ignore if not found
		if apierrors.IsNotFound(err) {
			cmd.Success(eventClient, job, "Deleted controller secret")
			return
		} else if err != nil {
			cmd.Fail(eventClient, job, fmt.Sprintf("DeleteControllerSecret ERROR: %s", err.Error()))
			return
		}

		err = secretClient.Delete(context.TODO(), secretName, deleteOptions)
		if err != nil {
			cmd.Fail(eventClient, job, fmt.Sprintf("DeleteControllerSecret ERROR: %s", err.Error()))
		} else {
			cmd.Success(eventClient, job, "Deleted Controller secret")
		}
	}(wg)
}

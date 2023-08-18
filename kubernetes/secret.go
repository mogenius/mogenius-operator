package kubernetes

import (
	"context"
	"encoding/base64"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	punq "github.com/mogenius/punq/kubernetes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateSecret(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create Kubernetes secret", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating secret '%s'.", namespace.Name))

		kubeProvider := punq.NewKubeProvider()
		secretClient := kubeProvider.ClientSet.CoreV1().Secrets(namespace.Name)
		secret := utils.InitSecret()
		secret.ObjectMeta.Name = service.Name
		secret.ObjectMeta.Namespace = namespace.Name
		delete(secret.StringData, "PRIVATE_KEY") // delete example data

		for _, envVar := range service.EnvVars {
			if envVar.Type == "KEY_VAULT" ||
				envVar.Type == "PLAINTEXT" ||
				envVar.Type == "HOSTNAME" {
				secret.StringData[envVar.Name] = envVar.Value
			}
		}

		secret.Labels = MoUpdateLabels(&secret.Labels, job.ProjectId, &namespace, &service)

		_, err := secretClient.Create(context.TODO(), &secret, MoCreateOptions())
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateSecret ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created secret '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func DeleteSecret(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete Kubernetes secret", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting secret '%s'.", namespace.Name))

		kubeProvider := punq.NewKubeProvider()
		secretClient := kubeProvider.ClientSet.CoreV1().Secrets(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: utils.Pointer[int64](5),
		}

		err := secretClient.Delete(context.TODO(), service.Name, deleteOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteSecret ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted secret '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func CreateOrUpdateContainerSecret(job *structs.Job, project dtos.K8sProjectDto, namespace dtos.K8sNamespaceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create Container secret", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating Container secret '%s'.", namespace.Name))

		secretName := "container-secret-" + namespace.Name

		kubeProvider := punq.NewKubeProvider()
		secretClient := kubeProvider.ClientSet.CoreV1().Secrets(namespace.Name)

		secret := utils.InitContainerSecret()
		secret.ObjectMeta.Name = secretName
		secret.ObjectMeta.Namespace = namespace.Name

		authStr := fmt.Sprintf("%s:%s", project.ContainerRegistryUser, project.ContainerRegistryPat)
		authStrBase64 := base64.StdEncoding.EncodeToString([]byte(authStr))
		jsonData := fmt.Sprintf(`{"auths":{"%s":{"username":"%s","password":"%s","auth":"%s"}}}`, project.ContainerRegistryUrl, project.ContainerRegistryUser, project.ContainerRegistryPat, authStrBase64)

		secretStringData := make(map[string]string)
		secretStringData[".dockerconfigjson"] = jsonData // base64.StdEncoding.EncodeToString([]byte(jsonData))
		secret.StringData = secretStringData

		secret.Labels = MoUpdateLabels(&secret.Labels, job.ProjectId, &namespace, nil)

		// Check if exists
		_, err := secretClient.Update(context.TODO(), &secret, MoUpdateOptions())
		if err == nil {
			// UPDATED
			cmd.Success(fmt.Sprintf("Created Container secret '%s'.", namespace.Name))
		} else {
			if apierrors.IsNotFound(err) {
				_, err = secretClient.Create(context.TODO(), &secret, MoCreateOptions())
				if err != nil {
					cmd.Fail(fmt.Sprintf("CreateContainerSecret (create) ERROR: %s", err.Error()))
				} else {
					// CREATED
					cmd.Success(fmt.Sprintf("Created Container secret '%s'.", namespace.Name))
				}
			} else {
				cmd.Fail(fmt.Sprintf("CreateContainerSecret ERROR: %s", err.Error()))
			}
		}
	}(cmd, wg)
	return cmd
}

func DeleteContainerSecret(job *structs.Job, namespace dtos.K8sNamespaceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete Container secret", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting Container secret '%s'.", namespace.Name))

		kubeProvider := punq.NewKubeProvider()
		secretClient := kubeProvider.ClientSet.CoreV1().Secrets(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: utils.Pointer[int64](5),
		}

		err := secretClient.Delete(context.TODO(), "container-secret-"+namespace.Name, deleteOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteContainerSecret ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted Container secret '%s'.", namespace.Name))
		}
	}(cmd, wg)
	return cmd
}

func UpdateSecrete(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Update Kubernetes secret", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Updating secret '%s'.", namespace.Name))

		kubeProvider := punq.NewKubeProvider()
		secretClient := kubeProvider.ClientSet.CoreV1().Secrets(namespace.Name)
		secret := utils.InitSecret()
		secret.ObjectMeta.Name = service.Name
		secret.ObjectMeta.Namespace = namespace.Name
		delete(secret.StringData, "PRIVATE_KEY") // delete example data

		for _, envVar := range service.EnvVars {
			if envVar.Type == "KEY_VAULT" ||
				envVar.Type == "PLAINTEXT" ||
				envVar.Type == "HOSTNAME" {
				secret.StringData[envVar.Name] = envVar.Value
			}
		}

		updateOptions := metav1.UpdateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		_, err := secretClient.Update(context.TODO(), &secret, updateOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("UpdateSecret ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Update secret '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func ContainerSecretDoesExistForStage(namespace dtos.K8sNamespaceDto) bool {
	provider := punq.NewKubeProvider()
	secret, err := provider.ClientSet.CoreV1().Secrets(namespace.Name).Get(context.TODO(), "container-secret-"+namespace.Name, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return secret != nil
}

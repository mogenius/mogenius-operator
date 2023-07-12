package kubernetes

import (
	"context"
	"encoding/base64"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"os/exec"
	"sync"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateSecret(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create Kubernetes secret", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating secret '%s'.", stage.Name))

		kubeProvider := NewKubeProvider()
		secretClient := kubeProvider.ClientSet.CoreV1().Secrets(stage.Name)
		secret := utils.InitSecret()
		secret.ObjectMeta.Name = service.Name
		secret.ObjectMeta.Namespace = stage.Name
		delete(secret.StringData, "PRIVATE_KEY") // delete example data

		for _, envVar := range service.EnvVars {
			if envVar.Type == "KEY_VAULT" ||
				envVar.Type == "PLAINTEXT" ||
				envVar.Type == "HOSTNAME" {
				secret.StringData[envVar.Name] = envVar.Value
			}
		}

		secret.Labels = MoUpdateLabels(&secret.Labels, &job.NamespaceId, &stage, &service)

		_, err := secretClient.Create(context.TODO(), &secret, MoCreateOptions())
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateSecret ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created secret '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func DeleteSecret(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete Kubernetes secret", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting secret '%s'.", stage.Name))

		kubeProvider := NewKubeProvider()
		secretClient := kubeProvider.ClientSet.CoreV1().Secrets(stage.Name)

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

func CreateOrUpdateContainerSecret(job *structs.Job, namespace dtos.K8sNamespaceDto, stage dtos.K8sStageDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create Container secret", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating Container secret '%s'.", stage.Name))

		secretName := "container-secret-" + stage.Name

		kubeProvider := NewKubeProvider()
		secretClient := kubeProvider.ClientSet.CoreV1().Secrets(stage.Name)

		secret := utils.InitContainerSecret()
		secret.ObjectMeta.Name = secretName
		secret.ObjectMeta.Namespace = stage.Name

		authStr := fmt.Sprintf("%s:%s", namespace.ContainerRegistryUser, namespace.ContainerRegistryPat)
		authStrBase64 := base64.StdEncoding.EncodeToString([]byte(authStr))
		jsonData := fmt.Sprintf(`{"auths":{"%s":{"username":"%s","password":"%s","auth":"%s"}}}`, namespace.ContainerRegistryUrl, namespace.ContainerRegistryUser, namespace.ContainerRegistryPat, authStrBase64)

		secretStringData := make(map[string]string)
		secretStringData[".dockerconfigjson"] = jsonData // base64.StdEncoding.EncodeToString([]byte(jsonData))
		secret.StringData = secretStringData

		secret.Labels = MoUpdateLabels(&secret.Labels, &job.NamespaceId, &stage, nil)

		// Check if exists
		_, err := secretClient.Update(context.TODO(), &secret, MoUpdateOptions())
		if err == nil {
			// UPDATED
			cmd.Success(fmt.Sprintf("Created Container secret '%s'.", stage.Name))
		} else {
			if apierrors.IsNotFound(err) {
				_, err = secretClient.Create(context.TODO(), &secret, MoCreateOptions())
				if err != nil {
					cmd.Fail(fmt.Sprintf("CreateContainerSecret (create) ERROR: %s", err.Error()))
				} else {
					// CREATED
					cmd.Success(fmt.Sprintf("Created Container secret '%s'.", stage.Name))
				}
			} else {
				cmd.Fail(fmt.Sprintf("CreateContainerSecret ERROR: %s", err.Error()))
			}
		}
	}(cmd, wg)
	return cmd
}

func DeleteContainerSecret(job *structs.Job, stage dtos.K8sStageDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete Container secret", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting Container secret '%s'.", stage.Name))

		kubeProvider := NewKubeProvider()
		secretClient := kubeProvider.ClientSet.CoreV1().Secrets(stage.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: utils.Pointer[int64](5),
		}

		err := secretClient.Delete(context.TODO(), "container-secret-"+stage.Name, deleteOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteContainerSecret ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted Container secret '%s'.", stage.Name))
		}
	}(cmd, wg)
	return cmd
}

func UpdateSecrete(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Update Kubernetes secret", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Updating secret '%s'.", stage.Name))

		kubeProvider := NewKubeProvider()
		secretClient := kubeProvider.ClientSet.CoreV1().Secrets(stage.Name)
		secret := utils.InitSecret()
		secret.ObjectMeta.Name = service.Name
		secret.ObjectMeta.Namespace = stage.Name
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

func AllSecrets(namespaceName string) []v1.Secret {
	result := []v1.Secret{}

	provider := NewKubeProvider()
	secretList, err := provider.ClientSet.CoreV1().Secrets(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllSecrets ERROR: %s", err.Error())
		return result
	}

	for _, secret := range secretList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, secret.ObjectMeta.Namespace) {
			result = append(result, secret)
		}
	}
	return result
}

func AllK8sSecrets(namespaceName string) K8sWorkloadResult {
	result := []v1.Secret{}

	provider := NewKubeProvider()
	secretList, err := provider.ClientSet.CoreV1().Secrets(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllSecrets ERROR: %s", err.Error())
		return WorkloadResult(nil, err)
	}

	for _, secret := range secretList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, secret.ObjectMeta.Namespace) {
			result = append(result, secret)
		}
	}
	return WorkloadResult(result, nil)
}

func UpdateK8sSecret(data v1.Secret) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	secretClient := kubeProvider.ClientSet.CoreV1().Secrets(data.Namespace)
	_, err := secretClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DeleteK8sSecret(data v1.Secret) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	secretClient := kubeProvider.ClientSet.CoreV1().Secrets(data.Namespace)
	err := secretClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DescribeK8sSecret(namespace string, name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "secret", name, "-n", namespace)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(string(output), nil)
}

func ContainerSecretDoesExistForStage(stage dtos.K8sStageDto) bool {
	provider := NewKubeProvider()
	secret, err := provider.ClientSet.CoreV1().Secrets(stage.Name).Get(context.TODO(), "container-secret-"+stage.Name, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return secret != nil
}

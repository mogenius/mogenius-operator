package kubernetes

import (
	"context"
	"encoding/base64"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateSecret(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create Kubernetes secret", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating secret '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateSecret ERROR: %s", err.Error()), c)
			return
		}

		secretClient := kubeProvider.ClientSet.CoreV1().Secrets(stage.K8sName)
		secret := utils.InitSecret()
		secret.ObjectMeta.Name = service.K8sName
		secret.ObjectMeta.Namespace = stage.K8sName
		delete(secret.StringData, "PRIVATE_KEY") // delete example data

		for _, envVar := range service.EnvVars {
			if envVar.Type == "KEY_VAULT" ||
				envVar.Type == "PLAINTEXT" ||
				envVar.Type == "HOSTNAME" {
				secret.StringData[envVar.Name] = envVar.Value
			}
		}

		createOptions := metav1.CreateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		_, err = secretClient.Create(context.TODO(), &secret, createOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateSecret ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Created secret '%s'.", service.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func DeleteSecret(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete Kubernetes secret", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting secret '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteSecret ERROR: %s", err.Error()), c)
			return
		}

		secretClient := kubeProvider.ClientSet.CoreV1().Secrets(stage.K8sName)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: utils.Pointer[int64](5),
		}

		err = secretClient.Delete(context.TODO(), service.K8sName, deleteOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteSecret ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Deleted secret '%s'.", service.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func CreateContainerSecret(job *structs.Job, namespace dtos.K8sNamespaceDto, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create Container secret", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating Container secret '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateContainerSecret ERROR: %s", err.Error()), c)
			return
		}

		secretClient := kubeProvider.ClientSet.CoreV1().Secrets(stage.K8sName)
		secret := utils.InitContainerSecret()
		secret.ObjectMeta.Name = "container-secret-" + stage.K8sName
		secret.ObjectMeta.Namespace = stage.K8sName

		authStr := fmt.Sprintf("%s:%s", namespace.ContainerRegistryUser, namespace.ContainerRegistryPat)
		authStrBase64 := base64.StdEncoding.EncodeToString([]byte(authStr))
		jsonData := fmt.Sprintf(`{"auths":{"%s":{"username":"%s","password":"%s","auth":"%s"}}}`, namespace.ContainerRegistryUrl, namespace.ContainerRegistryUser, namespace.ContainerRegistryPat, authStrBase64)

		secretStringData := make(map[string]string)
		secretStringData[".dockerconfigjson"] = jsonData // base64.StdEncoding.EncodeToString([]byte(jsonData))
		secret.StringData = secretStringData

		createOptions := metav1.CreateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		_, err = secretClient.Create(context.TODO(), &secret, createOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateContainerSecret ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Created Container secret '%s'.", stage.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func DeleteContainerSecret(job *structs.Job, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete Container secret", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting Container secret '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteContainerSecret ERROR: %s", err.Error()), c)
			return
		}

		secretClient := kubeProvider.ClientSet.CoreV1().Secrets(stage.K8sName)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: utils.Pointer[int64](5),
		}

		err = secretClient.Delete(context.TODO(), "container-secret-"+stage.K8sName, deleteOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteContainerSecret ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Deleted Container secret '%s'.", stage.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func UpdateSecrete(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Update Kubernetes secret", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Updating secret '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}
		if err != nil {
			cmd.Fail(fmt.Sprintf("UpdateSecret ERROR: %s", err.Error()), c)
			return
		}
		secretClient := kubeProvider.ClientSet.CoreV1().Secrets(stage.K8sName)
		secret := utils.InitSecret()
		secret.ObjectMeta.Name = service.K8sName
		secret.ObjectMeta.Namespace = stage.K8sName
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

		_, err = secretClient.Update(context.TODO(), &secret, updateOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("UpdateSecret ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Update secret '%s'.", service.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func AllSecrets(namespaceName string) []v1.Secret {
	result := []v1.Secret{}

	var provider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		provider, err = NewKubeProviderLocal()
	} else {
		provider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("AllSecrets ERROR: %s", err.Error())
		return result
	}

	secretList, err := provider.ClientSet.CoreV1().Secrets(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllSecrets podMetricsList ERROR: %s", err.Error())
		return result
	}

	for _, secret := range secretList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, secret.ObjectMeta.Namespace) {
			result = append(result, secret)
		}
	}
	return result
}

func ContainerSecretDoesExistForStage(stage dtos.K8sStageDto) bool {
	var provider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		provider, err = NewKubeProviderLocal()
	} else {
		provider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("SecretDoesExistForStage ERROR: %s", err.Error())
		return false
	}

	secret, err := provider.ClientSet.CoreV1().Secrets(stage.K8sName).Get(context.TODO(), "container-secret-"+stage.K8sName, metav1.GetOptions{})
	if err != nil {
		logger.Log.Errorf("SecretDoesExistForStage ERROR: %s", err.Error())
		return false
	}
	return secret != nil
}

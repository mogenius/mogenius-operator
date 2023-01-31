package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	"github.com/gorilla/websocket"
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
			logger.Log.Errorf("CreateSecret ERROR: %s", err.Error())
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
			logger.Log.Errorf("DeleteSecret ERROR: %s", err.Error())
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
			logger.Log.Errorf("UpdateSecret ERROR: %s", err.Error())
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

package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateConfigMap(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create Kubernetes ConfigMap", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating ConfigMap '%s'.", namespace.Name))

		kubeProvider := punq.NewKubeProvider()
		configMapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(namespace.Name)
		configMap := utils.InitConfigMap()
		configMap.ObjectMeta.Name = service.Name
		configMap.ObjectMeta.Namespace = namespace.Name
		delete(configMap.Data, "XXX") // delete example data

		// TODO: WRITE STUFF INTO CONFIGMAP
		configMap.Labels = MoUpdateLabels(&configMap.Labels, job.ProjectId, &namespace, &service)

		_, err := configMapClient.Create(context.TODO(), &configMap, MoCreateOptions())
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateConfigMap ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created ConfigMap '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func DeleteConfigMap(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete Kubernetes configMap", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting configMap '%s'.", namespace.Name))

		kubeProvider := punq.NewKubeProvider()
		configMapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: punqUtils.Pointer[int64](5),
		}

		err := configMapClient.Delete(context.TODO(), service.Name, deleteOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteConfigMap ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted configMap '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func UpdateConfigMap(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Update Kubernetes configMap", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Updating configMap '%s'.", namespace.Name))

		kubeProvider := punq.NewKubeProvider()
		configMapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(namespace.Name)
		configMap := utils.InitConfigMap()
		configMap.ObjectMeta.Name = service.Name
		configMap.ObjectMeta.Namespace = namespace.Name
		delete(configMap.Data, "XXX") // delete example data

		// TODO: WRITE STUFF INTO CONFIGMAP

		updateOptions := metav1.UpdateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		_, err := configMapClient.Update(context.TODO(), &configMap, updateOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("UpdateConfigMap ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Update configMap '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func AddKeyToConfigMap(job *structs.Job, namespace string, configMapName string, key string, value string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Update Kubernetes configMap", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Updating configMap '%s'.", configMapName))

		configMap := punq.ConfigMapFor(namespace, configMapName)
		if configMap != nil {
			kubeProvider := punq.NewKubeProvider()
			configMapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(namespace)
			configMap.Data[key] = value

			_, err := configMapClient.Update(context.TODO(), configMap, metav1.UpdateOptions{})
			if err != nil {
				cmd.Fail(fmt.Sprintf("UpdateConfigMap ERROR: %s", err.Error()))
				return
			} else {
				cmd.Success(fmt.Sprintf("Update configMap '%s'.", configMap))
				return
			}
		}
		cmd.Fail(fmt.Sprintf("ConfigMap '%s/%s' not found.", namespace, configMapName))
	}(cmd, wg)
	return cmd
}

func RemoveKeyFromConfigMap(job *structs.Job, namespace string, configMapName string, key string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Update Kubernetes configMap", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start("Update Kubernetes configMap.")

		configMap := punq.ConfigMapFor(namespace, configMapName)
		if configMap != nil {
			if configMap.Data == nil {
				cmd.Success("ConfigMap contains no data. No key was removed.")
				return
			} else {
				delete(configMap.Data, key)

				kubeProvider := punq.NewKubeProvider()
				updateOptions := metav1.UpdateOptions{
					FieldManager: DEPLOYMENTNAME,
				}
				configMapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(namespace)
				_, err := configMapClient.Update(context.TODO(), configMap, updateOptions)
				if err != nil {
					cmd.Fail(fmt.Sprintf("RemoveKey ERROR: %s", err.Error()))
					return
				}
				cmd.Success(fmt.Sprintf("Key %s successfully removed.", key))
				return
			}
		}
		cmd.Fail(fmt.Sprintf("ConfigMap '%s/%s' not found.", namespace, configMapName))
	}(cmd, wg)
	return cmd
}

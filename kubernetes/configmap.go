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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateConfigMap(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create Kubernetes ConfigMap", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating ConfigMap '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}
		if err != nil {
			logger.Log.Errorf("CreateConfigMap ERROR: %s", err.Error())
		}

		configMapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(stage.K8sName)
		configMap := utils.InitConfigMap()
		configMap.ObjectMeta.Name = service.K8sName
		configMap.ObjectMeta.Namespace = stage.K8sName
		delete(configMap.Data, "XXX") // delete example data

		// TODO: WRITE STUFF INTO CONFIGMAP

		createOptions := metav1.CreateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		_, err = configMapClient.Create(context.TODO(), &configMap, createOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateConfigMap ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Created ConfigMap '%s'.", service.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func DeleteConfigMap(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete Kubernetes configMap", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting configMap '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}
		if err != nil {
			logger.Log.Errorf("DeleteConfigMap ERROR: %s", err.Error())
		}

		configMapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(stage.K8sName)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: utils.Pointer[int64](5),
		}

		err = configMapClient.Delete(context.TODO(), service.K8sName, deleteOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteConfigMap ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Deleted configMap '%s'.", service.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func UpdateConfigMap(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Update Kubernetes configMap", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Updating configMap '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}
		if err != nil {
			logger.Log.Errorf("UpdateConfigmap ERROR: %s", err.Error())
		}
		configMapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(stage.K8sName)
		configMap := utils.InitConfigMap()
		configMap.ObjectMeta.Name = service.K8sName
		configMap.ObjectMeta.Namespace = stage.K8sName
		delete(configMap.Data, "XXX") // delete example data

		// TODO: WRITE STUFF INTO CONFIGMAP

		updateOptions := metav1.UpdateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		_, err = configMapClient.Update(context.TODO(), &configMap, updateOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("UpdateConfigMap ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Update configMap '%s'.", service.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func AddKeyToConfigMap(job *structs.Job, namespace string, configMapName string, key string, value string, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Update Kubernetes configMap", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Updating configMap '%s'.", configMapName), c)

		configMap := ConfigMapFor(namespace, configMapName)
		if configMap != nil {
			var kubeProvider *KubeProvider
			var err error
			if !utils.CONFIG.Kubernetes.RunInCluster {
				kubeProvider, err = NewKubeProviderLocal()
			} else {
				kubeProvider, err = NewKubeProviderInCluster()
			}
			if err != nil {
				cmd.Fail(fmt.Sprintf("UpdateConfigMap ERROR: %s", err.Error()), c)
				return
			}
			configMapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(namespace)
			configMap.Data[key] = value

			_, err = configMapClient.Update(context.TODO(), configMap, metav1.UpdateOptions{})
			if err != nil {
				cmd.Fail(fmt.Sprintf("UpdateConfigMap ERROR: %s", err.Error()), c)
				return
			} else {
				cmd.Success(fmt.Sprintf("Update configMap '%s'.", configMap), c)
				return
			}
		}
		cmd.Fail(fmt.Sprintf("ConfigMap '%s/%s' not found.", namespace, configMapName), c)
	}(cmd, wg)
	return cmd
}

func RemoveKeyFromConfigMap(job *structs.Job, namespace string, configMapName string, key string, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Update Kubernetes configMap", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start("Update Kubernetes configMap.", c)

		configMap := ConfigMapFor(namespace, configMapName)
		if configMap != nil {
			if configMap.Data == nil {
				cmd.Success("ConfigMap contains no data. No key was removed.", c)
				return
			} else {
				delete(configMap.Data, key)

				var kubeProvider *KubeProvider
				var err error
				if !utils.CONFIG.Kubernetes.RunInCluster {
					kubeProvider, err = NewKubeProviderLocal()
				} else {
					kubeProvider, err = NewKubeProviderInCluster()
				}
				if err != nil {
					cmd.Fail(fmt.Sprintf("RemoveKey ERROR: %s", err.Error()), c)
					return
				}
				updateOptions := metav1.UpdateOptions{
					FieldManager: DEPLOYMENTNAME,
				}
				configMapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(namespace)
				_, err = configMapClient.Update(context.TODO(), configMap, updateOptions)
				if err != nil {
					cmd.Fail(fmt.Sprintf("RemoveKey ERROR: %s", err.Error()), c)
					return
				}
				cmd.Success(fmt.Sprintf("Key %s successfully removed.", key), c)
				return
			}
		}
		cmd.Fail(fmt.Sprintf("ConfigMap '%s/%s' not found.", namespace, configMapName), c)
	}(cmd, wg)
	return cmd
}

func ConfigMapFor(namespace string, configMapName string) *v1.ConfigMap {
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}

	if err != nil {
		logger.Log.Errorf("ConfigMapFor ERROR: %s", err.Error())
	}

	configMapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(namespace)
	configMap, err := configMapClient.Get(context.TODO(), configMapName, metav1.GetOptions{})
	if err != nil {
		logger.Log.Errorf("ConfigMapFor ERROR: %s", err.Error())
	}
	return configMap
}

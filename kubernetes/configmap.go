package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"os/exec"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateConfigMap(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create Kubernetes ConfigMap", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating ConfigMap '%s'.", stage.Name))

		kubeProvider := NewKubeProvider()
		configMapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(stage.Name)
		configMap := utils.InitConfigMap()
		configMap.ObjectMeta.Name = service.Name
		configMap.ObjectMeta.Namespace = stage.Name
		delete(configMap.Data, "XXX") // delete example data

		// TODO: WRITE STUFF INTO CONFIGMAP
		configMap.Labels = MoUpdateLabels(&configMap.Labels, &job.NamespaceId, &stage, &service)

		_, err := configMapClient.Create(context.TODO(), &configMap, MoCreateOptions())
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateConfigMap ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created ConfigMap '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func DeleteConfigMap(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete Kubernetes configMap", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting configMap '%s'.", stage.Name))

		kubeProvider := NewKubeProvider()
		configMapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(stage.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: utils.Pointer[int64](5),
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

func UpdateConfigMap(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Update Kubernetes configMap", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Updating configMap '%s'.", stage.Name))

		kubeProvider := NewKubeProvider()
		configMapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(stage.Name)
		configMap := utils.InitConfigMap()
		configMap.ObjectMeta.Name = service.Name
		configMap.ObjectMeta.Namespace = stage.Name
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

		configMap := ConfigMapFor(namespace, configMapName)
		if configMap != nil {
			kubeProvider := NewKubeProvider()
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

		configMap := ConfigMapFor(namespace, configMapName)
		if configMap != nil {
			if configMap.Data == nil {
				cmd.Success("ConfigMap contains no data. No key was removed.")
				return
			} else {
				delete(configMap.Data, key)

				kubeProvider := NewKubeProvider()
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

func ConfigMapFor(namespace string, configMapName string) *v1.ConfigMap {
	kubeProvider := NewKubeProvider()
	configMapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(namespace)
	configMap, err := configMapClient.Get(context.TODO(), configMapName, metav1.GetOptions{})
	if err != nil {
		logger.Log.Errorf("ConfigMapFor ERROR: %s", err.Error())
		return nil
	}
	return configMap
}

func AllConfigmaps(namespaceName string) []v1.ConfigMap {
	result := []v1.ConfigMap{}

	provider := NewKubeProvider()
	configmapList, err := provider.ClientSet.CoreV1().ConfigMaps(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllConfigmaps ERROR: %s", err.Error())
		return result
	}

	for _, configmap := range configmapList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, configmap.ObjectMeta.Namespace) {
			result = append(result, configmap)
		}
	}
	return result
}

func AllK8sConfigmaps(namespaceName string) K8sWorkloadResult {
	result := []v1.ConfigMap{}

	provider := NewKubeProvider()
	configmapList, err := provider.ClientSet.CoreV1().ConfigMaps(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllConfigmaps ERROR: %s", err.Error())
		return WorkloadResult(nil, err)
	}

	for _, configmap := range configmapList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, configmap.ObjectMeta.Namespace) {
			result = append(result, configmap)
		}
	}
	return WorkloadResult(result, nil)
}

func UpdateK8sConfigMap(data v1.ConfigMap) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	configmapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(data.Namespace)
	_, err := configmapClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		logger.Log.Errorf("UpdateK8sConfigMap ERROR: %s", err.Error())
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DeleteK8sConfigmap(data v1.ConfigMap) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	configmapClient := kubeProvider.ClientSet.CoreV1().ConfigMaps(data.Namespace)
	err := configmapClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		logger.Log.Errorf("DeleteK8sConfigmap ERROR: %s", err.Error())
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DescribeK8sConfigmap(namespace string, name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "configmap", name, "-n", namespace)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		logger.Log.Errorf("Error: %s", string(output))
		return WorkloadResult(nil, string(output))
	}
	return WorkloadResult(string(output), nil)
}

func NewK8sConfigmap() K8sNewWorkload {
	return NewWorkload(
		RES_CONFIGMAP,
		utils.InitConfigMapYaml(),
		"ConfigMaps allow you to decouple configuration artifacts from image content to keep containerized applications portable. In this example, a ConfigMap named 'my-configmap' is created with two key-value pairs: my-key and my-value, another-key and another-value. ConfigMap data can be referenced in many ways depending on where you need the data to be used. For example, you could use a ConfigMap to set environment variables for a Pod, or mount a ConfigMap as a volume in a Pod.")
}

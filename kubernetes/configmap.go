package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"strings"
	"sync"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateConfigMap(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create Kubernetes ConfigMap", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating ConfigMap '%s'.", namespace.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		if provider == nil || err != nil {
			return
		}
		configMapClient := provider.ClientSet.CoreV1().ConfigMaps(namespace.Name)
		configMap := punqUtils.InitConfigMap()
		configMap.ObjectMeta.Name = service.Name
		configMap.ObjectMeta.Namespace = namespace.Name
		delete(configMap.Data, "XXX") // delete example data

		// TODO: WRITE STUFF INTO CONFIGMAP
		configMap.Labels = MoUpdateLabels(&configMap.Labels, job.ProjectId, &namespace, &service)

		_, err = configMapClient.Create(context.TODO(), &configMap, MoCreateOptions())
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

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		configMapClient := provider.ClientSet.CoreV1().ConfigMaps(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: punqUtils.Pointer[int64](5),
		}

		err = configMapClient.Delete(context.TODO(), service.Name, deleteOptions)
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

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		if provider == nil || err != nil {
			return
		}
		configMapClient := provider.ClientSet.CoreV1().ConfigMaps(namespace.Name)
		configMap := punqUtils.InitConfigMap()
		configMap.ObjectMeta.Name = service.Name
		configMap.ObjectMeta.Namespace = namespace.Name
		delete(configMap.Data, "XXX") // delete example data

		// TODO: WRITE STUFF INTO CONFIGMAP

		updateOptions := metav1.UpdateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		_, err = configMapClient.Update(context.TODO(), &configMap, updateOptions)
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

		configMap := punq.ConfigMapFor(namespace, configMapName, false, nil)
		if configMap != nil {
			provider, err := punq.NewKubeProvider(nil)
			if err != nil {
				cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
				return
			}
			configMapClient := provider.ClientSet.CoreV1().ConfigMaps(namespace)
			configMap.Data[key] = value

			_, err = configMapClient.Update(context.TODO(), configMap, metav1.UpdateOptions{})
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

		configMap := punq.ConfigMapFor(namespace, configMapName, false, nil)
		if configMap != nil {
			if configMap.Data == nil {
				cmd.Success("ConfigMap contains no data. No key was removed.")
				return
			} else {
				delete(configMap.Data, key)

				provider, err := punq.NewKubeProvider(nil)
				if err != nil {
					cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
					return
				}
				updateOptions := metav1.UpdateOptions{
					FieldManager: DEPLOYMENTNAME,
				}
				configMapClient := provider.ClientSet.CoreV1().ConfigMaps(namespace)
				_, err = configMapClient.Update(context.TODO(), configMap, updateOptions)
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

func WriteConfigMap(namespace string, name string, data string, labels map[string]string) error {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return err
	}
	client := provider.ClientSet.CoreV1().ConfigMaps(namespace)

	cfgMap, err := client.Get(context.TODO(), name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		newConfigMap := v1.ConfigMap{}
		newConfigMap.Data = make(map[string]string)
		newConfigMap.Name = name
		newConfigMap.Namespace = namespace
		newConfigMap.Labels = labels
		newConfigMap.Data["data"] = data
		_, err := client.Create(context.TODO(), &newConfigMap, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else if err == nil && cfgMap != nil {
		cfgMap.Data["data"] = data
		// merge new configmap labels with existing ones
		for key, value := range labels {
			cfgMap.Labels[key] = value
		}

		_, err := client.Update(context.TODO(), cfgMap, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		logger.Log.Errorf("CreateOrUpdateConfigMap ERROR: %s", err.Error())
		return err
	}
	return nil
}

func GetConfigMap(namespace string, name string) K8sWorkloadResult {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return WorkloadResult(nil, err)
	}
	client := provider.ClientSet.CoreV1().ConfigMaps(namespace)

	cfgMap, err := client.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(cfgMap.Data["data"], err)
}

func ListConfigMapWithFieldSelector(namespace string, labelSelector string, prefix string) K8sWorkloadResult {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return WorkloadResult(nil, err)
	}
	client := provider.ClientSet.CoreV1().ConfigMaps(namespace)

	cfgMaps, err := client.List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return WorkloadResult(nil, err)
	}

	// delete all configmaps that do not start with prefix
	if prefix != "" {
		for i := len(cfgMaps.Items) - 1; i >= 0; i-- {
			if !strings.HasPrefix(cfgMaps.Items[i].Name, prefix) {
				cfgMaps.Items = append(cfgMaps.Items[:i], cfgMaps.Items[i+1:]...)
			}
		}
	}

	return WorkloadResult(cfgMaps.Items, err)
}

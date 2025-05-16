package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/websocket"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ClusterForceReconnect() bool {
	// restart deployments/daemonsets for
	// - traffic
	// - podstats
	// - k8s-manager

	clientset := clientProvider.K8sClientSet()
	podClient := clientset.CoreV1().Pods(config.Get("MO_OWN_NAMESPACE"))

	podsToKill := []string{}
	podsToKill = append(podsToKill, AllPodNamesForLabel(config.Get("MO_OWN_NAMESPACE"), "app", GetOwnDeploymentName(config))...)

	for _, podName := range podsToKill {
		k8sLogger.Warn("Restarting pod ...", "podName", podName)
		err := podClient.Delete(context.TODO(), podName, metav1.DeleteOptions{})
		if err != nil {
			k8sLogger.Error("failed to delete pod", "podName", podName, "error", err)
		}
	}

	return true
}

func ClusterForceDisconnect() bool {
	// restart deployments/daemonsets for
	// - traffic
	// - podstats
	// - k8s-manager

	clientset := clientProvider.K8sClientSet()
	podClient := clientset.CoreV1().Pods(config.Get("MO_OWN_NAMESPACE"))

	// stop k8s-manager
	deploymentClient := clientset.AppsV1().Deployments(config.Get("MO_OWN_NAMESPACE"))
	deployment, _ := deploymentClient.Get(context.TODO(), GetOwnDeploymentName(config), metav1.GetOptions{})
	deployment.Spec.Replicas = utils.Pointer[int32](0)
	_, err := deploymentClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		k8sLogger.Error("Error updating deployment", "deployment", deployment, "error", err)
	}

	podsToKill := []string{}
	podsToKill = append(podsToKill, AllPodNamesForLabel(config.Get("MO_OWN_NAMESPACE"), "app", GetOwnDeploymentName(config))...)

	for _, podName := range podsToKill {
		k8sLogger.Warn("Restarting pod...", "pod", podName)
		err := podClient.Delete(context.TODO(), podName, metav1.DeleteOptions{})

		if err != nil {
			k8sLogger.Error("failed to delete pod", "pod", podName, "error", err)
		}
	}

	return true
}

func UpgradeMyself(eventClient websocket.WebsocketClient, job *structs.Job, command string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand(eventClient, "upgrade operator", "Upgrade mogenius platform ...", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(eventClient, job, "Upgrade mogenius platform ...")

		clientset := clientProvider.K8sClientSet()
		jobClient := clientset.BatchV1().Jobs(config.Get("MO_OWN_NAMESPACE"))
		configmapClient := clientset.CoreV1().ConfigMaps(config.Get("MO_OWN_NAMESPACE"))

		configmap := utils.InitUpgradeConfigMap()
		configmap.Namespace = config.Get("MO_OWN_NAMESPACE")
		configmap.Data["values.command"] = command

		k8sjob := utils.InitUpgradeJob()
		k8sjob.Namespace = config.Get("MO_OWN_NAMESPACE")
		k8sjob.Name = fmt.Sprintf("%s-%s", k8sjob.Name, utils.NanoIdSmallLowerCase())

		// CONFIGMAP
		_, err := configmapClient.Get(context.TODO(), configmap.Name, metav1.GetOptions{})
		if err != nil {
			// CREATE
			_, err = configmapClient.Create(context.TODO(), &configmap, MoCreateOptions(config))
			if err != nil {
				cmd.Fail(eventClient, job, fmt.Sprintf("UpgradeMyself (configmap) ERROR: %s", err.Error()))
				return
			}
		} else {
			// UPDATE
			_, err = configmapClient.Update(context.TODO(), &configmap, metav1.UpdateOptions{})
			if err != nil {
				cmd.Fail(eventClient, job, fmt.Sprintf("UpgradeMyself (update_configmap) ERROR: %s", err.Error()))
				return
			}
		}

		// JOB
		_, err = jobClient.Get(context.TODO(), k8sjob.Name, metav1.GetOptions{})
		if err != nil {
			// CREATE
			_, err = jobClient.Create(context.TODO(), &k8sjob, MoCreateOptions(config))
			if err != nil {
				cmd.Fail(eventClient, job, fmt.Sprintf("UpgradeMyself (job) ERROR: %s", err.Error()))
				return
			}
		} else {
			// UPDATE
			_, err = jobClient.Update(context.TODO(), &k8sjob, metav1.UpdateOptions{})
			if err != nil {
				cmd.Fail(eventClient, job, fmt.Sprintf("UpgradeMyself (update_job) ERROR: %s", err.Error()))
				return
			}
		}
		cmd.Success(eventClient, job, "Upgraded platform successfully.")
	}(wg)
}

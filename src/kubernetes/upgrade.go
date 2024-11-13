package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"sync"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ClusterForceReconnect() bool {
	// restart deployments/daemonsets for
	// - traffic
	// - podstats
	// - k8s-manager

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return false
	}
	podClient := provider.ClientSet.CoreV1().Pods(config.Get("MO_OWN_NAMESPACE"))

	podsToKill := []string{}
	podsToKill = append(podsToKill, punq.AllPodNamesForLabel(config.Get("MO_OWN_NAMESPACE"), "app", utils.HelmReleaseNameTrafficCollector, nil)...)
	podsToKill = append(podsToKill, punq.AllPodNamesForLabel(config.Get("MO_OWN_NAMESPACE"), "app", utils.HelmReleaseNamePodStatsCollector, nil)...)
	podsToKill = append(podsToKill, punq.AllPodNamesForLabel(config.Get("MO_OWN_NAMESPACE"), "app", DEPLOYMENTNAME, nil)...)

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

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return false
	}
	podClient := provider.ClientSet.CoreV1().Pods(config.Get("MO_OWN_NAMESPACE"))

	// stop k8s-manager
	deploymentClient := provider.ClientSet.AppsV1().Deployments(config.Get("MO_OWN_NAMESPACE"))
	deployment, _ := deploymentClient.Get(context.TODO(), DEPLOYMENTNAME, metav1.GetOptions{})
	deployment.Spec.Paused = true
	deployment.Spec.Replicas = punqUtils.Pointer[int32](0)
	_, err = deploymentClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		k8sLogger.Error("Error updating deployment", "deployment", deployment, "error", err)
	}

	podsToKill := []string{}
	podsToKill = append(podsToKill, punq.AllPodNamesForLabel(config.Get("MO_OWN_NAMESPACE"), "app", DEPLOYMENTNAME, nil)...)

	for _, podName := range podsToKill {
		k8sLogger.Warn("Restarting pod...", "pod", podName)
		err := podClient.Delete(context.TODO(), podName, metav1.DeleteOptions{})

		if err != nil {
			k8sLogger.Error("failed to delete pod", "pod", podName, "error", err)
		}
	}

	return true
}

func UpgradeMyself(job *structs.Job, command string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("upgrade operator", "Upgrade mogenius platform ...", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Upgrade mogenius platform ...")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		jobClient := provider.ClientSet.BatchV1().Jobs(config.Get("MO_OWN_NAMESPACE"))
		configmapClient := provider.ClientSet.CoreV1().ConfigMaps(config.Get("MO_OWN_NAMESPACE"))

		configmap := utils.InitUpgradeConfigMap()
		configmap.Namespace = config.Get("MO_OWN_NAMESPACE")
		configmap.Data["values.command"] = command

		k8sjob := utils.InitUpgradeJob()
		k8sjob.Namespace = config.Get("MO_OWN_NAMESPACE")
		k8sjob.Name = fmt.Sprintf("%s-%s", k8sjob.Name, punqUtils.NanoIdSmallLowerCase())

		// CONFIGMAP
		_, err = configmapClient.Get(context.TODO(), configmap.Name, metav1.GetOptions{})
		if err != nil {
			// CREATE
			_, err = configmapClient.Create(context.TODO(), &configmap, MoCreateOptions())
			if err != nil {
				cmd.Fail(job, fmt.Sprintf("UpgradeMyself (configmap) ERROR: %s", err.Error()))
				return
			}
		} else {
			// UPDATE
			_, err = configmapClient.Update(context.TODO(), &configmap, metav1.UpdateOptions{})
			if err != nil {
				cmd.Fail(job, fmt.Sprintf("UpgradeMyself (update_configmap) ERROR: %s", err.Error()))
				return
			}
		}

		// JOB
		_, err = jobClient.Get(context.TODO(), k8sjob.Name, metav1.GetOptions{})
		if err != nil {
			// CREATE
			_, err = jobClient.Create(context.TODO(), &k8sjob, MoCreateOptions())
			if err != nil {
				cmd.Fail(job, fmt.Sprintf("UpgradeMyself (job) ERROR: %s", err.Error()))
				return
			}
		} else {
			// UPDATE
			_, err = jobClient.Update(context.TODO(), &k8sjob, metav1.UpdateOptions{})
			if err != nil {
				cmd.Fail(job, fmt.Sprintf("UpgradeMyself (update_job) ERROR: %s", err.Error()))
				return
			}
		}
		cmd.Success(job, "Upgraded platform successfully.")
	}(wg)
}

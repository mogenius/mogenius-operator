package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
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
	podClient := provider.ClientSet.CoreV1().Pods(utils.CONFIG.Kubernetes.OwnNamespace)

	podsToKill := []string{}
	podsToKill = append(podsToKill, punq.AllPodNamesForLabel(utils.CONFIG.Kubernetes.OwnNamespace, "app", utils.HelmReleaseNameTrafficCollector, nil)...)
	podsToKill = append(podsToKill, punq.AllPodNamesForLabel(utils.CONFIG.Kubernetes.OwnNamespace, "app", utils.HelmReleaseNamePodStatsCollector, nil)...)
	podsToKill = append(podsToKill, punq.AllPodNamesForLabel(utils.CONFIG.Kubernetes.OwnNamespace, "app", DEPLOYMENTNAME, nil)...)

	for _, podName := range podsToKill {
		log.Warningf("Restarting %s ...", podName)
		err := podClient.Delete(context.TODO(), podName, metav1.DeleteOptions{})
		if err != nil {
			log.Errorf("ClusterForceReconnect ERR: %s", err.Error())
		}
	}

	return true
}

func UpgradeMyself(job *structs.Job, command string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("Upgrade mogenius platform ...", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Upgrade mogenius platform ...")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		jobClient := provider.ClientSet.BatchV1().Jobs(NAMESPACE)
		configmapClient := provider.ClientSet.CoreV1().ConfigMaps(NAMESPACE)

		configmap := utils.InitUpgradeConfigMap()
		configmap.Namespace = NAMESPACE
		configmap.Data["values.command"] = command

		k8sjob := utils.InitUpgradeJob()
		k8sjob.Namespace = NAMESPACE
		k8sjob.Name = fmt.Sprintf("%s-%s", &k8sjob.Name, punqUtils.NanoIdSmallLowerCase())

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

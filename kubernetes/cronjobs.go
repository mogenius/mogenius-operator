package kubernetes

import (
	"context"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	v1job "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllCronjobs(namespaceName string) []v1job.CronJob {
	result := []v1job.CronJob{}

	var provider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		provider, err = NewKubeProviderLocal()
	} else {
		provider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("AllCronjobs ERROR: %s", err.Error())
		return result
	}

	cronJobList, err := provider.ClientSet.BatchV1().CronJobs(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllCronjobs ERROR: %s", err.Error())
		return result
	}

	for _, cronJob := range cronJobList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, cronJob.ObjectMeta.Namespace) {
			result = append(result, cronJob)
		}
	}
	return result
}

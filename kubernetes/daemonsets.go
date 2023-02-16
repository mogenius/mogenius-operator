package kubernetes

import (
	"context"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllDaemonsets(namespaceName string) []v1.DaemonSet {
	result := []v1.DaemonSet{}

	var provider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		provider, err = NewKubeProviderLocal()
	} else {
		provider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("AllDaemonsets ERROR: %s", err.Error())
	}

	daemonsetList, err := provider.ClientSet.AppsV1().DaemonSets(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllDaemonsets ERROR: %s", err.Error())
	}

	for _, daemonset := range daemonsetList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, daemonset.ObjectMeta.Namespace) {
			result = append(result, daemonset)
		}
	}
	return result
}

package kubernetes

import (
	"context"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllReplicasets(namespaceName string) []v1.ReplicaSet {
	result := []v1.ReplicaSet{}

	var provider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		provider, err = NewKubeProviderLocal()
	} else {
		provider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("AllReplicasets ERROR: %s", err.Error())
		return result
	}

	replicaSetList, err := provider.ClientSet.AppsV1().ReplicaSets(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllReplicasets ERROR: %s", err.Error())
		return result
	}

	for _, replicaSet := range replicaSetList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, replicaSet.ObjectMeta.Namespace) {
			result = append(result, replicaSet)
		}
	}
	return result
}

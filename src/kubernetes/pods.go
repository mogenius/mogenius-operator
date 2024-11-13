package kubernetes

import (
	"context"

	punq "github.com/mogenius/punq/kubernetes"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func KeplerPod() *v1.Pod {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		k8sLogger.Error("failed to create kube provider", "error", err)
		return nil
	}
	podClient := provider.ClientSet.CoreV1().Pods("")
	labelSelector := "app.kubernetes.io/component=exporter,app.kubernetes.io/name=kepler"
	pods, err := podClient.List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		k8sLogger.Error("failed to list kepler pods", "labelSelector", labelSelector, "error", err.Error())
		return nil
	}
	for _, pod := range pods.Items {
		if pod.GenerateName == "kepler-" {
			return &pod
		}
	}
	return nil
}

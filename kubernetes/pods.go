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
		K8sLogger.Errorf("KeplerPod ERROR: %s", err.Error())
		return nil
	}
	podClient := provider.ClientSet.CoreV1().Pods("")
	pods, err := podClient.List(context.TODO(), metav1.ListOptions{LabelSelector: "app.kubernetes.io/component=exporter,app.kubernetes.io/name=kepler"})
	if err != nil {
		K8sLogger.Errorf("KeplerPod ERROR: %s", err.Error())
		return nil
	}
	for _, pod := range pods.Items {
		if pod.GenerateName == "kepler-" {
			return &pod
		}
	}
	return nil
}

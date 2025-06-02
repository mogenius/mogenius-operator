package kubernetes

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

func StreamPreviousLog(namespace string, podId string) (*rest.Request, error) {
	clientset := clientProvider.K8sClientSet()
	podClient := clientset.CoreV1().Pods(namespace)

	opts := v1.PodLogOptions{
		Previous:   true,
		Timestamps: true,
	}

	restReq := podClient.GetLogs(podId, &opts)
	return restReq, nil
}

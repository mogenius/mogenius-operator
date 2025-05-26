package kubernetes

import (
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

type ServiceGetLogErrorResult struct {
	Namespace string `json:"namespace"`
	PodId     string `json:"podId"`
	Restarts  int32  `json:"restarts"`
	Log       string `json:"log"`
}

type ServiceGetLogResult struct {
	Namespace       string    `json:"namespace"`
	PodId           string    `json:"podId"`
	ServerTimestamp time.Time `json:"serverTimestamp"`
	Log             string    `json:"log"`
}

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

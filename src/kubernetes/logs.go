package kubernetes

import (
	"bytes"
	"context"
	"io"
	"time"

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

func GetPodLogs(namespace, podName, container string, tailLines int64, previous bool) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := v1.PodLogOptions{
		TailLines:  &tailLines,
		Previous:   previous,
		Timestamps: true,
	}
	if container != "" {
		opts.Container = container
	}

	req := clientProvider.K8sClientSet().CoreV1().Pods(namespace).GetLogs(podName, &opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, stream); err != nil {
		return "", err
	}
	return buf.String(), nil
}

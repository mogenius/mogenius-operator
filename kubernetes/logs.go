package kubernetes

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

func GetLog(namespace string, podId string) string {
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("CreateNamespace ERROR: %s", err.Error())
		return ""
	}

	podClient := kubeProvider.ClientSet.CoreV1().Pods(namespace)

	opts := v1.PodLogOptions{
		TailLines: utils.Pointer[int64](2000),
	}

	restReq := podClient.GetLogs(podId, &opts)
	stream, err := restReq.Stream(context.TODO())
	reader := bufio.NewReader(stream)
	if err != nil {
		return err.Error()
	}
	defer stream.Close()

	resultMsg := ""
	for {
		buf := make([]byte, 2000)
		numBytes, err := reader.Read(buf)
		if numBytes == 0 {
			break
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err.Error()
		}
		message := string(buf[:numBytes])
		resultMsg += message
	}
	return resultMsg
}

func StreamLog(namespace string, podId string, sindceSeconds int64) (*rest.Request, error) {
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("CreateNamespace ERROR: %s", err.Error())
		return nil, fmt.Errorf("CreateNamespace ERROR: %s", err.Error())
	}

	podClient := kubeProvider.ClientSet.CoreV1().Pods(namespace)

	opts := v1.PodLogOptions{
		Follow:    true,
		TailLines: utils.Pointer[int64](2000),
	}

	restReq := podClient.GetLogs(podId, &opts)
	return restReq, nil
}

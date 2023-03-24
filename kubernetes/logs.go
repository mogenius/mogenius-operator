package kubernetes

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type ServiceGetLogErrorResult struct {
	Namespace string `json:"namespace"`
	PodId     string `json:"podId"`
	Restarts  int32  `json:"restarts"`
	Log       string `json:"log"`
}

func GetLog(namespace string, podId string, timestamp *time.Time) string {
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("GetLog ERROR: %s", err.Error())
		return ""
	}

	podClient := kubeProvider.ClientSet.CoreV1().Pods(namespace)

	var kubernetesTime metav1.Time
	if timestamp != nil {
		kubernetesTime = metav1.NewTime(*timestamp)
	}
	opts := v1.PodLogOptions{
		TailLines: utils.Pointer[int64](2000),
		SinceTime: &kubernetesTime,
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

func GetLogError(namespace string, podId string) ServiceGetLogErrorResult {
	result := ServiceGetLogErrorResult{
		Namespace: namespace,
		PodId:     podId,
		Restarts:  0,
		Log:       "",
	}

	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("GetLogError ERROR: %s", err.Error())
		result.Log = err.Error()
		return result
	}

	podClient := kubeProvider.ClientSet.CoreV1().Pods(namespace)

	pod, err := podClient.Get(context.TODO(), podId, metav1.GetOptions{})
	if err != nil {
		logger.Log.Errorf("GetLogError ERROR: %s", err.Error())
		result.Log = err.Error()
		return result
	}
	if len(pod.Status.ContainerStatuses) > 0 {
		result.Restarts = pod.Status.ContainerStatuses[0].RestartCount
	}

	// show empty message if no restart have occoured
	if result.Restarts <= 0 {
		return result
	}

	restReq := podClient.GetLogs(podId, &v1.PodLogOptions{
		TailLines: utils.Pointer[int64](2000),
	})
	stream, err := restReq.Stream(context.TODO())
	if err != nil {
		result.Log = err.Error()
		return result
	}
	defer stream.Close()

	data, err := ioutil.ReadAll(stream)
	if err != nil {
		result.Log = err.Error()
		return result
	}

	result.Log = string(data)
	return result
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

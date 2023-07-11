package kubernetes

import (
	"context"
	"os/exec"

	"mogenius-k8s-manager/logger"

	storage "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllVolumeAttachments() []storage.VolumeAttachment {
	result := []storage.VolumeAttachment{}

	provider := NewKubeProvider()
	volAttachList, err := provider.ClientSet.StorageV1().VolumeAttachments().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Log.Errorf("AllCertificateSigningRequests ERROR: %s", err.Error())
		return result
	}

	result = append(result, volAttachList.Items...)
	return result
}

func UpdateK8sVolumeAttachment(data storage.VolumeAttachment) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	volAttachClient := kubeProvider.ClientSet.StorageV1().VolumeAttachments()
	_, err := volAttachClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DeleteK8sVolumeAttachment(data storage.VolumeAttachment) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	volAttachClient := kubeProvider.ClientSet.StorageV1().VolumeAttachments()
	err := volAttachClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DescribeK8sVolumeAttachment(name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "volumeattachment", name)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		return WorkloadResult(err.Error())
	}
	return WorkloadResult(string(output))
}

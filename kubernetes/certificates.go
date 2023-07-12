package kubernetes

import (
	"context"
	"os/exec"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllCertificates(namespaceName string) K8sWorkloadResult {
	result := []cmapi.Certificate{}

	provider := NewKubeProviderCertManager()
	certificatesList, err := provider.ClientSet.CertmanagerV1().Certificates(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllCertificates ERROR: %s", err.Error())
		return WorkloadResult(nil, err)
	}

	for _, certificate := range certificatesList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, certificate.ObjectMeta.Namespace) {
			result = append(result, certificate)
		}
	}
	return WorkloadResult(result, nil)
}

func UpdateK8sCertificate(data cmapi.Certificate) K8sWorkloadResult {
	kubeProvider := NewKubeProviderCertManager()
	certificateClient := kubeProvider.ClientSet.CertmanagerV1().Certificates(data.Namespace)
	_, err := certificateClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DeleteK8sCertificate(data cmapi.Certificate) K8sWorkloadResult {
	kubeProvider := NewKubeProviderCertManager()
	certificateClient := kubeProvider.ClientSet.CertmanagerV1().Certificates(data.Namespace)
	err := certificateClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DescribeK8sCertificate(namespace string, name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "certificate", name, "-n", namespace)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		logger.Log.Errorf("Error: %s", string(output))
		return WorkloadResult(nil, string(output))
	}
	return WorkloadResult(string(output), nil)
}

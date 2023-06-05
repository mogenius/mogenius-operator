package kubernetes

import (
	"context"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllCertificateSigningRequests(namespaceName string) []cmapi.CertificateRequest {
	result := []cmapi.CertificateRequest{}

	provider := NewKubeProviderCertManager()
	certificatesList, err := provider.ClientSet.CertmanagerV1().CertificateRequests(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllCertificateSigningRequests ERROR: %s", err.Error())
		return result
	}

	for _, certificate := range certificatesList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, certificate.ObjectMeta.Namespace) {
			result = append(result, certificate)
		}
	}
	return result
}

func UpdateK8sCertificateSigningRequest(data cmapi.CertificateRequest) K8sWorkloadResult {
	kubeProvider := NewKubeProviderCertManager()
	certificateClient := kubeProvider.ClientSet.CertmanagerV1().CertificateRequests(data.Namespace)
	_, err := certificateClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DeleteK8sCertificateSigningRequest(data cmapi.CertificateRequest) K8sWorkloadResult {
	kubeProvider := NewKubeProviderCertManager()
	certificateClient := kubeProvider.ClientSet.CertmanagerV1().CertificateRequests(data.Namespace)
	err := certificateClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

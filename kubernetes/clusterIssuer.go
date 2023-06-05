package kubernetes

import (
	"context"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllClusterIssuers() []cmapi.ClusterIssuer {
	result := []cmapi.ClusterIssuer{}

	provider := NewKubeProviderCertManager()
	issuersList, err := provider.ClientSet.CertmanagerV1().ClusterIssuers().List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllIssuer ERROR: %s", err.Error())
		return result
	}

	for _, issuer := range issuersList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, issuer.ObjectMeta.Namespace) {
			result = append(result, issuer)
		}
	}
	return result
}

func UpdateK8sClusterIssuer(data cmapi.ClusterIssuer) K8sWorkloadResult {
	kubeProvider := NewKubeProviderCertManager()
	issuerClient := kubeProvider.ClientSet.CertmanagerV1().ClusterIssuers()
	_, err := issuerClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DeleteK8sClusterIssuer(data cmapi.ClusterIssuer) K8sWorkloadResult {
	kubeProvider := NewKubeProviderCertManager()
	issuerClient := kubeProvider.ClientSet.CertmanagerV1().ClusterIssuers()
	err := issuerClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

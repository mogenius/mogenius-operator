package kubernetes

import (
	"context"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	v1 "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllOrders(namespaceName string) []v1.Order {
	result := []v1.Order{}

	provider := NewKubeProviderCertManager()
	orderList, err := provider.ClientSet.AcmeV1().Orders(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllCertificateSigningRequests ERROR: %s", err.Error())
		return result
	}

	for _, certificate := range orderList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, certificate.ObjectMeta.Namespace) {
			result = append(result, certificate)
		}
	}
	return result
}

func UpdateK8sOrder(data v1.Order) K8sWorkloadResult {
	kubeProvider := NewKubeProviderCertManager()
	orderClient := kubeProvider.ClientSet.AcmeV1().Orders(data.Namespace)
	_, err := orderClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DeleteK8sOrder(data v1.Order) K8sWorkloadResult {
	kubeProvider := NewKubeProviderCertManager()
	orderClient := kubeProvider.ClientSet.AcmeV1().Orders(data.Namespace)
	err := orderClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

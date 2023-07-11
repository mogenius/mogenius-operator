package kubernetes

import (
	"context"
	"os/exec"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllIssuer(namespaceName string) []cmapi.Issuer {
	result := []cmapi.Issuer{}

	provider := NewKubeProviderCertManager()
	issuersList, err := provider.ClientSet.CertmanagerV1().Issuers(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
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

func UpdateK8sIssuer(data cmapi.Issuer) K8sWorkloadResult {
	kubeProvider := NewKubeProviderCertManager()
	issuerClient := kubeProvider.ClientSet.CertmanagerV1().Issuers(data.Namespace)
	_, err := issuerClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DeleteK8sIssuer(data cmapi.Issuer) K8sWorkloadResult {
	kubeProvider := NewKubeProviderCertManager()
	issuerClient := kubeProvider.ClientSet.CertmanagerV1().Issuers(data.Namespace)
	err := issuerClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DescribeK8sIssuer(namespace string, name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "issuer", name, "-n", namespace)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		return WorkloadResult(err.Error())
	}
	return WorkloadResult(string(output))
}

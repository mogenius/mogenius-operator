package kubernetes

import (
	"context"
	"os/exec"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllClusterIssuers() K8sWorkloadResult {
	result := []cmapi.ClusterIssuer{}

	provider := NewKubeProviderCertManager()
	issuersList, err := provider.ClientSet.CertmanagerV1().ClusterIssuers().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Log.Errorf("AllIssuer ERROR: %s", err.Error())
		return WorkloadResult(nil, err)
	}

	for _, issuer := range issuersList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, issuer.ObjectMeta.Namespace) {
			result = append(result, issuer)
		}
	}
	return WorkloadResult(result, nil)
}

func UpdateK8sClusterIssuer(data cmapi.ClusterIssuer) K8sWorkloadResult {
	kubeProvider := NewKubeProviderCertManager()
	issuerClient := kubeProvider.ClientSet.CertmanagerV1().ClusterIssuers()
	_, err := issuerClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DeleteK8sClusterIssuer(data cmapi.ClusterIssuer) K8sWorkloadResult {
	kubeProvider := NewKubeProviderCertManager()
	issuerClient := kubeProvider.ClientSet.CertmanagerV1().ClusterIssuers()
	err := issuerClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DescribeK8sClusterIssuer(name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "ingress", name)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(string(output), nil)
}

package kubernetes

import (
	"context"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllClusterIssuers() []cmapi.ClusterIssuer {
	result := []cmapi.ClusterIssuer{}

	provider, err := NewKubeProviderCertManager()
	if err != nil {
		return result
	}
	issuersList, err := provider.ClientSet.CertmanagerV1().ClusterIssuers().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		k8sLogger.Error("AllIssuer", "error", err.Error())
		return result
	}

	for _, issuer := range issuersList.Items {
		issuer.Kind = "ClusterIssuer"
		issuer.APIVersion = "cert-manager.io/v1"
		result = append(result, issuer)
	}
	return result
}

func GetClusterIssuer(name string) (*cmapi.ClusterIssuer, error) {
	provider, err := NewKubeProviderCertManager()
	if err != nil {
		return nil, err
	}
	issuer, err := provider.ClientSet.CertmanagerV1().ClusterIssuers().Get(context.TODO(), name, metav1.GetOptions{})
	issuer.Kind = "ClusterIssuer"
	issuer.APIVersion = "cert-manager.io/v1"
	return issuer, err
}

func DeleteK8sClusterIssuerBy(name string) error {
	provider, err := NewKubeProviderCertManager()
	if err != nil {
		return err
	}
	client := provider.ClientSet.CertmanagerV1().ClusterIssuers()
	return client.Delete(context.TODO(), name, metav1.DeleteOptions{})
}

package kubernetes

import (
	"context"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

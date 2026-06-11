package kubernetes

import (
	"context"
	"mogenius-operator/src/utils"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetClusterIssuer(name string) (*cmapi.ClusterIssuer, error) {
	provider, err := NewKubeProviderCertManager()
	if err != nil {
		return nil, err
	}

	issuer, err := provider.ClientSet.CertmanagerV1().ClusterIssuers().Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	issuer.Kind = utils.ClusterIssuerResource.Kind
	issuer.APIVersion = utils.ClusterIssuerResource.ApiVersion
	return issuer, nil
}

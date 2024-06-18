package kubernetes

import (
	"context"

	punq "github.com/mogenius/punq/kubernetes"
	log "github.com/sirupsen/logrus"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getCoreClient() v1.CoreV1Interface {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		log.Fatal("Error creating kubeprovider")
	}
	client := provider.ClientSet.CoreV1()

	return client
}

func CreateServiceAccount(serviceAccountName string, namespace string) (*core.ServiceAccount, error) {
	serviceAccount := &core.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: SERVICEACCOUNTNAME,
		},
	}

	return getCoreClient().ServiceAccounts(NAMESPACE).Create(context.TODO(), serviceAccount, MoCreateOptions())
}

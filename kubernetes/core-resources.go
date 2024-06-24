package kubernetes

import (
	"context"
	"fmt"

	punq "github.com/mogenius/punq/kubernetes"
	"github.com/mogenius/punq/logger"
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

func ApplyServiceAccount(serviceAccountName string, namespace string, annotations map[string]string) error {
	serviceAccount := &core.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceAccountName,
			Annotations: annotations,
		},
	}
	client := getCoreClient().ServiceAccounts(namespace)

	_, err := client.Create(context.TODO(), serviceAccount, MoCreateOptions())
	if err == nil {
		logger.Log.Info("ServiceAccount created successfully ✅")
	} else {
		// Check if already exists
		serviceAccount, err := GetServiceAccount(serviceAccountName, namespace)
		if err != nil {
			return err
		} else {
			logger.Log.Info(fmt.Sprintf("ServiceAccount retrieved ns: %s - name: %s", serviceAccount.GetNamespace(), serviceAccount.GetName()))
		}
		if serviceAccount.Annotations == nil {
			serviceAccount.Annotations = make(map[string]string)
		}
		// add/overwrite new annotations to existing
		for key, value := range annotations {
			serviceAccount.Annotations[key] = value
		}

		// Try update if already exists
		return UpdateServiceAccount(serviceAccount)
	}
	return nil
}

func UpdateServiceAccount(serviceAccount *core.ServiceAccount) error {
	client := getCoreClient().ServiceAccounts(serviceAccount.GetNamespace())

	_, err := client.Update(context.TODO(), serviceAccount, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	logger.Log.Info("ServiceAccount updated successfully ✅")
	return nil
}

func GetServiceAccount(serviceAccountName string, namespace string) (*core.ServiceAccount, error) {
	client := getCoreClient().ServiceAccounts(namespace)
	return client.Get(context.TODO(), serviceAccountName, metav1.GetOptions{})
}

func DeleteServiceAccount(serviceAccountName string, namespace string) error {
	client := getCoreClient().ServiceAccounts(namespace)
	err := client.Delete(context.TODO(), serviceAccountName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	logger.Log.Info("ServiceAccount deleted successfully ✅")
	return nil
}

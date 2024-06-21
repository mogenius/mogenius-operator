package kubernetes

import (
	"context"
	"fmt"

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

func CreateServiceAccount(serviceAccountName string, namespace string) error {
	serviceAccount := &core.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceAccountName,
		},
	}
	client := getCoreClient().ServiceAccounts(namespace)

	_, err := client.Create(context.TODO(), serviceAccount, MoCreateOptions())
	if err == nil {
		fmt.Println("Resource created successfully")
	} else {
		// res, err := client.Get()(gvr.Group, gvr.Version, gvr.Resource, obj.GetName(), namespace, isClusterWideResource)
		// if err != nil {
		// 	return err
		// } else {
		// 	logger.Log.Info(fmt.Sprintf("Resource retrieved %s:%s", gvr.Resource, res.GetName()))
		// }
		// Try update if already exists
		_, err = client.Update(context.TODO(), serviceAccount, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		fmt.Println("Resource updated successfully")
	}
	return nil
}

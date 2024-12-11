package kubernetes

import (
	"context"

	core "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ApplyServiceAccount(serviceAccountName string, namespace string, annotations map[string]string) error {
	serviceAccount := &core.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceAccountName,
			Annotations: annotations,
		},
	}
	clientset := clientProvider.K8sClientSet()
	_, err := clientset.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), serviceAccount, MoCreateOptions())
	if err == nil {
		k8sLogger.Info("ServiceAccount created successfully ✅")
	} else {
		// Check if already exists
		serviceAccount, err := GetServiceAccount(serviceAccountName, namespace)
		if err != nil {
			return err
		} else {
			k8sLogger.Info("ServiceAccount retrieved", "namespace", serviceAccount.GetNamespace(), "name", serviceAccount.GetName())
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
	clientset := clientProvider.K8sClientSet()
	_, err := clientset.CoreV1().ServiceAccounts(serviceAccount.GetNamespace()).Update(context.TODO(), serviceAccount, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	k8sLogger.Info("ServiceAccount updated successfully ✅")
	return nil
}

func GetServiceAccount(serviceAccountName string, namespace string) (*core.ServiceAccount, error) {
	clientset := clientProvider.K8sClientSet()
	return clientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), serviceAccountName, metav1.GetOptions{})
}

func DeleteServiceAccount(serviceAccountName string, namespace string) error {
	clientset := clientProvider.K8sClientSet()
	err := clientset.CoreV1().ServiceAccounts(namespace).Delete(context.TODO(), serviceAccountName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	k8sLogger.Info("ServiceAccount deleted successfully ✅")
	return nil
}

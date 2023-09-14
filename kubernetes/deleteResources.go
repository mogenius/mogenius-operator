package kubernetes

import (
	"context"

	"mogenius-k8s-manager/logger"

	punq "github.com/mogenius/punq/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Remove() {
	provider := punq.NewKubeProvider(nil)
	if provider == nil {
		panic("error creating kubeprovider.")
	}

	// namespace is not deleted on purpose
	removeRbac(provider)
	removeDeployment(provider)
	// secret is not deleted on purpose
}

func removeDeployment(kubeProvider *punq.KubeProvider) {
	deploymentClient := kubeProvider.ClientSet.AppsV1().Deployments(NAMESPACE)

	// DELETE Deployment
	logger.Log.Info("Deleting mogenius-k8s-manager deployment ...")
	deletePolicy := metav1.DeletePropagationForeground
	err := deploymentClient.Delete(context.TODO(), DEPLOYMENTNAME, metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
	if err != nil {
		logger.Log.Error(err)
		return
	}
	logger.Log.Info("Deleted mogenius-k8s-manager deployment.")
}

func removeRbac(kubeProvider *punq.KubeProvider) {
	// CREATE RBAC
	logger.Log.Info("Deleting mogenius-k8s-manager RBAC ...")
	err := kubeProvider.ClientSet.CoreV1().ServiceAccounts(NAMESPACE).Delete(context.TODO(), SERVICEACCOUNTNAME, metav1.DeleteOptions{})
	if err != nil {
		logger.Log.Error(err)
		return
	}
	err = kubeProvider.ClientSet.RbacV1().ClusterRoles().Delete(context.TODO(), CLUSTERROLENAME, metav1.DeleteOptions{})
	if err != nil {
		logger.Log.Error(err)
		return
	}
	err = kubeProvider.ClientSet.RbacV1().ClusterRoleBindings().Delete(context.TODO(), CLUSTERROLEBINDINGNAME, metav1.DeleteOptions{})
	if err != nil {
		logger.Log.Error(err)
		return
	}
	logger.Log.Info("Deleted mogenius-k8s-manager RBAC.")
}

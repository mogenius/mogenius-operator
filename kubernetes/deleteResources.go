package kubernetes

import (
	"context"

	punq "github.com/mogenius/punq/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Remove() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		panic("error creating provider.")
	}

	// namespace is not deleted on purpose
	removeRbac(provider)
	removeDeployment(provider)
	// secret is not deleted on purpose
}

func removeDeployment(provider *punq.KubeProvider) {
	deploymentClient := provider.ClientSet.AppsV1().Deployments(NAMESPACE)

	// DELETE Deployment
	K8sLogger.Info("Deleting mogenius-k8s-manager deployment ...")
	deletePolicy := metav1.DeletePropagationForeground
	err := deploymentClient.Delete(context.TODO(), DEPLOYMENTNAME, metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
	if err != nil {
		K8sLogger.Errorf("Error deleting mogenius-k8s-manager deployment: %s", err.Error())
		return
	}
	K8sLogger.Info("Deleted mogenius-k8s-manager deployment.")
}

func removeRbac(provider *punq.KubeProvider) {
	// CREATE RBAC
	K8sLogger.Info("Deleting mogenius-k8s-manager RBAC ...")
	err := provider.ClientSet.CoreV1().ServiceAccounts(NAMESPACE).Delete(context.TODO(), SERVICEACCOUNTNAME, metav1.DeleteOptions{})
	if err != nil {
		K8sLogger.Errorf("Error deleting mogenius-k8s-manager ServiceAccounts: %s", err.Error())
		return
	}
	err = provider.ClientSet.RbacV1().ClusterRoles().Delete(context.TODO(), CLUSTERROLENAME, metav1.DeleteOptions{})
	if err != nil {
		K8sLogger.Errorf("Error deleting mogenius-k8s-manager ClosterRoles: %s", err.Error())
		return
	}
	err = provider.ClientSet.RbacV1().ClusterRoleBindings().Delete(context.TODO(), CLUSTERROLEBINDINGNAME, metav1.DeleteOptions{})
	if err != nil {
		K8sLogger.Errorf("Error deleting mogenius-k8s-manager ClusterRoleBindings: %s", err.Error())
		return
	}
	K8sLogger.Info("Deleted mogenius-k8s-manager RBAC.")
}

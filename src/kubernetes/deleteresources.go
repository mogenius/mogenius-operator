package kubernetes

import (
	"context"
	"mogenius-k8s-manager/src/shutdown"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Remove() {
	provider, err := NewKubeProvider()
	if provider == nil || err != nil {
		k8sLogger.Error("error creating provider.", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}

	// namespace is not deleted on purpose
	removeRbac(provider)
	removeDeployment(provider)
	// secret is not deleted on purpose
}

func removeDeployment(provider *KubeProvider) {
	deploymentClient := provider.ClientSet.AppsV1().Deployments(config.Get("MO_OWN_NAMESPACE"))

	// DELETE Deployment
	k8sLogger.Info("Deleting mogenius-k8s-manager deployment ...")
	deletePolicy := metav1.DeletePropagationForeground
	err := deploymentClient.Delete(context.TODO(), DEPLOYMENTNAME, metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
	if err != nil {
		k8sLogger.Error("Error deleting mogenius-k8s-manager deployment", "error", err.Error())
		return
	}
	k8sLogger.Info("Deleted mogenius-k8s-manager deployment.")
}

func removeRbac(provider *KubeProvider) {
	// CREATE RBAC
	k8sLogger.Info("Deleting mogenius-k8s-manager RBAC ...")
	err := provider.ClientSet.CoreV1().ServiceAccounts(config.Get("MO_OWN_NAMESPACE")).Delete(context.TODO(), SERVICEACCOUNTNAME, metav1.DeleteOptions{})
	if err != nil {
		k8sLogger.Error("Error deleting mogenius-k8s-manager ServiceAccounts", "error", err)
		return
	}
	err = provider.ClientSet.RbacV1().ClusterRoles().Delete(context.TODO(), CLUSTERROLENAME, metav1.DeleteOptions{})
	if err != nil {
		k8sLogger.Error("Error deleting mogenius-k8s-manager ClosterRoles", "error", err)
		return
	}
	err = provider.ClientSet.RbacV1().ClusterRoleBindings().Delete(context.TODO(), CLUSTERROLEBINDINGNAME, metav1.DeleteOptions{})
	if err != nil {
		k8sLogger.Error("Error deleting mogenius-k8s-manager ClusterRoleBindings", "error", err)
		return
	}
	k8sLogger.Info("Deleted mogenius-k8s-manager RBAC.")
}

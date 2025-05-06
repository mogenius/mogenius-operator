package kubernetes

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Remove() {
	// namespace is not deleted on purpose
	removeRbac()
	removeDeployment()
	// secret is not deleted on purpose
}

func removeDeployment() {
	clientset := clientProvider.K8sClientSet()
	deploymentClient := clientset.AppsV1().Deployments(config.Get("MO_OWN_NAMESPACE"))

	// DELETE Deployment
	k8sLogger.Info("Deleting mogenius-k8s-manager deployment ...")
	deletePolicy := metav1.DeletePropagationForeground
	err := deploymentClient.Delete(context.TODO(), GetOwnDeploymentName(config), metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
	if err != nil {
		k8sLogger.Error("Error deleting mogenius-k8s-manager deployment", "error", err.Error())
		return
	}
	k8sLogger.Info("Deleted mogenius-k8s-manager deployment.")
}

func removeRbac() {
	clientset := clientProvider.K8sClientSet()

	// CREATE RBAC
	k8sLogger.Info("Deleting mogenius-k8s-manager RBAC ...")
	err := clientset.CoreV1().ServiceAccounts(config.Get("MO_OWN_NAMESPACE")).Delete(context.TODO(), SERVICEACCOUNTNAME, metav1.DeleteOptions{})
	if err != nil {
		k8sLogger.Error("Error deleting mogenius-k8s-manager ServiceAccounts", "error", err)
		return
	}
	err = clientset.RbacV1().ClusterRoles().Delete(context.TODO(), CLUSTERROLENAME, metav1.DeleteOptions{})
	if err != nil {
		k8sLogger.Error("Error deleting mogenius-k8s-manager ClosterRoles", "error", err)
		return
	}
	err = clientset.RbacV1().ClusterRoleBindings().Delete(context.TODO(), CLUSTERROLEBINDINGNAME, metav1.DeleteOptions{})
	if err != nil {
		k8sLogger.Error("Error deleting mogenius-k8s-manager ClusterRoleBindings", "error", err)
		return
	}
	k8sLogger.Info("Deleted mogenius-k8s-manager RBAC.")
}

package kubernetes

import (
	"context"
	"os/exec"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	v1 "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllClusterRoleBindings(namespaceName string) []v1.ClusterRoleBinding {
	result := []v1.ClusterRoleBinding{}

	provider := NewKubeProvider()
	rolesList, err := provider.ClientSet.RbacV1().ClusterRoleBindings().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Log.Errorf("AllClusterRoleBindings ERROR: %s", err.Error())
		return result
	}

	for _, role := range rolesList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, role.ObjectMeta.Namespace) {
			result = append(result, role)
		}
	}
	return result
}

func UpdateK8sClusterRoleBinding(data v1.ClusterRoleBinding) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	roleClient := kubeProvider.ClientSet.RbacV1().ClusterRoleBindings()
	_, err := roleClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DeleteK8sClusterRoleBinding(data v1.ClusterRoleBinding) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	roleClient := kubeProvider.ClientSet.RbacV1().ClusterRoleBindings()
	err := roleClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DescribeK8sClusterRoleBinding(name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "clusterrolebinding", name)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		return WorkloadResult(err.Error())
	}
	return WorkloadResult(string(output))
}

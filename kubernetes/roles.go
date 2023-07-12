package kubernetes

import (
	"context"
	"os/exec"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	v1 "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllRoleBindings(namespaceName string) K8sWorkloadResult {
	result := []v1.RoleBinding{}

	provider := NewKubeProvider()
	rolesList, err := provider.ClientSet.RbacV1().RoleBindings(namespaceName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Log.Errorf("AllRoleBindings ERROR: %s", err.Error())
		return WorkloadResult(nil, err)
	}

	for _, roleBinding := range rolesList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, roleBinding.ObjectMeta.Namespace) {
			result = append(result, roleBinding)
		}
	}
	return WorkloadResult(result, nil)
}

func UpdateK8sRoleBinding(data v1.RoleBinding) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	roleClient := kubeProvider.ClientSet.RbacV1().RoleBindings(data.Namespace)
	_, err := roleClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DeleteK8sRoleBinding(data v1.RoleBinding) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	roleClient := kubeProvider.ClientSet.RbacV1().RoleBindings(data.Namespace)
	err := roleClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DescribeK8sRole(namespace string, name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "role", name, "-n", namespace)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		logger.Log.Errorf("Error: %s", string(output))
		return WorkloadResult(nil, string(output))
	}
	return WorkloadResult(string(output), nil)
}

package kubernetes

import (
	"context"
	"os/exec"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	v1 "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllRoles(namespaceName string) K8sWorkloadResult {
	result := []v1.Role{}

	provider := NewKubeProvider()
	rolesList, err := provider.ClientSet.RbacV1().Roles(namespaceName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Log.Errorf("AllRoles ERROR: %s", err.Error())
		return WorkloadResult(nil, err)
	}

	for _, role := range rolesList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, role.ObjectMeta.Namespace) {
			result = append(result, role)
		}
	}
	return WorkloadResult(result, nil)
}

func UpdateK8sRole(data v1.Role) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	roleClient := kubeProvider.ClientSet.RbacV1().Roles(data.Namespace)
	_, err := roleClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DeleteK8sRole(data v1.Role) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	roleClient := kubeProvider.ClientSet.RbacV1().Roles(data.Namespace)
	err := roleClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DescribeK8sRoleBinding(namespace string, name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "rolebinding", name, "-n", namespace)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		logger.Log.Errorf("Error: %s", string(output))
		return WorkloadResult(nil, string(output))
	}
	return WorkloadResult(string(output), nil)
}

func NewK8sRoleBinding() K8sNewWorkload {
	return NewWorkload(
		utils.InitRoleBindingYaml(),
		"RoleBindings in Kubernetes provide a way to bind a Role or ClusterRole to a set of subjects (which can be Users, Groups, or ServiceAccounts) within a particular namespace. In this example, a RoleBinding named 'read-pods' is created in the 'default' namespace. This RoleBinding binds the Role 'pod-reader' to the user 'jane'.")
}

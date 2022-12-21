package kubernetes

import (
	"context"
	"os"

	"mogenius-k8s-manager/logger"

	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyconfapp "k8s.io/client-go/applyconfigurations/apps/v1"
	applyconfcore "k8s.io/client-go/applyconfigurations/core/v1"
	applyconfmeta "k8s.io/client-go/applyconfigurations/meta/v1"
)

func Deploy() {
	provider, err := NewKubeProviderLocal()
	if err != nil {
		panic(err)
	}

	applyNamespace(provider)
	addRbac(provider)
	addDeployment(provider)
}

func addRbac(kubeProvider *KubeProvider) error {
	serviceAccount := &core.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: SERVICEACCOUNTNAME,
		},
	}
	clusterRole := &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: CLUSTERROLENAME,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{"", "extensions", "apps"},
				Resources: RBACRESOURCES,
				Verbs:     []string{"list", "get", "watch"},
			},
		},
	}
	clusterRoleBinding := &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: CLUSTERROLEBINDINGNAME,
		},
		RoleRef: rbac.RoleRef{
			Name:     CLUSTERROLENAME,
			Kind:     "ClusterRole",
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      SERVICEACCOUNTNAME,
				Namespace: NAMESPACE,
			},
		},
	}

	// CREATE RBAC
	logger.Log.Info("Creating mogenius-k8s-manager RBAC ...")
	_, err := kubeProvider.ClientSet.CoreV1().ServiceAccounts(NAMESPACE).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	_, err = kubeProvider.ClientSet.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	_, err = kubeProvider.ClientSet.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	logger.Log.Info("Created mogenius-k8s-manager RBAC.")
	return nil
}

func applyNamespace(kubeProvider *KubeProvider) {
	serviceClient := kubeProvider.ClientSet.CoreV1().Namespaces()

	namespace := applyconfcore.Namespace(NAMESPACE)

	applyOptions := metav1.ApplyOptions{
		Force:        true,
		FieldManager: DEPLOYMENTNAME,
	}

	logger.Log.Info("Creating mogenius-k8s-manager namespace ...")
	result, err := serviceClient.Apply(context.TODO(), namespace, applyOptions)
	if err != nil {
		logger.Log.Error(err)
	}
	logger.Log.Info("Created mogenius-k8s-manager namespace", result.GetObjectMeta().GetName(), ".")
}

func addDeployment(kubeProvider *KubeProvider) {
	deploymentClient := kubeProvider.ClientSet.AppsV1().Deployments(NAMESPACE)

	deploymentContainer := applyconfcore.Container()
	deploymentContainer.WithImagePullPolicy(core.PullAlways)
	deploymentContainer.WithName(DEPLOYMENTNAME)
	deploymentContainer.WithImage(DEPLOYMENTIMAGE)
	deploymentContainer.WithEnv(
		applyconfcore.EnvVar().WithName("STAGE").WithValue(os.Getenv("STAGE")),
	)
	agentResourceLimits := core.ResourceList{
		"cpu":               resource.MustParse("300m"),
		"memory":            resource.MustParse("256Mi"),
		"ephemeral-storage": resource.MustParse("100Mi"),
	}
	agentResourceRequests := core.ResourceList{
		"cpu":               resource.MustParse("100m"),
		"memory":            resource.MustParse("128Mi"),
		"ephemeral-storage": resource.MustParse("10Mi"),
	}
	agentResources := applyconfcore.ResourceRequirements().WithRequests(agentResourceRequests).WithLimits(agentResourceLimits)
	deploymentContainer.WithResources(agentResources)
	deploymentContainer.WithName(DEPLOYMENTNAME)

	podSpec := applyconfcore.PodSpec()
	podSpec.WithTerminationGracePeriodSeconds(0)
	podSpec.WithServiceAccountName(SERVICEACCOUNTNAME)

	podSpec.WithContainers(deploymentContainer)

	applyOptions := metav1.ApplyOptions{
		Force:        true,
		FieldManager: DEPLOYMENTNAME,
	}

	labelSelector := applyconfmeta.LabelSelector()
	labelSelector.WithMatchLabels(map[string]string{"app": DEPLOYMENTNAME})

	podTemplate := applyconfcore.PodTemplateSpec()
	podTemplate.WithLabels(map[string]string{
		"app": DEPLOYMENTNAME,
	})
	podTemplate.WithSpec(podSpec)

	deployment := applyconfapp.Deployment(DEPLOYMENTNAME, NAMESPACE)
	deployment.WithSpec(applyconfapp.DeploymentSpec().WithSelector(labelSelector).WithTemplate(podTemplate))

	// Create Deployment
	logger.Log.Info("Creating mogenius-k8s-manager deployment ...")
	result, err := deploymentClient.Apply(context.TODO(), deployment, applyOptions)
	if err != nil {
		logger.Log.Error(err)
	}
	logger.Log.Info("Created mogenius-k8s-manager deployment.", result.GetObjectMeta().GetName(), ".")
}

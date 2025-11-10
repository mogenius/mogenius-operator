package kubernetes

import (
	"context"
	"fmt"
	"mogenius-operator/src/assert"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/websocket"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

func ClusterForceReconnect() bool {
	// restart deployments/daemonsets for
	// - traffic
	// - podstats
	// - k8s-manager

	clientset := clientProvider.K8sClientSet()
	podClient := clientset.CoreV1().Pods(config.Get("MO_OWN_NAMESPACE"))

	podsToKill := []string{}
	podsToKill = append(podsToKill, AllPodNamesForLabel(config.Get("MO_OWN_NAMESPACE"), "app", GetOwnDeploymentName(config))...)

	for _, podName := range podsToKill {
		k8sLogger.Warn("Restarting pod ...", "podName", podName)
		err := podClient.Delete(context.Background(), podName, metav1.DeleteOptions{})
		if err != nil {
			k8sLogger.Error("failed to delete pod", "podName", podName, "error", err)
		}
	}

	return true
}

func ClusterForceDisconnect() bool {
	// restart deployments/daemonsets for
	// - traffic
	// - podstats
	// - k8s-manager

	clientset := clientProvider.K8sClientSet()
	podClient := clientset.CoreV1().Pods(config.Get("MO_OWN_NAMESPACE"))

	// stop k8s-manager
	deploymentClient := clientset.AppsV1().Deployments(config.Get("MO_OWN_NAMESPACE"))
	deployment, _ := deploymentClient.Get(context.Background(), GetOwnDeploymentName(config), metav1.GetOptions{})
	deployment.Spec.Replicas = utils.Pointer[int32](0)
	_, err := deploymentClient.Update(context.Background(), deployment, metav1.UpdateOptions{})
	if err != nil {
		k8sLogger.Error("Error updating deployment", "deployment", deployment, "error", err)
	}

	podsToKill := []string{}
	podsToKill = append(podsToKill, AllPodNamesForLabel(config.Get("MO_OWN_NAMESPACE"), "app", GetOwnDeploymentName(config))...)

	for _, podName := range podsToKill {
		k8sLogger.Warn("Restarting pod...", "pod", podName)
		err := podClient.Delete(context.Background(), podName, metav1.DeleteOptions{})

		if err != nil {
			k8sLogger.Error("failed to delete pod", "pod", podName, "error", err)
		}
	}

	return true
}

func UpgradeMyself(eventClient websocket.WebsocketClient, job *structs.Job, command string) {
	cmd := structs.CreateCommand(eventClient, "upgrade operator", "Upgrade mogenius platform ...", job)
	cmd.Start(eventClient, job, "Upgrade mogenius platform ...")

	clientset := clientProvider.K8sClientSet()
	jobClient := clientset.BatchV1().Jobs(config.Get("MO_OWN_NAMESPACE"))
	configmapClient := clientset.CoreV1().ConfigMaps(config.Get("MO_OWN_NAMESPACE"))

	ownerReference, err := GetOwnDeploymentOwnerReference(clientset, config)
	if err != nil {
		k8sLogger.Error("Error getting owner reference for upgrade job", "error", err)
	}

	configmap := utils.InitUpgradeConfigMap()
	configmap.Namespace = config.Get("MO_OWN_NAMESPACE")
	configmap.Data["values.command"] = command
	configmap.OwnerReferences = ownerReference

	k8sjob := utils.InitUpgradeJob()
	k8sjob.Namespace = config.Get("MO_OWN_NAMESPACE")
	k8sjob.Name = fmt.Sprintf("%s-%s", k8sjob.Name, utils.NanoIdSmallLowerCase())
	k8sjob.OwnerReferences = ownerReference

	// CONFIGMAP
	_, err = configmapClient.Get(context.Background(), configmap.Name, metav1.GetOptions{})
	if err != nil {
		// CREATE
		_, err = configmapClient.Create(context.Background(), &configmap, MoCreateOptions(config))
		if err != nil {
			cmd.Fail(eventClient, job, fmt.Sprintf("UpgradeMyself (configmap) ERROR: %s", err.Error()))
			return
		}
	} else {
		// UPDATE
		_, err = configmapClient.Update(context.Background(), &configmap, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(eventClient, job, fmt.Sprintf("UpgradeMyself (update_configmap) ERROR: %s", err.Error()))
			return
		}
	}

	// JOB
	_, err = jobClient.Get(context.Background(), k8sjob.Name, metav1.GetOptions{})
	if err != nil {
		// CREATE
		_, err = jobClient.Create(context.Background(), &k8sjob, MoCreateOptions(config))
		if err != nil {
			cmd.Fail(eventClient, job, fmt.Sprintf("UpgradeMyself (job) ERROR: %s", err.Error()))
			return
		}
	} else {
		// UPDATE
		_, err = jobClient.Update(context.Background(), &k8sjob, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(eventClient, job, fmt.Sprintf("UpgradeMyself (update_job) ERROR: %s", err.Error()))
			return
		}
	}
	cmd.Success(eventClient, job, "Upgraded platform successfully.")
}

func GetOwnDeploymentOwnerReference(clientset *kubernetes.Clientset, config cfg.ConfigModule) ([]metav1.OwnerReference, error) {
	ownDeploymentName := config.Get("OWN_DEPLOYMENT_NAME")
	assert.Assert(ownDeploymentName != "")

	namespace := config.Get("MO_OWN_NAMESPACE")
	assert.Assert("MO_OWN_NAMESPACE" != "")

	ownDeployment := store.GetDeployment(namespace, ownDeploymentName) // to cache it
	if ownDeployment == nil {
		return nil, fmt.Errorf("failed to find own deployment %s in namespace %s", ownDeploymentName, namespace)
	}
	reference := []metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       ownDeployment.GetName(),
			UID:        ownDeployment.GetUID(),
			Controller: ptr.To(true),
		},
	}

	return reference, nil
}

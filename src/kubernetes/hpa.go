package kubernetes

import (
	"context"
	"fmt"

	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/websocket"

	v2 "k8s.io/api/autoscaling/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	HpaNameSuffix = "-hpa"
)

func HandleHpa(eventClient websocket.WebsocketClient, job *structs.Job, namespaceName, controllerName string, service dtos.K8sServiceDto) {
	if service.HpaEnabled() {
		CreateOrUpdateHpa(eventClient, job, namespaceName, service.ControllerName, service.HpaSettings)
	} else {
		hpa, error := GetHpa(namespaceName, service.ControllerName+HpaNameSuffix)
		if error == nil && hpa.DeletionTimestamp == nil {
			DeleteHpa(eventClient, job, namespaceName, service.ControllerName)
		}
	}
}

func DeleteHpa(eventClient websocket.WebsocketClient, job *structs.Job, namespaceName, controllerName string) {
	cmd := structs.CreateCommand(eventClient, "delete", fmt.Sprintf("Delete hpa '%s'.", controllerName+HpaNameSuffix), job)
	cmd.Start(eventClient, job, "Delete hpa")

	err := DeleteK8sHpaBy(namespaceName, controllerName+HpaNameSuffix)
	if err != nil {
		cmd.Fail(eventClient, job, fmt.Sprintf("Deleting hpa ERROR: '%s'", err.Error()))
	} else {
		cmd.Success(eventClient, job, "Deleted hpa")
	}
}

func GetHpa(namespaceName string, name string) (*v2.HorizontalPodAutoscaler, error) {
	clientset := clientProvider.K8sClientSet()
	hpa, err := clientset.AutoscalingV2().HorizontalPodAutoscalers(namespaceName).Get(context.Background(), name, metav1.GetOptions{})
	hpa.Kind = "HorizontalPodAutoscaler"
	hpa.APIVersion = "autoscaling/v2"

	return hpa, err
}

func DeleteK8sHpaBy(namespace string, name string) error {
	clientset := clientProvider.K8sClientSet()
	client := clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace)
	return client.Delete(context.Background(), name, metav1.DeleteOptions{})
}

func CreateHpa(namespaceName, controllerName string, hpaSettings *dtos.K8sHpaSettingsDto) (*v2.HorizontalPodAutoscaler, error) {
	deployment := store.GetDeployment(namespaceName, controllerName)
	if deployment == nil {
		return nil, fmt.Errorf("cannot create hpa, deployment not found")
	}

	meta := &metav1.ObjectMeta{
		Name:      controllerName + HpaNameSuffix,
		Namespace: namespaceName,
		Labels: map[string]string{
			"app": controllerName,
		},
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       controllerName,
				UID:        deployment.UID,
			},
		},
	}

	hpa := &v2.HorizontalPodAutoscaler{
		ObjectMeta: *meta,
		Spec:       *hpaSettings.HorizontalPodAutoscalerSpec,
	}

	hpa.Spec.ScaleTargetRef = v2.CrossVersionObjectReference{
		Kind:       "Deployment",
		Name:       controllerName,
		APIVersion: "apps/v1",
	}

	return hpa, nil
}

func CreateOrUpdateHpa(eventClient websocket.WebsocketClient, job *structs.Job, namespaceName, controllerName string, hpaSettings *dtos.K8sHpaSettingsDto) {
	if hpaSettings == nil {
		k8sLogger.Warn("CreateOrUpdate hpa warning: hpaSettings is nil")
		return
	}

	cmd := structs.CreateCommand(eventClient, "CreateOrUpdate", "CreateOrUpdate hpa", job)
	cmd.Start(eventClient, job, "CreateOrUpdate hpa")

	clientset := clientProvider.K8sClientSet()

	hpaClient := clientset.AutoscalingV2().HorizontalPodAutoscalers(namespaceName)
	newHpa, err := CreateHpa(namespaceName, controllerName, hpaSettings)
	if err != nil {
		cmd.Fail(eventClient, job, fmt.Sprintf("Creating hpa ERROR: %s", err.Error()))
		return
	}

	_, err = hpaClient.Update(context.Background(), newHpa, MoUpdateOptions(config))
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = hpaClient.Create(context.Background(), newHpa, MoCreateOptions(config))
			if err != nil {
				cmd.Fail(eventClient, job, fmt.Sprintf("Creating hpa ERROR: %s", err.Error()))
			} else {
				cmd.Success(eventClient, job, "Created hpa")
			}
		} else {
			cmd.Fail(eventClient, job, fmt.Sprintf("Updating hpa ERROR: %s", err.Error()))
		}
	} else {
		cmd.Success(eventClient, job, "Updated hpa")
	}
}

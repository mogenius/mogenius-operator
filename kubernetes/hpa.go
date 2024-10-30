package kubernetes

import (
	"context"
	"fmt"
	"sync"

	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"

	punq "github.com/mogenius/punq/kubernetes"
	v2 "k8s.io/api/autoscaling/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	HpaNameSuffix = "-hpa"
)

func HandleHpa(job *structs.Job, namespaceName, controllerName string, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	if service.HpaEnabled() {
		CreateOrUpdateHpa(job, namespaceName, service.ControllerName, service.HpaSettings, wg)
	} else {
		hpa, error := punq.GetHpa(namespaceName, service.ControllerName+HpaNameSuffix, nil)
		if error == nil && hpa.DeletionTimestamp == nil {
			DeleteHpa(job, namespaceName, service.ControllerName, wg)
		}
	}
}

func DeleteHpa(job *structs.Job, namespaceName, controllerName string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", fmt.Sprintf("Delete hpa '%s'.", controllerName+HpaNameSuffix), job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Delete hpa")

		err := punq.DeleteK8sHpaBy(namespaceName, controllerName+HpaNameSuffix, nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("Deleting hpa ERROR: '%s'", err.Error()))
		} else {
			cmd.Success(job, "Deleted hpa")
		}
	}(wg)
}

func CreateHpa(namespaceName, controllerName string, hpaSettings *dtos.K8sHpaSettingsDto) (*v2.HorizontalPodAutoscaler, error) {
	deployment, err := punq.GetK8sDeployment(namespaceName, controllerName, nil)
	if err != nil || deployment == nil {
		return nil, fmt.Errorf("Cannot create hpa, Deployment not found")
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

func CreateOrUpdateHpa(job *structs.Job, namespaceName, controllerName string, hpaSettings *dtos.K8sHpaSettingsDto, wg *sync.WaitGroup) {
	if hpaSettings == nil {
		K8sLogger.Warn("CreateOrUpdate hpa warning: hpaSettings is nil")
		return
	}

	cmd := structs.CreateCommand("CreateOrUpdate", "CreateOrUpdate hpa", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "CreateOrUpdate hpa")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("Creating hpa ERROR: %s", err.Error()))
			return
		}

		hpaClient := provider.ClientSet.AutoscalingV2().HorizontalPodAutoscalers(namespaceName)
		newHpa, err := CreateHpa(namespaceName, controllerName, hpaSettings)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("Creating hpa ERROR: %s", err.Error()))
			return
		}

		_, err = hpaClient.Update(context.TODO(), newHpa, MoUpdateOptions())
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = hpaClient.Create(context.TODO(), newHpa, MoCreateOptions())
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("Creating hpa ERROR: %s", err.Error()))
				} else {
					cmd.Success(job, "Created hpa")
				}
			} else {
				cmd.Fail(job, fmt.Sprintf("Updating hpa ERROR: %s", err.Error()))
			}
		} else {
			cmd.Success(job, "Updated hpa")
		}
	}(wg)
}

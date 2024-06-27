package kubernetes

import (
	"context"
	"fmt"
	"sync"

	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"

	punq "github.com/mogenius/punq/kubernetes"
	log "github.com/sirupsen/logrus"
	v2 "k8s.io/api/autoscaling/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteHpa(job *structs.Job, name, namespace string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", fmt.Sprintf("Delete hpa '%s' in '%s'.", name, namespace), job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Delete hpa")

		punq.DeleteK8sHpaBy(namespace, name, nil)
	}(wg)
}

func CreateHpa(hpaSettings *dtos.K8sHpaSettingsDto) (*v2.HorizontalPodAutoscaler, error) {
	deployment, err := punq.GetK8sDeployment(hpaSettings.Namespace, hpaSettings.Name, nil)
	if err != nil || deployment == nil {
		return nil, fmt.Errorf("Cannot create HPA, Deployment not found")
	}

	meta := &metav1.ObjectMeta{
		Name:      hpaSettings.Name + "-hpa",
		Namespace: hpaSettings.Namespace,
		Labels: map[string]string{
			"app": hpaSettings.Name,
		},
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       hpaSettings.Name,
				UID:        deployment.UID,
			},
		},
	}

	hpa := &v2.HorizontalPodAutoscaler{
		ObjectMeta: *meta,
		Spec:       hpaSettings.Data.Spec,
	}

	return hpa, nil
}

func CreateOrUpdateHpa(job *structs.Job, hpaSettings *dtos.K8sHpaSettingsDto, wg *sync.WaitGroup) {
	if hpaSettings == nil {
		log.Warningf("CreateOrUpdateHpa warning: hpaSettings is nil")
		return
	}

	cmd := structs.CreateCommand("CreateOrUpdate", "Hpa", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "CreateOrUpdate Hpa")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}

		hpaClient := provider.ClientSet.AutoscalingV2().HorizontalPodAutoscalers(hpaSettings.Name)
		newHpa, err := CreateHpa(hpaSettings)
		if err != nil {
			log.Errorf("error: %s", err.Error())
		}

		_, err = hpaClient.Update(context.TODO(), newHpa, MoUpdateOptions())
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = hpaClient.Create(context.TODO(), newHpa, MoCreateOptions())
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("CreateOrUpdate ERROR: %s", err.Error()))
				} else {
					cmd.Success(job, "Created Hpa")
				}
			} else {
				cmd.Fail(job, fmt.Sprintf("Updating Hpa ERROR: %s", err.Error()))
			}
		} else {
			cmd.Success(job, "Updating Hpa")
		}
	}(wg)
}

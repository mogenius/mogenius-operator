package kubernetes

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mogenius-k8s-manager/dtos"
	iacmanager "mogenius-k8s-manager/iac-manager"
	"mogenius-k8s-manager/structs"

	punq "github.com/mogenius/punq/kubernetes"
	log "github.com/sirupsen/logrus"
	v2 "k8s.io/api/autoscaling/v2"
	v1Core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

const (
	hpaNameSuffix = "-hpa"
)

func DeleteHpa(job *structs.Job, name, namespace string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", fmt.Sprintf("Delete hpa '%s' in '%s'.", name, namespace), job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Delete hpa")

		punq.DeleteK8sHpaBy(namespace, name+hpaNameSuffix, nil)
	}(wg)
}

func CreateHpa(hpaSettings *dtos.K8sHpaSettingsDto) (*v2.HorizontalPodAutoscaler, error) {
	deployment, err := punq.GetK8sDeployment(hpaSettings.Namespace, hpaSettings.Name, nil)
	if err != nil || deployment == nil {
		return nil, fmt.Errorf("Cannot create HPA, Deployment not found")
	}

	meta := &metav1.ObjectMeta{
		Name:      hpaSettings.Name + hpaNameSuffix,
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

func WatchHpas() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		log.Fatalf("Error creating provider for watcher. Cannot continue because it is vital: %s", err.Error())
		return
	}

	// Retry watching resources with exponential backoff in case of failures
	err = retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		return watchHpas(provider, dtos.KindHorizontalPodAutoscalers)
	})
	if err != nil {
		log.Fatalf("Error watching cronjobs: %s", err.Error())
	}

	// Wait forever
	select {}
}

func watchHpas(provider *punq.KubeProvider, kindName string) error {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			castedObj := obj.(*v2.HorizontalPodAutoscaler)
			castedObj.Kind = "HorizontalPodAutoscaler"
			castedObj.APIVersion = "autoscaling/v2"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			castedObj := newObj.(*v2.HorizontalPodAutoscaler)
			castedObj.Kind = "HorizontalPodAutoscaler"
			castedObj.APIVersion = "autoscaling/v2"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		DeleteFunc: func(obj interface{}) {
			castedObj := obj.(*v2.HorizontalPodAutoscaler)
			castedObj.Kind = "HorizontalPodAutoscaler"
			castedObj.APIVersion = "autoscaling/v2"
			iacmanager.DeleteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, obj)
		},
	}
	listWatch := cache.NewListWatchFromClient(
		provider.ClientSet.AutoscalingV2().RESTClient(),
		kindName,
		v1Core.NamespaceAll,
		fields.Nothing(),
	)
	resourceInformer := cache.NewSharedInformer(listWatch, &v2.HorizontalPodAutoscaler{}, 0)
	_, err := resourceInformer.AddEventHandler(handler)
	if err != nil {
		return err
	}

	stopCh := make(chan struct{})
	go resourceInformer.Run(stopCh)

	// Wait for the informer to sync and start processing events
	if !cache.WaitForCacheSync(stopCh, resourceInformer.HasSynced) {
		return fmt.Errorf("failed to sync cache")
	}

	// This loop will keep the function alive as long as the stopCh is not closed
	for {
		select {
		case <-stopCh:
			// stopCh closed, return from the function
			return nil
		case <-time.After(30 * time.Second):
			// This is to avoid a tight loop in case stopCh is never closed.
			// You can adjust the time as per your needs.
		}
	}
}

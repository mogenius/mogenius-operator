package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	iacmanager "mogenius-k8s-manager/iac-manager"
	"mogenius-k8s-manager/structs"
	"sync"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	v1Core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	applyconfcore "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

func CreateNamespace(job *structs.Job, project dtos.K8sProjectDto, namespace dtos.K8sNamespaceDto) *structs.Command {
	cmd := structs.CreateCommand("Create Kubernetes namespace", job)
	cmd.Start(fmt.Sprintf("Creating namespace '%s'.", namespace.Name))

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
		return cmd
	}
	namespaceClient := provider.ClientSet.CoreV1().Namespaces()
	newNamespace := applyconfcore.Namespace(namespace.Name)

	applyOptions := metav1.ApplyOptions{
		Force:        true,
		FieldManager: DEPLOYMENTNAME,
	}

	newNamespace.WithLabels(map[string]string{
		"name": namespace.Name,
	})

	_, err = namespaceClient.Apply(context.TODO(), newNamespace, applyOptions)
	if err != nil {
		cmd.Fail(fmt.Sprintf("CreateNamespace ERROR: %s", err.Error()))
	} else {
		cmd.Success(fmt.Sprintf("Created namespace '%s'.", *newNamespace.Name))
	}
	return cmd
}

func DeleteNamespace(job *structs.Job, namespace dtos.K8sNamespaceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete Kubernetes namespace", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting namespace '%s'.", namespace.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		namespaceClient := provider.ClientSet.CoreV1().Namespaces()

		err = namespaceClient.Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteNamespace ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted namespace '%s'.", namespace.Name))
		}
	}(cmd, wg)
	return cmd
}

func WatchNamespaces() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		log.Fatalf("Error creating provider for watcher. Cannot continue because it is vital: %s", err.Error())
		return
	}

	// Retry watching resources with exponential backoff in case of failures
	retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		return watchNamespaces(provider, "namespaces")
	})

	// Wait forever
	select {}
}

func watchNamespaces(provider *punq.KubeProvider, kindName string) error {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			castedObj := obj.(*v1.Namespace)
			castedObj.Kind = "Namespace"
			castedObj.APIVersion = "v1"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			castedObj := newObj.(*v1.Namespace)
			castedObj.Kind = "Namespace"
			castedObj.APIVersion = "v1"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		DeleteFunc: func(obj interface{}) {
			castedObj := obj.(*v1.Namespace)
			castedObj.Kind = "Namespace"
			castedObj.APIVersion = "v1"
			iacmanager.DeleteResourceYaml(kindName, castedObj.Namespace, castedObj.Name)
		},
	}
	listWatch := cache.NewListWatchFromClient(
		provider.ClientSet.CoreV1().RESTClient(),
		kindName,
		v1Core.NamespaceAll,
		fields.Nothing(),
	)
	resourceInformer := cache.NewSharedInformer(listWatch, &v1.Namespace{}, 0)
	resourceInformer.AddEventHandler(handler)

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

package kubernetes

import (
	"context"
	"encoding/base64"
	"fmt"
	"mogenius-k8s-manager/dtos"
	iacmanager "mogenius-k8s-manager/iac-manager"
	"mogenius-k8s-manager/structs"
	"sync"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	v1Core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

func CreateSecret(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Create Kubernetes secret", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating secret")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		secretClient := provider.ClientSet.CoreV1().Secrets(namespace.Name)
		secret := punqUtils.InitSecret()
		secret.ObjectMeta.Name = service.ControllerName
		secret.ObjectMeta.Namespace = namespace.Name
		delete(secret.StringData, "PRIVATE_KEY") // delete example data

		for _, container := range service.Containers {
			for _, envVar := range container.EnvVars {
				if envVar.Type == "KEY_VAULT" ||
					envVar.Type == "PLAINTEXT" ||
					envVar.Type == "HOSTNAME" {
					secret.StringData[envVar.Name] = envVar.Value
				}
			}
		}

		secret.Labels = MoUpdateLabels(&secret.Labels, nil, nil, &service)

		_, err = secretClient.Create(context.TODO(), &secret, MoCreateOptions())
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("CreateSecret ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Created secret")
		}
	}(wg)
}

func DeleteSecret(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Delete Kubernetes secret", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting secret")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		secretClient := provider.ClientSet.CoreV1().Secrets(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: punqUtils.Pointer[int64](5),
		}

		err = secretClient.Delete(context.TODO(), service.ControllerName, deleteOptions)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteSecret ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Deleted secret")
		}
	}(wg)
}

func CreateOrUpdateContainerSecret(job *structs.Job, project dtos.K8sProjectDto, namespace dtos.K8sNamespaceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Create Container secret", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating Container secret")

		secretName := "container-secret-" + namespace.Name

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		secretClient := provider.ClientSet.CoreV1().Secrets(namespace.Name)

		secret := punqUtils.InitContainerSecret()
		secret.ObjectMeta.Name = secretName
		secret.ObjectMeta.Namespace = namespace.Name

		if project.ContainerRegistryUser == nil || project.ContainerRegistryPat == nil || project.ContainerRegistryUrl == nil {
			cmd.Fail(job, "ERROR: ContainerRegistryUser, ContainerRegistryPat & ContainerRegistryUrl cannot be nil.")
			return
		}

		authStr := fmt.Sprintf("%s:%s", *project.ContainerRegistryUser, *project.ContainerRegistryPat)
		authStrBase64 := base64.StdEncoding.EncodeToString([]byte(authStr))
		jsonData := fmt.Sprintf(`{"auths":{"%s":{"username":"%s","password":"%s","auth":"%s"}}}`, *project.ContainerRegistryUrl, *project.ContainerRegistryUser, *project.ContainerRegistryPat, authStrBase64)

		secretStringData := make(map[string]string)
		secretStringData[".dockerconfigjson"] = jsonData // base64.StdEncoding.EncodeToString([]byte(jsonData))
		secret.StringData = secretStringData

		secret.Labels = MoUpdateLabels(&secret.Labels, nil, nil, nil)

		// Check if exists
		_, err = secretClient.Update(context.TODO(), &secret, MoUpdateOptions())
		if err == nil {
			// UPDATED
			cmd.Success(job, "Created Container secret")
		} else {
			if apierrors.IsNotFound(err) {
				_, err = secretClient.Create(context.TODO(), &secret, MoCreateOptions())
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("CreateContainerSecret (create) ERROR: %s", err.Error()))
				} else {
					// CREATED
					cmd.Success(job, "Created Container secret")
				}
			} else {
				cmd.Fail(job, fmt.Sprintf("CreateContainerSecret ERROR: %s", err.Error()))
			}
		}
	}(wg)
}

func CreateOrUpdateContainerSecretForService(job *structs.Job, project dtos.K8sProjectDto, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	// DO NOT CREATE SECRET IF NO IMAGE REPO SECRET IS PROVIDED
	if service.GetImageRepoSecretDecryptValue() == nil {
		return
	}

	cmd := structs.CreateCommand("create", "Create Container secret for service", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating Container secret")

		secretName := "container-secret-service-" + service.ControllerName

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		secretClient := provider.ClientSet.CoreV1().Secrets(namespace.Name)

		secret := punqUtils.InitContainerSecret()
		secret.ObjectMeta.Name = secretName
		secret.ObjectMeta.Namespace = namespace.Name

		secretStringData := make(map[string]string)
		secretStringData[".dockerconfigjson"] = *service.GetImageRepoSecretDecryptValue()
		secret.StringData = secretStringData

		secret.Labels = MoUpdateLabels(&secret.Labels, nil, nil, nil)

		// Check if exists
		_, err = secretClient.Update(context.TODO(), &secret, MoUpdateOptions())
		if err == nil {
			// UPDATED
			cmd.Success(job, "Created Container secret")
		} else {
			if apierrors.IsNotFound(err) {
				_, err = secretClient.Create(context.TODO(), &secret, MoCreateOptions())
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("CreateContainerSecret (create) ERROR: %s", err.Error()))
				} else {
					// CREATED
					cmd.Success(job, "Created Container secret for service")
				}
			} else {
				cmd.Fail(job, fmt.Sprintf("CreateContainerSecret ERROR: %s", err.Error()))
			}
		}
	}(wg)
}

func DeleteContainerSecret(job *structs.Job, namespace dtos.K8sNamespaceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Delete Container secret", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting Container secret")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		secretClient := provider.ClientSet.CoreV1().Secrets(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: punqUtils.Pointer[int64](5),
		}

		err = secretClient.Delete(context.TODO(), "container-secret-"+namespace.Name, deleteOptions)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteContainerSecret ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Deleted Container secret")
		}
	}(wg)
}

func UpdateOrCreateSecrete(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("update", "Update Kubernetes secret", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Updating secret")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		secretClient := provider.ClientSet.CoreV1().Secrets(namespace.Name)
		secret := punqUtils.InitSecret()
		secret.ObjectMeta.Name = service.ControllerName
		secret.ObjectMeta.Namespace = namespace.Name
		delete(secret.StringData, "PRIVATE_KEY") // delete example data

		for _, container := range service.Containers {
			for _, envVar := range container.EnvVars {
				if envVar.Type == "KEY_VAULT" ||
					envVar.Type == "PLAINTEXT" ||
					envVar.Type == "HOSTNAME" {
					secret.StringData[envVar.Name] = envVar.Value
				}
			}
		}

		_, err = secretClient.Update(context.TODO(), &secret, MoUpdateOptions())
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = secretClient.Create(context.TODO(), &secret, MoCreateOptions())
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("CreateSecret ERROR: %s", err.Error()))
				} else {
					cmd.Success(job, "Created secret")
				}
			} else {
				cmd.Fail(job, fmt.Sprintf("UpdateSecret ERROR: %s", err.Error()))
			}
		} else {
			cmd.Success(job, "Update secret")
		}
	}(wg)
}

func ContainerSecretDoesExistForStage(namespace dtos.K8sNamespaceDto) bool {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		return false
	}
	secret, err := provider.ClientSet.CoreV1().Secrets(namespace.Name).Get(context.TODO(), "container-secret-"+namespace.Name, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return secret != nil
}

func WatchSecrets() {
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
		return watchSecrets(provider, "secrets")
	})

	// Wait forever
	select {}
}

func watchSecrets(provider *punq.KubeProvider, kindName string) error {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			castedObj := obj.(*v1.Secret)
			castedObj.Kind = "Secret"
			castedObj.APIVersion = "v1"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			castedObj := newObj.(*v1.Secret)
			castedObj.Kind = "Secret"
			castedObj.APIVersion = "v1"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		DeleteFunc: func(obj interface{}) {
			castedObj := obj.(*v1.Secret)
			castedObj.Kind = "Secret"
			castedObj.APIVersion = "v1"
			iacmanager.DeleteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, obj)
		},
	}
	listWatch := cache.NewListWatchFromClient(
		provider.ClientSet.CoreV1().RESTClient(),
		kindName,
		v1Core.NamespaceAll,
		fields.Nothing(),
	)
	resourceInformer := cache.NewSharedInformer(listWatch, &v1.Secret{}, 0)
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

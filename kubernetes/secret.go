package kubernetes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

const ClusterImagePullSecretName = "cluster-img-pull-sec"
const ContainerImagePullSecretName = "container-img-pull-sec"

func CreateSecret(namespace string, secret *v1.Secret) (*v1.Secret, error) {
	client := GetCoreClient()
	if secret == nil {
		var err error
		secret, err = exampleSecret(namespace)
		if err != nil {
			return nil, err
		}
	}

	return client.Secrets(namespace).Create(context.TODO(), secret, MoCreateOptions())
}

func exampleSecret(namespace string) (*v1.Secret, error) {
	jsonData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	jsonString, err := json.Marshal(jsonData)
	if err != nil {
		return nil, err
	}
	// encodedJson := base64.StdEncoding.EncodeToString(jsonString)
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "customer-blue-backend-project-vault-secret-list",
			Namespace: namespace,
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{
			"project001":      []byte(jsonString),
			"backend-project": []byte(jsonString),
		},
	}, nil

}

func GetDecodedSecret(secretName string, namespace string) (map[string]string, error) {
	client := GetCoreClient()
	secret, err := client.Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s in namespace %s: %w", secretName, namespace, err)
	}

	decodedData := make(map[string]string)
	for key, value := range secret.Data {
		decodedData[key] = string(value)
	}

	return decodedData, nil
}

// -----------------------------------------------------
// Cluster Image Pull Secret
// -----------------------------------------------------

func CreateOrUpdateClusterImagePullSecret(job *structs.Job, project dtos.K8sProjectDto, namespace dtos.K8sNamespaceDto, wg *sync.WaitGroup) {
	secretName := utils.ParseK8sName(fmt.Sprintf("%s-%s", ClusterImagePullSecretName, namespace.Name))

	// delete old secret
	// TODO: remove this after a while
	err := punq.DeleteK8sSecretBy(namespace.Name, "container-secret-"+namespace.Name, nil)
	if err != nil {
		K8sLogger.Error(err)
	}

	// DO NOT CREATE SECRET IF NO IMAGE REPO SECRET IS PROVIDED
	if project.ContainerRegistryUser == nil || project.ContainerRegistryPat == nil || project.ContainerRegistryUrl == nil {
		// delete if exists
		err := punq.DeleteK8sSecretBy(namespace.Name, secretName, nil)
		if err != nil {
			K8sLogger.Error(err)
		}
		return
	}

	cmd := structs.CreateCommand("create", "Create Cluster Image-Pull secret", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating Cluster Image-Pull secret")

		secretClient := GetCoreClient().Secrets(namespace.Name)

		secret := punqUtils.InitContainerSecret()
		secret.ObjectMeta.Name = secretName
		secret.ObjectMeta.Namespace = namespace.Name
		secret.Labels = MoUpdateLabels(&secret.Labels, nil, nil, nil)

		secretStringData := make(map[string]string)

		authStr := fmt.Sprintf("%s:%s", *project.ContainerRegistryUser, *project.ContainerRegistryPat)
		authStrBase64 := base64.StdEncoding.EncodeToString([]byte(authStr))
		jsonData := fmt.Sprintf(`{"auths":{"%s":{"username":"%s","password":"%s","auth":"%s"}}}`, *project.ContainerRegistryUrl, *project.ContainerRegistryUser, *project.ContainerRegistryPat, authStrBase64)
		secretStringData[".dockerconfigjson"] = jsonData // base64.StdEncoding.EncodeToString([]byte(jsonData))

		secret.StringData = secretStringData

		// Check if exists
		_, err := secretClient.Update(context.TODO(), &secret, MoUpdateOptions())
		if err == nil {
			// UPDATED
			cmd.Success(job, "Created Cluster Image-Pull secret")
		} else {
			if apierrors.IsNotFound(err) {
				_, err = secretClient.Create(context.TODO(), &secret, MoCreateOptions())
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("CreateOrUpdateClusterImagePullSecret (create) ERROR: %s", err.Error()))
				} else {
					// CREATED
					cmd.Success(job, "Created Cluster Image-Pull secret")
				}
			} else {
				cmd.Fail(job, fmt.Sprintf("CreateOrUpdateClusterImagePullSecret ERROR: %s", err.Error()))
			}
		}
	}(wg)
}

func ExistsClusterImagePullSecret(namespace string) bool {
	secretName := utils.ParseK8sName(fmt.Sprintf("%s-%s", ClusterImagePullSecretName, namespace))
	secret, err := GetCoreClient().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return secret != nil
}

// -----------------------------------------------------
// Container Image Pull Secret
// -----------------------------------------------------

func CreateOrUpdateContainerImagePullSecret(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	secretName := utils.ParseK8sName(fmt.Sprintf("%s-%s", ContainerImagePullSecretName, service.ControllerName))

	// delete old secret
	// TODO: remove this after a while
	err := punq.DeleteK8sSecretBy(namespace.Name, "container-secret-service-"+service.ControllerName, nil)
	if err != nil {
		K8sLogger.Error(err)
	}

	// DO NOT CREATE SECRET IF NO IMAGE REPO SECRET IS PROVIDED
	if service.GetImageRepoSecretDecryptValue() == nil {
		// delete if exists
		err := punq.DeleteK8sSecretBy(namespace.Name, secretName, nil)
		if err != nil {
			K8sLogger.Error(err)
		}
		return
	}

	cmd := structs.CreateCommand("create", "Create Container Image-Pull secret", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating Container Image-Pull secret")

		secretClient := GetCoreClient().Secrets(namespace.Name)

		secret := punqUtils.InitContainerSecret()
		secret.ObjectMeta.Name = secretName
		secret.ObjectMeta.Namespace = namespace.Name

		secretStringData := make(map[string]string)
		secretStringData[".dockerconfigjson"] = *service.GetImageRepoSecretDecryptValue()
		secret.StringData = secretStringData

		secret.Labels = MoUpdateLabels(&secret.Labels, nil, nil, nil)

		// Check if exists
		_, err := secretClient.Update(context.TODO(), &secret, MoUpdateOptions())
		if err == nil {
			// UPDATED
			cmd.Success(job, "Created Container Image-Pull secret")
		} else {
			if apierrors.IsNotFound(err) {
				_, err = secretClient.Create(context.TODO(), &secret, MoCreateOptions())
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("CreateOrUpdateContainerImagePullSecret (create) ERROR: %s", err.Error()))
				} else {
					// CREATED
					cmd.Success(job, "Created Container Image-Pull secret")
				}
			} else {
				cmd.Fail(job, fmt.Sprintf("CreateOrUpdateContainerImagePullSecret ERROR: %s", err.Error()))
			}
		}
	}(wg)
}

func DeleteContainerImagePullSecret(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	secretName := utils.ParseK8sName(fmt.Sprintf("%s-%s", ContainerImagePullSecretName, service.ControllerName))

	// delete old secret
	// TODO: remove this after a while
	err := punq.DeleteK8sSecretBy(namespace.Name, "container-secret-service-"+service.ControllerName, nil)
	if err != nil {
		K8sLogger.Error(err)
	}

	cmd := structs.CreateCommand("delete", "Delete Container secret", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting Container secret")

		secretClient := GetCoreClient().Secrets(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: punqUtils.Pointer[int64](5),
		}

		_, err := secretClient.Get(context.TODO(), secretName, metav1.GetOptions{})

		// ignore if not found
		if apierrors.IsNotFound(err) {
			cmd.Success(job, "Deleted Container secret")
			return
		} else if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteContainerSecret ERROR: %s", err.Error()))
			return
		}

		err = secretClient.Delete(context.TODO(), secretName, deleteOptions)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteContainerSecret ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Deleted Container secret")
		}
	}(wg)
}

// -----------------------------------------------------
// Service Secret
// -----------------------------------------------------

func UpdateOrCreateControllerSecret(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("update", "Update Kubernetes secret", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Updating secret")

		secretClient := GetCoreClient().Secrets(namespace.Name)
		secret := punqUtils.InitSecret()
		secret.ObjectMeta.Name = service.ControllerName
		secret.ObjectMeta.Namespace = namespace.Name
		delete(secret.StringData, "exampleData") // delete example data

		for _, container := range service.Containers {
			for _, envVar := range container.EnvVars {
				if envVar.Type == dtos.EnvVarKeyVault && envVar.Data.VaultType == dtos.EnvVarVaultTypeMogeniusVault {
					secret.StringData[envVar.Name] = envVar.Value
					secret.Labels = make(map[string]string)
					secret.Labels[envVar.Name] = envVar.Data.Value
				}
				if envVar.Type == dtos.EnvVarPlainText ||
					envVar.Type == dtos.EnvVarHostname ||
					envVar.Type == dtos.EnvVarVolumeMount ||
					(envVar.Type == dtos.EnvVarKeyVault && envVar.Data.VaultType == dtos.EnvVarVaultTypeHashicorpExternalVault) {
					delete(secret.StringData, envVar.Name)
				}
			}
		}

		// delete secret if empty
		if len(secret.StringData) == 0 {
			_, err := secretClient.Get(context.TODO(), service.ControllerName, metav1.GetOptions{})

			// ignore if not found
			if apierrors.IsNotFound(err) {
				cmd.Success(job, "Deleted unneeded secret")
				return
			} else if err != nil {
				cmd.Fail(job, fmt.Sprintf("Deleted unneeded secret ERROR: %s", err.Error()))
				return
			}

			err = secretClient.Delete(context.TODO(), service.ControllerName, metav1.DeleteOptions{})
			if err != nil {
				cmd.Fail(job, fmt.Sprintf("Deleted unneeded secret ERROR: %s", err.Error()))
			} else {
				cmd.Success(job, "Deleted unneeded secret")
			}
			return
		}

		_, err := secretClient.Update(context.TODO(), &secret, MoUpdateOptions())
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = secretClient.Create(context.TODO(), &secret, MoCreateOptions())
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("UpdateOrCreateControllerSecrete ERROR: %s", err.Error()))
				} else {
					cmd.Success(job, "Created secret")
				}
			} else {
				cmd.Fail(job, fmt.Sprintf("UpdateOrCreateControllerSecrete ERROR: %s", err.Error()))
			}
		} else {
			cmd.Success(job, "Update secret")
		}
	}(wg)
}

func DeleteControllerSecret(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	secretName := service.ControllerName

	cmd := structs.CreateCommand("delete", "Delete Controller secret", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting Controller secret")

		secretClient := GetCoreClient().Secrets(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: punqUtils.Pointer[int64](5),
		}

		_, err := secretClient.Get(context.TODO(), secretName, metav1.GetOptions{})

		// ignore if not found
		if apierrors.IsNotFound(err) {
			cmd.Success(job, "Deleted controller secret")
			return
		} else if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteControllerSecret ERROR: %s", err.Error()))
			return
		}

		err = secretClient.Delete(context.TODO(), secretName, deleteOptions)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteControllerSecret ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Deleted Controller secret")
		}
	}(wg)
}

//-----------------------------------------------------
// Watch Secrets
//-----------------------------------------------------

func WatchSecrets() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		K8sLogger.Fatalf("Error creating provider for watcher. Cannot continue because it is vital: %s", err.Error())
		return
	}

	// Retry watching resources with exponential backoff in case of failures
	err = retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		return watchSecrets(provider, "secrets")
	})

	if err != nil {
		K8sLogger.Fatalf("Error watching secrets: %s", err.Error())
	}

	// Wait forever
	select {}
}

func watchSecrets(provider *punq.KubeProvider, kindName string) error {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			castedObj := obj.(*v1.Secret)
			castedObj.Kind = "Secret"
			castedObj.APIVersion = "v1"
			IacManagerWriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			castedObj := newObj.(*v1.Secret)
			castedObj.Kind = "Secret"
			castedObj.APIVersion = "v1"
			IacManagerWriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		DeleteFunc: func(obj interface{}) {
			castedObj := obj.(*v1.Secret)
			castedObj.Kind = "Secret"
			castedObj.APIVersion = "v1"
			IacManagerDeleteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, obj)
		},
	}
	listWatch := cache.NewListWatchFromClient(
		provider.ClientSet.CoreV1().RESTClient(),
		kindName,
		v1.NamespaceAll,
		fields.Nothing(),
	)
	resourceInformer := cache.NewSharedInformer(listWatch, &v1.Secret{}, 0)
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

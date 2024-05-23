package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/dtos"
	iacmanager "mogenius-k8s-manager/iac-manager"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

const (
	INGRESS_PREFIX = "ingress"
)

func UpdateIngress(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("update", "Update Ingress", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Updating ingress")

		ingressControllerType, err := punq.DetermineIngressControllerType(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		if ingressControllerType == punq.UNKNOWN || ingressControllerType == punq.NONE {
			cmd.Fail(job, "ERROR: Unknown or NONE ingress controller installed. Supported are NGINX and TRAEFIK")
			return
		}

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		ingressClient := provider.ClientSet.NetworkingV1().Ingresses(namespace.Name)

		for _, container := range service.Containers {
			ingressName := INGRESS_PREFIX + "-" + service.ControllerName + "-" + container.Name

			var ingressToUpdate *v1.Ingress

			// check if ingress already exists
			existingIngress, err := ingressClient.Get(context.TODO(), ingressName, metav1.GetOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				cmd.Fail(job, fmt.Sprintf("Get Ingress ERROR: %s", err.Error()))
				return
			}
			if apierrors.IsNotFound(err) {
				existingIngress = nil
				ingressToUpdate = &v1.Ingress{}
				ingressToUpdate.Name = ingressName
				ingressToUpdate.Namespace = namespace.Name
				ingressToUpdate.Annotations = map[string]string{}
				ingressToUpdate.Labels = map[string]string{}
			} else {
				ingressToUpdate = existingIngress.DeepCopy()
			}

			ingressToUpdate.Labels = MoUpdateLabels(&ingressToUpdate.Labels, &job.ProjectId, &namespace, &service)
			ingressToUpdate.Annotations = loadDefaultAnnotations() // TODO MERGE MAPS INSTEAD OF OVERWRITE

			if IsLocalClusterSetup() {
				delete(ingressToUpdate.Annotations, "cert-manager.io/cluster-issuer")
			}

			if ingressControllerType == punq.NGINX {
				ingressToUpdate.Spec.IngressClassName = punqUtils.Pointer("nginx")
			} else if ingressControllerType == punq.TRAEFIK {
				ingressToUpdate.Spec.IngressClassName = punqUtils.Pointer("traefik")
			}
			tlsHosts := []string{}

			ingressToUpdate.Spec.Rules = []v1.IngressRule{} // reset rules to regenerate them

			for _, port := range container.Ports {
				// SKIP UNEXPOSED PORTS
				if !port.Expose {
					continue
				}
				if port.PortType != dtos.PortTypeHTTPS {
					continue
				}

				// 2. ALL CNAMES
				for _, cname := range container.CNames {
					ingressToUpdate.Spec.Rules = append(ingressToUpdate.Spec.Rules, *createIngressRule(cname.CName, service.ControllerName, int32(port.InternalPort)))
					if cname.AddToTlsHosts {
						tlsHosts = append(tlsHosts, cname.CName)
					}
				}
			}
			ingressToUpdate.Spec.TLS = []v1.IngressTLS{
				{
					Hosts:      tlsHosts,
					SecretName: service.ControllerName,
				},
			}

			// if redirectTo != nil {
			// 	config.Annotations["nginx.ingress.kubernetes.io/permanent-redirect"] = *redirectTo
			// }

			// was not needed anymore. can be remove in a while
			// BEFORE UPDATING INGRESS WE SETUP THE CERTIFICATES FOR ALL HOSTNAMES
			// UpdateNamespaceCertificate(namespace.Name, tlsHosts)

			// update
			if existingIngress != nil {
				// delete if no rules
				if len(existingIngress.Spec.Rules) <= 0 {
					err := ingressClient.Delete(context.TODO(), ingressName, metav1.DeleteOptions{})
					if err != nil {
						cmd.Fail(job, fmt.Sprintf("Delete Ingress ERROR: %s", err.Error()))
						return
					} else {
						cmd.Success(job, fmt.Sprintf("Ingress '%s' deleted (not needed anymore)", ingressName))
					}
				} else {
					_, err := ingressClient.Update(context.TODO(), ingressToUpdate, metav1.UpdateOptions{})
					if err != nil {
						cmd.Fail(job, fmt.Sprintf("Update Ingress ERROR: %s", err.Error()))
						return
					} else {
						cmd.Success(job, fmt.Sprintf("Ingress '%s' updated", ingressName))
					}
				}
			} else {
				if len(ingressToUpdate.Spec.Rules) <= 0 {
					ingressClient.Delete(context.TODO(), ingressName, metav1.DeleteOptions{})
					cmd.Success(job, fmt.Sprintf("Ingress '%s' deleted (not needed anymore)", ingressName))
				} else {
					// create
					_, err := ingressClient.Create(context.TODO(), ingressToUpdate, metav1.CreateOptions{FieldManager: DEPLOYMENTNAME})
					if err != nil {
						cmd.Fail(job, fmt.Sprintf("Create Ingress ERROR: %s", err.Error()))
						return
					} else {
						cmd.Success(job, fmt.Sprintf("Ingress '%s' created", ingressName))
					}
				}
			}
		}
	}(wg)
}

func DeleteIngress(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Deleting ingress", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting ingress")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		ingressClient := provider.ClientSet.NetworkingV1().Ingresses(namespace.Name)

		for _, container := range service.Containers {
			ingressName := INGRESS_PREFIX + "-" + service.ControllerName + "-" + container.Name
			existingIngress, err := ingressClient.Get(context.TODO(), ingressName, metav1.GetOptions{})
			if existingIngress != nil && err == nil {
				err := ingressClient.Delete(context.TODO(), ingressName, metav1.DeleteOptions{})
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("Delete Ingress ERROR: %s", err.Error()))
					return
				} else {
					cmd.Success(job, "Deleted Ingress")
				}
			} else {
				cmd.Success(job, "Ingress already deleted")
			}
		}
	}(wg)
}

func loadDefaultAnnotations() map[string]string {
	result := map[string]string{
		"cert-manager.io/cluster-issuer":                 "letsencrypt-cluster-issuer",
		"nginx.ingress.kubernetes.io/rewrite-target":     "/",
		"nginx.ingress.kubernetes.io/use-regex":          "true",
		"nginx.ingress.kubernetes.io/cors-allow-headers": "DNT,X-CustomHeader,Keep-Alive,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Authorization,correlation-id,device-version,device,access-token,refresh-token",
		"nginx.ingress.kubernetes.io/proxy-body-size":    "200m",
		"nginx.ingress.kubernetes.io/server-snippet": `location @custom {
			proxy_pass https://errorpages.mogenius.io;
			proxy_set_header Host            \"errorpages.mogenius.io\";
			internal;
		}
		error_page 400 401 403 404 405 406 408 413 417 500 502 503 504 @custom;`,
	}

	defaultIngAnnotations := punq.ConfigMapFor(utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-default-ingress-values", false, nil)
	if defaultIngAnnotations != nil {
		if annotationsRaw, exists := defaultIngAnnotations.Data["annotations"]; exists {
			var annotations map[string]string
			if err := json.Unmarshal([]byte(annotationsRaw), &annotations); err != nil {
				log.Errorf("Error unmarshalling annotations from mogenius-default-ingress-values: %s", err.Error())
				return result
			}
			for key, value := range annotations {
				result[key] = value
			}
		}
	}
	return result
}

func createIngressRule(hostname string, controllerName string, port int32) *v1.IngressRule {
	rule := v1.IngressRule{}
	rule.Host = hostname
	path := "/"
	pathType := v1.PathTypePrefix

	rule.HTTP = &v1.HTTPIngressRuleValue{
		Paths: []v1.HTTPIngressPath{
			{
				PathType: &pathType,
				Path:     path,
				Backend: v1.IngressBackend{
					Service: &v1.IngressServiceBackend{
						Name: controllerName,
						Port: v1.ServiceBackendPort{
							Number: port,
						},
					},
				},
			},
		},
	}

	return &rule
}

func CleanupIngressControllerServicePorts(ports []dtos.NamespaceServicePortDto) {
	indexesToRemove := []int{}
	service := punq.ServiceFor(utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-ingress-nginx-controller", nil)
	if service != nil {
		portsDb := []dtos.NamespaceServicePortDto{}
		for _, port := range ports {
			if port.ExternalPort != 0 {
				portsDb = append(portsDb, port)
			}
		}
		if service.Spec.Ports != nil {
			for index, ingressPort := range service.Spec.Ports {
				if ingressPort.Port < 9999 {
					continue
				}
				isInDb := false
				for _, item := range portsDb {
					if item.ExternalPort == int(ingressPort.Port) && string(item.PortType) == string(ingressPort.Protocol) {
						isInDb = true
						break
					}
				}
				if !isInDb {
					indexesToRemove = append(indexesToRemove, index)
				}
			}
			log.Infof("Following indexes will be remove: %v", indexesToRemove)
			if len(indexesToRemove) > 0 {
				for _, indexToRemove := range indexesToRemove {
					service.Spec.Ports = punqUtils.Remove(service.Spec.Ports, indexToRemove)
				}
				log.Infof("%d indexes successfully remove", len(indexesToRemove))

				// TODO wieder einkommentieren wenn ordentlich getest in DEV. sieht gut aus.
				//UpdateServiceWith(service)
			}
			return
		}
		log.Error("IngressController has no ports defined")
	}
	log.Error("Could not load service mogenius/mogenius-ingress-nginx-controller")
}

func CreateMogeniusContainerRegistryIngress() {
	ing := utils.InitMogeniusContainerRegistryIngress()
	ing.Namespace = utils.CONFIG.Kubernetes.OwnNamespace

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		log.Error(fmt.Sprintf("CreateMogeniusContainerRegistryIngress ERROR: %s", err.Error()))
	}

	client := provider.ClientSet.NetworkingV1().Ingresses(ing.Namespace)
	_, err = client.Get(context.TODO(), ing.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = client.Create(context.TODO(), &ing, metav1.CreateOptions{})
		if err == nil {
			log.Infof("Created ingress '%s' in namespace '%s'", ing.Name, ing.Namespace)
		} else {
			log.Errorf("CreateMogeniusContainerRegistryIngress ERROR: %s", err.Error())
		}
	} else {
		log.Infof("Ingress '%s' in namespace '%s' already exists", ing.Name, ing.Namespace)
	}
}

func CreateMogeniusContainerRegistryTlsSecret(crt string, key string) error {
	secret := utils.InitMogeniusContainerRegistrySecret(crt, key)
	secret.Namespace = utils.CONFIG.Kubernetes.OwnNamespace

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		log.Error(fmt.Sprintf("CreateMogeniusContainerRegistryTlsSecret ERROR: %s", err.Error()))
	}

	client := provider.ClientSet.CoreV1().Secrets(secret.Namespace)

	_, err = client.Get(context.TODO(), secret.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = client.Create(context.TODO(), &secret, metav1.CreateOptions{})
		if err == nil {
			log.Infof("Created secret '%s' in namespace '%s'", secret.Name, secret.Namespace)
		} else {
			log.Errorf("CreateMogeniusContainerRegistryTlsSecret ERROR: %s", err.Error())
			return err
		}
	} else {
		_, err = client.Update(context.TODO(), &secret, metav1.UpdateOptions{})
		if err == nil {
			log.Infof("Secret '%s' in namespace '%s' updated", secret.Name, secret.Namespace)
		} else {
			log.Errorf("CreateMogeniusContainerRegistryTlsSecret ERROR: %s", err.Error())
			return err
		}
	}
	return nil
}

func WatchIngresses() {
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
		return watchIngresses(provider, "ingresses")
	})
	if err != nil {
		log.Fatalf("Error watching ingresses: %s", err.Error())
	}

	// Wait forever
	select {}
}

func watchIngresses(provider *punq.KubeProvider, kindName string) error {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			castedObj := obj.(*v1.Ingress)
			castedObj.Kind = "Ingress"
			castedObj.APIVersion = "networking.k8s.io/v1"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			castedObj := newObj.(*v1.Ingress)
			castedObj.Kind = "Ingress"
			castedObj.APIVersion = "networking.k8s.io/v1"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		DeleteFunc: func(obj interface{}) {
			castedObj := obj.(*v1.Ingress)
			castedObj.Kind = "Ingress"
			castedObj.APIVersion = "networking.k8s.io/v1"
			iacmanager.DeleteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, obj)
		},
	}
	listWatch := cache.NewListWatchFromClient(
		provider.ClientSet.NetworkingV1().RESTClient(),
		kindName,
		v1Core.NamespaceAll,
		fields.Nothing(),
	)
	resourceInformer := cache.NewSharedInformer(listWatch, &v1.Ingress{}, 0)
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

package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateExternalSecretsStore() {
	extSecr := utils.InitExternalSecretsStoreYaml()
	extSecr.Namespace = utils.CONFIG.Kubernetes.OwnNamespace

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		log.Error(fmt.Sprintf("CreateExternalSecretsStore ERROR: %s", err.Error()))
	}

	//TODO probably need to use this schema style? 
	// projectsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceProject}
	// raw := newObj.ToUnstructuredProject(name)
	// _, err = provider.ClientSet.Resource(projectsGVR).Create(context.Background(), raw, metav1.CreateOptions{})
	// if err != nil {
	// 	log.Errorf("Error creating project: %s", err.Error())
	// 	return err
	// }
	
	client := provider.ClientSet.
	_, err = client.Get(context.TODO(), extSecr.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = client.Create(context.TODO(), &extSecr, metav1.CreateOptions{})
		if err == nil {
			log.Infof("Created ingress '%s' in namespace '%s'", extSecr.Name, extSecr.Namespace)
		} else {
			log.Errorf("CreateMogeniusContainerRegistryIngress ERROR: %s", err.Error())
		}
	} else {
		log.Infof("Ingress '%s' in namespace '%s' already exists", extSecr.Name, extSecr.Namespace)
	}
}

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

			if len(tlsHosts) > 0 {
				ingressToUpdate.Spec.TLS = []v1.IngressTLS{
					{
						Hosts:      tlsHosts,
						SecretName: service.ControllerName,
					},
				}
			} else {
				ingressToUpdate.Spec.TLS = []v1.IngressTLS{}
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

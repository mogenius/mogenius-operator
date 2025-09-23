package kubernetes

import (
	"context"
	"fmt"
	"maps"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/websocket"

	json "github.com/json-iterator/go"

	v1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	INGRESS_PREFIX = "ingress"
)

func AllIngresses(namespaceName string) []v1.Ingress {
	result := []v1.Ingress{}

	clientset := clientProvider.K8sClientSet()
	ingressList, err := clientset.NetworkingV1().Ingresses(namespaceName).List(context.Background(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		k8sLogger.Error("AllIngresses", "error", err.Error())
		return result
	}

	for _, ingress := range ingressList.Items {
		ingress.Kind = "Ingress"
		ingress.APIVersion = "networking.k8s.io/v1"
		result = append(result, ingress)
	}
	return result
}

func UpdateIngress(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto) {
	cmd := structs.CreateCommand(eventClient, "update", "Update Ingress", job)
	cmd.Start(eventClient, job, "Updating ingress")

	ingressControllerType, err := DetermineIngressControllerType()
	if err != nil {
		cmd.Fail(eventClient, job, fmt.Sprintf("ERROR: %s", err.Error()))
		return
	}
	if ingressControllerType == UNKNOWN || ingressControllerType == NONE {
		cmd.Fail(eventClient, job, "ERROR: Unknown or NONE ingress controller installed. Supported are NGINX and TRAEFIK")
		return
	}

	clientset := clientProvider.K8sClientSet()
	ingressClient := clientset.NetworkingV1().Ingresses(namespace.Name)

	for _, container := range service.Containers {
		containerIngressName := INGRESS_PREFIX + "-" + service.ControllerName + "-" + container.Name
		existingIngress, err := ingressClient.Get(context.Background(), containerIngressName, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			continue
		}
		if existingIngress != nil {
			err := ingressClient.Delete(context.Background(), containerIngressName, metav1.DeleteOptions{})
			if err != nil {
				k8sLogger.Error("Error deleting ingress", "error", err)
			}
		}
	}
	ingressName := INGRESS_PREFIX + "-" + service.ControllerName //  + "-" + container.Name

	var ingressToUpdate *v1.Ingress

	// check if ingress already exists
	existingIngress, err := ingressClient.Get(context.Background(), ingressName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		cmd.Fail(eventClient, job, fmt.Sprintf("Get Ingress ERROR: %s", err.Error()))
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

	ingressToUpdate.Labels = MoUpdateLabels(&ingressToUpdate.Labels, &job.ProjectId, &namespace, &service, config)
	ingressToUpdate.Annotations = loadDefaultAnnotations() // TODO MERGE MAPS INSTEAD OF OVERWRITE

	if IsLocalClusterSetup() {
		delete(ingressToUpdate.Annotations, "cert-manager.io/cluster-issuer")
	}

	switch ingressControllerType {
	case NGINX:
		ingressToUpdate.Spec.IngressClassName = utils.Pointer("nginx")
	case TRAEFIK:
		ingressToUpdate.Spec.IngressClassName = utils.Pointer("traefik")
	}
	tlsHosts := []string{}

	// TODO REMOVE
	// ingressToUpdate.Spec.Rules = []v1.IngressRule{} // reset rules to regenerate them

	// clean up rules and paths for this service
	if len(ingressToUpdate.Spec.Rules) > 0 {
		for ruleIndex := len(ingressToUpdate.Spec.Rules) - 1; ruleIndex >= 0; ruleIndex-- {
			rule := ingressToUpdate.Spec.Rules[ruleIndex]
			if rule.HTTP != nil {
				for pathIndex := len(rule.HTTP.Paths) - 1; pathIndex >= 0; pathIndex-- {
					path := rule.HTTP.Paths[pathIndex]
					if path.Backend.Service.Name == service.ControllerName {
						// remove path
						rule.HTTP.Paths = append(rule.HTTP.Paths[:pathIndex], rule.HTTP.Paths[pathIndex+1:]...)
					}
				}
				// If no paths left, remove rule
				if len(rule.HTTP.Paths) == 0 {
					ingressToUpdate.Spec.Rules = append(ingressToUpdate.Spec.Rules[:ruleIndex], ingressToUpdate.Spec.Rules[ruleIndex+1:]...)
				}
			}
		}
	}

	for _, port := range service.Ports {
		// SKIP UNEXPOSED PORTS
		if !port.Expose {
			continue
		}
		if port.PortType != dtos.PortTypeHTTPS {
			continue
		}

		// 2. ALL CNAMES
		for _, cname := range port.CNames {
			ingressToUpdate.Spec.Rules = append(ingressToUpdate.Spec.Rules, *createIngressRule(cname.CName, service.ControllerName, intstr.Parse(port.InternalPort).IntVal))
			if cname.AddToTlsHosts {
				tlsHosts = append(tlsHosts, cname.CName)
			}
		}
	}

	if len(tlsHosts) > 0 {
		ingressToUpdate.Spec.TLS = []v1.IngressTLS{
			{
				Hosts:      tlsHosts,
				SecretName: service.ControllerName + "-tls",
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
		if len(ingressToUpdate.Spec.Rules) <= 0 {
			err := ingressClient.Delete(context.Background(), ingressName, metav1.DeleteOptions{})
			if err != nil {
				cmd.Fail(eventClient, job, fmt.Sprintf("Delete Ingress ERROR: %s", err.Error()))
				return
			} else {
				cmd.Success(eventClient, job, fmt.Sprintf("Ingress '%s' deleted (not needed anymore)", ingressName))
			}
		} else {
			_, err := ingressClient.Update(context.Background(), ingressToUpdate, metav1.UpdateOptions{})
			if err != nil {
				cmd.Fail(eventClient, job, fmt.Sprintf("Update Ingress ERROR: %s", err.Error()))
				return
			} else {
				cmd.Success(eventClient, job, fmt.Sprintf("Ingress '%s' updated", ingressName))
			}
		}
	} else {
		if len(ingressToUpdate.Spec.Rules) <= 0 {
			err := ingressClient.Delete(context.Background(), ingressName, metav1.DeleteOptions{})
			if err != nil {
				k8sLogger.Error("Error deleting ingress", "error", err)
			}
			cmd.Success(eventClient, job, fmt.Sprintf("Ingress '%s' deleted (not needed anymore)", ingressName))
		} else {
			// create
			_, err := ingressClient.Create(context.Background(), ingressToUpdate, metav1.CreateOptions{FieldManager: GetOwnDeploymentName(config)})
			if err != nil {
				cmd.Fail(eventClient, job, fmt.Sprintf("Create Ingress ERROR: %s", err.Error()))
				return
			} else {
				cmd.Success(eventClient, job, fmt.Sprintf("Ingress '%s' created", ingressName))
			}
		}
	}
}

func DeleteIngress(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto) {
	cmd := structs.CreateCommand(eventClient, "delete", "Deleting ingress", job)
	cmd.Start(eventClient, job, "Deleting ingress")

	clientset := clientProvider.K8sClientSet()
	ingressClient := clientset.NetworkingV1().Ingresses(namespace.Name)

	for _, container := range service.Containers {
		ingressName := INGRESS_PREFIX + "-" + service.ControllerName + "-" + container.Name
		existingIngress, err := ingressClient.Get(context.Background(), ingressName, metav1.GetOptions{})
		if existingIngress != nil && err == nil {
			err := ingressClient.Delete(context.Background(), ingressName, metav1.DeleteOptions{})
			if err != nil {
				cmd.Fail(eventClient, job, fmt.Sprintf("Delete Ingress ERROR: %s", err.Error()))
				return
			} else {
				cmd.Success(eventClient, job, "Deleted Ingress")
			}
		} else {
			cmd.Success(eventClient, job, "Ingress already deleted")
		}
	}
}

func loadDefaultAnnotations() map[string]string {
	result := map[string]string{
		"cert-manager.io/cluster-issuer": "letsencrypt-cluster-issuer",
	}

	defaultIngAnnotations, err := ConfigMapFor(config.Get("MO_OWN_NAMESPACE"), "mogenius-default-ingress-values", false)
	if err != nil {
		return result
	}
	assert.Assert(defaultIngAnnotations != nil)

	if annotationsRaw, exists := defaultIngAnnotations.Data["annotations"]; exists {
		var annotations map[string]string
		err := json.Unmarshal([]byte(annotationsRaw), &annotations)
		if err != nil {
			k8sLogger.Error("Error unmarshalling annotations from mogenius-default-ingress-values", "error", err)
			return result
		}

		maps.Copy(result, annotations)
	}

	return result
}

func createIngressRule(hostname string, controllerName string, port int32) *v1.IngressRule {
	rule := v1.IngressRule{}
	rule.Host = hostname
	rule.HTTP = &v1.HTTPIngressRuleValue{
		Paths: []v1.HTTPIngressPath{
			{
				PathType: utils.Pointer(v1.PathTypePrefix),
				Path:     "/",
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

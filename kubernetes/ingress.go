package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	networkingv1 "k8s.io/client-go/applyconfigurations/networking/v1"
)

const (
	INGRESS_PREFIX = "ingress"
)

func UpdateIngress(job *structs.Job, namespaceShortId string, stage dtos.K8sStageDto, redirectTo *string, skipForDelete *dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Updating ingress setup.", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start("Updating ingress setup.", c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}

		if err != nil {
			cmd.Fail(fmt.Sprintf("UpdateIngress ERROR: %s", err.Error()), c)
			return
		}

		ingressClient := kubeProvider.ClientSet.NetworkingV1().Ingresses(stage.K8sName)

		applyOptions := metav1.ApplyOptions{
			Force:        true,
			FieldManager: DEPLOYMENTNAME,
		}

		ingressName := INGRESS_PREFIX + "-" + namespaceShortId

		config := networkingv1.Ingress(ingressName, stage.K8sName)
		config.WithAnnotations(map[string]string{
			"kubernetes.io/ingress.class":                    "nginx",
			"nginx.ingress.kubernetes.io/rewrite-target":     "/",
			"nginx.ingress.kubernetes.io/use-regex":          "true",
			"nginx.ingress.kubernetes.io/cors-allow-headers": "DNT,X-CustomHeader,Keep-Alive,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Authorization,correlation-id,device-version,device,access-token,refresh-token",
			"nginx.ingress.kubernetes.io/proxy-body-size":    "100m",
			"nginx.ingress.kubernetes.io/server-snippet": `location @custom {
				proxy_pass https://errorpages.mogenius.io;
				proxy_set_header Host            \"errorpages.mogenius.io\";
				internal;
			}
			error_page 400 401 403 404 405 406 408 413 417 500 502 503 504 @custom;`,
		})
		if !stage.CloudflareProxied {
			config.Annotations["cert-manager.io/cluster-issuer"] = "letsencrypt-cluster-issuer"
		}

		spec := networkingv1.IngressSpec()
		tlsHosts := []string{}

		// 1. All Services
		for _, service := range stage.Services {
			// SKIP service if marked for delete
			if skipForDelete != nil && skipForDelete.Id == service.Id {
				continue
			}
			// SWITCHED OFF
			if !service.SwitchedOn {
				continue
			}

			for _, port := range service.Ports {
				// SKIP UNEXPOSED PORTS
				if !port.Expose {
					continue
				}
				if port.PortType != "HTTPS" {
					continue
				}

				// 2. ALL CNAMES
				spec.Rules = append(spec.Rules, *createIngressRule(service.FullHostname, service.K8sName, int32(port.InternalPort)))
				for _, cname := range service.CNames {
					spec.Rules = append(spec.Rules, *createIngressRule(cname, service.K8sName, int32(port.InternalPort)))
					if !stage.CloudflareProxied {
						tlsHosts = append(tlsHosts, cname)
					}
				}
			}
			if !stage.CloudflareProxied {
				tlsHosts = append(tlsHosts, service.FullHostname)
			}

		}
		if !stage.CloudflareProxied {
			spec.TLS = append(spec.TLS, networkingv1.IngressTLSApplyConfiguration{
				Hosts:      tlsHosts,
				SecretName: &stage.K8sName,
			})
		}

		if redirectTo != nil {
			config.Annotations["nginx.ingress.kubernetes.io/permanent-redirect"] = *redirectTo
		}

		config.WithSpec(spec)

		if len(spec.Rules) <= 0 {
			existingIngress, _ := ingressClient.Get(context.TODO(), ingressName, metav1.GetOptions{})
			if existingIngress != nil {
				err = ingressClient.Delete(context.TODO(), ingressName, metav1.DeleteOptions{})
				if err != nil {
					cmd.Fail(fmt.Sprintf("Delete Ingress ERROR: %s", err.Error()), c)
					return
				} else {
					cmd.Success(fmt.Sprintf("Ingress '%s' deleted (not needed anymore).", ingressName), c)
				}
			} else {
				cmd.Success(fmt.Sprintf("Ingress '%s' already deleted.", ingressName), c)
			}
		} else {
			_, err = ingressClient.Apply(context.TODO(), config, applyOptions)
			if err != nil {
				cmd.Fail(fmt.Sprintf("UpdateIngress ERROR: %s", err.Error()), c)
				return
			} else {
				cmd.Success(fmt.Sprintf("Updated Ingress '%s'.", ingressName), c)
			}
		}
	}(cmd, wg)
	return cmd
}

func createIngressRule(hostname string, serviceName string, port int32) *networkingv1.IngressRuleApplyConfiguration {
	rule := networkingv1.IngressRule()
	rule.Host = &hostname
	path := "/"
	pathType := v1.PathTypePrefix

	rule.HTTP = &networkingv1.HTTPIngressRuleValueApplyConfiguration{
		Paths: []networkingv1.HTTPIngressPathApplyConfiguration{
			{
				PathType: &pathType,
				Path:     &path,
				Backend: &networkingv1.IngressBackendApplyConfiguration{
					Service: &networkingv1.IngressServiceBackendApplyConfiguration{
						Name: &serviceName,
						Port: &networkingv1.ServiceBackendPortApplyConfiguration{
							Number: &port,
						},
					},
				},
			},
		},
	}

	return rule
}

func CleanupIngressControllerServicePorts(ports []dtos.NamespaceServicePortDto) {
	indexesToRemove := []int{}
	service := ServiceFor("default", "nginx-ingress-ingress-nginx-controller")
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
					if item.ExternalPort == int(ingressPort.Port) && item.PortType == string(ingressPort.Protocol) {
						isInDb = true
						break
					}
				}
				if !isInDb {
					indexesToRemove = append(indexesToRemove, index)
				}
			}
			logger.Log.Infof("Following indexes will be remove: %v", indexesToRemove)
			if len(indexesToRemove) > 0 {
				for _, indexToRemove := range indexesToRemove {
					service.Spec.Ports = utils.Remove(service.Spec.Ports, indexToRemove)
				}
				logger.Log.Infof("%d indexes successfully remove.", len(indexesToRemove))

				// TODO wieder einkommentieren wenn ordentlich getest in DEV. sieht gut aus.
				//UpdateServiceWith(service)
			}
			return
		}
		logger.Log.Error("IngressController has no ports defined.")
	}
	logger.Log.Error("Could not load service default/nginx-ingress-ingress-nginx-controller.")
}

func AllIngresses(namespaceName string) []v1.Ingress {
	result := []v1.Ingress{}

	var provider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		provider, err = NewKubeProviderLocal()
	} else {
		provider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("AllIngresses ERROR: %s", err.Error())
		return result
	}

	ingressList, err := provider.ClientSet.NetworkingV1().Ingresses(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllIngresses ERROR: %s", err.Error())
		return result
	}

	for _, ingress := range ingressList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, ingress.ObjectMeta.Namespace) {
			result = append(result, ingress)
		}
	}
	return result
}

func UpdateK8sIngress(data v1.Ingress) K8sWorkloadResult {
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}

	if err != nil {
		return WorkloadResult(err.Error())
	}

	ingressClient := kubeProvider.ClientSet.NetworkingV1().Ingresses(data.Namespace)
	_, err = ingressClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

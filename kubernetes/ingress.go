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

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}

		if err != nil {
			logger.Log.Errorf("UpdateIngress ERROR: %s", err.Error())
		}

		ingressClient := kubeProvider.ClientSet.NetworkingV1().Ingresses(stage.K8sName)

		applyOptions := metav1.ApplyOptions{
			Force:        true,
			FieldManager: DEPLOYMENTNAME,
		}

		config := networkingv1.Ingress(INGRESS_PREFIX+"-"+namespaceShortId, stage.K8sName)
		config.WithAnnotations(map[string]string{
			"kubernetes.io/ingress.class":                    "nginx",
			"nginx.ingress.kubernetes.io/rewrite-target":     "/",
			"nginx.ingress.kubernetes.io/use-regex":          "true",
			"nginx.ingress.kubernetes.io/cors-allow-headers": "DNT,X-CustomHeader,Keep-Alive,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Authorization,correlation-id,device-version,device,access-token,refresh-token",
			"nginx.ingress.kubernetes.io/proxy-body-size":    "100m",
		})
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
					spec.Rules = append(spec.Rules, *createIngressRule(cname.CName, service.K8sName, int32(port.InternalPort)))
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
			err = ingressClient.Delete(context.TODO(), stage.K8sName, metav1.DeleteOptions{})
			if err != nil {
				cmd.Fail(fmt.Sprintf("Delete Ingress ERROR: %s", err.Error()), c)
			} else {
				cmd.Success(fmt.Sprintf("Ingress '%s' deleted (not needed anymore).", stage.K8sName), c)
			}
		} else {
			_, err = ingressClient.Apply(context.TODO(), config, applyOptions)
			if err != nil {
				cmd.Fail(fmt.Sprintf("UpdateIngress ERROR: %s", err.Error()), c)
			} else {
				cmd.Success(fmt.Sprintf("Updated Ingress '%s'.", stage.K8sName), c)
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

// func CreateIngressRule(hostname string, path string, pathType networking.PathType, backendServiceName string, port int32) *networking.IngressRule {
// 	return &networking.IngressRule{
// 		Host: hostname,
// 		IngressRuleValue: networking.IngressRuleValue{
// 			HTTP: &networking.HTTPIngressRuleValue{
// 				Paths: []networking.HTTPIngressPath{{
// 					Path:     path,
// 					PathType: &pathType,
// 					Backend: networking.IngressBackend{
// 						Service: &networking.IngressServiceBackend{
// 							Name: backendServiceName,
// 							Port: networking.ServiceBackendPort{
// 								Number: port, //todo add optional port configuration via PortName right here
// 							},
// 						},
// 					},
// 				}},
// 			},
// 		},
// 	}
// }

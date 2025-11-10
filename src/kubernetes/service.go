package kubernetes

import (
	"context"
	"fmt"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/store"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllServices(namespaceName string) []v1.Service {
	result := []v1.Service{}

	services := store.GetServices(namespaceName, "*")
	for _, service := range services {
		if service.Namespace == "kube-system" {
			continue
		}
		result = append(result, service)
	}
	return result
}

func FindPrometheusService() (namespace string, service string, port int32, err error) {
	clientset := clientProvider.K8sClientSet()
	serviceClient := clientset.CoreV1().Services("")
	serviceList, err := serviceClient.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		k8sLogger.Error("findPrometheusService", "error", err.Error())
		return "", "", -1, fmt.Errorf("failed to list services: %v", err)
	}
	for _, service := range serviceList.Items {
		if service.Name == "prometheus-kube-prometheus-prometheus" ||
			service.Name == "kube-prometheus-stack-prometheus" ||
			service.Name == "prometheus-server" ||
			service.Name == "prometheus-service" ||
			service.Name == "prometheus" ||
			service.Name == "prometheus-prometheus-server" {
			if len(service.Spec.Ports) > 0 {
				return service.Namespace, service.Name, service.Spec.Ports[0].Port, nil
			}
		}
	}
	return "", "", -1, fmt.Errorf("prometheus service not found in any namespace")
}

func FindSealedSecretsService(cfg cfg.ConfigModule) (namespace string, service string, port int32, err error) {
	clientset := clientProvider.K8sClientSet()

	ownNamespace := cfg.Get("MO_OWN_NAMESPACE")

	// exists mogenius sealed-secrets config
	sealedSecretsConfig, err := clientset.CoreV1().ConfigMaps(ownNamespace).Get(context.Background(), "sealed-secrets-config", metav1.GetOptions{})
	if err == nil {
		if namespaceName, ok := sealedSecretsConfig.Data["namespaceName"]; ok {
			if releaseName, ok := sealedSecretsConfig.Data["releaseName"]; ok {
				sealedSecretsService, err := clientset.CoreV1().Services(namespaceName).Get(context.Background(), releaseName, metav1.GetOptions{})
				if err == nil && len(sealedSecretsService.Spec.Ports) > 0 && sealedSecretsService.Spec.Ports[0].Port != 0 {
					return sealedSecretsService.Namespace, sealedSecretsService.Name, sealedSecretsService.Spec.Ports[0].Port, nil
				}
			}
		}
	}

	serviceClient := clientset.CoreV1().Services("")
	serviceList, err := serviceClient.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		k8sLogger.Error("findSealedSecretsService", "error", err.Error())
		return "", "", -1, fmt.Errorf("failed to list services: %v", err)
	}
	for _, service := range serviceList.Items {
		if service.Name == "sealed-secrets" {
			if len(service.Spec.Ports) > 0 {
				return service.Namespace, service.Name, service.Spec.Ports[0].Port, nil
			}
		}
	}
	return "", "", -1, fmt.Errorf("sealed-secrets service not found in any namespace")
}

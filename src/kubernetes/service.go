package kubernetes

import (
	"fmt"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/store"

	v1 "k8s.io/api/core/v1"
)

func AllServices(namespaceName string) []v1.Service {
	services := store.GetServices(namespaceName, "*")
	result := make([]v1.Service, 0, len(services))

	for _, service := range services {
		if service.Namespace == "kube-system" {
			continue
		}
		result = append(result, service)
	}
	return result
}

func FindPrometheusService() (namespace string, service string, port int32, err error) {
	services := store.GetServices("", "*")
	for _, svc := range services {
		if svc.Name == "prometheus-kube-prometheus-prometheus" ||
			svc.Name == "kube-prometheus-stack-prometheus" ||
			svc.Name == "prometheus-server" ||
			svc.Name == "prometheus-service" ||
			svc.Name == "prometheus" ||
			svc.Name == "prometheus-prometheus-server" {
			if len(svc.Spec.Ports) > 0 {
				return svc.Namespace, svc.Name, svc.Spec.Ports[0].Port, nil
			}
		}
	}
	return "", "", -1, fmt.Errorf("prometheus service not found in any namespace")
}

func FindSealedSecretsService(cfg cfg.ConfigModule) (namespace string, service string, port int32, err error) {
	ownNamespace := cfg.Get("MO_OWN_NAMESPACE")

	// exists mogenius sealed-secrets config
	sealedSecretsConfig := store.GetConfigMap(ownNamespace, "sealed-secrets-config")
	if sealedSecretsConfig != nil {
		if namespaceName, ok := sealedSecretsConfig.Data["namespaceName"]; ok {
			if releaseName, ok := sealedSecretsConfig.Data["releaseName"]; ok {
				sealedSecretsService := store.GetService(namespaceName, releaseName)
				if sealedSecretsService != nil && len(sealedSecretsService.Spec.Ports) > 0 && sealedSecretsService.Spec.Ports[0].Port != 0 {
					return sealedSecretsService.Namespace, sealedSecretsService.Name, sealedSecretsService.Spec.Ports[0].Port, nil
				}
			}
		}
	}

	for _, svc := range store.GetServices("", "*") {
		if svc.Name == "sealed-secrets" {
			if len(svc.Spec.Ports) > 0 {
				return svc.Namespace, svc.Name, svc.Spec.Ports[0].Port, nil
			}
		}
	}
	return "", "", -1, fmt.Errorf("sealed-secrets service not found in any namespace")
}

package kubernetes

import (
	"fmt"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/store"
	"strings"

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

// prometheusNames lists the well-known Prometheus service names in priority
// order. The first name that matches a service in the cluster wins, so the
// canonical kube-prometheus-stack names are checked before the generic ones.
var prometheusNames = []string{
	"prometheus-kube-prometheus-prometheus",
	"kube-prometheus-stack-prometheus",
	"prometheus-server",
	"prometheus-service",
	"prometheus",
	"prometheus-prometheus-server",
}

// alertmanagerNames lists the well-known Alertmanager service names in priority order.
var alertmanagerNames = []string{
	"alertmanager-kube-prometheus-alertmanager",
	"kube-prometheus-stack-alertmanager",
	"alertmanager-service",
	"alertmanager",
	"prometheus-alertmanager",
}

func FindPrometheusService() (namespace string, service string, port int32, err error) {
	return findServiceByPriority(prometheusNames, "prometheus")
}

func FindAlertmanagerService() (namespace string, service string, port int32, err error) {
	return findServiceByPriority(alertmanagerNames, "alertmanager")
}

// findServiceByPriority returns the first cluster service whose name matches one
// of candidateNames, evaluated in the order given. The matched service's
// HTTP/web port is preferred over an arbitrary first port (see selectHTTPPort).
func findServiceByPriority(candidateNames []string, kind string) (namespace string, service string, port int32, err error) {
	byName := make(map[string]v1.Service)
	for _, svc := range store.GetServices("*", "*") {
		if len(svc.Spec.Ports) == 0 {
			continue
		}
		// Keep the first occurrence so behaviour is deterministic when the same
		// service name exists in multiple namespaces.
		if _, seen := byName[svc.Name]; !seen {
			byName[svc.Name] = svc
		}
	}

	for _, name := range candidateNames {
		if svc, ok := byName[name]; ok {
			return svc.Namespace, svc.Name, selectHTTPPort(svc.Spec.Ports), nil
		}
	}
	return "", "", -1, fmt.Errorf("%s service not found in any namespace", kind)
}

// selectHTTPPort picks the most likely HTTP API port from a service's ports:
// the standard Prometheus/Alertmanager port 9090/9093, or a port whose name
// hints at HTTP ("web"/"http"), falling back to the first declared port. This
// avoids accidentally querying a sidecar/gRPC port (e.g. Thanos) that happens
// to be listed first.
func selectHTTPPort(ports []v1.ServicePort) int32 {
	for _, p := range ports {
		if p.Port == 9090 || p.Port == 9093 {
			return p.Port
		}
	}
	for _, p := range ports {
		name := strings.ToLower(p.Name)
		if strings.Contains(name, "web") || strings.Contains(name, "http") {
			return p.Port
		}
	}
	return ports[0].Port
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

	for _, svc := range store.GetServices("*", "*") {
		if svc.Name == "sealed-secrets" {
			if len(svc.Spec.Ports) > 0 {
				return svc.Namespace, svc.Name, svc.Spec.Ports[0].Port, nil
			}
		}
	}
	return "", "", -1, fmt.Errorf("sealed-secrets service not found in any namespace")
}

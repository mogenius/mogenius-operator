package kubernetes

import (
	"fmt"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/store"
	"mogenius-operator/src/utils"
	"sort"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
)

// clusterServicesCached caches the cluster-wide service list for discovery
// lookups (Prometheus, Alertmanager, sealed-secrets, LB IPs). Each uncached
// call SCANs the whole Valkey keyspace and deserializes every service, and
// FindPrometheusService runs per chart query. 30s staleness only delays
// discovery of newly installed services.
var clusterServicesCached = utils.NewTTLCache(30*time.Second, func() []v1.Service {
	return store.GetServices("*", "*")
})

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

// serviceCandidate describes how to recognize a well-known monitoring service:
// by exact name, or by name prefix for operator-managed services whose names
// embed the Helm release name (e.g. VictoriaMetrics' vmsingle-<release>).
// basePath is appended to the discovered service URL when the Prometheus API
// is not served at the root of the service.
type serviceCandidate struct {
	name     string
	prefix   bool
	basePath string
}

// prometheusCandidates lists the well-known Prometheus-compatible services in
// priority order. The first candidate that matches a service in the cluster
// wins, so the canonical kube-prometheus-stack names are checked before the
// generic ones, and VictoriaMetrics single-node before cluster mode.
var prometheusCandidates = []serviceCandidate{
	{name: "prometheus-kube-prometheus-prometheus"},
	{name: "kube-prometheus-stack-prometheus"},
	{name: "prometheus-server"},
	{name: "prometheus-service"},
	{name: "prometheus"},
	{name: "prometheus-prometheus-server"},
	// VictoriaMetrics (victoria-metrics-k8s-stack) single-node: serves the
	// Prometheus query API at the service root.
	{name: "vmsingle-", prefix: true},
	// VictoriaMetrics cluster mode: vmselect serves the Prometheus query API
	// under the tenant path (tenant 0 is the default single-tenant setup).
	{name: "vmselect-", prefix: true, basePath: "/select/0/prometheus"},
}

// alertmanagerCandidates lists the well-known Alertmanager services in priority order.
var alertmanagerCandidates = []serviceCandidate{
	{name: "alertmanager-kube-prometheus-alertmanager"},
	{name: "kube-prometheus-stack-alertmanager"},
	{name: "alertmanager-service"},
	{name: "alertmanager"},
	{name: "prometheus-alertmanager"},
	// VictoriaMetrics (victoria-metrics-k8s-stack) managed Alertmanager.
	{name: "vmalertmanager-", prefix: true},
}

func FindPrometheusService() (namespace string, service string, port int32, basePath string, err error) {
	return matchServiceByPriority(prometheusCandidates, "prometheus")
}

func FindAlertmanagerService() (namespace string, service string, port int32, basePath string, err error) {
	return matchServiceByPriority(alertmanagerCandidates, "alertmanager")
}

// matchServiceByPriority returns the first service whose name matches one of
// the candidates, evaluated in the order given. The matched service's
// HTTP/web port is preferred over an arbitrary first port (see selectHTTPPort).
func matchServiceByPriority(candidates []serviceCandidate, kind string) (namespace string, service string, port int32, basePath string, err error) {
	byName := make(map[string]v1.Service)
	services := clusterServicesCached.Get()
	names := make([]string, 0, len(services))
	for _, svc := range services {
		if len(svc.Spec.Ports) == 0 {
			continue
		}
		// Keep the first occurrence so behaviour is deterministic when the same
		// service name exists in multiple namespaces.
		if _, seen := byName[svc.Name]; !seen {
			byName[svc.Name] = svc
			names = append(names, svc.Name)
		}
	}
	// Prefix candidates can match several services; pick the lexicographically
	// smallest name so discovery is deterministic across restarts.
	sort.Strings(names)

	for _, c := range candidates {
		if !c.prefix {
			if svc, ok := byName[c.name]; ok {
				return svc.Namespace, svc.Name, selectHTTPPort(svc.Spec.Ports), c.basePath, nil
			}
			continue
		}
		for _, name := range names {
			if strings.HasPrefix(name, c.name) {
				svc := byName[name]
				return svc.Namespace, svc.Name, selectHTTPPort(svc.Spec.Ports), c.basePath, nil
			}
		}
	}
	return "", "", -1, "", fmt.Errorf("%s service not found in any namespace", kind)
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

	for _, svc := range clusterServicesCached.Get() {
		if svc.Name == "sealed-secrets" {
			if len(svc.Spec.Ports) > 0 {
				return svc.Namespace, svc.Name, svc.Spec.Ports[0].Port, nil
			}
		}
	}
	return "", "", -1, fmt.Errorf("sealed-secrets service not found in any namespace")
}

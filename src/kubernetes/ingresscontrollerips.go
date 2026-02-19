// If you have an ingress controller which is processing the traffic from the load balancer
// most of the external traffic will be counted as local traffic because it is ingress-controller
// to pod communication. To identify this traffic we gather the ingress-controller internal ips
// to exclude this traffic from the local traffic counting.

package kubernetes

import (
	"mogenius-operator/src/store"
)

func GetClusterExternalIps() []string {
	var result []string = []string{}

	services := store.GetServices("", "*")
	for _, service := range services {
		for _, ingress := range service.Status.LoadBalancer.Ingress {
			if ingress.IP != "" {
				result = append(result, ingress.IP)
			} else if ingress.IP == "" && ingress.Hostname != "" {
				result = append(result, ingress.Hostname)
			}
		}
	}

	return result
}

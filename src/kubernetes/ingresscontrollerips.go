// If you have an ingress controller which is processing the traffic from the load balancer
// most of the external traffic will be counted as local traffic because it is ingress-controller
// to pod communication. To identify this traffic we gather the ingress-controller internal ips
// to exclude this traffic from the local traffic counting.

package kubernetes

import (
	"context"
	"net"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetIngressControllerIps(useLocalKubeConfig bool) []net.IP {
	var result []net.IP
	labelSelector := "app.kubernetes.io/component=controller,app.kubernetes.io/instance=nginx-ingress,app.kubernetes.io/name=ingress-nginx"

	clientset := clientProvider.K8sClientSet()
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})

	for _, pod := range pods.Items {
		ip := net.ParseIP(pod.Status.PodIP)
		if ip != nil {
			result = append(result, ip)
		}
	}

	if err != nil {
		k8sLogger.Error(err.Error())
		return result
	}
	return result
}

func GetClusterExternalIps() []string {
	var result []string = []string{}
	var allServices []v1.Service = []v1.Service{}

	clientset := clientProvider.K8sClientSet()
	labelSelector := "app.kubernetes.io/component=controller,app.kubernetes.io/name=ingress-nginx"
	services, err := clientset.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		k8sLogger.Error(err.Error())
		return result
	}
	allServices = append(allServices, services.Items...)

	// check if traefik is used
	if len(result) <= 0 {
		traefikSelector := "app.kubernetes.io/name=traefik"
		services, err := clientset.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{LabelSelector: traefikSelector})
		if err != nil {
			k8sLogger.Error(err.Error())
			return result
		}
		allServices = append(allServices, services.Items...)
	}

	for _, service := range allServices {
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

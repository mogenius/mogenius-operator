// If you have an ingress controller which is processing the traffic from the load balancer
// most of the external traffic will be counted as local traffic because it is ingress-controller
// to pod communication. To identify this traffic we gather the ingress-controller internal ips
// to exclude this traffic from the local traffic counting.

package kubernetes

import (
	"context"
	"fmt"
	"net"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetIngressControllerIps() []net.IP {
	var result []net.IP
	kubeProvider := NewKubeProvider()
	labelSelector := "app.kubernetes.io/component=controller,app.kubernetes.io/name=ingress-nginx"

	pods, err := kubeProvider.ClientSet.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})

	for _, pod := range pods.Items {
		ip := net.ParseIP(pod.Status.PodIP)
		fmt.Println(pod.Name, ip)
		if ip != nil {
			result = append(result, ip)
		}
	}

	if err != nil {
		fmt.Println("Error:", err)
		return result
	}
	return result
}

func GetClusterExternalIps() []string {
	var result []string = []string{}
	kubeProvider := NewKubeProvider()
	labelSelector := "app.kubernetes.io/component=controller,app.kubernetes.io/name=ingress-nginx"
	services, err := kubeProvider.ClientSet.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})

	// check if traefik is used
	if services != nil && len(services.Items) <= 0 {
		labelSelector := "app.kubernetes.io/name=traefik"
		services, err = kubeProvider.ClientSet.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	}

	for _, service := range services.Items {
		for _, ingress := range service.Status.LoadBalancer.Ingress {
			fmt.Println(ingress.IP)
			result = append(result, ingress.IP)
		}
	}

	if err != nil {
		fmt.Println("Error:", err)
		return result
	}
	return result
}

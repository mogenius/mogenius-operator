package kubernetes

import (
	"context"

	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllIngressClasses() []v1.IngressClass {
	result := []v1.IngressClass{}

	clientset := clientProvider.K8sClientSet()
	ingressList, err := clientset.NetworkingV1().IngressClasses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		k8sLogger.Error("AllIngressClasses", "error", err.Error())
		return result
	}

	for _, ingress := range ingressList.Items {
		ingress.Kind = "IngressClass"
		ingress.APIVersion = "networking.k8s.io/v1"
		result = append(result, ingress)
	}

	return result
}

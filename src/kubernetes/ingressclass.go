package kubernetes

import (
	"context"

	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllIngressClasses() []v1.IngressClass {
	result := []v1.IngressClass{}

	provider, err := NewKubeProvider()
	if err != nil {
		return result
	}
	ingressList, err := provider.ClientSet.NetworkingV1().IngressClasses().List(context.TODO(), metav1.ListOptions{})
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

func GetK8sIngressClass(name string) (*v1.IngressClass, error) {
	provider, err := NewKubeProvider()
	if err != nil {
		return nil, err
	}
	ingress, err := provider.ClientSet.NetworkingV1().IngressClasses().Get(context.TODO(), name, metav1.GetOptions{})
	ingress.Kind = "IngressClass"
	ingress.APIVersion = "networking.k8s.io/v1"

	return ingress, err
}

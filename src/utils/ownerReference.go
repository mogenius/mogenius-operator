package utils

import (
	"context"
	"mogenius-k8s-manager/src/assert"
	cfg "mogenius-k8s-manager/src/config"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

func GetOwnDeploymentOwnerReference(clientset *kubernetes.Clientset, config cfg.ConfigModule) ([]metav1.OwnerReference, error) {

	ownDeploymentName := config.Get("OWN_DEPLOYMENT_NAME")
	assert.Assert(ownDeploymentName != "")

	namespace := config.Get("MO_OWN_NAMESPACE")
	assert.Assert("MO_OWN_NAMESPACE" != "")

	ownDeployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), ownDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	reference := []metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       ownDeployment.GetName(),
			UID:        ownDeployment.GetUID(),
			Controller: ptr.To(true),
		},
	}

	return reference, nil
}

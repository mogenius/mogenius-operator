package crds

import (
	"k8s.io/client-go/rest"
	mogeniusclient "mogenius-k8s-manager/src/crds/client"
)

type MogeniusClientSet struct {
	MogeniusV1alpha1 *mogeniusclient.MogeniusV1alpha1
}

func NewMogeniusClientSet(config rest.Config) (*MogeniusClientSet, error) {
	clientset := new(MogeniusClientSet)

	mogeniusV1alpha1, err := mogeniusclient.NewMogeniusV1alpha1(config)
	if err != nil {
		return nil, err
	}
	clientset.MogeniusV1alpha1 = mogeniusV1alpha1

	return clientset, nil
}

package client

import (
	"k8s.io/client-go/rest"
	mogeniuscrdsv1alpha1 "mogenius-k8s-manager/src/crds/v1alpha1"
)

type MogeniusV1alpha1 struct {
	restClient *rest.RESTClient
	config     rest.Config
}

func NewMogeniusV1alpha1(config rest.Config) (*MogeniusV1alpha1, error) {
	config.APIPath = "/apis"
	config.ContentConfig.GroupVersion = &mogeniuscrdsv1alpha1.GroupVersion

	v1alpha1client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	self := new(MogeniusV1alpha1)
	self.restClient = v1alpha1client
	self.config = config

	return self, nil
}

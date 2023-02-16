package dtos

import (
	v1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

type NamespaceResourcesDto struct {
	Pods        []core.Pod       `json:"pods"`
	Services    []core.Service   `json:"services"`
	Deployments []v1.Deployment  `json:"deployments"`
	Daemonsets  []v1.DaemonSet   `json:"daemonsets"`
	Replicasets []v1.ReplicaSet  `json:"replicasets"`
	Ingresses   []netv1.Ingress  `json:"ingresses"`
	Secrets     []core.Secret    `json:"secrets"`
	Configmaps  []core.ConfigMap `json:"configmaps"`
}

func CreateNamespaceResourcesDto() NamespaceResourcesDto {
	return NamespaceResourcesDto{
		Pods:        []core.Pod{},
		Services:    []core.Service{},
		Deployments: []v1.Deployment{},
		Daemonsets:  []v1.DaemonSet{},
		Replicasets: []v1.ReplicaSet{},
		Ingresses:   []netv1.Ingress{},
		Secrets:     []core.Secret{},
		Configmaps:  []core.ConfigMap{},
	}
}

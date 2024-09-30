package controllers

import "mogenius-k8s-manager/dtos"

type CreateLabeledNetworkPolicyRequest struct {
	Namespace                  dtos.K8sNamespaceDto               `json:"namespace" validate:"required"`
	LabeledNetworkPolicyParams dtos.K8sLabeledNetworkPolicyParams `json:"labeledNetworkPolicyParams" validate:"required"`
}

type CreateLabeledNetworkPolicyResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
}

type ListLabeledNetworkPolicyPortsResponse struct {
	Ports []dtos.K8sPortsDto `json:"ports"`

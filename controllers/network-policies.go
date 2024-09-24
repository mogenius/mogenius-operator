package controllers

import "mogenius-k8s-manager/dtos"

type CreateLabeledNetworkPolicyRequest struct {
	// Secrets stores are bound to a projects,
	// so that customers can decide which team controls which secrets
	Namespace                  dtos.K8sNamespaceDto               `json:"namespace" validate:"required"`
	LabeledNetworkPolicyParams dtos.K8sLabeledNetworkPolicyParams `json: "labeledNetworkPolicyParams" validate:"required"`
}

type CreateLabeledNetworkPolicyResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
}

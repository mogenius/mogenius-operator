package controllers

import (
	mokubernetes "mogenius-k8s-manager/kubernetes"
	servicesExternal "mogenius-k8s-manager/services-external"
	"mogenius-k8s-manager/utils"

	"github.com/mogenius/punq/logger"
)

type CreateSecretsStoreRequest struct {
	// Secrets stores are bound to a projects,
	// so that customers can decide which team controls which secrets
	ProjectName string `json:"projectName" validate:"required"`
	// customers might want to create multiple stores	and should have IDs to differentiate
	NamePrefix     string `json:"namePrefix" validate:"required"`
	Role           string `json:"role" validate:"required"`
	VaultServerUrl string `json:"vaultServerUrl" validate:"required"`
	MoSharedPath   string `json:"moSharedPath" validate:"required"`
}

func CreateSecretsStoreRequestExample() CreateSecretsStoreRequest {
	return CreateSecretsStoreRequest{
		ProjectName:    "phoenix",
		NamePrefix:     "mo-test",
		Role:           "mogenius-external-secrets",
		VaultServerUrl: "http://vault.default.svc.cluster.local:8200",
		MoSharedPath:   "mogenius-external-secrets",
	}
}

type CreateSecretsStoreResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
}

type ListSecretsStoresResponse struct {
	StoresInCluster []string `json:"storesInCluster"`
}
type ListSecretsRequest struct {
	NamePrefix  string `json:"namePrefix" validate:"required"`
	ProjectName string `json:"projectName" validate:"required"`
}
type ListSecretsResponse struct {
	SecretsInProject []string `json:"secretsInProject"`
}
type DeleteSecretsStoreRequest struct {
	NamePrefix   string `json:"namePrefix" validate:"required"`
	ProjectName  string `json:"projectName" validate:"required"`
	MoSharedPath string `json:"moSharedPath" validate:"required"`
}

func DeleteSecretsStoreRequestExample() DeleteSecretsStoreRequest {
	return DeleteSecretsStoreRequest{
		NamePrefix:  "mo-test",
		ProjectName: "phoenix",
	}
}

type DeleteSecretsStoreResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
}

func newExternalSecretStoreProps(data CreateSecretsStoreRequest) servicesExternal.ExternalSecretStoreProps {
	return servicesExternal.ExternalSecretStoreProps{
		ProjectName:    data.ProjectName,
		Role:           data.Role,
		VaultServerUrl: data.VaultServerUrl,
		MoSharedPath:   data.MoSharedPath,
		NamePrefix:     data.NamePrefix,
		Name:           utils.GetSecretStoreName(data.NamePrefix, data.ProjectName),
		ServiceAccount: utils.GetServiceAccountName(data.MoSharedPath),
	}
}

func CreateExternalSecretStore(data CreateSecretsStoreRequest) CreateSecretsStoreResponse {
	err := servicesExternal.CreateExternalSecretsStore(newExternalSecretStoreProps(data))
	if err != nil {
		return CreateSecretsStoreResponse{
			Status:       "ERROR",
			ErrorMessage: err.Error(),
		}
	}
	return CreateSecretsStoreResponse{
		Status: "SUCCESS",
	}
}

func ListExternalSecretsStores() ListSecretsStoresResponse {
	stores, err := mokubernetes.ListExternalSecretsStores()
	if err != nil {
		logger.Log.Error("Getting secret stores failed with error: %v", err)
	}
	return ListSecretsStoresResponse{
		StoresInCluster: stores,
	}
}

func ListAvailableExternalSecrets(data ListSecretsRequest) ListSecretsResponse {
	availSecrets := servicesExternal.ListAvailableExternalSecrets(data.NamePrefix, data.ProjectName)
	if availSecrets == nil {
		logger.Log.Error("Getting available secrets failed")
		return ListSecretsResponse{}
	}
	return ListSecretsResponse{
		SecretsInProject: availSecrets,
	}
}

func DeleteExternalSecretsStore(data DeleteSecretsStoreRequest) DeleteSecretsStoreResponse {
	err := servicesExternal.DeleteExternalSecretsStore(data.NamePrefix, data.ProjectName, data.MoSharedPath)
	if err != nil {
		logger.Log.Error("Deleting secret store failed with error: %v", err)
		return DeleteSecretsStoreResponse{
			Status:       "ERROR",
			ErrorMessage: err.Error(),
		}
	}
	return DeleteSecretsStoreResponse{
		Status: "SUCCESS",
	}
}

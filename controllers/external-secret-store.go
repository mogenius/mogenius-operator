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
	DisplayName    string `json:"displayName" validate:"required"`
	ProjectId      string `json:"projectId" validate:"required"`
	Role           string `json:"role" validate:"required"`
	VaultServerUrl string `json:"vaultServerUrl" validate:"required"`
	SecretPath     string `json:"secretPath" validate:"required"`
}

func CreateSecretsStoreRequestExample() CreateSecretsStoreRequest {
	return CreateSecretsStoreRequest{
		DisplayName:    "Vault Secret Store 1",
		ProjectId:      "234jhkl-lklj-234lkj-234lkj",
		Role:           "mogenius-external-secrets",
		VaultServerUrl: "http://vault.default.svc.cluster.local:8200",
		SecretPath:     "mogenius-external-secrets/data/phoenix",
	}
}

type CreateSecretsStoreResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
}

type ListSecretStoresRequest struct {
	ProjectId string `json:"projectId" validate:"required"`
}

type ListSecretsRequest struct {
	NamePrefix string `json:"namePrefix" validate:"required"`
}

type DeleteSecretsStoreRequest struct {
	Name string `json:"name" validate:"required"`
}

func DeleteSecretsStoreRequestExample() DeleteSecretsStoreRequest {
	return DeleteSecretsStoreRequest{
		Name: "s78fdsf78a-vault-secret-store",
	}
}

type DeleteSecretsStoreResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
}

func newExternalSecretStoreProps(data CreateSecretsStoreRequest) servicesExternal.ExternalSecretStoreProps {
	return servicesExternal.ExternalSecretStoreProps{
		DisplayName:    data.DisplayName,
		ProjectId:      data.ProjectId,
		Role:           data.Role,
		VaultServerUrl: data.VaultServerUrl,
		SecretPath:     data.SecretPath,
		ServiceAccount: utils.GetServiceAccountName(data.SecretPath),
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

func ListExternalSecretsStores(data ListSecretStoresRequest) []mokubernetes.SecretStore {
	stores, err := mokubernetes.ListExternalSecretsStores(data.ProjectId)
	if err != nil {
		logger.Log.Error("Getting secret stores failed with error: %v", err)
	}
	return stores
}

func ListAvailableExternalSecrets(data ListSecretsRequest) []string {
	availSecrets := servicesExternal.ListAvailableExternalSecrets(data.NamePrefix)
	if availSecrets == nil {
		logger.Log.Error("Getting available secrets failed")
		return []string{}
	}
	return availSecrets
}

func DeleteExternalSecretsStore(data DeleteSecretsStoreRequest) DeleteSecretsStoreResponse {
	err := servicesExternal.DeleteExternalSecretsStore(data.Name)
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

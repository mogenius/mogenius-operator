package controllers

import (
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	servicesExternal "mogenius-k8s-manager/src/servicesexternal"
	"mogenius-k8s-manager/src/utils"
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
		controllerLogger.Error("Getting secret stores failed", "error", err)
	}
	return stores
}

func ListAvailableExternalSecrets(data ListSecretsRequest) []string {
	availSecrets := servicesExternal.ListAvailableExternalSecrets(data.NamePrefix)
	if availSecrets == nil {
		controllerLogger.Error("Getting available secrets failed")
		return []string{}
	}
	return availSecrets
}

func DeleteExternalSecretsStore(data DeleteSecretsStoreRequest) DeleteSecretsStoreResponse {
	err := servicesExternal.DeleteExternalSecretsStore(data.Name)
	if err != nil {
		controllerLogger.Error("Deleting secret store failed", "error", err)
		return DeleteSecretsStoreResponse{
			Status:       "ERROR",
			ErrorMessage: err.Error(),
		}
	}
	return DeleteSecretsStoreResponse{
		Status: "SUCCESS",
	}
}

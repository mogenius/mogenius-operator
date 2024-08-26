package controllers

import (
	mokubernetes "mogenius-k8s-manager/kubernetes"
	servicesExternal "mogenius-k8s-manager/services-external"
	"mogenius-k8s-manager/utils"

	"github.com/mogenius/punq/logger"
)

// curl --header "X-Vault-Token: root" --request GET http://vault.vault:8200/v1/sys/mounts
type CreateSecretsStoreRequest struct {
	// Secrets stores are bound to a projects,
	// so that customers can decide which team controls which secrets
	ProjectId string `json:"projectId" validate:"required"` // api
	// customers might want to create multiple stores	and should have IDs to differentiate
	NamePrefix     string `json:"namePrefix" validate:"required"`
	Role           string `json:"role" validate:"required"`
	VaultServerUrl string `json:"vaultServerUrl" validate:"required"`
	MoSharedPath   string `json:"moSharedPath" validate:"required"`
}

func CreateSecretsStoreRequestExample() CreateSecretsStoreRequest {
	return CreateSecretsStoreRequest{
		ProjectId:      "e9ff3afc-83bd-4fae-a93e-4eaeea453b",
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
	Stores []string `json:"stores"`
}
type ListSecretStoresRequest struct {
	ProjectId string `json:"projectId" validate:"required"`
}

type ListSecretsRequest struct {
	Name string `json:"name" validate:"required"`
	//NamePrefix  string `json:"namePrefix" validate:"required"`
	//ProjectName string `json:"projectName" validate:"required"`
}
type ListSecretsResponse struct {
	SecretsInProject []string `json:"secretsInProject"`
}
type DeleteSecretsStoreRequest struct {
	Name string `json:"name" validate:"required"`
	//NamePrefix   string `json:"namePrefix" validate:"required"`
	//ProjectName  string `json:"projectName" validate:"required"`
	//MoSharedPath string `json:"moSharedPath" validate:"required"`
}

func DeleteSecretsStoreRequestExample() DeleteSecretsStoreRequest {
	return DeleteSecretsStoreRequest{
		Name: "mo-test-e9ff3afc-83bd-4fae-a93e-4eaeea453b-vault-secret-list",
		//NamePrefix:  "mo-test",
		//ProjectName: "phoenix",
	}
}

type DeleteSecretsStoreResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
}

func newExternalSecretStoreProps(data CreateSecretsStoreRequest) servicesExternal.ExternalSecretStoreProps {
	return servicesExternal.ExternalSecretStoreProps{
		ProjectId:      data.ProjectId,
		Role:           data.Role,
		VaultServerUrl: data.VaultServerUrl,
		MoSharedPath:   data.MoSharedPath,
		NamePrefix:     data.NamePrefix,
		Name:           utils.GetSecretStoreName(data.NamePrefix, data.ProjectId),
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

func ListExternalSecretsStores(data ListSecretStoresRequest) ListSecretsStoresResponse {
	stores, err := mokubernetes.ListExternalSecretsStores(data.ProjectId)
	if err != nil {
		logger.Log.Error("Getting secret stores failed with error: %v", err)
	}
	return ListSecretsStoresResponse{
		Stores: stores,
	}
}

func ListAvailableExternalSecrets(data ListSecretsRequest) ListSecretsResponse {
	availSecrets := servicesExternal.ListAvailableExternalSecrets(data.Name)
	if availSecrets == nil {
		logger.Log.Error("Getting available secrets failed")
		return ListSecretsResponse{}
	}
	return ListSecretsResponse{
		SecretsInProject: availSecrets,
	}
}

func DeleteExternalSecretsStore(data DeleteSecretsStoreRequest) DeleteSecretsStoreResponse {
	// err := servicesExternal.DeleteExternalSecretsStore(data.NamePrefix, data.ProjectName, data.MoSharedPath)
	// TODO
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

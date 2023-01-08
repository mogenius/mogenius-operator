package services

import "mogenius-k8s-manager/dtos"

func BuildInfo(r BuildInfoRequest) []dtos.CiCdPipelineDto {
	// TODO: Implement
	return []dtos.CiCdPipelineDto{}
}

func BuildInfoArray(r BuildInfoArrayRequest) interface{} {
	// TODO: Implement
	return nil
}

func BuildLog(r BuildLogRequest) dtos.NamespaceAzureBuildLogDto {
	// TODO: Implement
	return dtos.NamespaceAzureBuildLogDto{}
}

// cicd/build-info GET
type BuildInfoRequest struct {
	BuildId int `json:"buildId"`
}

// cicd/build-info-array POST
type BuildInfoArrayRequest struct {
	BuildIds []int `json:"buildIds"`
}

// cicd/build-log GET
type BuildLogRequest struct {
	BuildId int `json:"buildId"`
	LogId   int `json:"logId"`
}

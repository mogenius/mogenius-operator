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

// GET cicd/build-info
type BuildInfoRequest struct {
	BuildId int `json:"buildId"`
}

// POST cicd/build-info-array
type BuildInfoArrayRequest struct {
	BuildIds []int `json:"buildIds"`
}

// GET cicd/build-log
type BuildLogRequest struct {
	BuildId int `json:"buildId"`
	LogId   int `json:"logId"`
}

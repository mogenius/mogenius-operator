package services

import (
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
)

func BuildInfo(r BuildInfoRequest) []dtos.CiCdPipelineDto {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return []dtos.CiCdPipelineDto{}
}

func BuildInfoArray(r BuildInfoArrayRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func BuildLog(r BuildLogRequest) dtos.NamespaceAzureBuildLogDto {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
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

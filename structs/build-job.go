package structs

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/utils"
	"time"

	punq "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"

	jsoniter "github.com/json-iterator/go"
)

type ScanImageRequest struct {
	ProjectId             string  `json:"projectId" validate:"required"`
	NamespaceId           string  `json:"namespaceId" validate:"required"`
	NamespaceName         string  `json:"namespaceName" validate:"required"`
	ServiceId             string  `json:"serviceId" validate:"required"`
	ControllerName        string  `json:"controllerName" validate:"required"`
	ContainerName         string  `json:"containerName"`
	ContainerImage        string  `json:"containerImage"`
	ContainerRegistryPath *string `json:"containerRegistryPath"`
	ContainerRegistryUrl  *string `json:"containerRegistryUrl"`
	ContainerRegistryUser *string `json:"containerRegistryUser"`
	ContainerRegistryPat  *string `json:"containerRegistryPat"`
}

func (s *ScanImageRequest) AddSecretsToRedaction() {
	utils.AddSecret(s.ContainerRegistryUser)
	utils.AddSecret(s.ContainerRegistryPat)
}

func ScanImageRequestExample() ScanImageRequest {
	return ScanImageRequest{
		ProjectId:             "6dbd5930-e3f0-4594-9888-2003c6325f9a",
		NamespaceId:           "32a399ba-3a48-462b-8293-11b667d3a1fa",
		NamespaceName:         "docker-desktop-prod-8ds57s",
		ServiceId:             "ef7af4d2-8939-4c94-bbe1-a3e7018e8306",
		ControllerName:        "alpinetest",
		ContainerName:         "alpinetest-container",
		ContainerImage:        "mysql:latest",
		ContainerRegistryPath: punqUtils.Pointer("docker.io/biltisberger"),
		ContainerRegistryUrl:  punqUtils.Pointer("docker.io"),
		ContainerRegistryUser: nil,
		ContainerRegistryPat:  nil,
	}
}

type BuildJob struct {
	JobId          string            `json:"jobId"`
	StartTimestamp string            `json:"startTimestamp"`
	EndTimestamp   string            `json:"endTimestamp"`
	State          punq.JobStateEnum `json:"state"`
	StartedAt      string            `json:"startedAt"`
	DurationMs     int               `json:"durationMs"`
	BuildId        uint64            `json:"buildId"`

	Project   dtos.K8sProjectDto   `json:"project" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
	Service   dtos.K8sServiceDto   `json:"service" validate:"required"`
}

func (b BuildJob) IsEmpty() bool {
	return b.JobId == "" &&
		b.StartTimestamp == "" &&
		b.EndTimestamp == "" &&
		b.State == "" &&
		b.StartedAt == "" &&
		b.DurationMs == 0 &&
		b.BuildId == 0
}

func BuildJobFrom(jobId string, scanRequest ScanImageRequest) BuildJob {
	return BuildJob{
		JobId: jobId,
		Project: dtos.K8sProjectDto{
			Id: scanRequest.ProjectId,
		},
		Namespace: dtos.K8sNamespaceDto{
			Id:   scanRequest.NamespaceId,
			Name: scanRequest.NamespaceName,
		},
		Service: dtos.K8sServiceDto{
			Id:             scanRequest.ServiceId,
			ControllerName: scanRequest.ControllerName,
		},
	}
}

// type BuildJobListEntry struct {
// 	// only "clusterId" was removed because we dont need it anymore
// 	JobId                 string `json:"jobId"`
// 	ProjectId             string `json:"projectId"`
// 	NamespaceId           string `json:"namespaceId"`
// 	Namespace             string `json:"namespace"`
// 	ServiceId             string `json:"serviceId"`
// 	ControllerName        string `json:"controllerName"`
// 	GitRepo               string `json:"gitRepo"`
// 	GitBranch             string `json:"gitBranch"`
// 	GitCommitAuthor       string `json:"gitCommitAuthor"`
// 	GitCommitHash         string `json:"gitCommitHash"`
// 	GitCommitMessage      string `json:"gitCommitMessage"`
// 	DockerFile            string `json:"dockerFile"`
// 	DockerContext         string `json:"dockerContext"`
// 	ContainerRegistryPath string `json:"containerRegistryPath"`
// 	ContainerRegistryUrl  string `json:"containerRegistryUrl"`
// 	StartTimestamp        string `json:"startTimestamp"`
// 	InjectDockerEnvVars   string `json:"injectDockerEnvVars"`
// 	State                 string `json:"state"` // FAILED, SUCCEEDED, STARTED, PENDING
// 	StartedAt             string `json:"startedAt"`
// 	DurationMs            int    `json:"durationMs"`
// 	BuildId               int    `json:"buildId"`
// }

// func (b BuildJobListEntry) IsEmpty() bool {
// 	return b.JobId == "" &&
// 		b.ProjectId == "" &&
// 		b.NamespaceId == "" &&
// 		b.Namespace == "" &&
// 		b.ServiceId == "" &&
// 		b.ControllerName == "" &&
// 		b.GitRepo == "" &&
// 		b.GitBranch == "" &&
// 		b.GitCommitAuthor == "" &&
// 		b.GitCommitHash == "" &&
// 		b.GitCommitMessage == "" &&
// 		b.DockerFile == "" &&
// 		b.DockerContext == "" &&
// 		b.ContainerRegistryPath == "" &&
// 		b.ContainerRegistryUrl == "" &&
// 		b.StartTimestamp == "" &&
// 		b.InjectDockerEnvVars == "" &&
// 		b.State == "" &&
// 		b.StartedAt == "" &&
// 		b.DurationMs == 0 &&
// 		b.BuildId == 0
// }

func BuildJobExample() BuildJob {
	return BuildJob{
		JobId: "na8ggegq2p0pepbvjldlger",
		// ProjectId:             "6dbd5930-e3f0-4594-9888-2003c6325f9a",
		// NamespaceId:           "32a399ba-3a48-462b-8293-11b667d3a1fa",
		// Namespace:             "docker-desktop-prod-8ds57s",
		// ServiceId:             "ef7af4d2-8939-4c94-bbe1-a3e7018e8306",
		// ControllerName:        "alpinetest",
		// GitRepo:               "https://x-access-token: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@github.com/beneiltis/bene.git",
		// GitBranch:             "main",
		// GitCommitAuthor:  "mogenius git-user",
		// GitCommitHash:    "abe52a64e682cedf77f131e595119f6c2f6a1c84",
		// GitCommitMessage: "[skip ci]: Add initial files.",
		// DockerFile:            "Dockerfile",
		// DockerContext:         ".",
		// ContainerRegistryPath: "docker.io/biltisberger",
		// ContainerRegistryUser: "biltisberger",
		// ContainerRegistryPat:  "YYY",
		// ContainerRegistryUrl:  "docker.io",
		StartTimestamp: "1689684071841",
		// InjectDockerEnvVars:   "--build-arg PLACEHOLDER=MOGENIUS",
		State:        punq.JobStatePending,
		StartedAt:    time.Now().Format(time.RFC3339),
		EndTimestamp: time.Now().Format(time.RFC3339),
		DurationMs:   0,
		BuildId:      1,
	}
}

type BuildJobStatusRequest struct {
	BuildId uint64 `json:"buildId" validate:"required"`
}

type LastBuildTaskListRequest struct {
	Namespace  string `json:"namespace" validate:"required"`
	Controller string `json:"controller" validate:"required"`
	Container  string `json:"container" validate:"required"`
}

func BuildJobStatusRequestExample() BuildJobStatusRequest {
	return BuildJobStatusRequest{
		BuildId: 1234,
	}
}

type BuildServicesStatusRequest struct {
	ServiceIds []string `json:"serviceIds" validate:"required"`
	MaxResults int      `json:"maxResults" validate:"required"`
}

func BuildServicesStatusRequestExample() BuildServicesStatusRequest {
	return BuildServicesStatusRequest{
		ServiceIds: []string{"YYY", "ef7af4d2-8939-4c94-bbe1-a3e7018e8306", "ZZZ"},
		MaxResults: 14,
	}
}

type BuildServiceRequest struct {
	ServiceId  string `json:"serviceId" validate:"required"`
	MaxResults int    `json:"maxResults,omitempty"`
}

func BuildServiceRequestExample() BuildServiceRequest {
	return BuildServiceRequest{
		ServiceId:  "ef7af4d2-8939-4c94-bbe1-a3e7018e8306",
		MaxResults: 12,
	}
}

type ListBuildByProjectIdRequest struct {
	ProjectId string `json:"projectId" validate:"required"`
}

func ListBuildByProjectIdRequestExample() ListBuildByProjectIdRequest {
	return ListBuildByProjectIdRequest{
		ProjectId: "6dbd5930-e3f0-4594-9888-2003c6325f9a",
	}
}

type BuildAddResult struct {
	BuildId int `json:"buildId"`
}

type BuildScanResult struct {
	Result *BuildJobInfoEntry `json:"result"`
	Error  *string            `json:"error"`
}

func CreateBuildScanResult(message string, err string) BuildScanResult {
	result := BuildScanResult{
		Result: &BuildJobInfoEntry{
			State:      punq.JobStatePending,
			Result:     message,
			StartTime:  time.Now().Format(time.RFC3339),
			FinishTime: "",
		},
		Error: &err,
	}
	if message == "" {
		result.Result = nil
	}
	return result
}

type BuildCancelResult struct {
	Result string `json:"result"`
	Error  string `json:"error"`
}

type BuildDeleteResult struct {
	Result string `json:"result"`
	Error  string `json:"error"`
}

type BuildJobInfos struct {
	BuildId    uint64 `json:"buildId"`
	ProjectId  string `json:"projectId"`
	Namespace  string `json:"namespace"`
	Controller string `json:"controller"`
	Container  string `json:"container"`

	StartTime  string `json:"startTime"`
	FinishTime string `json:"finishTime"`

	Clone BuildJobInfoEntry `json:"clone"`
	Ls    BuildJobInfoEntry `json:"ls"`
	Login BuildJobInfoEntry `json:"login"`
	Build BuildJobInfoEntry `json:"build"`
	Push  BuildJobInfoEntry `json:"push"`
}

type BuildPrefixEnum string

const (
	PrefixGitClone BuildPrefixEnum = "clone"
	PrefixLs       BuildPrefixEnum = "ls"
	PrefixLogin    BuildPrefixEnum = "login"
	PrefixBuild    BuildPrefixEnum = "build"
	PrefixPull     BuildPrefixEnum = "pull"
	PrefixPush     BuildPrefixEnum = "push"

	PrefixNone BuildPrefixEnum = "none"
)

type BuildJobInfoEntry struct {
	Prefix     BuildPrefixEnum `json:"prefix"`
	BuildId    uint64          `json:"buildId"`
	ProjectId  string          `json:"projectId"`
	Namespace  string          `json:"namespace"`
	Controller string          `json:"controller"`
	Container  string          `json:"container"`

	State      punq.JobStateEnum `json:"state"`
	Result     string            `json:"result"`
	StartTime  string            `json:"startTime"`
	FinishTime string            `json:"finishTime"`
}

type BuildScanImageEntry struct {
	Result    string `json:"result"`
	CreatedAt string `json:"createdAt"`
}

func BuildJobInfoEntryKey(prefix BuildPrefixEnum, buildId uint64, namespace string, controller string, container string) string {
	return fmt.Sprintf("%s___%s___%s___%s___%s", prefix, utils.SequenceToKey(buildId), namespace, controller, container)
}

func LastBuildJobInfosKeySuffix(namespace string, controller string, container string) string {
	return fmt.Sprintf("___%s___%s___%s", namespace, controller, container)
}

func GetBuildJobInfosPrefix(prefix BuildPrefixEnum, buildId uint64) string {
	return fmt.Sprintf("%s___%s___", prefix, utils.SequenceToKey(buildId))
}

func CreateBuildJobInfoEntryFromScanImageReq(req ScanImageRequest) BuildJobInfoEntry {
	return BuildJobInfoEntry{
		State:      punq.JobStatePending,
		Result:     "",
		StartTime:  time.Now().Format(time.RFC3339),
		FinishTime: "",
	}
}

func CreateBuildJobInfos(clone []byte, ls []byte, login []byte, build []byte, push []byte) BuildJobInfos {
	result := BuildJobInfos{}

	cloneEntity := CreateBuildJobEntryFromData(clone)
	lsEntity := CreateBuildJobEntryFromData(ls)
	loginEntity := CreateBuildJobEntryFromData(login)
	buildEntity := CreateBuildJobEntryFromData(build)
	pushEntity := CreateBuildJobEntryFromData(push)

	result.BuildId = cloneEntity.BuildId
	result.ProjectId = cloneEntity.ProjectId
	result.Namespace = cloneEntity.Namespace
	result.Controller = cloneEntity.Controller
	result.Container = cloneEntity.Container

	result.StartTime = cloneEntity.StartTime
	result.FinishTime = pushEntity.FinishTime

	//result.StartTime = job.StartedAt
	//result.FinishTime = job.EndTimestamp

	result.Clone = cloneEntity
	result.Ls = lsEntity
	result.Login = loginEntity
	result.Build = buildEntity
	result.Push = pushEntity

	return result
}

func CreateBuildJobEntryFromData(data []byte) BuildJobInfoEntry {
	result := BuildJobInfoEntry{}

	if data != nil {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		err := json.Unmarshal(data, &result)
		if err != nil {
			log.Errorf("createBuildJobEntryFromData ERR: %s", err.Error())
		}
	}

	return result
}

func CreateBuildJobInfoEntryBytes(
	state punq.JobStateEnum,
	cmdOutput string,
	startTime time.Time,
	finishTime time.Time,
	prefix BuildPrefixEnum,
	job *BuildJob,
	container *dtos.K8sContainerDto,
) []byte {
	entry := BuildJobInfoEntry{
		Prefix:     prefix,
		BuildId:    job.BuildId,
		ProjectId:  job.Project.Id,
		Namespace:  job.Namespace.Name,
		Controller: job.Service.ControllerName,
		Container:  container.Name,

		State:      state,
		Result:     cmdOutput,
		StartTime:  startTime.Format(time.RFC3339),
		FinishTime: finishTime.Format(time.RFC3339),
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	bytes, err := json.Marshal(entry)
	if err != nil {
		log.Errorf("createBuildJobInfoEntryBytes ERR: %s", err.Error())
	}
	return bytes

}

func CreateScanImageEntryBytes(
	cmdOutput string,
) []byte {
	entry := BuildScanImageEntry{
		Result:    cmdOutput,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	bytes, err := json.Marshal(entry)
	if err != nil {
		log.Errorf("createBuildJobInfoEntryBytes ERR: %s", err.Error())
	}
	return bytes

}

func CreateBuildJobInfoEntryBytesForScan(state punq.JobStateEnum, cmdOutput []byte, startTime time.Time, finishTime time.Time) []byte {
	entry := BuildJobInfoEntry{
		State:      state,
		Result:     string(cmdOutput),
		StartTime:  startTime.Format(time.RFC3339),
		FinishTime: finishTime.Format(time.RFC3339),
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	bytes, err := json.Marshal(entry)
	if err != nil {
		log.Errorf("CreateBuildJobInfoEntryBytesForScan ERR: %s", err.Error())
	}
	return bytes

}

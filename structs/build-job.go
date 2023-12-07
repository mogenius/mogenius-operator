package structs

import (
	"mogenius-k8s-manager/logger"
	"time"

	jsoniter "github.com/json-iterator/go"
)

const (
	BUILD_STATE_FAILED    string = "FAILED"
	BUILD_STATE_SUCCEEDED string = "SUCCEEDED"
	BUILD_STATE_STARTED   string = "STARTED"
	BUILD_STATE_PENDING   string = "PENDING"
	BUILD_STATE_CANCELED  string = "CANCELED"
	BUILD_STATE_TIMEOUT   string = "TIMEOUT"
)

type ScanImageRequest struct {
	ProjectId             string `json:"projectId"`
	NamespaceId           string `json:"namespaceId"`
	ServiceId             string `json:"serviceId"`
	NamespaceName         string `json:"namespaceName"`
	ServiceName           string `json:"serviceName"`
	ContainerImage        string `json:"containerImage"`
	ContainerRegistryUser string `json:"containerRegistryUser"`
	ContainerRegistryPat  string `json:"containerRegistryPat"`
	ContainerRegistryUrl  string `json:"containerRegistryUrl"`
}

func ScanImageRequestExample() ScanImageRequest {
	return ScanImageRequest{
		ProjectId:             "6dbd5930-e3f0-4594-9888-2003c6325f9a",
		NamespaceId:           "32a399ba-3a48-462b-8293-11b667d3a1fa",
		ServiceId:             "ef7af4d2-8939-4c94-bbe1-a3e7018e8306",
		NamespaceName:         "mac-prod-1xh4p1",
		ServiceName:           "angular1",
		ContainerImage:        "mysql:latest",
		ContainerRegistryUser: "",
		ContainerRegistryPat:  "",
		ContainerRegistryUrl:  "docker.io",
	}
}

type BuildJob struct {
	// only "clusterId" was removed because we dont need it anymore
	JobId                 string `json:"jobId"`
	ProjectId             string `json:"projectId"`
	NamespaceId           string `json:"namespaceId"`
	Namespace             string `json:"namespace"`
	ServiceId             string `json:"serviceId"`
	ServiceName           string `json:"serviceName"`
	GitRepo               string `json:"gitRepo"`
	GitBranch             string `json:"gitBranch"`
	GitCommitAuthor       string `json:"gitCommitAuthor"`
	GitCommitHash         string `json:"gitCommitHash"`
	GitCommitMessage      string `json:"gitCommitMessage"`
	DockerFile            string `json:"dockerFile"`
	DockerContext         string `json:"dockerContext"`
	ContainerRegistryPath string `json:"containerRegistryPath"`
	ContainerRegistryUser string `json:"containerRegistryUser"`
	ContainerRegistryPat  string `json:"containerRegistryPat"`
	ContainerRegistryUrl  string `json:"containerRegistryUrl"`
	StartTimestamp        string `json:"startTimestamp"`
	EndTimestamp          string `json:"endTimestamp"`
	InjectDockerEnvVars   string `json:"injectDockerEnvVars"`
	State                 string `json:"state"` // FAILED, SUCCEEDED, STARTED, PENDING, TIMEOUT
	StartedAt             string `json:"startedAt"`
	DurationMs            int    `json:"durationMs"`
	BuildId               int    `json:"buildId"`
}

type BuildJobListEntry struct {
	// only "clusterId" was removed because we dont need it anymore
	JobId                 string `json:"jobId"`
	ProjectId             string `json:"projectId"`
	NamespaceId           string `json:"namespaceId"`
	Namespace             string `json:"namespace"`
	ServiceId             string `json:"serviceId"`
	ServiceName           string `json:"serviceName"`
	GitRepo               string `json:"gitRepo"`
	GitBranch             string `json:"gitBranch"`
	GitCommitAuthor       string `json:"gitCommitAuthor"`
	GitCommitHash         string `json:"gitCommitHash"`
	GitCommitMessage      string `json:"gitCommitMessage"`
	DockerFile            string `json:"dockerFile"`
	DockerContext         string `json:"dockerContext"`
	ContainerRegistryPath string `json:"containerRegistryPath"`
	ContainerRegistryUrl  string `json:"containerRegistryUrl"`
	StartTimestamp        string `json:"startTimestamp"`
	InjectDockerEnvVars   string `json:"injectDockerEnvVars"`
	State                 string `json:"state"` // FAILED, SUCCEEDED, STARTED, PENDING
	StartedAt             string `json:"startedAt"`
	DurationMs            int    `json:"durationMs"`
	BuildId               int    `json:"buildId"`
}

func BuildJobExample() BuildJob {
	return BuildJob{
		JobId:                 "na8ggegq2p0pepbvjldlger",
		ProjectId:             "6dbd5930-e3f0-4594-9888-2003c6325f9a",
		NamespaceId:           "32a399ba-3a48-462b-8293-11b667d3a1fa",
		Namespace:             "docker-desktop-prod-8ds57s",
		ServiceId:             "ef7af4d2-8939-4c94-bbe1-a3e7018e8306",
		ServiceName:           "alpinetest",
		GitRepo:               "https://x-access-token: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@github.com/beneiltis/bene.git",
		GitBranch:             "main",
		GitCommitAuthor:       "mogenius git-user",
		GitCommitHash:         "abe52a64e682cedf77f131e595119f6c2f6a1c84",
		GitCommitMessage:      "[skip ci]: Add initial files.",
		DockerFile:            "Dockerfile",
		DockerContext:         ".",
		ContainerRegistryPath: "docker.io/biltisberger",
		ContainerRegistryUser: "biltisberger",
		ContainerRegistryPat:  "XXXX",
		ContainerRegistryUrl:  "docker.io",
		StartTimestamp:        "1689684071841",
		InjectDockerEnvVars:   "--build-arg PLACEHOLDER=MOGENIUS",
		State:                 BUILD_STATE_PENDING,
		StartedAt:             time.Now().Format(time.RFC3339),
		EndTimestamp:          time.Now().Format(time.RFC3339),
		DurationMs:            0,
		BuildId:               1,
	}
}

type BuildJobStatusRequest struct {
	BuildId int `json:"buildId"`
}

func BuildJobStatusRequestExample() BuildJobStatusRequest {
	return BuildJobStatusRequest{
		BuildId: 1234,
	}
}

type BuildServicesStatusRequest struct {
	ServiceIds []string `json:"serviceIds"`
	MaxResults int      `json:"maxResults"`
}

func BuildServicesStatusRequestExample() BuildServicesStatusRequest {
	return BuildServicesStatusRequest{
		ServiceIds: []string{"XXX", "ef7af4d2-8939-4c94-bbe1-a3e7018e8306", "ZZZ"},
		MaxResults: 14,
	}
}

type BuildServiceRequest struct {
	ServiceId  string `json:"serviceId"`
	MaxResults int    `json:"maxResults,omitempty"`
}

func BuildServiceRequestExample() BuildServiceRequest {
	return BuildServiceRequest{
		ServiceId:  "ef7af4d2-8939-4c94-bbe1-a3e7018e8306",
		MaxResults: 12,
	}
}

type ListBuildByProjectIdRequest struct {
	ProjectId string `json:"projectId"`
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
			State:      BUILD_STATE_PENDING,
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
	BuildId    int               `json:"buildId"`
	Clone      BuildJobInfoEntry `json:"clone"`
	Ls         BuildJobInfoEntry `json:"ls"`
	Login      BuildJobInfoEntry `json:"login"`
	Build      BuildJobInfoEntry `json:"build"`
	Push       BuildJobInfoEntry `json:"push"`
	Scan       BuildJobInfoEntry `json:"scan"`
	StartTime  string            `json:"startTime"`
	FinishTime string            `json:"finishTime"`
}

type BuildJobInfoEntry struct {
	ProjectId   string `json:"projectId,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
	State       string `json:"state"`
	Result      string `json:"result"`
	StartTime   string `json:"startTime"`
	FinishTime  string `json:"finishTime"`
}

func CreateBuildJobInfos(job BuildJob, clone []byte, ls []byte, login []byte, build []byte, push []byte, scan []byte) BuildJobInfos {
	result := BuildJobInfos{}

	result.BuildId = job.BuildId
	result.StartTime = job.StartedAt
	result.FinishTime = job.EndTimestamp
	result.Clone = CreateBuildJobEntryFromData(clone)
	result.Ls = CreateBuildJobEntryFromData(ls)
	result.Login = CreateBuildJobEntryFromData(login)
	result.Build = CreateBuildJobEntryFromData(build)
	result.Push = CreateBuildJobEntryFromData(push)
	result.Scan = CreateBuildJobEntryFromData(scan)

	return result
}

func CreateBuildJobEntryFromData(data []byte) BuildJobInfoEntry {
	result := BuildJobInfoEntry{}

	if data != nil {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		err := json.Unmarshal(data, &result)
		if err != nil {
			logger.Log.Errorf("createBuildJobEntryFromData ERR: %s", err.Error())
		}
	}

	return result
}

func CreateBuildJobInfoEntryBytes(state string, cmdOutput []byte, startTime time.Time, finishTime time.Time, job *BuildJob) []byte {
	entry := BuildJobInfoEntry{
		ProjectId:   job.ProjectId,
		Namespace:   job.Namespace,
		ServiceName: job.ServiceName,
		State:       state,
		Result:      string(cmdOutput),
		StartTime:   startTime.Format(time.RFC3339),
		FinishTime:  finishTime.Format(time.RFC3339),
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	bytes, err := json.Marshal(entry)
	if err != nil {
		logger.Log.Errorf("createBuildJobInfoEntryBytes ERR: %s", err.Error())
	}
	return bytes

}

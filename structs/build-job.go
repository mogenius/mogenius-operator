package structs

import (
	"time"
)

const (
	BUILD_STATE_FAILED    string = "FAILED"
	BUILD_STATE_SUCCEEDED string = "SUCCEEDED"
	BUILD_STATE_STARTED   string = "STARTED"
	BUILD_STATE_PENDING   string = "PENDING"
	BUILD_STATE_CANCELED  string = "CANCELED"
	BUILD_STATE_TIMEOUT   string = "TIMEOUT"
)

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
		ProjectId:             "6dbd5930-e3f0-4594-9888-2003c6325f9f",
		NamespaceId:           "32a399ba-3a48-462b-8293-11b667d3a1fa",
		Namespace:             "benegeilomat-prod-cp4wh9",
		ServiceId:             "ef7af4d2-8939-4c94-bbe1-a3e7018e8305",
		ServiceName:           "lalalalalalala",
		GitRepo:               "https://x-access-token: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@github.com/beneiltis/lalalalalalala.git",
		GitBranch:             "main",
		GitCommitAuthor:       "mogenius git-user",
		GitCommitHash:         "abe52a64e682cedf77f131e595119f6c2f6a1c84",
		GitCommitMessage:      "[skip ci]: Add initial files.",
		DockerFile:            "Dockerfile",
		DockerContext:         ".",
		ContainerRegistryPath: "mo7registry.azurecr.io",
		ContainerRegistryUser: "pipelinetoken",
		ContainerRegistryPat:  "I1JYKVN5FlFYLLWIGYQ9axq5V5x5oF14lWsPcD3Yk1+ACRAXvj9j",
		ContainerRegistryUrl:  "mo7registry.azurecr.io",
		StartTimestamp:        "1689684071841",
		InjectDockerEnvVars:   "--build-arg PLACEHOLDER=MOGENIUS",
		State:                 BUILD_STATE_PENDING,
		StartedAt:             time.Now().Format(time.RFC3339),
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

type BuildAddResult struct {
	BuildId int `json:"buildId"`
}

type BuildScanResult struct {
	Result string `json:"result"`
	Error  string `json:"error"`
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
	BuildId int    `json:"buildId"`
	Clone   string `json:"clone"`
	Ls      string `json:"ls"`
	Login   string `json:"login"`
	Build   string `json:"build"`
	Push    string `json:"push"`
	Scan    string `json:"scan"`
}

func CreateBuildJobInfos(buildId int, clone []byte, ls []byte, login []byte, build []byte, push []byte, scan []byte) BuildJobInfos {
	result := BuildJobInfos{}

	result.BuildId = buildId
	result.Clone = string(clone)
	result.Ls = string(ls)
	result.Login = string(login)
	result.Build = string(build)
	result.Push = string(push)
	result.Scan = string(scan)

	return result
}

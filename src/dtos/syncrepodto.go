package dtos

import (
	"mogenius-k8s-manager/src/logging"
	utils "mogenius-k8s-manager/src/utils"
	"strconv"

	core "k8s.io/api/core/v1"
)

type Commit struct {
	Author    string   `json:"author"`
	Message   string   `json:"message"`
	FilePaths []string `json:"filePaths"`
}

type ResetFileRequest struct {
	FilePath   string `json:"filePath"`
	CommitHash string `json:"commitHash"`
}

type SyncRepoData struct {
	Repo               string                    `json:"repo"`
	Pat                string                    `json:"pat"`
	Branch             string                    `json:"branch"`
	AllowPull          bool                      `json:"allowPull"`
	AllowPush          bool                      `json:"allowPush"`
	SyncFrequencyInSec int                       `json:"syncFrequencyInSec"`
	SyncResources      []utils.SyncResourceEntry `json:"syncResources"`
	IgnoredNamespaces  []string                  `json:"ignoredNamespaces"`
}

func (p *SyncRepoData) AddSecretsToRedaction() {
	if p.Pat != "***" {
		logging.AddSecret(p.Pat)
	}
}

func SyncRepoDataExampleData() SyncRepoData {
	return SyncRepoData{
		Repo:               "https://github.com/beneiltis/fuckumucku.git",
		Pat:                "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",
		Branch:             "main",
		AllowPull:          true,
		AllowPush:          true,
		SyncFrequencyInSec: 5,
		SyncResources:      []utils.SyncResourceEntry{},
		IgnoredNamespaces:  DefaultIgnoredNamespaces(),
	}
}

func CreateSyncRepoDataFrom(secret *core.Secret) SyncRepoData {
	result := SyncRepoData{
		Repo:   string(secret.Data["sync-repo-url"]),
		Pat:    string(secret.Data["sync-repo-pat"]),
		Branch: string(secret.Data["sync-repo-branch"]),
	}

	if result.Branch == "" {
		result.Branch = "main"
	}

	pull, err := strconv.ParseBool(string(secret.Data["sync-allow-pull"]))
	if err != nil {
		result.AllowPull = false
	} else {
		result.AllowPull = pull
	}

	push, err := strconv.ParseBool(string(secret.Data["sync-allow-push"]))
	if err != nil {
		result.AllowPush = false
	} else {
		result.AllowPush = push
	}

	freqSec, err := strconv.ParseInt(string(secret.Data["sync-frequency-in-sec"]), 10, 64)
	if err != nil {
		result.SyncFrequencyInSec = 10
	} else {
		result.SyncFrequencyInSec = int(freqSec)
	}

	return result
}

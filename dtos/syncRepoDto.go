package dtos

type AddSyncRepoRequest struct {
	Repo                      string `json:"repo" validate:"required"`
	Pat                       string `json:"pat"`
	AllowPull                 bool   `json:"allowPull"`
	AllowPush                 bool   `json:"allowPush"`
	AllowManualClusterChanges bool   `json:"allowManualClusterChanges"`
	SyncFrequencyInSec        int    `json:"syncFrequencyInSec"`
}

func AddSyncRepoRequestExampleData() AddSyncRepoRequest {
	return AddSyncRepoRequest{
		Repo:                      "https://github.com/beneiltis/fuckumucku.git",
		Pat:                       "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",
		AllowPull:                 true,
		AllowPush:                 true,
		AllowManualClusterChanges: true,
		SyncFrequencyInSec:        5,
	}
}

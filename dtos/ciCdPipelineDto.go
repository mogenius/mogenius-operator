package dtos

import "time"

type CiCdPipelineDto struct {
	Id         string                  `json:"id" validate:"required"`
	Type       string                  `json:"type" validate:"required"`
	Name       string                  `json:"name" validate:"required"`
	StartTime  string                  `json:"startTime" validate:"required"`
	FinishTime string                  `json:"finishTime" validate:"required"`
	State      string                  `json:"state" validate:"required"` // "completed", "inProgress", "pending"
	Result     string                  `json:"result,omitempty" `         // "abandoned", "canceled", "failed", "skipped", "succeeded", "succeededWithIssues"
	ResultCode string                  `json:"resultCode,omitempty"`
	WorkerName string                  `json:"workerName,omitempty"`
	Log        CiCdPipelineLogEntryDto `json:"log,omitempty"`
}

func CiCdPipelineDtoExampleData() CiCdPipelineDto {
	return CiCdPipelineDto{
		Id:         "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Type:       "type",
		Name:       "name",
		StartTime:  time.Now().Format(time.RFC3339),
		FinishTime: time.Now().Format(time.RFC3339),
		State:      "state",
		Result:     "result",
		ResultCode: "resultCode",
		WorkerName: "workerName",
		Log:        CiCdPipelineLogEntryDtoExampleData(),
	}
}

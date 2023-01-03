package enums

//go:generate stringer -type=CiCdPipelineStatusEnum
type CiCdPipelineStatusEnum string

const (
	Completed  CiCdPipelineStatusEnum = "completed"
	InProgress CiCdPipelineStatusEnum = "inProgress"
	Pending    CiCdPipelineStatusEnum = "pending"
)

package dtos

type CiCdPipelineLogEntryDto struct {
	Id  int    `json:"id" validate:"required"`
	Url string `json:"url" validate:"required"`
}

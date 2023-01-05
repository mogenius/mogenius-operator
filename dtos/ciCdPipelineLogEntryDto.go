package dtos

type CiCdPipelineLogEntryDto struct {
	Id  int    `json:"id" validate:"required"`
	Url string `json:"url" validate:"required"`
}

func CiCdPipelineLogEntryDtoExampleData() CiCdPipelineLogEntryDto {
	return CiCdPipelineLogEntryDto{
		Id:  1,
		Url: "url",
	}
}

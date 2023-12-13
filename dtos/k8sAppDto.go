package dtos

type K8sAppDto struct {
	Id                        string             `json:"id" validate:"required"`
	Name                      string             `json:"name" validate:"required"`
	Type                      K8sServiceTypeEnum `json:"type" validate:"required"`
	SetupCommands             string             `json:"setupCommands" validate:"required"`
	RepositoryLink            string             `json:"repositoryLink" validate:"required"`
	ContainerImage            string             `json:"containerImage" validate:"required"`
	ContainerImageCommand     string             `json:"containerImageCommand" validate:"required"`
	ContainerImageCommandArgs string             `json:"containerImageCommandArgs" validate:"required"`
}

func K8sAppDtoDockerExampleData() K8sAppDto {
	return K8sAppDto{
		Id:                        "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Name:                      "name",
		Type:                      ContainerImageTemplate,
		SetupCommands:             "",
		RepositoryLink:            "",
		ContainerImage:            "",
		ContainerImageCommand:     "",
		ContainerImageCommandArgs: "",
	}
}

func K8sAppDtoContainerImageExampleData() K8sAppDto {
	return K8sAppDto{
		Id:                        "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Name:                      "name",
		Type:                      ContainerImageTemplate,
		SetupCommands:             "",
		RepositoryLink:            "",
		ContainerImage:            "",
		ContainerImageCommand:     "",
		ContainerImageCommandArgs: "",
	}
}

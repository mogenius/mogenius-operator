package dtos

type K8sAppDto struct {
	Id                        string             `json:"id" validate:"required"`
	Name                      string             `json:"name" validate:"required"`
	Type                      K8sServiceTypeEnum `json:"type" validate:"required"`
	SetupCommands             string             `json:"setupCommands"`
	RepositoryLink            string             `json:"repositoryLink"`
	ContainerImage            string             `json:"containerImage"`
	ContainerImageCommand     string             `json:"containerImageCommand"`
	ContainerImageCommandArgs string             `json:"containerImageCommandArgs"`
}

package dtos

type K8sPortsDto struct {
	PortType     PortTypeEnum `json:"portType" validate:"required"`
	InternalPort int          `json:"internalPort" validate:"required"`
	ExternalPort int          `json:"externalPort" validate:"required"`
	Expose       bool         `json:"expose" validate:"required"`
}

func K8sPortsDtoExampleData() K8sPortsDto {
	return K8sPortsDto{
		PortType:     PortTypeHTTPS,
		InternalPort: 80,
		ExternalPort: 80,
		Expose:       true,
	}
}

func K8sPortsDtoExternalExampleData() K8sPortsDto {
	return K8sPortsDto{
		PortType:     PortTypeTCP,
		InternalPort: 6379,
		ExternalPort: 12345,
		Expose:       true,
	}
}

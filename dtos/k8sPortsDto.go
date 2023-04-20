package dtos

type K8sPortsDto struct {
	PortType     string `json:"portType" validate:"required"` // "HTTPS", "TCP", "UDP"
	InternalPort int    `json:"internalPort" validate:"required"`
	ExternalPort int    `json:"externalPort" validate:"required"`
	Expose       bool   `json:"expose" validate:"required"`
}

func K8sPortsDtoExampleData() K8sPortsDto {
	return K8sPortsDto{
		PortType:     "HTTPS",
		InternalPort: 80,
		ExternalPort: 80,
		Expose:       true,
	}
}

func K8sPortsDtoExternalExampleData() K8sPortsDto {
	return K8sPortsDto{
		PortType:     "TCP",
		InternalPort: 6379,
		ExternalPort: 12345,
		Expose:       true,
	}
}

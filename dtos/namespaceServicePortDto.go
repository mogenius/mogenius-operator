package dtos

type NamespaceServicePortDto struct {
	PortType     string `json:"portType" validate:"required"` // "HTTPS", "TCP", "UDP"
	InternalPort int    `json:"internalPort" validate:"required"`
	ExternalPort int    `json:"externalPort" validate:"required"`
	Expose       bool   `json:"expose" validate:"required"`
}

func NamespaceServicePortDtoExampleData() NamespaceServicePortDto {
	return NamespaceServicePortDto{
		PortType:     "TCP",
		InternalPort: 80,
		ExternalPort: 12345,
		Expose:       true,
	}
}

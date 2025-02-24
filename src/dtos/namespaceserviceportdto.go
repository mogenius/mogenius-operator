package dtos

type NamespaceServicePortDto struct {
	PortType     PortTypeEnum `json:"portType" validate:"required"`
	InternalPort int          `json:"internalPort" validate:"required"`
	ExternalPort int          `json:"externalPort" validate:"required"`
	Expose       bool         `json:"expose" validate:"required"`
}

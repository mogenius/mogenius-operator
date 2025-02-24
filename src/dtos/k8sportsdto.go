package dtos

type K8sPortsDto struct {
	PortType     PortTypeEnum  `json:"portType" validate:"required"`
	InternalPort int           `json:"internalPort" validate:"required"`
	ExternalPort int           `json:"externalPort" validate:"required"`
	Expose       bool          `json:"expose" validate:"required"`
	CNames       []K8sCnameDto `json:"cNames"`
}

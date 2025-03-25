package dtos

type K8sPortsDto struct {
	PortType     PortTypeEnum  `json:"portType" validate:"required"`
	InternalPort string        `json:"internalPort" validate:"required"`
	ExternalPort string        `json:"externalPort" validate:"required"`
	Expose       bool          `json:"expose" validate:"required"`
	CNames       []K8sCnameDto `json:"cNames"`
}

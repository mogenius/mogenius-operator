package dtos

type K8sCnameDto struct {
	CName         string `json:"cName" validate:"required"`
	AddToTlsHosts bool   `json:"addToTlsHosts" validate:"required"`
}

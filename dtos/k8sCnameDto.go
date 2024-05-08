package dtos

type K8sCnameDto struct {
	CName         string `json:"cName" validate:"required"`
	AddToTlsHosts bool   `json:"addToTlsHosts" validate:"required"`
}

func K8sCnameDtoExampleData() K8sCnameDto {
	return K8sCnameDto{
		CName:         "name",
		AddToTlsHosts: true,
	}
}

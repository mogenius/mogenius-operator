package dtos

type K8sCnameDto struct {
	Name          string `json:"name" validate:"required"`
	AddToTlsHosts bool   `json:"addToTlsHosts" validate:"required"`
}

func K8sCnameDtoExampleData() K8sCnameDto {
	return K8sCnameDto{
		Name:          "name",
		AddToTlsHosts: true,
	}
}

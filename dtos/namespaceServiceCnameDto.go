package dtos

import "time"

type NamespaceServiceCnameDto struct {
	Id                       string                      `json:"id" validate:"required"`
	CreatedAt                string                      `json:"createdAt" validate:"required"`
	UpdatedAt                string                      `json:"updatedAt" validate:"required"`
	CName                    string                      `json:"cName" validate:"required"`
	CloudflareCustomHostname CloudflareCustomHostnameDto `json:"cloudflareCustomHostname" validate:"required"`
}

func NamespaceServiceCnameDtoExampleData() NamespaceServiceCnameDto {
	return NamespaceServiceCnameDto{
		Id:                       "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		CreatedAt:                time.Now().Format(time.RFC3339),
		UpdatedAt:                time.Now().Format(time.RFC3339),
		CName:                    "cName",
		CloudflareCustomHostname: CloudflareCustomHostnameDtoExampleData(),
	}
}

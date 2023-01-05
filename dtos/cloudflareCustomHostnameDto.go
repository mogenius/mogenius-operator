package dtos

import "time"

type CloudflareCustomHostnameDto struct {
	NamespaceServiceCNames []NamespaceServiceCnameDto `json:"namespaceServiceCNames" validate:"required"`
	Id                     string                     `json:"id" validate:"required"`
	CustomHostnameId       string                     `json:"customHostnameId" validate:"required"`
	Hostname               string                     `json:"hostname" validate:"required"`
	Status                 string                     `json:"status" validate:"required"`    // "pending", "active", "moved", "deleted", "blocked"
	SslStatus              string                     `json:"sslStatus" validate:"required"` // "initializing", "pending_validation", "pending_issuance", "pending_deployment", "active"
	CloudflareResponse     interface{}                `json:"cloudflareResponse" validate:"required"`
	CreatedAt              string                     `json:"createdAt" validate:"required"`
	UpdatedAt              string                     `json:"updatedAt" validate:"required"`
}

func CloudflareCustomHostnameDtoExampleData() CloudflareCustomHostnameDto {
	return CloudflareCustomHostnameDto{
		NamespaceServiceCNames: []NamespaceServiceCnameDto{},
		Id:                     "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		CustomHostnameId:       "customHostnameId",
		Hostname:               "hostname",
		Status:                 "active",
		SslStatus:              "active",
		CloudflareResponse: map[string]interface{}{
			"key": "value",
		},
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
}

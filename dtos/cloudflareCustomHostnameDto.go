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
	CreatedAt              time.Time                  `json:"createdAt" validate:"required"`
	UpdatedAt              time.Time                  `json:"updatedAt" validate:"required"`
}

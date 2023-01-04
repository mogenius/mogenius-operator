package dtos

type NamespaceServiceCnameDto struct {
	Id                       string                      `json:"id" validate:"required"`
	CreatedAt                string                      `json:"createdAt" validate:"required"`
	UpdatedAt                string                      `json:"updatedAt" validate:"required"`
	CName                    string                      `json:"cName" validate:"required"`
	CloudflareCustomHostname CloudflareCustomHostnameDto `json:"cloudflareCustomHostname" validate:"required"`
}

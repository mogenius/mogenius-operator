package structs

type TCPRequest struct {
	Pattern string `json:"pattern" validate:"required"`
	Id      string `json:"id" validate:"required"`
}

package structs

type TCPResponse struct {
	Response string `json:"response"`
	Id       string `json:"id"`
	Err      string `json:"err,omitempty"`
}

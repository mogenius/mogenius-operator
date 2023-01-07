package structs

import (
	"github.com/google/uuid"
)

type Datagram struct {
	Id      string      `json:"id" validate:"required"`
	Pattern string      `json:"pattern" validate:"required"`
	Payload interface{} `json:"payload,omitempty"`
	Err     string      `json:"err,omitempty"`
}

func CreateDatagramRequest(request Datagram, data interface{}) Datagram {
	datagram := Datagram{
		Id:      request.Id,
		Pattern: request.Pattern,
		Payload: data,
	}
	return datagram
}

func CreateDatagramFrom(pattern string, data interface{}) Datagram {
	datagram := Datagram{
		Id:      uuid.New().String(),
		Pattern: pattern,
		Payload: data,
	}
	return datagram
}

func CreateDatagram(pattern string) Datagram {
	datagram := Datagram{
		Id:      uuid.New().String(),
		Pattern: pattern,
	}
	return datagram
}

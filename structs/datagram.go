package structs

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var sendMutex sync.Mutex

type Datagram struct {
	Id         string          `json:"id" validate:"required"`
	Pattern    string          `json:"pattern" validate:"required"`
	Payload    string          `json:"payload,omitempty"`
	Err        string          `json:"err,omitempty"`
	Connection *websocket.Conn // websocket connection of the player
}

func CreateDatagramRequest(request Datagram, data interface{}, c *websocket.Conn) Datagram {
	datagram := Datagram{
		Id:         request.Id,
		Pattern:    request.Pattern,
		Payload:    PrettyPrintString(data),
		Connection: c,
	}
	return datagram
}

func CreateDatagramFrom(pattern string, data interface{}, c *websocket.Conn) Datagram {
	datagram := Datagram{
		Id:         uuid.New().String(),
		Pattern:    pattern,
		Payload:    PrettyPrintString(data),
		Connection: c,
	}
	return datagram
}

func CreateDatagram(pattern string, c *websocket.Conn) Datagram {
	datagram := Datagram{
		Id:         uuid.New().String(),
		Pattern:    pattern,
		Connection: c,
	}
	return datagram
}

func (d *Datagram) Send() error {
	if d.Connection != nil {
		sendMutex.Lock()
		defer sendMutex.Unlock()
		return d.Connection.WriteJSON(d)

	} else {
		return fmt.Errorf("Connection cannot be nil.")
	}
}

package structs

import (
	"time"

	"github.com/gorilla/websocket"
)

type ClusterConnection struct {
	Connection  *websocket.Conn
	ClusterName string
	AddedAt     time.Time
}

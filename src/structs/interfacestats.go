package structs

type SocketConnections struct {
	LastUpdate  string            `json:"lastUpdate"`
	Connections map[string]uint64 `json:"connections"`
}

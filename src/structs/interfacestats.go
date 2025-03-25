package structs

import (
	jsoniter "github.com/json-iterator/go"
)

type SocketConnections struct {
	LastUpdate  string            `json:"lastUpdate"`
	Connections map[string]uint64 `json:"connections"`
}

func UnmarshalSocketConnections(dst *SocketConnections, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	return nil
}

package structs

import (
	jsoniter "github.com/json-iterator/go"
)

const PingSeconds = 3

func MarshalUnmarshal(datagram *Datagram, data any) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary

	bytes, err := json.Marshal(datagram.Payload)
	if err != nil {
		datagram.Err = err.Error()
		return
	}
	err = json.Unmarshal(bytes, data)
	if err != nil {
		datagram.Err = err.Error()
	}
}

package structs

import "encoding/json"

const PingSeconds = 3

func MarshalUnmarshal(datagram *Datagram, data any) {
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

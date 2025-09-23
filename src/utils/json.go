package utils

import jsoniter "github.com/json-iterator/go"

func Marshal(data any) ([]byte, error) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Marshal(data)
}

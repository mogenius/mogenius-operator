package utils

import jsoniter "github.com/json-iterator/go"

func Marshal(data interface{}) ([]byte, error) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Marshal(data)
}

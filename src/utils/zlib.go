package utils

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"io"
)

func ZlibCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)
	_, err := writer.Write(data)
	if err != nil {
		return nil, err
	}
	writer.Close()

	return buf.Bytes(), nil
}

func ZlibDecompress(compressedData []byte) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(compressedData)
	reader, err := zlib.NewReader(&buf)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var outBuf bytes.Buffer
	_, err = io.Copy(&outBuf, reader)
	if err != nil {
		return nil, err
	}

	return outBuf.Bytes(), nil
}

// the data variable will be modified in place (inOut-variable)
func TryZlibCompress(data any) (any, int64, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, 0, err
	}
	compressedPayload, err := ZlibCompress([]byte(dataBytes))
	if err != nil {
		return nil, 0, err
	}
	return compressedPayload, int64(len(compressedPayload)), nil
}

// the data variable will be modified in place (inOut-variable)
func TryZlibDecompress(data any) (any, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	decompressedPayload, err := ZlibDecompress([]byte(dataBytes))
	if err != nil {
		return nil, err
	}
	return decompressedPayload, nil
}

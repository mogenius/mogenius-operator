package utils

import (
	"bytes"
	"compress/zlib"
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
func TryZlibCompress(data interface{}) (interface{}, error) {
	dataBytes, err := Marshal(data)
	if err != nil {
		return nil, err
	}
	compressedPayload, err := ZlibCompress([]byte(dataBytes))
	if err != nil {
		return nil, err
	}
	return compressedPayload, nil
}

// the data variable will be modified in place (inOut-variable)
func TryZlibDecompress(data interface{}) (interface{}, error) {
	dataBytes, err := Marshal(data)
	if err != nil {
		return nil, err
	}
	decompressedPayload, err := ZlibDecompress([]byte(dataBytes))
	if err != nil {
		return nil, err
	}
	return decompressedPayload, nil
}

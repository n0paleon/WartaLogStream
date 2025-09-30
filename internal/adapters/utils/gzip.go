package utils

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
)

func Compress(data string) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write([]byte(data))
	_ = gz.Close()
	return buf.Bytes(), err
}

func Decompress(data []byte) (string, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer gz.Close()
	result, err := ioutil.ReadAll(gz)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

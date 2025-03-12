package main

import (
	"compress/zlib"
	"fmt"
	"io"
)

func zlibCompress(w io.Writer, b []byte) error {
	zw := zlib.NewWriter(w)
	defer zw.Close()

	n, err := zw.Write(b)
	if err != nil {
		return fmt.Errorf("failed to compress data with zlib: %s", err)
	}
	if n != len(b) {
		return fmt.Errorf("failed to write complete byte contents with zlib")
	}

	return nil
}

func zlibUncompress(r io.Reader) ([]byte, error) {
	zr, err := zlib.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize zlib reader: %s", err)
	}
	defer zr.Close()

	data, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("failed to uncompress data with zlib: %s", err)
	}

	return data, nil
}

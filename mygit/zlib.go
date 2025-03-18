package main

import (
	"bytes"
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
	if err := zw.Flush(); err != nil {
		return fmt.Errorf("failed to flush zlib writer: %s", err)
	}

	return nil
}

func zlibDecompress(r io.Reader) ([]byte, error) {
	zr, err := zlib.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize zlib reader: %s", err)
	}
	defer zr.Close()

	decompressed, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data with zlib: %s", err)
	}

	return decompressed, nil
}

func zlibDecompressWithReadCount(r *bytes.Reader) ([]byte, int, error) {
	zr, err := zlib.NewReader(r)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to initialize zlib reader: %s", err)
	}
	defer zr.Close()

	decompressed, err := io.ReadAll(zr)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to decompress data with zlib: %s", err)
	}

	bytesRead := int(r.Size()) - r.Len()
	return decompressed, bytesRead, nil
}

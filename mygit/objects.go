package main

import (
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const OBJECT_HASH_LENGTH = 40

// Represents a Git blob object, which stores the contents of a file
type BlobObject struct {
	hash      string
	sizeBytes int
	content   string
}

func isValidObjectHash(objHash string) bool {
	if len(objHash) != OBJECT_HASH_LENGTH {
		return false
	}

	is_alphanumeric := regexp.MustCompile(`^[a-z0-9]*$`).MatchString(objHash)
	return is_alphanumeric
}

func readBlobObjectFromFile(objHash string) (*BlobObject, error) {
	objPath := fmt.Sprintf("%s/.git/objects/%s/%s", REPO_DIR, objHash[:2], objHash[2:])
	file, err := os.Open(objPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open object file")
	}

	r, err := zlib.NewReader(io.Reader(file))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize zlib reader")
	}
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read from object file")
	}

	parts := strings.Split(string(data), "\x00")
	headerParts := strings.Split(parts[0], " ")
	if headerParts[0] != "blob" {
		return nil, fmt.Errorf("object file poorly formatted - header does not start with 'blob'")
	}

	sizeBytes, err := strconv.Atoi(headerParts[1])
	if err != nil {
		return nil, fmt.Errorf("object file poorly formatted - contents size is not an integer")
	}

	content := parts[1]

	return &BlobObject{
		hash:      objHash,
		sizeBytes: sizeBytes,
		content:   content,
	}, nil
}

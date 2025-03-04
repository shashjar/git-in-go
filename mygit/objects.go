package main

import (
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
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

func readBlobObjectFile(objHash string) (*BlobObject, error) {
	objPath := REPO_DIR + fmt.Sprintf(".git/objects/%s/%s", objHash[:2], objHash[2:])
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

func createBlobObjectFromFile(filePath string) (*BlobObject, error) {
	contentBytes, err := os.ReadFile(REPO_DIR + filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file")
	}
	content := string(contentBytes)
	sizeBytes := len(contentBytes)

	objFileContent := fmt.Sprintf("blob %d\x00%s", sizeBytes, content)
	objFileContentBytes := []byte(objFileContent)
	objHashBytes := sha1.Sum(objFileContentBytes)
	objHash := hex.EncodeToString(objHashBytes[:])

	objPath := "./" + REPO_DIR + fmt.Sprintf(".git/objects/%s/%s", objHash[:2], objHash[2:])
	objFile, err := os.Create(objPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create object file")
	}

	w := zlib.NewWriter(objFile)
	defer w.Close()
	n, err := w.Write(objFileContentBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to write to object file")
	}
	if n != len(objFileContentBytes) {
		return nil, fmt.Errorf("failed to write complete contents to object file")
	}

	return &BlobObject{
		hash:      objHash,
		sizeBytes: sizeBytes,
		content:   content,
	}, nil
}

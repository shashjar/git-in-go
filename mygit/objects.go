package main

import (
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const OBJECT_HASH_LENGTH = 40

type ObjectType int

const (
	Blob   ObjectType = iota // 0
	Tree                     // 1
	Commit                   // 2
)

func (ot *ObjectType) toString() string {
	if *ot == Blob {
		return "blob"
	} else if *ot == Tree {
		return "tree"
	} else if *ot == Commit {
		return "commit"
	} else {
		return "unknown"
	}
}

var VALID_MODES = []int{100644, 100755, 12000, 40000}

// Represents a Git blob object, which stores the contents of a file
type BlobObject struct {
	hash      string
	sizeBytes int
	content   string
}

// Represents a Git tree object, which stores a directory structure
type TreeObject struct {
	hash      string
	sizeBytes int
	entries   []TreeObjectEntry
}

// Represents an entry (either a blob or another tree) within a Git tree object
type TreeObjectEntry struct {
	hash    string
	mode    int
	name    string
	objType ObjectType
}

/** GENERIC TO ALL OBJECTS */

func isValidObjectHash(objHash string) bool {
	if len(objHash) != OBJECT_HASH_LENGTH {
		return false
	}

	is_alphanumeric := regexp.MustCompile(`^[a-z0-9]*$`).MatchString(objHash)
	return is_alphanumeric
}

func isValidMode(mode int) bool {
	return slices.Contains(VALID_MODES, mode)
}

func getObjectTypeFromMode(mode int) ObjectType {
	if mode == 40000 {
		return Tree
	} else {
		return Blob
	}
}

func readObjectFile(objHash string) ([]byte, error) {
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

	return data, nil
}

/** BLOBS */

func readBlobObjectFile(objHash string) (*BlobObject, error) {
	data, err := readObjectFile(objHash)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(string(data), "\x00")
	if len(parts) != 2 {
		return nil, fmt.Errorf("object file poorly formatted - header and contents should be separated by null byte")
	}

	headerParts := strings.Split(parts[0], " ")
	if len(headerParts) != 2 {
		return nil, fmt.Errorf("object file poorly formatted - header parts should be space-separated")
	}

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

/** TREES */

func (e *TreeObjectEntry) toString(nameOnly bool) string {
	if nameOnly {
		return e.name
	} else {
		return fmt.Sprintf("%d %s %s    %s", e.mode, e.objType.toString(), e.hash, e.name)
	}
}

func parseTreeObjectEntry(entryHeader string, entryHash string) (*TreeObjectEntry, error) {
	entryHeaderParts := strings.Split(entryHeader, " ")
	if len(entryHeaderParts) != 2 {
		return nil, fmt.Errorf("entry mode and name should be space-separated")
	}

	mode, err := strconv.Atoi(entryHeaderParts[0])
	if err != nil {
		return nil, fmt.Errorf("entry mode should be an integer")
	}
	if !isValidMode(mode) {
		return nil, fmt.Errorf("invalid entry mode: %d", mode)
	}

	if !isValidObjectHash(entryHash) {
		return nil, fmt.Errorf("invalid entry object hash: %s", entryHash)
	}

	return &TreeObjectEntry{
		hash:    entryHash,
		mode:    mode,
		name:    entryHeaderParts[1],
		objType: getObjectTypeFromMode(mode),
	}, nil
}

func readTreeObjectFile(objHash string) (*TreeObject, error) {
	data, err := readObjectFile(objHash)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(string(data), "\x00")
	if len(parts) < 1 || len(parts)%2 == 0 {
		return nil, fmt.Errorf("object file poorly formatted - header, entry headers, and entry hashes should be null byte-separated")
	}

	headerParts := strings.Split(parts[0], " ")
	if len(headerParts) != 2 {
		return nil, fmt.Errorf("object file poorly formatted - header parts should be space-separated")
	}

	if headerParts[0] != "tree" {
		return nil, fmt.Errorf("object file poorly formatted - header does not start with 'tree'")
	}
	sizeBytes, err := strconv.Atoi(headerParts[1])
	if err != nil {
		return nil, fmt.Errorf("object file poorly formatted - size is not an integer")
	}

	entries := []TreeObjectEntry{}
	for i := 1; i < len(parts); i += 2 {
		entry, err := parseTreeObjectEntry(parts[i], parts[i+1])
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
	}
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].name < entries[j].name
	})

	return &TreeObject{
		hash:      objHash,
		sizeBytes: sizeBytes,
		entries:   entries,
	}, nil
}

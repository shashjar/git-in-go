package main

import (
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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

const MODE_LENGTH = 6

const (
	REGULAR_FILE_MODE    = 100644
	EXECUTABLE_FILE_MODE = 100755
	SYMBOLIC_LINK_MODE   = 120000
	DIRECTORY_MODE       = 40000
)

var VALID_MODES = []int{REGULAR_FILE_MODE, EXECUTABLE_FILE_MODE, SYMBOLIC_LINK_MODE, DIRECTORY_MODE}

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

func createObjectFile(objType string, contentBytes []byte) (string, error) {
	content := string(contentBytes)
	sizeBytes := len(contentBytes)

	objFileContent := fmt.Sprintf("%s %d\x00%s", objType, sizeBytes, content)
	objFileContentBytes := []byte(objFileContent)
	objHashBytes := sha1.Sum(objFileContentBytes)
	objHash := hex.EncodeToString(objHashBytes[:])

	objPath := REPO_DIR + fmt.Sprintf(".git/objects/%s/%s", objHash[:2], objHash[2:])

	dir := filepath.Dir(objPath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create directories storing object file")
	}

	objFile, err := os.Create(objPath)
	if err != nil {
		return "", fmt.Errorf("failed to create object file")
	}
	defer objFile.Close()

	w := zlib.NewWriter(objFile)
	defer w.Close()
	n, err := w.Write(objFileContentBytes)
	if err != nil {
		return "", fmt.Errorf("failed to write to object file")
	}
	if n != len(objFileContentBytes) {
		return "", fmt.Errorf("failed to write complete contents to object file")
	}

	return objHash, nil
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
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file")
	}
	content := string(contentBytes)
	sizeBytes := len(contentBytes)

	blobObjHash, err := createObjectFile("blob", contentBytes)
	if err != nil {
		return nil, err
	}

	return &BlobObject{
		hash:      blobObjHash,
		sizeBytes: sizeBytes,
		content:   content,
	}, nil
}

/** TREES */

func (e *TreeObjectEntry) toString(nameOnly bool) string {
	if nameOnly {
		return e.name
	} else {
		mode := fmt.Sprintf("%06d", e.mode)
		return fmt.Sprintf("%s %s %s    %s", mode, e.objType.toString(), e.hash, e.name)
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
	for i := 1; i < len(parts)-1; i++ {
		var entryHeader string
		if i == 1 {
			entryHeader = parts[i]
		} else {
			entryHeader = parts[i][OBJECT_HASH_LENGTH:]
		}

		entry, err := parseTreeObjectEntry(entryHeader, parts[i+1][:OBJECT_HASH_LENGTH])
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

func createTreeObjectFromDirectory(dir string) (*TreeObject, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("Could not read contents of directory")
	}

	entries := []TreeObjectEntry{}
	for _, dirEntry := range dirEntries {
		if strings.HasPrefix(dirEntry.Name(), ".") {
			continue
		}

		fullPath := filepath.Join(dir, dirEntry.Name())

		if dirEntry.IsDir() {
			subDirTreeObj, err := createTreeObjectFromDirectory(fullPath)
			if err != nil {
				return nil, err
			}
			entries = append(entries, TreeObjectEntry{
				hash:    subDirTreeObj.hash,
				mode:    DIRECTORY_MODE,
				name:    dirEntry.Name(),
				objType: Tree,
			})
		} else {
			fileInfo, err := os.Lstat(fullPath)
			if err != nil {
				return nil, err
			}

			var mode int
			if fileInfo.Mode()&os.ModeSymlink != 0 {
				mode = SYMBOLIC_LINK_MODE
			} else if fileInfo.Mode()&0111 != 0 {
				mode = EXECUTABLE_FILE_MODE
			} else {
				mode = REGULAR_FILE_MODE
			}

			fileBlobObj, err := createBlobObjectFromFile(fullPath)
			if err != nil {
				return nil, err
			}
			entries = append(entries, TreeObjectEntry{
				hash:    fileBlobObj.hash,
				mode:    mode,
				name:    dirEntry.Name(),
				objType: Blob,
			})
		}
	}

	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].name < entries[j].name
	})

	var contentBuilder strings.Builder
	for _, entry := range entries {
		fmt.Fprintf(&contentBuilder, "%d %s\x00%s", entry.mode, entry.name, entry.hash)
	}
	contentBytes := []byte(contentBuilder.String())
	sizeBytes := len(contentBytes)

	treeObjHash, err := createObjectFile("tree", contentBytes)
	if err != nil {
		return nil, err
	}

	return &TreeObject{
		hash:      treeObjHash,
		sizeBytes: sizeBytes,
		entries:   entries,
	}, nil
}

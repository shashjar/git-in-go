package main

import (
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
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

const (
	REGULAR_FILE_MODE    = 100644
	EXECUTABLE_FILE_MODE = 100755
	SYMBOLIC_LINK_MODE   = 120000
	DIRECTORY_MODE       = 40000
)

var VALID_MODES = []int{REGULAR_FILE_MODE, EXECUTABLE_FILE_MODE, SYMBOLIC_LINK_MODE, DIRECTORY_MODE}

// GitObject is the common interface for all Git objects (blobs, trees, commits)
type GitObject interface {
	// Returns the type of this object
	GetObjectType() ObjectType

	// Gets the content size in bytes for this object
	GetSizeBytes() int

	// Returns a pretty-printed string representing the object and its contents
	PrettyPrint() string
}

// Represents a Git blob object, which stores the contents of a file
type BlobObject struct {
	hash      string
	sizeBytes int
	content   string
}

func (b *BlobObject) GetObjectType() ObjectType {
	return Blob
}

func (b *BlobObject) GetSizeBytes() int {
	return b.sizeBytes
}

func (b *BlobObject) PrettyPrint() string {
	return fmt.Sprintf("blob %d\n%s", b.sizeBytes, b.content)
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

func (t *TreeObject) GetObjectType() ObjectType {
	return Tree
}

func (t *TreeObject) GetSizeBytes() int {
	return t.sizeBytes
}

func (t *TreeObject) PrettyPrint() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "tree %d\n", t.sizeBytes)
	for _, entry := range t.entries {
		fmt.Fprintf(&sb, "%06d %s %s\n", entry.mode, entry.name, entry.hash)
	}
	return sb.String()
}

// Represents a Git commit object, which represents a snapshot of the repository at a point in time
type CommitObject struct {
	hash               string
	sizeBytes          int
	treeHash           string
	parentCommitHashes []string
	author             CommitUser
	committer          CommitUser
	commitMessage      string
}

// Represents a user (author or committer) associated with a Git commit
type CommitUser struct {
	name        string
	email       string
	dateSeconds int64
	timezone    string
}

func (c *CommitObject) GetObjectType() ObjectType {
	return Commit
}

func (c *CommitObject) GetSizeBytes() int {
	return c.sizeBytes
}

func (c *CommitObject) PrettyPrint() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "commit %d\n", c.sizeBytes)
	fmt.Fprintf(&sb, "tree %s\n", c.treeHash)
	for _, parentCommitHash := range c.parentCommitHashes {
		fmt.Fprintf(&sb, "parent %s", parentCommitHash)
	}
	fmt.Fprintf(&sb, "author %s <%s> %d %s\n", c.author.name, c.author.email, c.author.dateSeconds, c.author.timezone)
	fmt.Fprintf(&sb, "committer %s <%s> %d %s\n", c.committer.name, c.committer.email, c.committer.dateSeconds, c.committer.timezone)
	fmt.Fprintf(&sb, "\n%s\n", c.commitMessage)
	return sb.String()
}

/** GENERIC TO ALL OBJECTS */

func isValidObjectHash(objHash string) bool {
	if len(objHash) != OBJECT_HASH_LENGTH {
		return false
	}

	isAlphanumeric := regexp.MustCompile(`^[a-z0-9]*$`).MatchString(objHash)
	return isAlphanumeric
}

func isValidMode(mode int) bool {
	return slices.Contains(VALID_MODES, mode)
}

func getObjectType(objHash string) (ObjectType, error) {
	data, err := readObjectFile(objHash)
	if err != nil {
		return -1, err
	}

	parts := strings.SplitN(string(data), "\x00", 2)
	if len(parts) != 2 {
		return -1, fmt.Errorf("object file poorly formatted - header and contents should be separated by null byte")
	}

	headerParts := strings.Split(parts[0], " ")
	if len(headerParts) != 2 {
		return -1, fmt.Errorf("object file poorly formatted - header parts should be space-separated")
	}

	if headerParts[0] == "blob" {
		return Blob, nil
	} else if headerParts[0] == "tree" {
		return Tree, nil
	} else if headerParts[0] == "commit" {
		return Commit, nil
	} else {
		return -1, fmt.Errorf("unknown object type %s", headerParts[0])
	}
}

func getObjectTypeFromMode(mode int) ObjectType {
	if mode == 40000 {
		return Tree
	} else {
		return Blob
	}
}

func getObject(objHash string) (GitObject, error) {
	objType, err := getObjectType(objHash)
	if err != nil {
		return nil, err
	}

	var gitObj GitObject
	switch objType {
	case Blob:
		blobObj, err := readBlobObjectFile(objHash)
		if err != nil {
			return nil, err
		}
		gitObj = blobObj
	case Tree:
		treeObj, err := readTreeObjectFile(objHash)
		if err != nil {
			return nil, err
		}
		gitObj = treeObj
	case Commit:
		commitObj, err := readCommitObjectFile(objHash)
		if err != nil {
			return nil, err
		}
		gitObj = commitObj
	default:
		return nil, fmt.Errorf("unsupported Git object type")
	}

	return gitObj, nil
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

func parseCommitUser(s string) (*CommitUser, error) {
	parts := strings.Split(s, " ")
	dateSeconds, err := strconv.Atoi(parts[4])
	if err != nil {
		return nil, err
	}
	return &CommitUser{
		name:        parts[1] + " " + parts[2],
		email:       parts[3][1 : len(parts[3])-1],
		dateSeconds: int64(dateSeconds),
		timezone:    parts[5],
	}, nil
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

/** COMMITS */

// TODO: my commit object file format may not match Git's exactly - I added newline characters here to make parsing easier
func createCommitObjectFromTree(treeHash string, parentCommitHashes []string, commitMessage string) (*CommitObject, error) {
	var contentBuilder strings.Builder
	fmt.Fprintf(&contentBuilder, "tree %s\n", treeHash)

	for _, parentCommitHash := range parentCommitHashes {
		fmt.Fprintf(&contentBuilder, "parent %s\n", parentCommitHash)
	}

	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	_, offset := now.Zone()
	timezone := fmt.Sprintf("%+03d%02d", offset/3600, (offset%3600)/60)
	author_committer := CommitUser{
		name:        currentUser.Name,
		email:       fmt.Sprintf("%s@mygit.com", currentUser.Username),
		dateSeconds: now.Unix(),
		timezone:    timezone,
	}
	fmt.Fprintf(&contentBuilder, "author %s <%s> %d %s\n", author_committer.name, author_committer.email, author_committer.dateSeconds, author_committer.timezone)
	fmt.Fprintf(&contentBuilder, "committer %s <%s> %d %s\n", author_committer.name, author_committer.email, author_committer.dateSeconds, author_committer.timezone)

	fmt.Fprintf(&contentBuilder, "\n\n%s", commitMessage)

	contentBytes := []byte(contentBuilder.String())
	sizeBytes := len(contentBytes)
	commitObjHash, err := createObjectFile("commit", contentBytes)
	if err != nil {
		return nil, err
	}

	return &CommitObject{
		hash:               commitObjHash,
		sizeBytes:          sizeBytes,
		treeHash:           treeHash,
		parentCommitHashes: parentCommitHashes,
		author:             author_committer,
		committer:          author_committer,
		commitMessage:      commitMessage,
	}, nil
}

func readCommitObjectFile(objHash string) (*CommitObject, error) {
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

	if headerParts[0] != "commit" {
		return nil, fmt.Errorf("object file poorly formatted - header does not start with 'commit'")
	}
	sizeBytes, err := strconv.Atoi(headerParts[1])
	if err != nil {
		return nil, fmt.Errorf("object file poorly formatted - size is not an integer")
	}

	lines := strings.Split(parts[1], "\n")
	treeHash := strings.Split(lines[0], " ")[1]
	var parentCommitHashes []string
	i := 1
	for strings.HasPrefix(lines[i], "parent") {
		parentCommitHashes = append(parentCommitHashes, strings.Split(lines[i], " ")[1])
		i += 1
	}
	author, err := parseCommitUser(lines[i])
	if err != nil {
		return nil, err
	}
	committer, err := parseCommitUser(lines[i+1])
	if err != nil {
		return nil, err
	}
	commitMessage := lines[len(lines)-1]

	return &CommitObject{
		hash:               objHash,
		sizeBytes:          sizeBytes,
		treeHash:           treeHash,
		parentCommitHashes: parentCommitHashes,
		author:             *author,
		committer:          *committer,
		commitMessage:      commitMessage,
	}, nil
}

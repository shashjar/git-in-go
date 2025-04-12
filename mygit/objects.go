package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
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

const (
	OBJECT_HASH_LENGTH_STRING = 40
	OBJECT_HASH_LENGTH_BYTES  = 20
)

type ObjectType int

const (
	Blob   ObjectType = iota // 0
	Tree                     // 1
	Commit                   // 2
)

func (ot ObjectType) toString() string {
	if ot == Blob {
		return "blob"
	} else if ot == Tree {
		return "tree"
	} else if ot == Commit {
		return "commit"
	} else {
		return "unknown"
	}
}

func ObjTypeFromString(objType string) (ObjectType, error) {
	if objType == Blob.toString() {
		return Blob, nil
	} else if objType == Tree.toString() {
		return Tree, nil
	} else if objType == Commit.toString() {
		return Commit, nil
	} else {
		return -1, fmt.Errorf("unknown object type %s", objType)
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
	content   []byte
}

func (b *BlobObject) GetObjectType() ObjectType {
	return Blob
}

func (b *BlobObject) GetSizeBytes() int {
	return b.sizeBytes
}

func (b *BlobObject) PrettyPrint() string {
	return fmt.Sprintf("blob %d\n%s", b.sizeBytes, string(b.content))
}

// Represents a Git tree object, which stores a directory structure
type TreeObject struct {
	hash      string
	sizeBytes int
	entries   []TreeObjectEntry
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

// Represents an entry (either a blob or another tree) within a Git tree object
type TreeObjectEntry struct {
	hash    string
	mode    int
	name    string
	objType ObjectType
}

func (e *TreeObjectEntry) toString(nameOnly bool) string {
	if nameOnly {
		return e.name
	} else {
		mode := fmt.Sprintf("%06d", e.mode)
		return fmt.Sprintf("%s %s %s    %s", mode, e.objType.toString(), e.hash, e.name)
	}
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
	if len(objHash) != OBJECT_HASH_LENGTH_STRING {
		return false
	}

	isAlphanumeric := regexp.MustCompile(`^[a-z0-9]*$`).MatchString(objHash)
	return isAlphanumeric
}

func isValidMode(mode int) bool {
	return slices.Contains(VALID_MODES, mode)
}

func getObjectType(objHash string, repoDir string) (ObjectType, error) {
	objType, _, _, err := ReadObjectFile(objHash, repoDir)
	if err != nil {
		return -1, err
	}

	return objType, nil
}

func getObjectTypeFromMode(mode int) ObjectType {
	if mode == 40000 {
		return Tree
	} else {
		return Blob
	}
}

func getGitModeFromFileMode(fileMode os.FileMode) int {
	if fileMode.IsDir() {
		return DIRECTORY_MODE
	} else if fileMode&os.ModeSymlink != 0 {
		return SYMBOLIC_LINK_MODE
	} else if fileMode&0111 != 0 {
		return EXECUTABLE_FILE_MODE
	} else {
		return REGULAR_FILE_MODE
	}
}

func GetObject(objHash string, repoDir string) (GitObject, error) {
	objType, err := getObjectType(objHash, repoDir)
	if err != nil {
		return nil, err
	}

	var gitObj GitObject
	switch objType {
	case Blob:
		blobObj, err := ReadBlobObjectFile(objHash, repoDir)
		if err != nil {
			return nil, err
		}
		gitObj = blobObj
	case Tree:
		treeObj, err := ReadTreeObjectFile(objHash, repoDir)
		if err != nil {
			return nil, err
		}
		gitObj = treeObj
	case Commit:
		commitObj, err := ReadCommitObjectFile(objHash, repoDir)
		if err != nil {
			return nil, err
		}
		gitObj = commitObj
	default:
		return nil, fmt.Errorf("unsupported Git object type")
	}

	return gitObj, nil
}

func ReadObjectFile(objHash string, repoDir string) (ObjectType, int, []byte, error) {
	objPath := filepath.Join(repoDir, ".git", "objects", objHash[:2], objHash[2:])
	file, err := os.Open(objPath)
	if err != nil {
		return -1, -1, nil, fmt.Errorf("failed to open object file")
	}
	defer file.Close()

	data, err := zlibDecompress(file)
	if err != nil {
		return -1, -1, nil, err
	}

	nullByteIndex := bytes.IndexByte(data, 0)
	if nullByteIndex == -1 {
		return -1, -1, nil, fmt.Errorf("object file poorly formatted: missing null byte separator")
	}

	header := string(data[:nullByteIndex])
	headerParts := strings.Split(header, " ")
	headerObjTypeStr := headerParts[0]
	if len(headerParts) != 2 {
		return -1, -1, nil, fmt.Errorf("invalid object header: %s", header)
	}
	headerObjType, err := ObjTypeFromString(headerObjTypeStr)
	if err != nil {
		return -1, -1, nil, fmt.Errorf("invalid object type in header: %s", header)
	}

	sizeBytes, err := strconv.Atoi(headerParts[1])
	if err != nil {
		return -1, -1, nil, fmt.Errorf("invalid size in object header: %s", err)
	}

	content := data[nullByteIndex+1:]

	return headerObjType, sizeBytes, content, nil
}

func CreateObjectFile(objType ObjectType, contentBytes []byte, repoDir string) (string, error) {
	sizeBytes := len(contentBytes)
	header := fmt.Sprintf("%s %d\x00", objType.toString(), sizeBytes)
	headerBytes := []byte(header)

	fileBytes := make([]byte, len(headerBytes)+len(contentBytes))
	copy(fileBytes, headerBytes)
	copy(fileBytes[len(headerBytes):], contentBytes)

	objHashBytes := sha1.Sum(fileBytes)
	objHash := hex.EncodeToString(objHashBytes[:])

	objPath := filepath.Join(repoDir, ".git", "objects", objHash[:2], objHash[2:])

	dir := filepath.Dir(objPath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create directories storing object file")
	}

	objFile, err := os.Create(objPath)
	if err != nil {
		return "", fmt.Errorf("failed to create object file")
	}
	defer objFile.Close()

	err = zlibCompress(objFile, fileBytes)
	if err != nil {
		return "", err
	}

	return objHash, nil
}

/** BLOBS */

func ReadBlobObjectFile(objHash string, repoDir string) (*BlobObject, error) {
	headerObjType, sizeBytes, content, err := ReadObjectFile(objHash, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read blob object file: %s", err)
	}

	if headerObjType != Blob {
		return nil, fmt.Errorf("expected blob object, received %s", headerObjType.toString())
	}

	return &BlobObject{
		hash:      objHash,
		sizeBytes: sizeBytes,
		content:   content,
	}, nil
}

func CreateBlobObjectFromFile(filePath string, repoDir string) (*BlobObject, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file")
	}
	sizeBytes := len(content)

	blobObjHash, err := CreateObjectFile(Blob, content, repoDir)
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

func ReadTreeObjectFile(objHash string, repoDir string) (*TreeObject, error) {
	headerObjType, sizeBytes, content, err := ReadObjectFile(objHash, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tree object file: %s", err)
	}

	if headerObjType != Tree {
		return nil, fmt.Errorf("expected tree object, received %s", headerObjType.toString())
	}

	entries := []TreeObjectEntry{}
	i := 0
	for i < len(content) {
		nullByteIndex := bytes.IndexByte(content[i:], 0)
		if nullByteIndex == -1 {
			return nil, fmt.Errorf("invalid tree object entry: missing null byte separator")
		}

		entryHeader := string(content[i : i+nullByteIndex])
		entryHashStartIndex := i + nullByteIndex + 1
		if entryHashStartIndex+OBJECT_HASH_LENGTH_BYTES > len(content) {
			return nil, fmt.Errorf("invalid tree object entry: not long enough to contain SHA hash")
		}
		entryHash := fmt.Sprintf("%x", content[entryHashStartIndex:entryHashStartIndex+OBJECT_HASH_LENGTH_BYTES])

		entry, err := parseTreeObjectEntry(entryHeader, entryHash)
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)

		i = entryHashStartIndex + OBJECT_HASH_LENGTH_BYTES
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

func CreateTreeObjectFromDirectory(dir string, repoDir string) (*TreeObject, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read contents of directory %s", dir)
	}

	entries := []TreeObjectEntry{}
	for _, dirEntry := range dirEntries {
		if strings.HasPrefix(dirEntry.Name(), ".") {
			continue
		}

		fullPath := filepath.Join(dir, dirEntry.Name())

		if dirEntry.IsDir() {
			subDirTreeObj, err := CreateTreeObjectFromDirectory(fullPath, repoDir)
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

			mode := getGitModeFromFileMode(fileInfo.Mode())

			fileBlobObj, err := CreateBlobObjectFromFile(fullPath, repoDir)
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

	return createTreeObject(entries, repoDir)
}

func CreateTreeObjectFromIndex(repoDir string) (*TreeObject, error) {
	indexEntries, err := ReadIndex(repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read Git index file: %s", err)
	}

	dirSet := make(map[string]struct{})
	dirSet["."] = struct{}{}
	dirToSubDirs := make(map[string](map[string]struct{}))
	for _, entry := range indexEntries {
		path := entry.path
		currDir := filepath.Dir(path)
		for currDir != "." && currDir != "/" {
			dirSet[currDir] = struct{}{}
			parentDir := filepath.Dir(currDir)
			if _, exists := dirToSubDirs[parentDir]; !exists {
				dirToSubDirs[parentDir] = make(map[string]struct{})
			}
			dirToSubDirs[parentDir][currDir] = struct{}{}
			currDir = parentDir
		}
	}

	dirToEntries := make(map[string][]TreeObjectEntry)
	for _, indexEntry := range indexEntries {
		dir := filepath.Dir(indexEntry.path)
		entry := TreeObjectEntry{
			hash:    hex.EncodeToString(indexEntry.sha1[:]),
			mode:    int(indexEntry.mode),
			name:    filepath.Base(indexEntry.path),
			objType: Blob,
		}
		dirToEntries[dir] = append(dirToEntries[dir], entry)
	}

	for dir := range dirSet {
		if _, exists := dirToSubDirs[dir]; !exists {
			dirToSubDirs[dir] = make(map[string]struct{})
		}
		if _, exists := dirToEntries[dir]; !exists {
			dirToEntries[dir] = []TreeObjectEntry{}
		}
	}

	treeObj, err := createTreeObjectFromDirInfo(".", dirToSubDirs, dirToEntries, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create tree object from directory info: %s", err)
	}

	return treeObj, nil
}

func parseTreeObjectEntry(entryHeader string, entryHash string) (*TreeObjectEntry, error) {
	entryHeaderParts := strings.Split(entryHeader, " ")
	if len(entryHeaderParts) != 2 {
		return nil, fmt.Errorf("tree object entry mode and name should be space-separated")
	}

	mode, err := strconv.Atoi(entryHeaderParts[0])
	if err != nil {
		return nil, fmt.Errorf("tree object entry mode should be an integer")
	}
	if !isValidMode(mode) {
		return nil, fmt.Errorf("invalid tree object entry mode: %d", mode)
	}

	if !isValidObjectHash(entryHash) {
		return nil, fmt.Errorf("invalid tree object entry hash: %s", entryHash)
	}

	return &TreeObjectEntry{
		hash:    entryHash,
		mode:    mode,
		name:    entryHeaderParts[1],
		objType: getObjectTypeFromMode(mode),
	}, nil
}

func createTreeObject(entries []TreeObjectEntry, repoDir string) (*TreeObject, error) {
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].name < entries[j].name
	})

	var contentBuilder strings.Builder
	for _, entry := range entries {
		fmt.Fprintf(&contentBuilder, "%d %s\x00", entry.mode, entry.name)

		hashBytes, err := hex.DecodeString(entry.hash)
		if err != nil {
			return nil, fmt.Errorf("invalid hash format: %s", err)
		}
		contentBuilder.Write(hashBytes)
	}
	contentBytes := []byte(contentBuilder.String())
	sizeBytes := len(contentBytes)

	treeObjHash, err := CreateObjectFile(Tree, contentBytes, repoDir)
	if err != nil {
		return nil, err
	}

	return &TreeObject{
		hash:      treeObjHash,
		sizeBytes: sizeBytes,
		entries:   entries,
	}, nil
}

func createTreeObjectFromDirInfo(dir string, dirToSubDirs map[string](map[string]struct{}), dirToEntries map[string][]TreeObjectEntry, repoDir string) (*TreeObject, error) {
	subDirs, exists := dirToSubDirs[dir]
	if !exists {
		return nil, fmt.Errorf("directory %s does not exist in mapping to subdirectories", dir)
	}

	entries, exists := dirToEntries[dir]
	if !exists {
		return nil, fmt.Errorf("directory %s does not exist in mapping to tree object entries", dir)
	}

	for subDir, _ := range subDirs {
		subDirTreeObj, err := createTreeObjectFromDirInfo(subDir, dirToSubDirs, dirToEntries, repoDir)
		if err != nil {
			return nil, err
		}

		entries = append(entries, TreeObjectEntry{
			hash:    subDirTreeObj.hash,
			mode:    DIRECTORY_MODE,
			name:    filepath.Base(subDir),
			objType: Tree,
		})
	}

	return createTreeObject(entries, repoDir)
}

func getBlobsInTree(treeHash string, repoDir string) ([]string, error) {
	treeObj, err := ReadTreeObjectFile(treeHash, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tree object file: %s", err)
	}

	blobHashes := []string{}
	for _, entry := range treeObj.entries {
		switch entry.objType {
		case Blob:
			blobHashes = append(blobHashes, entry.hash)
		case Tree:
			subTreeBlobHashes, err := getBlobsInTree(entry.hash, repoDir)
			if err != nil {
				return nil, fmt.Errorf("failed to get blobs in sub-tree: %s", err)
			}
			blobHashes = append(blobHashes, subTreeBlobHashes...)
		default:
			return nil, fmt.Errorf("unexpected object type %s in tree %s", entry.objType.toString(), treeHash)
		}
	}

	return blobHashes, nil
}

/** COMMITS */

func ReadCommitObjectFile(objHash string, repoDir string) (*CommitObject, error) {
	headerObjType, sizeBytes, content, err := ReadObjectFile(objHash, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read commit object file: %s", err)
	}

	if headerObjType != Commit {
		return nil, fmt.Errorf("expected commit object, received %s", headerObjType.toString())
	}

	lines := strings.Split(string(content), "\n")
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
	commitMessage := strings.Join(lines[i+3:], "\n")

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

func CreateCommitObjectFromTree(treeHash string, parentCommitHashes []string, commitMessage string, repoDir string) (*CommitObject, error) {
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

	fmt.Fprintf(&contentBuilder, "\n%s", commitMessage)

	contentBytes := []byte(contentBuilder.String())
	sizeBytes := len(contentBytes)
	commitObjHash, err := CreateObjectFile(Commit, contentBytes, repoDir)
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

func GetBlobsInCommit(commitHash string, repoDir string) ([]string, error) {
	commitObj, err := ReadCommitObjectFile(commitHash, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read commit object file: %s", err)
	}

	blobHashes, err := getBlobsInTree(commitObj.treeHash, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get blobs in commit tree: %s", err)
	}

	return blobHashes, nil
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

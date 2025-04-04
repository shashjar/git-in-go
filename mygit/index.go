package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

const (
	INDEX_HEADER_LENGTH   = 12
	INDEX_SIGNATURE       = "DIRC"
	INDEX_CHECKSUM_LENGTH = 20
)

// Represents an entry (representing a file in the repository) in the Git index file
type IndexEntry struct {
	cTimeSec     uint32
	cTimeNanoSec uint32
	mTimeSec     uint32
	mTimeNanoSec uint32
	dev          uint32
	ino          uint32
	mode         uint32
	uid          uint32
	gid          uint32
	fileSize     uint32
	sha1         [OBJECT_HASH_LENGTH_BYTES]byte
	flags        uint16
	path         string
}

func ReadIndex(repoDir string) ([]*IndexEntry, error) {
	indexPath := filepath.Join(repoDir, ".git", "index")

	index, err := os.ReadFile(indexPath)
	if err != nil && os.IsNotExist(err) {
		return []*IndexEntry{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to read Git index file: %s", err)
	}

	err = verifyIndexChecksum(index)
	if err != nil {
		return nil, err
	}
	index = index[:len(index)-INDEX_CHECKSUM_LENGTH]

	i := 0

	numEntries, err := readIndexHeader(index)
	if err != nil {
		return nil, err
	}
	i += INDEX_HEADER_LENGTH

	entries, err := readIndexEntries(index, i, numEntries)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func AddFilesToIndex(paths []string, repoDir string) error {
	currIndexEntries, err := ReadIndex(repoDir)
	if err != nil {
		return err
	}

	pathsSet := make(map[string]bool, len(paths))
	for _, path := range paths {
		pathsSet[path] = true
	}

	entriesToKeep := []*IndexEntry{}
	for _, entry := range currIndexEntries {
		if _, adding := pathsSet[entry.path]; !adding {
			entriesToKeep = append(entriesToKeep, entry)
		}
	}

	newIndexEntries := entriesToKeep
	for path, _ := range pathsSet {
		entry, err := createIndexEntry(path, repoDir)
		if err != nil {
			return fmt.Errorf("failed to create index entry for '%s': %s", path, err)
		}
		newIndexEntries = append(newIndexEntries, entry)
	}

	err = writeIndex(newIndexEntries, repoDir)
	if err != nil {
		return fmt.Errorf("failed to write updated Git index file: %s", err)
	}

	return nil
}

func RemoveFilesFromIndex(paths []string, repoDir string) error {
	currIndexEntries, err := ReadIndex(repoDir)
	if err != nil {
		return err
	}

	pathsSet := make(map[string]bool, len(paths))
	for _, path := range paths {
		pathsSet[path] = true
	}

	entriesToKeep := []*IndexEntry{}
	for _, entry := range currIndexEntries {
		if _, removing := pathsSet[entry.path]; !removing {
			entriesToKeep = append(entriesToKeep, entry)
		}
	}

	err = writeIndex(entriesToKeep, repoDir)
	if err != nil {
		return fmt.Errorf("failed to write updated Git index file: %s", err)
	}

	return nil
}

func createIndexEntry(path string, repoDir string) (*IndexEntry, error) {
	info, err := os.Stat(filepath.Join(repoDir, path))
	if err != nil {
		return nil, err
	}
	stat := info.Sys().(*syscall.Stat_t)

	if info.IsDir() {
		return nil, fmt.Errorf("unable to create an index entry for a directory: '%s'", path)
	}

	blobObj, err := CreateBlobObjectFromFile(path, repoDir)
	if err != nil {
		return nil, fmt.Errorf("unable to create a blob object for this index entry: '%s'", path)
	}

	objHashBytes, err := hex.DecodeString(blobObj.hash)
	if err != nil {
		return nil, fmt.Errorf("invalid hash format: %s", err)
	}

	entry := &IndexEntry{
		cTimeSec:     uint32(stat.Ctimespec.Sec),
		cTimeNanoSec: uint32(stat.Ctimespec.Nsec),
		mTimeSec:     uint32(stat.Mtimespec.Sec),
		mTimeNanoSec: uint32(stat.Mtimespec.Nsec),
		dev:          uint32(stat.Dev),
		ino:          uint32(stat.Ino),
		mode:         uint32(getGitModeFromFileMode(info.Mode())),
		uid:          stat.Uid,
		gid:          stat.Gid,
		fileSize:     uint32(info.Size()),
		sha1:         [OBJECT_HASH_LENGTH_BYTES]byte{},
		flags:        0,
		path:         path,
	}
	copy(entry.sha1[:], objHashBytes)

	return entry, nil
}

func writeIndex(entries []*IndexEntry, repoDir string) error {
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].path < entries[j].path
	})

	var indexBuf bytes.Buffer

	indexBuf.WriteString(INDEX_SIGNATURE)
	binary.Write(&indexBuf, binary.BigEndian, uint32(2))
	binary.Write(&indexBuf, binary.BigEndian, uint32(len(entries)))

	for _, entry := range entries {
		binary.Write(&indexBuf, binary.BigEndian, entry.cTimeSec)
		binary.Write(&indexBuf, binary.BigEndian, entry.cTimeNanoSec)
		binary.Write(&indexBuf, binary.BigEndian, entry.mTimeSec)
		binary.Write(&indexBuf, binary.BigEndian, entry.mTimeNanoSec)
		binary.Write(&indexBuf, binary.BigEndian, entry.dev)
		binary.Write(&indexBuf, binary.BigEndian, entry.ino)
		binary.Write(&indexBuf, binary.BigEndian, entry.mode)
		binary.Write(&indexBuf, binary.BigEndian, entry.uid)
		binary.Write(&indexBuf, binary.BigEndian, entry.gid)
		binary.Write(&indexBuf, binary.BigEndian, entry.fileSize)
		indexBuf.Write(entry.sha1[:])
		binary.Write(&indexBuf, binary.BigEndian, entry.flags)
		indexBuf.WriteString(entry.path)
		indexBuf.WriteByte(0)
	}

	indexData := indexBuf.Bytes()
	indexChecksum := sha1.Sum(indexData)

	indexPath := filepath.Join(repoDir, ".git", "index")
	indexFile, err := os.Create(indexPath)
	if err != nil {
		return fmt.Errorf("failed to create index file: %s", err)
	}
	defer indexFile.Close()

	_, err = indexFile.Write(indexData)
	if err != nil {
		return fmt.Errorf("failed to write content to index file: %s", err)
	}
	_, err = indexFile.Write(indexChecksum[:])
	if err != nil {
		return fmt.Errorf("failed to write checksum to index file: %s", err)
	}

	return nil
}

func verifyIndexChecksum(index []byte) error {
	if len(index) < INDEX_CHECKSUM_LENGTH {
		return fmt.Errorf("invalid index file: too short to contain a checksum")
	}

	expectedChecksum := index[len(index)-INDEX_CHECKSUM_LENGTH:]
	actualChecksum := sha1.Sum(index[:len(index)-INDEX_CHECKSUM_LENGTH])

	if !bytes.Equal(expectedChecksum, actualChecksum[:]) {
		return fmt.Errorf("invalid index file: actual checksum does not match expected checksum")
	}

	return nil
}

func readIndexHeader(index []byte) (int, error) {
	if len(index) < INDEX_HEADER_LENGTH {
		return -1, fmt.Errorf("invalid index file: too short to contain a header")
	}

	signature := string(index[0:4])
	if signature != INDEX_SIGNATURE {
		return -1, fmt.Errorf("invalid index file signature: expected '%s', got '%s'", INDEX_SIGNATURE, signature)
	}

	versionNumber := binary.BigEndian.Uint32(index[4:8])
	if versionNumber != 2 {
		return -1, fmt.Errorf("unsupported index file version number: expected 2, got %d", versionNumber)
	}

	numEntries := binary.BigEndian.Uint32(index[8:12])
	return int(numEntries), nil
}

func readIndexEntries(index []byte, i int, numEntries int) ([]*IndexEntry, error) {
	entries := make([]*IndexEntry, 0, numEntries)
	for range numEntries {
		var entry *IndexEntry
		var err error
		entry, i, err = readIndexEntry(index, i)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	if i != len(index) {
		return nil, fmt.Errorf("leftover data in index file after reading all expected entries")
	}

	return entries, nil
}

func readIndexEntry(index []byte, i int) (*IndexEntry, int, error) {
	if i+62 > len(index) {
		return nil, i, fmt.Errorf("index file is too short to contain another entry")
	}

	entry := &IndexEntry{
		cTimeSec:     binary.BigEndian.Uint32(index[i : i+4]),
		cTimeNanoSec: binary.BigEndian.Uint32(index[i+4 : i+8]),
		mTimeSec:     binary.BigEndian.Uint32(index[i+8 : i+12]),
		mTimeNanoSec: binary.BigEndian.Uint32(index[i+12 : i+16]),
		dev:          binary.BigEndian.Uint32(index[i+16 : i+20]),
		ino:          binary.BigEndian.Uint32(index[i+20 : i+24]),
		mode:         binary.BigEndian.Uint32(index[i+24 : i+28]),
		uid:          binary.BigEndian.Uint32(index[i+28 : i+32]),
		gid:          binary.BigEndian.Uint32(index[i+32 : i+36]),
		fileSize:     binary.BigEndian.Uint32(index[i+36 : i+40]),
		sha1:         [OBJECT_HASH_LENGTH_BYTES]byte{},
		flags:        binary.BigEndian.Uint16(index[i+60 : i+62]),
		path:         "",
	}
	copy(entry.sha1[:], index[i+40:i+40+OBJECT_HASH_LENGTH_BYTES])

	pathStartPos := i + 62
	pathEndPos := pathStartPos
	for pathEndPos < len(index) && index[pathEndPos] != 0 {
		pathEndPos += 1
	}

	entry.path = string(index[pathStartPos:pathEndPos])

	return entry, pathEndPos + 1, nil
}

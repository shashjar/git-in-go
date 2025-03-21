package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
)

const PACKFILE_CHECKSUM_LENGTH = 20

type PackfileObjectType int

const (
	PACKFILE_OBJ_COMMIT    PackfileObjectType = 1
	PACKFILE_OBJ_TREE      PackfileObjectType = 2
	PACKFILE_OBJ_BLOB      PackfileObjectType = 3
	PACKFILE_OBJ_TAG       PackfileObjectType = 4
	PACKFILE_OBJ_OFS_DELTA PackfileObjectType = 6
	PACKFILE_OBJ_REF_DELTA PackfileObjectType = 7
)

func (pot *PackfileObjectType) toString() string {
	if *pot == PACKFILE_OBJ_COMMIT {
		return "commit"
	} else if *pot == PACKFILE_OBJ_TREE {
		return "tree"
	} else if *pot == PACKFILE_OBJ_BLOB {
		return "blob"
	} else if *pot == PACKFILE_OBJ_TAG {
		return "tag"
	} else if *pot == PACKFILE_OBJ_OFS_DELTA {
		return "ofs_delta"
	} else if *pot == PACKFILE_OBJ_REF_DELTA {
		return "ref_delta"
	} else {
		return "unknown"
	}
}

// TODO: should I be using the index file to help parse the packfile?
func readPackfile(packfile []byte, repoDir string) error {
	err := verifyPackfileChecksum(packfile)
	if err != nil {
		return err
	}
	packfile = packfile[:len(packfile)-PACKFILE_CHECKSUM_LENGTH]

	numObjects, remainingPackfile, err := readPackfileHeader(packfile)
	if err != nil {
		return err
	}
	fmt.Printf("numObjects: %d\n\n", numObjects)

	err = readPackfileObjects(remainingPackfile, numObjects, repoDir)
	if err != nil {
		return err
	}

	return nil
}

func verifyPackfileChecksum(packfile []byte) error {
	if len(packfile) < PACKFILE_CHECKSUM_LENGTH {
		return fmt.Errorf("invalid packfile: too short to contain a checksum")
	}

	expectedChecksum := packfile[len(packfile)-PACKFILE_CHECKSUM_LENGTH:]
	actualChecksum := sha1.Sum(packfile[:len(packfile)-PACKFILE_CHECKSUM_LENGTH])

	if !bytes.Equal(expectedChecksum, actualChecksum[:]) {
		return fmt.Errorf("invalid packfile: actual checksum does not match expected checksum")
	}

	return nil
}

func readPackfileHeader(packfile []byte) (int, []byte, error) {
	if len(packfile) < 4 {
		return -1, nil, fmt.Errorf("invalid packfile: too short to contain a header")
	}

	signature := string(packfile[0:4])
	if signature != "PACK" {
		return -1, nil, fmt.Errorf("invalid packfile signature: expected 'PACK', got '%s'", signature)
	}

	versionNumber := binary.BigEndian.Uint32(packfile[4:8])
	if versionNumber != 2 {
		return -1, nil, fmt.Errorf("unsupported packfile version number: expected 2, got %d", versionNumber)
	}

	numObjects := binary.BigEndian.Uint32(packfile[8:12])
	return int(numObjects), packfile[12:], nil
}

func readPackfileObjectHeader(data []byte) (PackfileObjectType, int, []byte, error) {
	b := data[0]
	shift := 4
	packfileObjectType := PackfileObjectType((b >> shift) & 0x07)

	// Only the rightmost 4 bits of the first byte are used for the variable length encoding, so passing shift as 4
	packfileObjectLength, remainingData, err := readVariableLengthEncoding(data, shift)
	if err != nil {
		return -1, -1, nil, err
	}

	return packfileObjectType, packfileObjectLength, remainingData, nil
}

func readVariableLengthEncoding(data []byte, shift int) (int, []byte, error) {
	b := data[0]
	mask := byte((1 << shift) - 1)
	packfileObjectLength := int(b & mask)
	bytesRead := 1

	for (b & 0x80) != 0 {
		if bytesRead >= len(data) {
			return -1, nil, fmt.Errorf("incomplete packfile object length")
		}

		b = data[bytesRead]
		packfileObjectLength |= int(b&0x7F) << shift
		shift += 7
		bytesRead += 1
	}

	return packfileObjectLength, data[bytesRead:], nil
}

func decompressPackfileObject(data []byte, packfileObjectLength int) ([]byte, []byte, error) {
	// TODO: is this logic correct?
	decompressedObjData, compressedBytesRead, err := zlibDecompressWithReadCount(data)
	if err != nil {
		return nil, nil, err
	}

	if len(decompressedObjData) != packfileObjectLength {
		return nil, nil, fmt.Errorf("decompressed object data length mismatch: expected %d, got %d", packfileObjectLength, len(decompressedObjData))
	}

	remainingData := data[compressedBytesRead:]
	return decompressedObjData, remainingData, nil
}

func readPackfileObjects(data []byte, numObjects int, repoDir string) error {
	remainingData := data
	var err error
	for range numObjects {
		remainingData, err = readPackfileObject(remainingData, repoDir)
		if err != nil {
			return err
		}
	}

	if len(remainingData) != 0 {
		return fmt.Errorf("leftover data in packfile after reading all expected objects")
	}

	return nil
}

func readPackfileObject(data []byte, repoDir string) ([]byte, error) {
	packfileObjectType, packfileObjectLength, remainingData, err := readPackfileObjectHeader(data)
	if err != nil {
		return nil, err
	}
	fmt.Printf("\nNext object type: %s\n", packfileObjectType.toString())
	fmt.Printf("Object length (decompressed): %d\n", packfileObjectLength)

	// TODO: skipping over ofs_delta and ref_delta objects for now
	var objType string
	switch packfileObjType := PackfileObjectType(packfileObjectType); packfileObjType {
	case PACKFILE_OBJ_COMMIT, PACKFILE_OBJ_TREE, PACKFILE_OBJ_BLOB, PACKFILE_OBJ_TAG: // TODO: make sure tag objects are correctly created/handled here
		objType = packfileObjType.toString()
	case PACKFILE_OBJ_OFS_DELTA:
		remainingData, err = readOfsDeltaPackfileObject(remainingData, packfileObjectLength)
		return remainingData, err
	case PACKFILE_OBJ_REF_DELTA:
		remainingData, err = readRefDeltaPackfileObject(remainingData, packfileObjectLength)
		return remainingData, err
	default:
		return nil, fmt.Errorf("unsupported packfile object type: %d", packfileObjectType)
	}

	decompressedObjData, remainingData, err := decompressPackfileObject(remainingData, packfileObjectLength)
	if err != nil {
		return nil, err
	}
	fmt.Println("decompressedObjData:\n", string(decompressedObjData))

	objHash, err := createObjectFile(objType, decompressedObjData, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create object file: %s", err)
	}
	fmt.Printf("Created %s object: %s\n\n", objType, objHash)

	return remainingData, nil
}

// TODO: just skipping over the ofs_delta object for now
func readOfsDeltaPackfileObject(data []byte, packfileObjectLength int) ([]byte, error) {
	// TODO: ignoring the `offset` parsed by readVariableLengthEncoding for now
	// The rightmost 7 bits of the first byte are used for the variable length encoding, so passing shift as 7
	_, remainingData, err := readVariableLengthEncoding(data, 7)
	if err != nil {
		return nil, err
	}

	// TODO: ignoring the `decompressedOfsDeltaObjData` returned here for now
	_, remainingData, err = decompressPackfileObject(remainingData, packfileObjectLength)
	if err != nil {
		return nil, err
	}

	return remainingData, nil
}

// TODO: just skipping over the ref_delta object for now
func readRefDeltaPackfileObject(data []byte, packfileObjectLength int) ([]byte, error) {
	if len(data) < 20 {
		return nil, fmt.Errorf("invalid ref_delta packfile object: too short to contain base object SHA")
	}

	baseObjSHA := fmt.Sprintf("%x", data[:20])
	remainingData := data[20:]
	fmt.Println("Base object SHA:", baseObjSHA)

	// TODO: ignoring the `decompressedRefDeltaObjData` returned here for now
	_, remainingData, err := decompressPackfileObject(remainingData, packfileObjectLength)
	if err != nil {
		return nil, err
	}

	return remainingData, nil
}

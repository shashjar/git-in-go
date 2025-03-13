package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
)

const (
	PACKFILE_OBJ_COMMIT    = 1
	PACKFILE_OBJ_TREE      = 2
	PACKFILE_OBJ_BLOB      = 3
	PACKFILE_OBJ_TAG       = 4
	PACKFILE_OBJ_OFS_DELTA = 6
	PACKFILE_OBJ_REF_DELTA = 7
)

func readPackfile(packfile []byte) error {
	err := verifyPackfileChecksum(packfile)
	if err != nil {
		return err
	}

	numObjects, remainingPackfile, err := parsePackfileHeader(packfile)
	if err != nil {
		return err
	}
	fmt.Println("numObjects:", numObjects)

	err = readPackfileObjects(remainingPackfile, numObjects)
	if err != nil {
		return err
	}

	return nil
}

func verifyPackfileChecksum(packfile []byte) error {
	if len(packfile) < 20 {
		return fmt.Errorf("invalid packfile: too short to contain a checksum")
	}

	expectedChecksum := packfile[len(packfile)-20:]
	actualChecksum := sha1.Sum(packfile[:len(packfile)-20])

	if !bytes.Equal(expectedChecksum, actualChecksum[:]) {
		return fmt.Errorf("invalid packfile: actual checksum does not match expected checksum")
	}

	return nil
}

func parsePackfileHeader(packfile []byte) (int, []byte, error) {
	if len(packfile) < 4 {
		return -1, nil, fmt.Errorf("invalid packfile: too short to contain a header")
	}

	signature := string(packfile[0:4])
	if signature != "PACK" {
		return -1, nil, fmt.Errorf("invalid packfile signature: expected 'PACK', got '%s'", signature)
	}

	versionNumber := binary.BigEndian.Uint32(packfile[4:8])
	if versionNumber != 2 && versionNumber != 3 {
		return -1, nil, fmt.Errorf("unsupported packfile version number: expected 2 or 3, got %d", versionNumber)
	}

	numObjects := binary.BigEndian.Uint32(packfile[8:12])
	return int(numObjects), packfile[12:], nil
}

func readPackfileObjects(data []byte, numObjects int) error {
	remainingData := data
	var err error
	for range numObjects {
		remainingData, err = readPackfileObject(remainingData)
		if err != nil {
			return err
		}
	}
	return nil
}

func readPackfileObject(data []byte) ([]byte, error) {
	packfileObjectType, packfileObjectLength, remainingData, err := parsePackfileObjectTypeAndLength(data)
	if err != nil {
		return nil, err
	}
	fmt.Println("\npackfileObjectType:", packfileObjectType)
	fmt.Println("packfileObjectLength:", packfileObjectLength)

	// TODO: this is still wrong - trying to decompress the wrong amount of data with zlib
	decompressedObjData, err := zlibDecompress(bytes.NewReader(remainingData))
	if err != nil {
		return nil, err
	}
	fmt.Println("decompressedObjData:\n", string(decompressedObjData))

	switch packfileObjectType {
	case PACKFILE_OBJ_COMMIT, PACKFILE_OBJ_TREE, PACKFILE_OBJ_BLOB:
		// These are valid, continue normally
	case PACKFILE_OBJ_TAG: // TODO: tags not supported right now, but might want to add them
		return nil, fmt.Errorf("unsupported packfile object type: tag")
	case PACKFILE_OBJ_OFS_DELTA:
		return nil, fmt.Errorf("unsupported packfile object type: ofs delta")
	case PACKFILE_OBJ_REF_DELTA:
		return nil, fmt.Errorf("unsupported packfile object type: ref delta")
	default:
		return nil, fmt.Errorf("unsupported packfile object type: %d", packfileObjectType)
	}

	return remainingData, nil
}

func parsePackfileObjectTypeAndLength(data []byte) (int, int, []byte, error) {
	b := data[0]
	packfileObjectType := int((b >> 4) & 0x07)
	packfileObjectLength := int(b & 0x0F)
	shift := 4
	bytesRead := 1

	for (b & 0x80) != 0 {
		if bytesRead >= len(data) {
			return -1, -1, nil, fmt.Errorf("incomplete packfile object header")
		}

		b = data[bytesRead]
		packfileObjectLength |= int(b&0x7F) << shift
		shift += 7
		bytesRead += 1
	}

	return packfileObjectType, packfileObjectLength, data[bytesRead:], nil
}

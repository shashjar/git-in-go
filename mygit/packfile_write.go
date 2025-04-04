package main

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
)

func CreatePackfile(objHashes []string, repoDir string) ([]byte, error) {
	packfile := []byte{}

	if len(objHashes) == 0 {
		return nil, fmt.Errorf("no objects provided for packfile creation")
	}

	packfile = append(packfile, PACKFILE_SIGNATURE...)
	packfile = binary.BigEndian.AppendUint32(packfile, PACKFILE_VERSION_NUMBER)
	packfile = binary.BigEndian.AppendUint32(packfile, uint32(len(objHashes)))

	for _, objHash := range objHashes {
		encodedObj, err := encodePackfileObject(objHash, repoDir)
		if err != nil {
			return nil, fmt.Errorf("failed to encode object %s: %s", objHash, err)
		}

		packfile = append(packfile, encodedObj...)
	}

	checksum := sha1.Sum(packfile)
	packfile = append(packfile, checksum[:]...)

	return packfile, nil
}

func encodePackfileObject(objHash string, repoDir string) ([]byte, error) {
	packfileObj := []byte{}

	objType, _, objContent, err := ReadObjectFile(objHash, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read object file with hash %s: %s", objHash, err)
	}

	packfileObjType, err := packfileObjTypeFromString(objType.toString())
	if err != nil {
		return nil, fmt.Errorf("invalid packfile object type: %s", objType.toString())
	}

	size := len(objContent)
	if size == 0 {
		return nil, fmt.Errorf("empty object content for hash %s", objHash)
	}

	header, err := encodePackfileObjectHeader(packfileObjType, size)
	if err != nil {
		return nil, fmt.Errorf("failed to encode packfile object header: %s", err)
	}
	packfileObj = append(packfileObj, header...)

	compressedObjData, err := zlibCompressBytes(objContent)
	if err != nil {
		return nil, fmt.Errorf("failed to compress packfile object content: %s", err)
	}
	packfileObj = append(packfileObj, compressedObjData...)

	return packfileObj, nil
}

func encodePackfileObjectHeader(packfileObjType PackfileObjectType, size int) ([]byte, error) {
	shift := 4
	encodedSize := encodeVariableLengthSize(size, shift)
	if len(encodedSize) == 0 {
		return nil, fmt.Errorf("failed to encode packfile object size: %d", size)
	}

	// Header byte: bits 6-4 are object type
	firstByte := encodedSize[0]
	firstByte |= byte(packfileObjType) << shift

	header := []byte{firstByte}
	header = append(header, encodedSize[1:]...)
	return header, nil
}

// Used for encoding sizes in the packfile (later values more significant)
func encodeVariableLengthSize(size int, shift int) []byte {
	encodedSize := []byte{}

	// Header byte: bits 3-0 start the size & bit 7 indicates if there are more size bytes
	firstByte := byte(size & ((1 << shift) - 1))
	size >>= shift
	if size > 0 {
		firstByte |= 0x80
	}
	encodedSize = append(encodedSize, firstByte)

	for size > 0 {
		sizeByte := byte(size & 0x7f)
		size >>= 7
		if size > 0 {
			sizeByte |= 0x80
		}
		encodedSize = append(encodedSize, sizeByte)
	}

	return encodedSize
}

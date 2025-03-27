package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
)

const (
	PACKFILE_CHECKSUM_LENGTH = 20
	PACKFILE_HEADER_LENGTH   = 12
)

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

// Represents a packfile deltified object (ref delta), which is a delta of a base object (referenced by hash)
// using COPY and ADD instructions for sequences of data
type PackfileRefDeltaObject struct {
	baseObjHash string
	deltaData   []byte
}

func readPackfile(packfile []byte, repoDir string) error {
	err := verifyPackfileChecksum(packfile)
	if err != nil {
		return err
	}
	packfile = packfile[:len(packfile)-PACKFILE_CHECKSUM_LENGTH]

	i := 0

	numObjects, err := readPackfileHeader(packfile)
	if err != nil {
		return err
	}
	i += PACKFILE_HEADER_LENGTH

	err = readPackfileObjects(packfile, i, numObjects, repoDir)
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

func readPackfileHeader(packfile []byte) (int, error) {
	if len(packfile) < PACKFILE_HEADER_LENGTH {
		return -1, fmt.Errorf("invalid packfile: too short to contain a header")
	}

	signature := string(packfile[0:4])
	if signature != "PACK" {
		return -1, fmt.Errorf("invalid packfile signature: expected 'PACK', got '%s'", signature)
	}

	versionNumber := binary.BigEndian.Uint32(packfile[4:8])
	if versionNumber != 2 {
		return -1, fmt.Errorf("unsupported packfile version number: expected 2, got %d", versionNumber)
	}

	numObjects := binary.BigEndian.Uint32(packfile[8:12])
	return int(numObjects), nil
}

func readPackfileObjectHeader(packfile []byte, i int) (PackfileObjectType, int, int, error) {
	b := packfile[i]
	shift := 4
	packfileObjectType := PackfileObjectType((b >> shift) & 0x07)

	// Only the rightmost 4 bits of the first byte are used for the variable length encoding, so passing shift as 4
	packfileObjectLength, i, err := readVariableSizeEncoding(packfile, i, shift)
	if err != nil {
		return -1, -1, -1, err
	}

	return packfileObjectType, packfileObjectLength, i, nil
}

// Used for encoding sizes in the packfile (later values more significant)
func readVariableSizeEncoding(data []byte, i int, shift int) (int, int, error) {
	b := data[i]
	mask := byte((1 << shift) - 1)
	decodedSize := int(b & mask)
	bytesRead := 1

	for (b & 0x80) != 0 {
		if i+bytesRead >= len(data) {
			return -1, -1, fmt.Errorf("data not long enough to read variable size encoding")
		}

		b = data[i+bytesRead]
		decodedSize |= int(b&0x7F) << shift // Shift the new 7 bits received, as they are the most significant
		shift += 7
		bytesRead += 1
	}

	return decodedSize, i + bytesRead, nil
}

// Used for encoding offsets (for ofs delta objects) in the packfile (later values less significant)
func readVariableOffsetEncoding(data []byte, i int) (int, int, error) {
	b := data[i]
	decodedOffset := int(b & 0x7F)
	bytesRead := 1

	for (b & 0x80) != 0 {
		if i+bytesRead >= len(data) {
			return -1, -1, fmt.Errorf("data not long enough to read variable offset encoding")
		}

		b = data[i+bytesRead]
		decodedOffset = (decodedOffset + 1) << 7 // Apply bias for multi-byte offsets
		decodedOffset |= int(b & 0x7F)           // Append next 7 bits
		bytesRead += 1
	}

	return decodedOffset, i + bytesRead, nil
}

func decompressPackfileObject(data []byte, i int, packfileObjectLength int) ([]byte, int, error) {
	decompressedObjData, compressedBytesRead, err := zlibDecompressWithReadCount(data[i:])
	if err != nil {
		return nil, -1, err
	}

	if len(decompressedObjData) != packfileObjectLength {
		return nil, -1, fmt.Errorf("decompressed object data length mismatch: expected %d, got %d", packfileObjectLength, len(decompressedObjData))
	}

	return decompressedObjData, i + compressedBytesRead, nil
}

func readPackfileObjects(packfile []byte, i int, numObjects int, repoDir string) error {
	refDeltaObjs := []*PackfileRefDeltaObject{}

	for range numObjects {
		var refDeltaObj *PackfileRefDeltaObject
		var err error
		refDeltaObj, i, err = readPackfileObject(packfile, i, repoDir)
		if err != nil {
			return err
		}

		if refDeltaObj != nil {
			refDeltaObjs = append(refDeltaObjs, refDeltaObj)
		}
	}

	if i != len(packfile) {
		return fmt.Errorf("leftover data in packfile after reading all expected objects")
	}

	err := applyRefDeltas(refDeltaObjs, repoDir)
	if err != nil {
		return err
	}

	return nil
}

func readPackfileObject(packfile []byte, i int, repoDir string) (*PackfileRefDeltaObject, int, error) {
	packfileObjectType, packfileObjectLength, i, err := readPackfileObjectHeader(packfile, i)
	if err != nil {
		return nil, -1, err
	}

	var objTypeStr string
	switch packfileObjType := PackfileObjectType(packfileObjectType); packfileObjType {
	case PACKFILE_OBJ_COMMIT, PACKFILE_OBJ_TREE, PACKFILE_OBJ_BLOB, PACKFILE_OBJ_TAG: // TODO: make sure tag objects are correctly created/handled here
		objTypeStr = packfileObjType.toString()
	case PACKFILE_OBJ_OFS_DELTA:
		i, err = readOfsDeltaPackfileObject(packfile, i, packfileObjectLength, repoDir)
		return nil, i, err
	case PACKFILE_OBJ_REF_DELTA:
		refDeltaObj, i, err := readRefDeltaPackfileObject(packfile, i, packfileObjectLength)
		return refDeltaObj, i, err
	default:
		return nil, -1, fmt.Errorf("unsupported packfile object type: %d", packfileObjectType)
	}

	decompressedObjData, i, err := decompressPackfileObject(packfile, i, packfileObjectLength)
	if err != nil {
		return nil, -1, err
	}

	objType, err := objTypeFromString(objTypeStr)
	if err != nil {
		return nil, -1, err
	}

	_, err = createObjectFile(objType, decompressedObjData, repoDir)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to create object file: %s", err)
	}

	return nil, i, nil
}

func readOfsDeltaPackfileObject(packfile []byte, i int, packfileObjectLength int, repoDir string) (int, error) {
	// The rightmost 7 bits of the first byte are used for the variable length encoding, so passing shift as 7
	baseObjOffset, i, err := readVariableLengthEncoding(packfile, i, 7)
	if err != nil {
		return -1, err
	}

	deltaData, i, err := decompressPackfileObject(packfile, i, packfileObjectLength)
	if err != nil {
		return -1, err
	}

	baseObjPos := i - baseObjOffset
	if baseObjPos < 0 || baseObjPos >= len(packfile) {
		fmt.Println("i is:", i)
		return -1, fmt.Errorf("invalid base object position indicated by ofs delta object: %d", baseObjPos)
	}

	packfileObjectType, packfileObjectLength, j, err := readPackfileObjectHeader(packfile, baseObjPos)
	if err != nil {
		return -1, err
	}
	objType, err := objTypeFromString(packfileObjectType.toString())
	if err != nil {
		return -1, err
	}

	baseObjContent, _, err := decompressPackfileObject(packfile, j, packfileObjectLength)
	if err != nil {
		return -1, err
	}

	targetObjContent, err := applyDelta(deltaData, baseObjContent)
	if err != nil {
		return -1, err
	}

	_, err = createObjectFile(objType, targetObjContent, repoDir)
	if err != nil {
		return -1, err
	}

	return i, nil
}

func readRefDeltaPackfileObject(packfile []byte, i int, packfileObjectLength int) (*PackfileRefDeltaObject, int, error) {
	if len(packfile[i:]) < OBJECT_HASH_LENGTH_BYTES {
		return nil, -1, fmt.Errorf("invalid ref_delta packfile object: too short to contain base object SHA")
	}

	baseObjSHA := fmt.Sprintf("%x", packfile[i:i+OBJECT_HASH_LENGTH_BYTES])
	i += OBJECT_HASH_LENGTH_BYTES

	deltaData, i, err := decompressPackfileObject(packfile, i, packfileObjectLength)
	if err != nil {
		return nil, -1, err
	}

	return &PackfileRefDeltaObject{
		baseObjHash: baseObjSHA,
		deltaData:   deltaData,
	}, i, nil
}

func applyRefDeltas(refDeltaObjs []*PackfileRefDeltaObject, repoDir string) error {
	for _, refDeltaObj := range refDeltaObjs {
		objType, _, baseObjContent, err := readObjectFile(refDeltaObj.baseObjHash, repoDir)
		if err != nil {
			return fmt.Errorf("failed to read base object referenced by delta object: %s", err)
		}

		targetObjContent, err := applyDelta(refDeltaObj.deltaData, baseObjContent)
		if err != nil {
			return err
		}

		_, err = createObjectFile(objType, targetObjContent, repoDir)
		if err != nil {
			return err
		}
	}

	return nil
}

func applyDelta(deltaData []byte, baseObjContent []byte) ([]byte, error) {
	i := 0

	sourceSize, i, err := readVariableSizeEncoding(deltaData, i, 7)
	if err != nil {
		return nil, fmt.Errorf("failed to read source size in delta data: %s", err)
	}

	targetSize, i, err := readVariableSizeEncoding(deltaData, i, 7)
	if err != nil {
		return nil, fmt.Errorf("failed to read target size in delta data: %s", err)
	}

	if sourceSize != len(baseObjContent) {
		return nil, fmt.Errorf("source size in delta data does not match size specified in base object")
	}

	deltaInstructions := deltaData[i:]
	i = 0
	targetObjContent := make([]byte, 0, targetSize)
	for i < len(deltaInstructions) {
		cmd := deltaInstructions[i]
		i += 1

		// Bit 7 stores the command type
		cmdType := cmd & 0x80
		if cmdType == 128 { // COPY
			// Bits 3-0 specify the offset in the base object at which to start copying
			baseOffset := 0
			for j := 0; j <= 3; j++ {
				if (cmd & (1 << j)) != 0 {
					baseOffset |= int(deltaInstructions[i]) << (8 * j)
					i += 1
				}
			}

			// Bits 6-4 store the number of bytes from the base object to copy into the target object
			numCopyBytes := 0
			for j := 0; j <= 2; j++ {
				if (cmd & (16 << j)) != 0 {
					numCopyBytes |= int(deltaInstructions[i]) << (8 * j)
					i += 1
				}
			}

			// If numCopyBytes is 0, default to copying 0x10000 (65536) bytes
			if numCopyBytes == 0 {
				numCopyBytes = 0x10000
			}

			if baseOffset+numCopyBytes > len(baseObjContent) {
				return nil, fmt.Errorf("delta copy instruction out of bounds: offset=%d, numCopyBytes=%d, baseObjContent length=%d", baseOffset, numCopyBytes, len(baseObjContent))
			}

			copyContent := baseObjContent[baseOffset : baseOffset+numCopyBytes]
			targetObjContent = append(targetObjContent, copyContent...)
		} else if cmdType == 0 { // ADD
			// Bits 6-0 specify the number of bytes in the following content to add to the target object
			numAddBytes := int(cmd & 0x7F)

			if i+numAddBytes > len(deltaInstructions) {
				return nil, fmt.Errorf("delta add instruction out of bounds: numAddBytes=%d, deltaInstructions length=%d", numAddBytes, len(deltaInstructions))
			}

			addContent := deltaInstructions[i : i+numAddBytes]
			i += numAddBytes
			targetObjContent = append(targetObjContent, addContent...)
		} else {
			return nil, fmt.Errorf("expected 1 (COPY) or 0 (ADD) as command type bit in delta instructions")
		}
	}

	if len(targetObjContent) != targetSize {
		return nil, fmt.Errorf("delta object application resulted in incorrect target size: expected %d, got %d", targetSize, len(targetObjContent))
	}

	return targetObjContent, nil
}

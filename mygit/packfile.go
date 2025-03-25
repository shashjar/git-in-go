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

// Represents a packfile deltified object, which represents a delta of a base object included elsewhere in the packfile
type PackfileDeltaObject struct {
	bashObjHash string
	deltaData   []byte
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
	deltaObjs := []*PackfileDeltaObject{}

	remainingData := data
	for range numObjects {
		deltaObj, remaining, err := readPackfileObject(remainingData, repoDir)
		if err != nil {
			return err
		}
		remainingData = remaining

		if deltaObj != nil {
			deltaObjs = append(deltaObjs, deltaObj)
		}
	}

	if len(remainingData) != 0 {
		return fmt.Errorf("leftover data in packfile after reading all expected objects")
	}

	err := applyDeltas(deltaObjs, repoDir)
	if err != nil {
		return err
	}

	return nil
}

func readPackfileObject(data []byte, repoDir string) (*PackfileDeltaObject, []byte, error) {
	packfileObjectType, packfileObjectLength, remainingData, err := readPackfileObjectHeader(data)
	if err != nil {
		return nil, nil, err
	}

	var objTypeStr string
	switch packfileObjType := PackfileObjectType(packfileObjectType); packfileObjType {
	case PACKFILE_OBJ_COMMIT, PACKFILE_OBJ_TREE, PACKFILE_OBJ_BLOB, PACKFILE_OBJ_TAG: // TODO: make sure tag objects are correctly created/handled here
		objTypeStr = packfileObjType.toString()
	case PACKFILE_OBJ_OFS_DELTA:
		remainingData, err = readOfsDeltaPackfileObject(remainingData, packfileObjectLength)
		return nil, remainingData, err
	case PACKFILE_OBJ_REF_DELTA:
		deltaObj, remainingData, err := readRefDeltaPackfileObject(remainingData, packfileObjectLength)
		return deltaObj, remainingData, err
	default:
		return nil, nil, fmt.Errorf("unsupported packfile object type: %d", packfileObjectType)
	}

	decompressedObjData, remainingData, err := decompressPackfileObject(remainingData, packfileObjectLength)
	if err != nil {
		return nil, nil, err
	}

	objType, err := objTypeFromString(objTypeStr)
	if err != nil {
		return nil, nil, err
	}

	_, err = createObjectFile(objType, decompressedObjData, repoDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create object file: %s", err)
	}

	return nil, remainingData, nil
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

func readRefDeltaPackfileObject(data []byte, packfileObjectLength int) (*PackfileDeltaObject, []byte, error) {
	if len(data) < 20 {
		return nil, nil, fmt.Errorf("invalid ref_delta packfile object: too short to contain base object SHA")
	}

	baseObjSHA := fmt.Sprintf("%x", data[:20])
	remainingData := data[20:]

	deltaData, remainingData, err := decompressPackfileObject(remainingData, packfileObjectLength)
	if err != nil {
		return nil, nil, err
	}

	return &PackfileDeltaObject{
		bashObjHash: baseObjSHA,
		deltaData:   deltaData,
	}, remainingData, nil
}

func applyDeltas(deltaObjs []*PackfileDeltaObject, repoDir string) error {
	for _, deltaObj := range deltaObjs {
		remainingData := deltaObj.deltaData

		sourceSize, remainingData, err := readVariableLengthEncoding(remainingData, 7)
		if err != nil {
			return fmt.Errorf("failed to read source size in delta object: %s", err)
		}

		targetSize, remainingData, err := readVariableLengthEncoding(remainingData, 7)
		if err != nil {
			return fmt.Errorf("failed to read target size in delta object: %s", err)
		}

		objType, baseObjSizeBytes, baseObjContent, err := readObjectFile(deltaObj.bashObjHash, repoDir)
		if err != nil {
			return fmt.Errorf("failed to base object referenced by delta object: %s", err)
		}

		if sourceSize != baseObjSizeBytes {
			return fmt.Errorf("source size in delta object does not match size specified in base object")
		}

		targetObjContent, err := applyDelta(remainingData, targetSize, baseObjContent)
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

func applyDelta(deltaInstructions []byte, targetSize int, baseObjContent []byte) ([]byte, error) {
	targetObjContent := make([]byte, 0, targetSize)
	i := 0
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

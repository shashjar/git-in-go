package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
)

func ReadPackfile(packfile []byte, repoDir string) error {
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
	fmt.Printf("remote: Enumerating objects: %d, done.\n", numObjects)
	i += PACKFILE_HEADER_LENGTH

	err = readPackfileObjects(packfile, i, numObjects, repoDir)
	if err != nil {
		return err
	}
	fmt.Printf("Reading objects: 100%% (%d/%d), done.\n", numObjects, numObjects)

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
	if signature != PACKFILE_SIGNATURE {
		return -1, fmt.Errorf("invalid packfile signature: expected 'PACK', got '%s'", signature)
	}

	versionNumber := binary.BigEndian.Uint32(packfile[4:8])
	if versionNumber != PACKFILE_VERSION_NUMBER {
		return -1, fmt.Errorf("unsupported packfile version number: expected %d, got %d", PACKFILE_VERSION_NUMBER, versionNumber)
	}

	numObjects := binary.BigEndian.Uint32(packfile[8:12])
	return int(numObjects), nil
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
	packfileObjectStartPos := i

	packfileObjectType, packfileObjectLength, i, err := readPackfileObjectHeader(packfile, i)
	if err != nil {
		return nil, -1, err
	}

	var objTypeStr string
	switch packfileObjType := PackfileObjectType(packfileObjectType); packfileObjType {
	case PACKFILE_OBJ_COMMIT, PACKFILE_OBJ_TREE, PACKFILE_OBJ_BLOB, PACKFILE_OBJ_TAG:
		objTypeStr = packfileObjType.toString()
	case PACKFILE_OBJ_OFS_DELTA:
		_, i, err = applyOfsDeltaPackfileObject(packfile, i, packfileObjectStartPos, packfileObjectLength, repoDir)
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

	objType, err := ObjTypeFromString(objTypeStr)
	if err != nil {
		return nil, -1, err
	}

	_, err = CreateObjectFile(objType, decompressedObjData, repoDir)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to create object file: %s", err)
	}

	return nil, i, nil
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

// Used for reading encoded sizes in the packfile (later values more significant)
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

// Used for reading encoded offsets (for ofs delta objects) in the packfile (later values less significant)
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

func applyOfsDeltaPackfileObject(packfile []byte, i int, deltaObjStartPos int, packfileObjectLength int, repoDir string) (string, int, error) {
	// This offset is a negative relative offset from the ofs delta object's position in the packfile, indicating where the base object starts
	baseObjOffset, i, err := readVariableOffsetEncoding(packfile, i)
	if err != nil {
		return "", -1, err
	}

	deltaData, i, err := decompressPackfileObject(packfile, i, packfileObjectLength)
	if err != nil {
		return "", -1, err
	}

	baseObjPos := deltaObjStartPos - baseObjOffset
	if baseObjPos < 0 || baseObjPos >= len(packfile) {
		return "", -1, fmt.Errorf("invalid base object position indicated by ofs delta object: %d", baseObjPos)
	}

	packfileObjectType, packfileObjectLength, j, err := readPackfileObjectHeader(packfile, baseObjPos)
	if err != nil {
		return "", -1, err
	}

	// We could have a chain of delta objects, so we may need to recursively resolve them
	var targetObjType ObjectType
	var baseObjContent []byte
	if packfileObjectType == PACKFILE_OBJ_OFS_DELTA {
		baseObjHash, _, err := applyOfsDeltaPackfileObject(packfile, j, baseObjPos, packfileObjectLength, repoDir)
		if err != nil {
			return "", -1, err
		}

		targetObjType, _, baseObjContent, err = ReadObjectFile(baseObjHash, repoDir)
		if err != nil {
			return "", -1, fmt.Errorf("failed to read base object referenced by delta object: %s", err)
		}
	} else if packfileObjectType == PACKFILE_OBJ_REF_DELTA {
		return "", -1, fmt.Errorf("ofs_delta object referencing a ref_delta object as its base object is not supported")
	} else {
		targetObjType, err = ObjTypeFromString(packfileObjectType.toString())
		if err != nil {
			return "", -1, err
		}

		baseObjContent, _, err = decompressPackfileObject(packfile, j, packfileObjectLength)
		if err != nil {
			return "", -1, err
		}
	}

	targetObjContent, err := applyDelta(deltaData, baseObjContent)
	if err != nil {
		return "", -1, err
	}

	targetObjHash, err := CreateObjectFile(targetObjType, targetObjContent, repoDir)
	if err != nil {
		return "", -1, err
	}

	return targetObjHash, i, nil
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
		objType, _, baseObjContent, err := ReadObjectFile(refDeltaObj.baseObjHash, repoDir)
		if err != nil {
			return fmt.Errorf("failed to read base object referenced by delta object: %s", err)
		}

		targetObjContent, err := applyDelta(refDeltaObj.deltaData, baseObjContent)
		if err != nil {
			return err
		}

		_, err = CreateObjectFile(objType, targetObjContent, repoDir)
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

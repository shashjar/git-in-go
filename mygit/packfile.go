package main

import (
	"fmt"
)

const (
	PACKFILE_HEADER_LENGTH   = 12
	PACKFILE_SIGNATURE       = "PACK"
	PACKFILE_VERSION_NUMBER  = 2
	PACKFILE_CHECKSUM_LENGTH = 20
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

func (pot PackfileObjectType) toString() string {
	if pot == PACKFILE_OBJ_COMMIT {
		return "commit"
	} else if pot == PACKFILE_OBJ_TREE {
		return "tree"
	} else if pot == PACKFILE_OBJ_BLOB {
		return "blob"
	} else if pot == PACKFILE_OBJ_TAG {
		return "tag"
	} else if pot == PACKFILE_OBJ_OFS_DELTA {
		return "ofs_delta"
	} else if pot == PACKFILE_OBJ_REF_DELTA {
		return "ref_delta"
	} else {
		return "unknown"
	}
}

func packfileObjTypeFromString(packfileObjType string) (PackfileObjectType, error) {
	if packfileObjType == PACKFILE_OBJ_COMMIT.toString() {
		return PACKFILE_OBJ_COMMIT, nil
	} else if packfileObjType == PACKFILE_OBJ_TREE.toString() {
		return PACKFILE_OBJ_TREE, nil
	} else if packfileObjType == PACKFILE_OBJ_BLOB.toString() {
		return PACKFILE_OBJ_BLOB, nil
	} else if packfileObjType == PACKFILE_OBJ_TAG.toString() {
		return PACKFILE_OBJ_TAG, nil
	} else if packfileObjType == PACKFILE_OBJ_OFS_DELTA.toString() {
		return PACKFILE_OBJ_OFS_DELTA, nil
	} else if packfileObjType == PACKFILE_OBJ_REF_DELTA.toString() {
		return PACKFILE_OBJ_REF_DELTA, nil
	} else {
		return -1, fmt.Errorf("unknown packfile object type %s", packfileObjType)
	}
}

// Represents a packfile deltified object (ref delta), which is a delta of a base object (referenced by hash)
// using COPY and ADD instructions for sequences of data
type PackfileRefDeltaObject struct {
	baseObjHash string
	deltaData   []byte
}

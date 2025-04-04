package main

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
)

type RepositoryFileState int

const (
	Untracked         RepositoryFileState = iota // exists in working tree but not in index or HEAD. working tree: f, index: _, HEAD: _
	ModifiedNotStaged                            // exists in index but is different in working tree. working tree: f', index: f, HEAD: f
	DeletedNotStaged                             // exists in index but not in working tree. working tree: _, index: f, HEAD: f
	ModifiedStaged                               // modified in index compared to HEAD. working tree: f', index: f', HEAD: f
	AddedStaged                                  // new file added to index. working tree: f', index: f', HEAD: _
	DeletedStaged                                // deleted in index compared to HEAD. working tree: _, index: _, HEAD: f
	Unmodified                                   // same in working tree, index, and HEAD. working tree: f, index: f, HEAD: f
)

// Represents the status of an individual file in the repository
type RepositoryFileStatus struct {
	path   string
	status RepositoryFileState
}

// Represents the status of the entire repository
type RepositoryStatus struct {
	branch          string
	stagedFiles     []*RepositoryFileStatus
	notStagedFiles  []*RepositoryFileStatus
	untrackedFiles  []*RepositoryFileStatus
	unmodifiedFiles []*RepositoryFileStatus
}

func GetRepoStatus(repoDir string) (*RepositoryStatus, error) {
	stagedFiles := []*RepositoryFileStatus{}
	notStagedFiles := []*RepositoryFileStatus{}
	untrackedFiles := []*RepositoryFileStatus{}
	unmodifiedFiles := []*RepositoryFileStatus{}

	branch, err := getCurrentBranch(repoDir)
	if err != nil {
		return nil, err
	}

	workingTreePaths, err := getWorkingTreeFilePaths(repoDir)
	if err != nil {
		return nil, fmt.Errorf("error scanning repository for all files in working tree: %s", err)
	}

	workingTreePathsSet := make(map[string]bool, len(workingTreePaths))
	for _, path := range workingTreePaths {
		workingTreePathsSet[path] = true
	}

	currIndexEntries, err := ReadIndex(repoDir)
	if err != nil {
		return nil, err
	}

	currIndexEntriesMap := make(map[string]*IndexEntry, len(currIndexEntries))
	for _, entry := range currIndexEntries {
		currIndexEntriesMap[entry.path] = entry
	}

	headCommitHash, commitsExist, err := ResolveRef("HEAD", repoDir)
	if err != nil {
		return nil, err
	}

	if !commitsExist {
		// If this is a new repository with no commits yet, return a status with just untracked files
		for path := range workingTreePathsSet {
			untrackedFiles = append(untrackedFiles, &RepositoryFileStatus{
				path:   path,
				status: Untracked,
			})
		}

		return &RepositoryStatus{
			branch:          branch,
			stagedFiles:     stagedFiles,
			notStagedFiles:  notStagedFiles,
			untrackedFiles:  untrackedFiles,
			unmodifiedFiles: unmodifiedFiles,
		}, nil
	}

	headCommitObj, err := ReadCommitObjectFile(headCommitHash, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read HEAD commit object file: %s", err)
	}

	headTreeObj, err := ReadTreeObjectFile(headCommitObj.treeHash, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tree object file for HEAD commit: %s", err)
	}

	// Create headTreeEntries with all files in the HEAD tree
	headTreeEntries := make(map[string]string) // path -> hash
	err = populateTreeEntriesMap(headTreeEntries, headTreeObj, "", repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to populate map with file entries in HEAD tree: %s", err)
	}

	for path := range workingTreePathsSet {
		indexEntry, inIndex := currIndexEntriesMap[path]
		headHash, inHead := headTreeEntries[path]

		// File exists in working tree but not index or HEAD, so Untracked
		if !inIndex && !inHead {
			untrackedFiles = append(untrackedFiles, &RepositoryFileStatus{
				path:   path,
				status: Untracked,
			})
			continue
		}

		if inIndex {
			indexHash := hex.EncodeToString(indexEntry.sha1[:])

			blobObj, err := CreateBlobObjectFromFile(path, repoDir)
			if err != nil {
				return nil, fmt.Errorf("failed to create blob object for %s", path)
			}
			workingTreeHash := blobObj.hash

			// File exists differently in working tree and index, so ModifiedNotStaged
			if workingTreeHash != indexHash {
				notStagedFiles = append(notStagedFiles, &RepositoryFileStatus{
					path:   path,
					status: ModifiedNotStaged,
				})
				continue
			}

			if inHead {
				// File exists differently in working tree/index and HEAD, so ModifiedStaged
				if workingTreeHash != headHash {
					stagedFiles = append(stagedFiles, &RepositoryFileStatus{
						path:   path,
						status: ModifiedStaged,
					})
					continue
				} else { // File is the same in working tree, index, and HEAD, so Unmodified
					unmodifiedFiles = append(unmodifiedFiles, &RepositoryFileStatus{
						path:   path,
						status: Unmodified,
					})
					continue
				}
			} else { // File is the same in working tree and index but doesn't exist in HEAD, so AddedStaged
				stagedFiles = append(stagedFiles, &RepositoryFileStatus{
					path:   path,
					status: AddedStaged,
				})
				continue
			}
		}
	}

	for path := range currIndexEntriesMap {
		// File exists in index but not working tree, so DeletedNotStaged
		if !workingTreePathsSet[path] {
			notStagedFiles = append(notStagedFiles, &RepositoryFileStatus{
				path:   path,
				status: DeletedNotStaged,
			})
		}
	}

	for path := range headTreeEntries {
		_, inIndex := currIndexEntriesMap[path]
		_, inWorkingTree := workingTreePathsSet[path]

		// File exists in HEAD but not index or working tree, so DeletedStaged
		if !inIndex && !inWorkingTree {
			stagedFiles = append(stagedFiles, &RepositoryFileStatus{
				path:   path,
				status: DeletedStaged,
			})
		}
	}

	return &RepositoryStatus{
		branch:          branch,
		stagedFiles:     stagedFiles,
		notStagedFiles:  notStagedFiles,
		untrackedFiles:  untrackedFiles,
		unmodifiedFiles: unmodifiedFiles,
	}, nil
}

func populateTreeEntriesMap(treeEntries map[string]string, treeObj *TreeObject, pathPrefix string, repoDir string) error {
	for _, entry := range treeObj.entries {
		path := filepath.Join(pathPrefix, entry.name)

		if entry.objType == Tree {
			subTree, err := ReadTreeObjectFile(entry.hash, repoDir)
			if err != nil {
				return err
			}

			err = populateTreeEntriesMap(treeEntries, subTree, path, repoDir)
			if err != nil {
				return err
			}
		} else {
			treeEntries[path] = entry.hash
		}
	}

	return nil
}

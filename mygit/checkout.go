package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func checkoutCommit(commitHash string, repoDir string) error {
	commitObj, err := readCommitObjectFile(commitHash, repoDir)
	if err != nil {
		return err
	}

	if err := clearWorkingDirectory(repoDir); err != nil {
		return err
	}

	return checkoutTree(commitObj.treeHash, repoDir, repoDir)
}

func checkoutTree(treeHash string, currDir string, repoDir string) error {
	if err := os.MkdirAll(currDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", currDir, err)
	}

	treeObj, err := readTreeObjectFile(treeHash, repoDir)
	if err != nil {
		return err
	}

	for _, entry := range treeObj.entries {
		entryPath := filepath.Join(currDir, entry.name)

		switch entry.objType {
		case Blob:
			if err := checkoutBlob(entry.hash, entryPath, entry.mode, repoDir); err != nil {
				return err
			}
		case Tree:
			if err := checkoutTree(entry.hash, entryPath, repoDir); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected object type %s in tree %s", entry.objType.toString(), treeHash)
		}
	}

	return nil
}

func checkoutBlob(blobHash string, filePath string, mode int, repoDir string) error {
	blobObj, err := readBlobObjectFile(blobHash, repoDir)
	if err != nil {
		return err
	}

	parentDir := filepath.Dir(filePath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", parentDir, err)
	}

	if err := os.WriteFile(filePath, blobObj.content, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	perm := os.FileMode(mode & 0777)
	if err := os.Chmod(filePath, perm); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", filePath, err)
	}

	return nil
}

func clearWorkingDirectory(repoDir string) error {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return fmt.Errorf("failed to read repository directory: %w", err)
	}

	// Remove all entries except .git directory
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}

		path := filepath.Join(repoDir, entry.Name())
		var removeErr error
		if entry.IsDir() {
			removeErr = os.RemoveAll(path)
		} else {
			removeErr = os.Remove(path)
		}

		if removeErr != nil {
			return fmt.Errorf("failed to remove %s: %w", path, removeErr)
		}
	}

	return nil
}

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func CheckoutCommit(commitHash string, repoDir string) error {
	commitObj, err := ReadCommitObjectFile(commitHash, repoDir)
	if err != nil {
		return err
	}

	if err := clearWorkingDirectory(repoDir); err != nil {
		return err
	}

	if err := checkoutTree(commitObj.treeHash, repoDir, repoDir); err != nil {
		return err
	}

	err = copyRunSh(repoDir)
	if err != nil {
		return fmt.Errorf("failed to copy mygit run.sh script into repository: %s", err)
	}

	if err := CreateIndexFromWorkingTree(repoDir); err != nil {
		return err
	}

	return nil
}

func checkoutTree(treeHash string, currDir string, repoDir string) error {
	if err := os.MkdirAll(currDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", currDir, err)
	}

	treeObj, err := ReadTreeObjectFile(treeHash, repoDir)
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
	blobObj, err := ReadBlobObjectFile(blobHash, repoDir)
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

	// If any executable bits are set, update the file permissions
	perm := os.FileMode(mode & 0777)
	if perm&0111 != 0 {
		if err := os.Chmod(filePath, perm); err != nil {
			return fmt.Errorf("failed to set permissions on %s: %w", filePath, err)
		}
	}

	return nil
}

func clearWorkingDirectory(repoDir string) error {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return fmt.Errorf("failed to read repository directory: %w", err)
	}

	// Remove all entries except hidden directories & files (such as .git/)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
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

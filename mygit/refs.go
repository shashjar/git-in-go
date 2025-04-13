package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ResolveHead(remote bool, repoDir string) (string, bool, error) {
	var headPath string
	if remote {
		headPath = filepath.Join(repoDir, ".git", "refs", "remotes", "origin", "HEAD")
	} else {
		headPath = filepath.Join(repoDir, ".git", "HEAD")
	}

	headContentBytes, err := os.ReadFile(headPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to read HEAD file %s: %s", headPath, err)
	}
	headContent := strings.TrimSpace(string(headContentBytes))

	// Check if HEAD is a symbolic reference
	if strings.HasPrefix(headContent, "ref: ") {
		refPath := strings.TrimPrefix(headContent, "ref: ")
		refFilePath := filepath.Join(repoDir, ".git", refPath)
		refContentBytes, err := os.ReadFile(refFilePath)
		if err != nil {
			// If the reference doesn't exist yet (e.g., in a new repo)
			if os.IsNotExist(err) {
				return "", false, nil
			}
			return "", false, fmt.Errorf("failed to read reference file %s: %s", refPath, err)
		}

		return strings.TrimSpace(string(refContentBytes)), true, nil
	} else { // HEAD points directly to a commit (detached HEAD state)
		return headContent, true, nil
	}
}

func UpdateHeadWithBranchRef(branchName string, remote bool, repoDir string) error {
	var headPath string
	if remote {
		headPath = filepath.Join(repoDir, ".git", "refs", "remotes", "origin", "HEAD")
	} else {
		headPath = filepath.Join(repoDir, ".git", "HEAD")
	}

	var branchRefContent string
	if remote {
		branchRefContent = fmt.Sprintf("ref: refs/remotes/origin/%s", branchName)
	} else {
		branchRefContent = fmt.Sprintf("ref: refs/heads/%s", branchName)
	}

	if err := os.WriteFile(headPath, []byte(branchRefContent), 0644); err != nil {
		return fmt.Errorf("failed to write to HEAD file %s: %s", headPath, err)
	}

	return nil
}

func ResolveBranchRef(branchName string, remote bool, repoDir string) (string, bool, error) {
	var branchRefPath string
	if remote {
		branchRefPath = filepath.Join(repoDir, ".git", "refs", "remotes", "origin", branchName)
	} else {
		branchRefPath = filepath.Join(repoDir, ".git", "refs", "heads", branchName)
	}

	branchRefContentBytes, err := os.ReadFile(branchRefPath)
	if err != nil {
		// If the reference doesn't exist yet (e.g., in a new repo)
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to read branch reference file %s: %s", branchRefPath, err)
	}

	return strings.TrimSpace(string(branchRefContentBytes)), true, nil
}

func UpdateCurrentBranchRef(commitHash string, remote bool, repoDir string) error {
	branchName, err := getCurrentBranch(repoDir)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %s", err)
	}

	return UpdateBranchRef(branchName, commitHash, remote, repoDir)
}

func UpdateBranchRef(branchName string, commitHash string, remote bool, repoDir string) error {
	var branchRefPath string
	if remote {
		branchRefPath = filepath.Join(repoDir, ".git", "refs", "remotes", "origin", branchName)
	} else {
		branchRefPath = filepath.Join(repoDir, ".git", "refs", "heads", branchName)
	}

	branchRefDir := filepath.Dir(branchRefPath)
	if err := os.MkdirAll(branchRefDir, 0755); err != nil {
		return fmt.Errorf("failed to create ref directory structure for branch %s: %s", branchName, err)
	}

	if err := os.WriteFile(branchRefPath, []byte(commitHash), 0644); err != nil {
		return fmt.Errorf("failed to write to branch reference file %s: %s", branchRefPath, err)
	}

	return nil
}

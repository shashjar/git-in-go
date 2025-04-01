package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveRef(refName string, repoDir string) (string, bool, error) {
	if refName == "HEAD" {
		headPath := filepath.Join(repoDir, ".git", "HEAD")
		headContent, err := os.ReadFile(headPath)
		if err != nil {
			return "", false, fmt.Errorf("failed to read HEAD file: %s", err)
		}

		headStr := strings.TrimSpace(string(headContent))

		// Check if HEAD is a symbolic reference
		if strings.HasPrefix(headStr, "ref: ") {
			refPath := strings.TrimPrefix(headStr, "ref: ")

			refFilePath := filepath.Join(repoDir, ".git", refPath)
			refContent, err := os.ReadFile(refFilePath)
			if err != nil {
				// If the reference doesn't exist yet (e.g., in a new repo)
				if os.IsNotExist(err) {
					return "", false, nil
				}
				return "", false, fmt.Errorf("failed to read reference file %s: %s", refPath, err)
			}

			return strings.TrimSpace(string(refContent)), true, nil
		} else { // HEAD points directly to a commit (detached HEAD state)
			return headStr, true, nil
		}
	}

	// Try as a branch name
	branchRefPath := filepath.Join(repoDir, ".git", "refs", "heads", refName)
	branchRefContent, err := os.ReadFile(branchRefPath)
	if err != nil {
		// If the reference doesn't exist yet (e.g., in a new repo)
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to read branch reference file %s: %s", branchRefPath, err)
	}

	return strings.TrimSpace(string(branchRefContent)), true, nil
}

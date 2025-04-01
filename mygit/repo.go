package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Supports two Git URL formats:
// (1) git://<host>[:<port>]/<path-to-git-repo>
// (2) http[s]://<host>[:<port>]/<path-to-git-repo>
func validateRepoURL(repoURL string) error {
	parts := strings.Split(repoURL, "//")
	if len(parts) != 2 || (parts[0] != "git:" && parts[0] != "http:" && parts[0] != "https:") {
		return fmt.Errorf("git URL must use git or http/https format")
	}

	parts = strings.SplitN(parts[1], "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("repo URL not well-formatted")
	}

	hostParts := strings.Split(parts[0], ":")
	if len(hostParts) != 1 && len(hostParts) != 2 {
		return fmt.Errorf("repo host/port not well-formatted")
	}
	if len(hostParts) == 2 {
		_, err := strconv.Atoi(hostParts[1])
		if err != nil {
			return fmt.Errorf("repo port is not an integer")
		}
	}

	return nil
}

func initRepo(repoDir string) (string, error) {
	for _, dir := range []string{".git", ".git/objects", ".git/refs", ".git/refs/heads", ".git/refs/remotes"} {
		if err := os.MkdirAll(repoDir+dir, 0755); err != nil {
			return "", fmt.Errorf("error creating directory: %s", err)
		}
	}

	headFileContents := []byte("ref: refs/heads/master\n")
	if err := os.WriteFile(repoDir+".git/HEAD", headFileContents, 0644); err != nil {
		return "", fmt.Errorf("error writing HEAD file: %s", err)
	}

	absPath, err := filepath.Abs(repoDir + ".git")
	if err != nil {
		return "", fmt.Errorf("error getting absolute path of Git repository: %s", err)
	}

	return absPath, nil
}

func getCurrentBranch(repoDir string) (string, error) {
	headPath := filepath.Join(repoDir, ".git", "HEAD")
	headData, err := os.ReadFile(headPath)
	if err != nil {
		return "", fmt.Errorf("failed to read HEAD file: %s", err)
	}

	headContent := string(headData)
	if strings.HasPrefix(headContent, "ref: refs/heads/") {
		return strings.TrimSpace(strings.TrimPrefix(headContent, "ref: refs/heads/")), nil
	}

	return "", fmt.Errorf("failed to get current branch: HEAD detached at %s", headContent[:7])
}

func getWorkingTreeFilePaths(repoDir string) ([]string, error) {
	var workingTreeFiles []string

	err := filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(repoDir, path)
		if err != nil {
			return err
		}

		if relPath == "." || d.IsDir() || strings.HasPrefix(relPath, ".git") {
			return nil
		}

		workingTreeFiles = append(workingTreeFiles, relPath)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return workingTreeFiles, nil
}

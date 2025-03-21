package main

import (
	"fmt"
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
	for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
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

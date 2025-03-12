package main

import (
	"fmt"
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

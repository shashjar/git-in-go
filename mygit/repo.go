package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
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

// TODO: implement
func cloneRepoIntoDir(repoURL string, dir string) {
	resp, err := http.Get(repoURL + "/info/refs?service=git-upload-pack")
	if err != nil {
		log.Fatalf("Failed to reach remote repository: %s\n", err)
	}
	if resp.StatusCode != 200 && resp.StatusCode != 304 {
		log.Fatalf("Received invalid status code when fetching refs from remote repository: %s\n", resp.Status)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response from remote repository: %s\n", err)
	}

	validFirstBytes := regexp.MustCompile(`^[0-9a-f]{4}#`).MatchString(string(body[:5]))
	if !validFirstBytes {
		log.Fatalf("Received invalid response when fetching refs from remote repository\n")
	}

	pktLines, err := readPktLines(bytes.NewReader(body))
	if err != nil {
		log.Fatalf("Failed to parse response when fetching refs from remote repository: %s\n", err)
	}

	if len(pktLines) == 0 || pktLines[0] != "# service=git-upload-pack" {
		log.Fatalf("Received invalid response when fetching refs from remote repository\n")
	}

	// err = os.Mkdir(dir, 0755)
	// if err != nil {
	// 	log.Fatalf("Failed to create repository directory: %s\n", err)
	// }
}

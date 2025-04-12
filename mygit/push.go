package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
)

// TODO: implement this
// TODO: need to update remote refs and local HEAD once done successfully pushing
func Push(localHead string, remoteHead string, repoURL string, repoDir string) error {
	username := os.Getenv("GIT_USERNAME")
	if username == "" {
		return fmt.Errorf("GIT_USERNAME environment variable not set")
	}

	password := os.Getenv("GIT_PASSWORD")
	if password == "" {
		return fmt.Errorf("GIT_PASSWORD environment variable not set")
	}

	missingObjHashes, err := calculateMissingObjects(localHead, remoteHead, repoDir)
	if err != nil {
		return fmt.Errorf("failed to calculate objects in local HEAD missing from remote HEAD: %s", err)
	}

	if len(missingObjHashes) == 0 {
		fmt.Println("Everything up-to-date")
		return nil
	}

	fmt.Printf("Updating remote HEAD %s to local HEAD %s\n", remoteHead, localHead)
	fmt.Printf("Found %d objects in local HEAD missing from remote HEAD\n", len(missingObjHashes))

	packfile, err := CreatePackfile(missingObjHashes, repoDir)
	if err != nil {
		return fmt.Errorf("failed to create packfile of objects to push: %s", err)
	}

	err = receivePackRequest(packfile, repoURL)
	if err != nil {
		return fmt.Errorf("failed to perform receive-pack request sending packfile to remote repository: %s", err)
	}

	return nil
}

func calculateMissingObjects(localHead string, remoteHead string, repoDir string) ([]string, error) {
	localObjHashes, err := GetBlobsInCommit(localHead, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get blobs in local HEAD: %s", err)
	}

	remoteObjHashes, err := GetBlobsInCommit(remoteHead, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get blobs in remote HEAD: %s", err)
	}

	remoteObjHashesSet := make(map[string]struct{}, len(remoteObjHashes))
	for _, obj := range remoteObjHashes {
		remoteObjHashesSet[obj] = struct{}{}
	}

	missingObjHashes := []string{}
	for _, obj := range localObjHashes {
		if _, exists := remoteObjHashesSet[obj]; !exists {
			missingObjHashes = append(missingObjHashes, obj)
		}
	}

	return missingObjHashes, nil
}

// TODO: implement
func receivePackRequest(packfile []byte, repoURL string) error {
	receivePackResp, err := http.Post(repoURL+"/git-receive-pack", "application/x-git-receive-pack-request", bytes.NewReader(packfile))
	if err != nil {
		return fmt.Errorf("git-receive-pack request failed: %s", err)
	}
	if receivePackResp.StatusCode != 200 {
		return fmt.Errorf("received invalid response status code %s for git-receive-pack request", receivePackResp.Status)
	}
	defer receivePackResp.Body.Close()

	receivePackBody, err := io.ReadAll(receivePackResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read git-receive-pack response: %s", err)
	}

	fmt.Printf("receive-pack response body: %s\n", string(receivePackBody))

	return nil
}

package main

import (
	"bytes"
	"fmt"
	"strings"
)

func Push(localHead string, remoteHead string, repoURL string, repoDir string) error {
	missingObjHashes, err := calculateMissingObjects(localHead, remoteHead, repoDir)
	if err != nil {
		return fmt.Errorf("failed to calculate objects in local HEAD missing from remote HEAD: %s", err)
	}

	if len(missingObjHashes) == 0 {
		fmt.Println("Everything up-to-date")
		return nil
	}

	branchName, err := getCurrentBranch(repoDir)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %s", err)
	}

	fmt.Printf("Updating remote HEAD %s to local HEAD %s on branch %s\n", remoteHead, localHead, branchName)
	fmt.Printf("Found %d objects in local HEAD missing from remote HEAD\n", len(missingObjHashes))

	packfile, err := CreatePackfile(missingObjHashes, repoDir)
	if err != nil {
		return fmt.Errorf("failed to create packfile of objects to push: %s", err)
	}

	err = receivePackRequest(branchName, localHead, remoteHead, packfile, repoURL)
	if err != nil {
		return fmt.Errorf("failed to perform receive-pack request sending packfile to remote repository: %s", err)
	}

	err = UpdateBranchRef(branchName, localHead, false, repoDir)
	if err != nil {
		return fmt.Errorf("failed to update local branch reference for %s: %s", branchName, err)
	}

	err = UpdateBranchRef(branchName, localHead, true, repoDir)
	if err != nil {
		return fmt.Errorf("failed to update remote branch reference for %s: %s", branchName, err)
	}

	return nil
}

func calculateMissingObjects(localHead string, remoteHead string, repoDir string) ([]string, error) {
	localObjHashes, err := GetAllObjectsInCommit(localHead, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get all objects in local HEAD: %s", err)
	}

	var remoteObjHashes []string
	if remoteHead == "" {
		remoteObjHashes = []string{}
	} else {
		remoteObjHashes, err = GetAllObjectsInCommit(remoteHead, repoDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get all objects in remote HEAD: %s", err)
		}
	}

	remoteObjHashesSet := make(map[string]struct{}, len(remoteObjHashes))
	for _, obj := range remoteObjHashes {
		remoteObjHashesSet[obj] = struct{}{}
	}

	missingObjHashesSet := make(map[string]struct{})
	for _, obj := range localObjHashes {
		if _, exists := remoteObjHashesSet[obj]; !exists {
			missingObjHashesSet[obj] = struct{}{}
		}
	}

	missingObjHashes := []string{}
	for obj := range missingObjHashesSet {
		missingObjHashes = append(missingObjHashes, obj)
	}

	return missingObjHashes, nil
}

func receivePackRequest(branchName string, localHead string, remoteHead string, packfile []byte, repoURL string) error {
	// When creating a new branch, old-value should be all zeros
	if remoteHead == "" {
		remoteHead = strings.Repeat("0", OBJECT_HASH_LENGTH_STRING)
	}

	// Format the ref update line according to the Git protocol
	// Format: <old-value> SP <new-value> SP <ref-name> NUL report-status
	refUpdateLine := fmt.Sprintf("%s %s refs/heads/%s\x00 report-status", remoteHead, localHead, branchName)
	refUpdate := createPktLineStream([]string{createPktLine(refUpdateLine)})

	var receivePackReqBody bytes.Buffer
	receivePackReqBody.WriteString(refUpdate)
	receivePackReqBody.Write(packfile)

	receivePackRespBody, err := makeHTTPRequest("POST", repoURL+"/git-receive-pack", receivePackReqBody, []int{200})
	if err != nil {
		return fmt.Errorf("git-receive-pack request failed: %s", err)
	}

	// Parse the pkt-line formatted response
	lines, err := readPktLines(bytes.NewReader(receivePackRespBody))
	if err != nil {
		return fmt.Errorf("failed to parse pkt-lines from response: %s", err)
	}

	if len(lines) < 2 {
		return fmt.Errorf("expected at least 2 lines in response, got %d", len(lines))
	}

	// The first line should be "unpack ok"
	if lines[0] != "unpack ok" {
		return fmt.Errorf("packfile unpack failed: %s", lines[0])
	}

	// The second line should be "ok refs/heads/<branch>"
	expectedOkMsg := fmt.Sprintf("ok refs/heads/%s", branchName)
	if lines[1] != expectedOkMsg {
		return fmt.Errorf("ref update failed: %s", lines[1])
	}

	return nil
}

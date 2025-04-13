package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"regexp"
)

func Pull(repoURL string, repoDir string) error {
	username := os.Getenv("GIT_USERNAME")
	if username == "" {
		return fmt.Errorf("GIT_USERNAME environment variable not set")
	}

	token := os.Getenv("GIT_TOKEN")
	if token == "" {
		return fmt.Errorf("GIT_TOKEN environment variable not set. Please create a personal access token at https://github.com/settings/tokens")
	}

	refsMap, err := refDiscovery(repoURL, username, token)
	if err != nil {
		log.Fatalf("Failed to perform reference discovery on the remote repository: %s\n", err)
	}

	branchName, err := getCurrentBranch(repoDir)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %s", err)
	}

	packfile, err := uploadPackRequest(repoURL, refsMap, []string{branchName}, username, token)
	if err != nil {
		return fmt.Errorf("failed to perform git-upload-pack request: %s", err)
	}

	headHash := refsMap[branchName]

	err = ReadPackfile(packfile, repoDir)
	if err != nil {
		return fmt.Errorf("failed to read packfile: %s", err)
	}

	err = CheckoutCommit(headHash, repoDir)
	if err != nil {
		return fmt.Errorf("failed to check out HEAD commit: %s", err)
	}

	err = copyRunSh(repoDir)
	if err != nil {
		return fmt.Errorf("failed to copy mygit run.sh script into repository: %s", err)
	}

	err = UpdateRef("HEAD", headHash, false, repoDir)
	if err != nil {
		return fmt.Errorf("failed to update local HEAD reference: %s", err)
	}

	err = UpdateRef(branchName, headHash, false, repoDir)
	if err != nil {
		return fmt.Errorf("failed to update local branch reference: %s", err)
	}

	err = UpdateRef("HEAD", headHash, true, repoDir)
	if err != nil {
		return fmt.Errorf("failed to update remote HEAD reference: %s", err)
	}

	err = UpdateRef(branchName, headHash, true, repoDir)
	if err != nil {
		return fmt.Errorf("failed to update remote branch reference: %s", err)
	}

	return nil
}

func refDiscovery(repoURL string, username string, token string) (map[string]string, error) {
	refDiscoveryRespBody, err := makeHTTPRequest("GET", repoURL+"/info/refs?service=git-upload-pack", username, token, bytes.Buffer{}, []int{200, 304})
	if err != nil {
		return nil, fmt.Errorf("ref discovery request failed: %s", err)
	}

	validFirstBytes := regexp.MustCompile(`^[0-9a-f]{4}#`).MatchString(string(refDiscoveryRespBody[:5]))
	if !validFirstBytes {
		return nil, fmt.Errorf("received invalid response when fetching refs from remote repository")
	}

	refsPktLines, err := readPktLines(bytes.NewReader(refDiscoveryRespBody))
	if err != nil {
		return nil, fmt.Errorf("failed to parse response when fetching refs from remote repository: %s", err)
	}

	if len(refsPktLines) == 0 || refsPktLines[0] != "# service=git-upload-pack" {
		return nil, fmt.Errorf("received invalid response when fetching refs from remote repository")
	}

	refsMap := make(map[string]string)
	for _, refPktLine := range refsPktLines {
		if len(refPktLine) > 45 && refPktLine[41:45] == "HEAD" {
			refsMap["HEAD"] = refPktLine[0:40]
		} else if len(refPktLine) > 52 && refPktLine[41:52] == "refs/heads/" {
			branchName := refPktLine[52:]
			refsMap[branchName] = refPktLine[0:40]
		}
	}

	for refName, refHash := range refsMap {
		if !isValidObjectHash(refHash) {
			return nil, fmt.Errorf("ref %s in remote repository contained invalid SHA hash: %s", refName, refHash)
		}
	}

	return refsMap, nil
}

func uploadPackRequest(repoURL string, refsMap map[string]string, wantRefs []string, username string, token string) ([]byte, error) {
	wantObjHashes := []string{}
	for _, wantRef := range wantRefs {
		if wantObjHash, exists := refsMap[wantRef]; exists {
			wantObjHashes = append(wantObjHashes, wantObjHash)
		} else {
			return nil, fmt.Errorf("ref %s not found in remote repository", wantRef)
		}
	}

	capabilities := "multi_ack ofs-delta thin-pack include-tag"
	uploadPackPktLines := []string{}
	for _, wantObjHash := range wantObjHashes {
		uploadPackPktLines = append(uploadPackPktLines, createPktLine(fmt.Sprintf("want %s %s", wantObjHash, capabilities)))
	}
	donePktLine := createPktLine("done")
	uploadPackRequestBody := createPktLineStream(uploadPackPktLines) + donePktLine

	var uploadPackReqBody bytes.Buffer
	uploadPackReqBody.WriteString(uploadPackRequestBody)
	uploadPackRespBody, err := makeHTTPRequest("POST", repoURL+"/git-upload-pack", username, token, uploadPackReqBody, []int{200})
	if err != nil {
		return nil, fmt.Errorf("git-upload-pack request failed: %s", err)
	}

	nakLine, err := readPktLine(bytes.NewReader(uploadPackRespBody))
	if err != nil || nakLine != "NAK" {
		return nil, fmt.Errorf("expected NAK in git-upload-pack response")
	}

	return uploadPackRespBody[8:], nil
}

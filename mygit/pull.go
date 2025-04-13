package main

import (
	"bytes"
	"fmt"
	"log"
	"regexp"
)

func Pull(repoURL string, repoDir string) error {
	refsMap, err := refDiscovery(repoURL)
	if err != nil {
		log.Fatalf("Failed to perform reference discovery on the remote repository: %s\n", err)
	}

	branchName, err := getCurrentBranch(repoDir)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %s", err)
	}

	packfile, err := uploadPackRequest(repoURL, refsMap)
	if err != nil {
		return fmt.Errorf("failed to perform git-upload-pack request: %s", err)
	}

	branchHeadHash, ok := refsMap[branchName]
	if !ok {
		log.Fatalf("No branch named %s found in remote repository", branchName)
	}

	err = ReadPackfile(packfile, repoDir)
	if err != nil {
		return fmt.Errorf("failed to read packfile: %s", err)
	}

	err = CheckoutCommit(branchHeadHash, repoDir)
	if err != nil {
		return fmt.Errorf("failed to check out HEAD commit: %s", err)
	}

	err = copyRunSh(repoDir)
	if err != nil {
		return fmt.Errorf("failed to copy mygit run.sh script into repository: %s", err)
	}

	err = updateRefsAfterPull(refsMap, repoDir)
	if err != nil {
		return err
	}

	return nil
}

func refDiscovery(repoURL string) (map[string]string, error) {
	refDiscoveryRespBody, err := makeHTTPRequest("GET", repoURL+"/info/refs?service=git-upload-pack", bytes.Buffer{}, []int{200, 304})
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

func uploadPackRequest(repoURL string, refsMap map[string]string) ([]byte, error) {
	wantObjHashes := []string{}
	for _, objHash := range refsMap {
		wantObjHashes = append(wantObjHashes, objHash)
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
	uploadPackRespBody, err := makeHTTPRequest("POST", repoURL+"/git-upload-pack", uploadPackReqBody, []int{200})
	if err != nil {
		return nil, fmt.Errorf("git-upload-pack request failed: %s", err)
	}

	nakLine, err := readPktLine(bytes.NewReader(uploadPackRespBody))
	if err != nil || nakLine != "NAK" {
		return nil, fmt.Errorf("expected NAK in git-upload-pack response")
	}

	return uploadPackRespBody[8:], nil
}

func updateRefsAfterPull(refsMap map[string]string, repoDir string) error {
	for branchName, refHash := range refsMap {
		if branchName == "HEAD" {
			continue
		}

		err := UpdateBranchRef(branchName, refHash, false, repoDir)
		if err != nil {
			return fmt.Errorf("failed to update local branch reference for %s: %s", branchName, err)
		}

		err = UpdateBranchRef(branchName, refHash, true, repoDir)
		if err != nil {
			return fmt.Errorf("failed to update remote branch reference for %s: %s", branchName, err)
		}
	}

	return nil
}

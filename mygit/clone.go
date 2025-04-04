package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func CloneRepo(repoURL string, repoDir string) {
	info, err := os.Stat(repoDir)
	if !os.IsNotExist(err) && info.IsDir() {
		log.Fatalf("Destination path '%s' already exists", repoDir)
	}

	err = os.MkdirAll(repoDir, 0755)
	if err != nil {
		log.Fatalf("Failed to create repository directory: %s\n", err)
	}

	fmt.Printf("Cloning into '%s'...\n", repoDir)

	_, err = initRepo(repoDir)
	if err != nil {
		log.Fatalf("Failed to initialize repository: %s\n", err)
	}

	refsPktLines, err := refDiscovery(repoURL)
	if err != nil {
		log.Fatalf("Failed to perform reference discovery on the remote repository: %s\n", err)
	}

	packfile, headHash, err := uploadPackRequest(repoURL, refsPktLines)
	if err != nil {
		log.Fatalf("Failed to perform git-upload-pack request: %s\n", err)
	}

	err = ReadPackfile(packfile, repoDir)
	if err != nil {
		log.Fatalf("Failed to read packfile: %s\n", err)
	}

	masterRefPath := filepath.Join(repoDir, ".git", "refs", "heads", "master")
	err = os.WriteFile(masterRefPath, []byte(headHash+"\n"), 0644)
	if err != nil {
		log.Fatalf("Failed to write master branch reference: %s\n", err)
	}

	err = CheckoutCommit(headHash, repoDir)
	if err != nil {
		log.Fatalf("Failed to check out HEAD commit: %s\n", err)
	}
}

func refDiscovery(repoURL string) ([]string, error) {
	refDiscoveryResp, err := http.Get(repoURL + "/info/refs?service=git-upload-pack")
	if err != nil {
		return []string{}, fmt.Errorf("failed to reach remote repository: %s", err)
	}
	if refDiscoveryResp.StatusCode != 200 && refDiscoveryResp.StatusCode != 304 {
		return []string{}, fmt.Errorf("received invalid status code %s when fetching refs from remote repository", refDiscoveryResp.Status)
	}
	defer refDiscoveryResp.Body.Close()

	refDiscoveryBody, err := io.ReadAll(refDiscoveryResp.Body)
	if err != nil {
		return []string{}, fmt.Errorf("failed to read response from remote repository: %s", err)
	}

	validFirstBytes := regexp.MustCompile(`^[0-9a-f]{4}#`).MatchString(string(refDiscoveryBody[:5]))
	if !validFirstBytes {
		return []string{}, fmt.Errorf("received invalid response when fetching refs from remote repository")
	}

	refsPktLines, err := readPktLines(bytes.NewReader(refDiscoveryBody))
	if err != nil {
		return []string{}, fmt.Errorf("failed to parse response when fetching refs from remote repository: %s", err)
	}

	if len(refsPktLines) == 0 || refsPktLines[0] != "# service=git-upload-pack" {
		return []string{}, fmt.Errorf("received invalid response when fetching refs from remote repository")
	}

	return refsPktLines, nil
}

func uploadPackRequest(repoURL string, refsPktLines []string) ([]byte, string, error) {
	var headHash string
	for _, refPktLine := range refsPktLines {
		if len(refPktLine) > 45 && refPktLine[41:45] == "HEAD" {
			headHash = refPktLine[0:40]
		}
	}
	if headHash == "" {
		return nil, "", fmt.Errorf("refs in remote repository do not contain SHA hash for HEAD")
	}
	if !isValidObjectHash(headHash) {
		return nil, "", fmt.Errorf("refs in remote repository contained invalid SHA hash for HEAD: %s", headHash)
	}

	capabilities := "multi_ack ofs-delta thin-pack include-tag"
	uploadPackPktLine := createPktLine(fmt.Sprintf("want %s %s", headHash, capabilities))
	donePktLine := createPktLine("done")
	uploadPackRequestBody := createPktLineStream([]string{uploadPackPktLine}) + donePktLine
	uploadPackResp, err := http.Post(repoURL+"/git-upload-pack", "application/x-git-upload-pack-request", strings.NewReader(uploadPackRequestBody))
	if err != nil {
		return nil, "", fmt.Errorf("git-upload-pack request failed: %s", err)
	}
	if uploadPackResp.StatusCode != 200 {
		return nil, "", fmt.Errorf("received invalid response status code %s for git-upload-pack request", uploadPackResp.Status)
	}
	defer uploadPackResp.Body.Close()

	uploadPackBody, err := io.ReadAll(uploadPackResp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read git-upload-pack response: %s", err)
	}

	nakLine, err := readPktLine(bytes.NewReader(uploadPackBody))
	if err != nil || nakLine != "NAK" {
		return nil, "", fmt.Errorf("expected NAK in git-upload-pack response")
	}

	return uploadPackBody[8:], headHash, nil
}

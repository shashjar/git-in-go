package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

// TODO: implement
func cloneRepoIntoDir(repoURL string, dir string) {
	refsPktLines, err := refDiscovery(repoURL)
	if err != nil {
		log.Fatalf("Failed to perform reference discovery on the remote repository: %s\n", err)
	}

	packfile, err := uploadPackRequest(repoURL, refsPktLines)
	if err != nil {
		log.Fatalf("Failed to perform git-upload-pack request: %s\n", err)
	}

	err = readPackfile(packfile)
	if err != nil {
		log.Fatalf("Failed to read packfile: %s\n", err)
	}

	// err = os.Mkdir(dir, 0755)
	// if err != nil {
	// 	log.Fatalf("Failed to create repository directory: %s\n", err)
	// }
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

func uploadPackRequest(repoURL string, refsPktLines []string) ([]byte, error) {
	var headHash string
	for _, refPktLine := range refsPktLines {
		if len(refPktLine) > 45 && refPktLine[41:45] == "HEAD" {
			headHash = refPktLine[0:40]
		}
	}
	if headHash == "" {
		return nil, fmt.Errorf("refs in remote repository do not contain SHA hash for HEAD")
	}

	capabilities := "multi_ack ofs-delta thin-pack include-tag" // TODO: could maybe add a progress bar for cloning
	uploadPackPktLine := createPktLine(fmt.Sprintf("want %s %s", headHash, capabilities))
	donePktLine := createPktLine("done")
	uploadPackRequestBody := createPktLineStream([]string{uploadPackPktLine}) + donePktLine
	uploadPackResp, err := http.Post(repoURL+"/git-upload-pack", "application/x-git-upload-pack-request", strings.NewReader(uploadPackRequestBody))
	if err != nil {
		return nil, fmt.Errorf("git-upload-pack request failed: %s", err)
	}
	if uploadPackResp.StatusCode != 200 {
		return nil, fmt.Errorf("received invalid response status code %s for git-upload-pack request", uploadPackResp.Status)
	}
	defer uploadPackResp.Body.Close()

	uploadPackBody, err := io.ReadAll(uploadPackResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read git-upload-pack response: %s", err)
	}

	nakLine, err := readPktLine(bytes.NewReader(uploadPackBody))
	if err != nil || nakLine != "NAK" {
		return nil, fmt.Errorf("expected NAK in git-upload-pack response")
	}

	return uploadPackBody[8:], nil
}
